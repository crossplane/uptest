// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

// Package internal implements the uptest runtime for running
// automated tests using resource example manifests
// using chainsaw.
package internal

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/kaessert/crossplane-update-tester/sidecar"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/uptest/v2/internal/config"
)

var (
	charset = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	dataSourceRegex = regexp.MustCompile(`\${data\.(.*?)}`)
	randomStrRegex  = regexp.MustCompile(`\${Rand\.(.*?)}`)

	caseDirectory = "case"
)

type injectedManifest struct {
	Path     string
	Manifest string
	// Sidecar holds the injected text of the manifest's <Path>.uptest
	// sidecar when one exists, and HasSidecar distinguishes "no sidecar"
	// from "an empty one" — the former is the ordinary un-migrated state
	// every example starts in, and it must behave exactly as it did before
	// sidecars existed.
	Sidecar    string
	HasSidecar bool
}

// PreparerOption is a functional option type for configuring a Preparer.
type PreparerOption func(*Preparer)

// WithDataSource is a functional option that sets the data source path for the Preparer.
func WithDataSource(path string) PreparerOption {
	return func(p *Preparer) {
		p.dataSourcePath = path
	}
}

// WithTestDirectory is a functional option that sets the test directory for the Preparer.
func WithTestDirectory(path string) PreparerOption {
	return func(p *Preparer) {
		p.testDirectory = path
	}
}

// NewPreparer creates a new Preparer instance with the provided test file paths and optional configurations.
// It applies any provided PreparerOption functions to customize the Preparer.
func NewPreparer(testFilePaths []string, opts ...PreparerOption) *Preparer {
	p := &Preparer{
		testFilePaths: testFilePaths,
		testDirectory: os.TempDir(), // Default test directory is the system's temporary directory.
	}
	// Apply each provided option to configure the Preparer.
	for _, f := range opts {
		f(p)
	}
	return p
}

// Preparer represents a structure used to prepare testing environments or configurations.
type Preparer struct {
	testFilePaths  []string // Paths to the test files.
	dataSourcePath string   // Path to the data source file.
	testDirectory  string   // Directory where tests will be executed.
}

// PrepareManifests prepares and processes manifests from test files.
// It performs the following steps:
// 1. Cleans and recreates the case directory.
// 2. Injects variables into test files.
// 3. Decodes, processes, and validates each manifest file, skipping any that require manual intervention.
// 4. Returns the processed manifests or an error if any step fails.
//
//nolint:gocyclo // This function is not complex, gocyclo threshold was reached due to the error handling.
func (p *Preparer) PrepareManifests() ([]config.Manifest, error) {
	caseDirectory := filepath.Join(p.testDirectory, caseDirectory)
	if err := os.RemoveAll(caseDirectory); err != nil {
		return nil, errors.Wrapf(err, "cannot clean directory %s", caseDirectory)
	}
	if err := os.MkdirAll(caseDirectory, os.ModePerm); err != nil { //nolint:gosec // directory permissions are not critical here
		return nil, errors.Wrapf(err, "cannot create directory %s", caseDirectory)
	}

	injectedFiles, err := p.injectVariables()
	if err != nil {
		return nil, errors.Wrap(err, "cannot inject variables")
	}

	manifests := make([]config.Manifest, 0, len(injectedFiles))
	sidecarsLoaded := 0
	for _, data := range injectedFiles {
		docs, err := decodeDocuments(data.Manifest)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot decode manifest %s", data.Path)
		}
		if data.HasSidecar {
			if err := applySidecar(data.Path, data.Sidecar, docs); err != nil {
				return nil, err
			}
			sidecarsLoaded++
		}
		for _, u := range docs {
			if v, ok := u.GetAnnotations()["upjet.upbound.io/manual-intervention"]; ok {
				log.Printf("Skipping %s with name %s since it requires the following manual intervention: %s\n", u.GroupVersionKind().String(), u.GetName(), v)
				continue
			}
			y, err := yaml.Marshal(u)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot marshal manifest for \"%s/%s\"", u.GetObjectKind(), u.GetName())
			}
			manifests = append(manifests, config.Manifest{
				FilePath: data.Path,
				Object:   u,
				YAML:     string(y),
			})
		}
	}
	// Visible in ordinary E2E output so an operator can tell a sidecar-aware
	// run from a blind one — an old binary on a migrated tree exits 0 having
	// read zero annotations, and this line is what makes that silent failure
	// mode detectable from the log alone.
	log.Printf("Loaded %d manifest sidecar(s)\n", sidecarsLoaded)
	return manifests, nil
}

// decodeDocuments decodes every YAML document of one manifest file's
// (already variable-injected) text into unstructured objects.
//
// Decoding the whole file before anything is applied to it is what lets a
// sidecar be resolved against the file as a WHOLE: an ambiguous selector, an
// unmatched selector and two sidecar documents claiming one object are
// properties of the pairing between a sidecar and its manifest's full
// document set, and a loop that handles one document at a time cannot see
// any of them.
func decodeDocuments(data string) ([]*unstructured.Unstructured, error) {
	decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(data), 1024)
	var docs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				return docs, nil
			}
			return nil, errors.Wrap(err, "cannot decode manifest")
		}
		if u == nil || len(u.Object) == 0 {
			continue
		}
		docs = append(docs, u)
	}
}

// ownsAnnotation reports whether key is one that uptest itself reads.
// uptest polices only the keys it owns — a key live in both a manifest and
// its sidecar has no defensible precedence — and every OTHER consumer of
// example manifests (e.g. the update-tester tool) owns its own keys the
// same way, so there is no shared closed set to drift between the two.
func ownsAnnotation(key string) bool {
	return strings.HasPrefix(key, "uptest.upbound.io/") || key == config.AnnotationKeyExampleID
}

