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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/uptest/internal/config"
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
}

type preparerOption func(*preparer)

func withDataSource(path string) preparerOption {
	return func(p *preparer) {
		p.dataSourcePath = path
	}
}

func withTestDirectory(path string) preparerOption {
	return func(p *preparer) {
		p.testDirectory = path
	}
}

func newPreparer(testFilePaths []string, opts ...preparerOption) *preparer {
	p := &preparer{
		testFilePaths: testFilePaths,
		testDirectory: os.TempDir(),
	}
	for _, f := range opts {
		f(p)
	}
	return p
}

type preparer struct {
	testFilePaths  []string
	dataSourcePath string
	testDirectory  string
}

//nolint:gocyclo // This function is not complex, gocyclo threshold was reached due to the error handling.
func (p *preparer) prepareManifests() ([]config.Manifest, error) {
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
	for _, data := range injectedFiles {
		decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(data.Manifest), 1024)
		for {
			u := &unstructured.Unstructured{}
			if err := decoder.Decode(&u); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, errors.Wrap(err, "cannot decode manifest")
			}
			if u != nil {
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
	}
	return manifests, nil
}

func (p *preparer) injectVariables() ([]injectedManifest, error) {
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
	}
	return inputs, nil
}

func (p *preparer) injectValues(manifestData string, dataSourceMap map[string]string) string {
	// Inject data source values such as tenantID, objectID, accountID
	dataSourceKeys := dataSourceRegex.FindAllStringSubmatch(manifestData, -1)
	for _, dataSourceKey := range dataSourceKeys {
		if v, ok := dataSourceMap[dataSourceKey[1]]; ok {
			manifestData = strings.ReplaceAll(manifestData, dataSourceKey[0], v)
		}
	}
	// Inject random strings
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