// applySidecar merges a manifest's sidecar onto its already-decoded
// documents, in place, before anything downstream consumes them.
//
// The annotations are set on the objects themselves rather than held in a
// side channel, so the rendered chainsaw case, the applied object and every
// assertion template see exactly what they would have seen had the
// annotations been written inline. The sidecar changes only where a file's
// author writes its harness configuration, never what uptest does with it.
func applySidecar(path, sidecarText string, docs []*unstructured.Unstructured) error {
	sc, err := sidecar.Parse([]byte(sidecarText))
	if err != nil {
		return errors.Wrapf(err, "cannot parse %s", sidecar.PathFor(path))
	}

	// Switch, not overlay: once a sidecar exists for this file, any document
	// in it still carrying one of uptest's own annotation keys inline is a
	// hard error rather than a silently-ignored duplicate. Checked across
	// EVERY document, not only the ones the sidecar targets — a stray
	// timeout left behind on a prerequisite Secret is exactly the residue a
	// migration leaves, and it is otherwise silent.
	for _, u := range docs {
		for key := range u.GetAnnotations() {
			if ownsAnnotation(key) {
				return errors.Errorf(
					"%s: %s %q carries annotation %q inline, but a sidecar exists at %s — "+
						"a sidecar REPLACES a manifest's harness annotations, it does not overlay them",
					path, u.GroupVersionKind().Kind, u.GetName(), key, sidecar.PathFor(path))
			}
		}
	}

	targets := make([]sidecar.ObjectID, len(docs))
	for i, u := range docs {
		apiVersion, kind := u.GroupVersionKind().ToAPIVersionAndKind()
		targets[i] = sidecar.ObjectID{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       u.GetName(),
			Namespace:  u.GetNamespace(),
		}
	}

	resolved, err := sidecar.Resolve(sc, targets)
	if err != nil {
		return errors.Wrapf(err, "cannot resolve %s", sidecar.PathFor(path))
	}
	for idx, anns := range resolved {
		merged := docs[idx].GetAnnotations()
		if merged == nil {
			merged = map[string]string{}
		}
		for k, v := range anns {
			merged[k] = v
		}
		docs[idx].SetAnnotations(merged)
	}
	return nil
}

func (p *Preparer) injectVariables() ([]injectedManifest, error) {
	dataSourceMap := make(map[string]string)
	if p.dataSourcePath != "" {
		dataSource, err := os.ReadFile(p.dataSourcePath)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read data source file")
		}
		if err := yaml.Unmarshal(dataSource, &dataSourceMap); err != nil {
			return nil, errors.Wrap(err, "cannot prepare data source map")
		}
	}

	inputs := make([]injectedManifest, len(p.testFilePaths))
	for i, f := range p.testFilePaths {
		manifestData, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read %s", f)
		}
		inputs[i] = injectedManifest{
			Path:     f,
			Manifest: p.injectValues(string(manifestData), dataSourceMap),
		}

		// A sidecar carries the same ${data.*} placeholders a manifest may,
		// so it goes through the same substitution step. It deliberately does
		// NOT go through the ${Rand.*} step: injectValues generates a fresh
		// random value at every occurrence, so a sidecar's copy of a random
		// placeholder could never equal the manifest's, and a name:/
		// namespace: selector built from one would silently never match.
		// Leaving ${Rand.*} untouched means the sidecar parser's own
		// templating-placeholder check rejects it outright instead.
		sidecarData, err := os.ReadFile(filepath.Clean(sidecar.PathFor(f)))
		switch {
		case err == nil:
			inputs[i].HasSidecar = true
			inputs[i].Sidecar = p.injectDataSource(string(sidecarData), dataSourceMap)
		case os.IsNotExist(err):
			// No sidecar: the un-migrated state every provider is in until it
			// migrates. Nothing changes for it.
		default:
			return nil, errors.Wrapf(err, "cannot read %s", sidecar.PathFor(f))
		}
	}
	return inputs, nil
}

func (p *Preparer) injectValues(manifestData string, dataSourceMap map[string]string) string {
	manifestData = p.injectDataSource(manifestData, dataSourceMap)
	return p.injectRandom(manifestData)
}

// injectDataSource substitutes ${data.*} placeholders such as tenantID,
// objectID or accountID. Split out from injectValues so a sidecar's text can
// go through the identical substitution without also going through
// injectRandom — see the comment at its one non-manifest call site.
func (p *Preparer) injectDataSource(manifestData string, dataSourceMap map[string]string) string {
	dataSourceKeys := dataSourceRegex.FindAllStringSubmatch(manifestData, -1)
	for _, dataSourceKey := range dataSourceKeys {
		if v, ok := dataSourceMap[dataSourceKey[1]]; ok {
			manifestData = strings.ReplaceAll(manifestData, dataSourceKey[0], v)
		}
	}
	return manifestData
}

// injectRandom substitutes ${Rand.*} placeholders with a freshly generated
// value at every occurrence.
func (p *Preparer) injectRandom(manifestData string) string {
	randomKeys := randomStrRegex.FindAllStringSubmatch(manifestData, -1)
	for _, randomKey := range randomKeys {
		switch randomKey[1] {
		case "RFC1123Subdomain":
			r := generateRFC1123SubdomainCompatibleString()
			manifestData = strings.Replace(manifestData, randomKey[0], r, 1)
		default:
			continue
		}
	}
	return manifestData
}

func generateRFC1123SubdomainCompatibleString() string {
	s := make([]rune, 8)
	for i := range s {
		s[i] = charset[rand.Intn(len(charset))] //nolint:gosec // no need for crypto/rand here
	}
	return fmt.Sprintf("op-%s", string(s))
}
