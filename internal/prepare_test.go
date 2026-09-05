// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/uptest/v2/internal/config"
)

const (
	widgetManifest = `apiVersion: widget.example.org/v1alpha1
kind: Widget
metadata:
  name: test-widget
spec:
  forProvider: {}
`

	secretManifest = `apiVersion: v1
kind: Secret
metadata:
  name: prerequisite-secret
  namespace: crossplane-system
type: Opaque
stringData:
  key: value
`
)

func TestDecodeDocuments(t *testing.T) {
	cases := map[string]struct {
		reason  string
		data    string
		wantLen int
		wantErr bool
	}{
		"SingleDocument": {
			reason:  "A single-document manifest decodes to exactly one object.",
			data:    widgetManifest,
			wantLen: 1,
		},
		"MultiDocument": {
			reason:  "A multi-document manifest decodes every document, in file order.",
			data:    secretManifest + "---\n" + widgetManifest,
			wantLen: 2,
		},
		"BlankDocument": {
			reason:  "A trailing '---' with nothing after it produces no extra document.",
			data:    widgetManifest + "---\n",
			wantLen: 1,
		},
		"Empty": {
			reason:  "An empty file decodes to zero documents and no error.",
			data:    "",
			wantLen: 0,
		},
		"Malformed": {
			reason:  "Malformed YAML is a decode error, not a silently skipped document.",
			data:    "not: valid: yaml: at: all: [",
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			docs, err := decodeDocuments(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("\n%s\ndecodeDocuments(...): expected an error, got none", tc.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("\n%s\ndecodeDocuments(...): unexpected error: %v", tc.reason, err)
			}
			if len(docs) != tc.wantLen {
				t.Errorf("\n%s\ndecodeDocuments(...): -want %d documents, +got %d", tc.reason, tc.wantLen, len(docs))
			}
		})
	}
}

func TestOwnsAnnotation(t *testing.T) {
	cases := map[string]struct {
		reason string
		key    string
		want   bool
	}{
		"Timeout": {
			reason: "uptest.upbound.io/timeout is an uptest key.",
			key:    config.AnnotationKeyTimeout,
			want:   true,
		},
		"PostAssertHook": {
			reason: "uptest.upbound.io/post-assert-hook is an uptest key.",
			key:    config.AnnotationKeyPostAssertHook,
			want:   true,
		},
		"ExampleID": {
			reason: "meta.upbound.io/example-id is an uptest key even though it does not share the uptest.upbound.io prefix.",
			key:    config.AnnotationKeyExampleID,
			want:   true,
		},
		"UpdateTest": {
			reason: "crossplane.io/update-test belongs to update-tester, not uptest. uptest polices only the keys it reads.",
			key:    "crossplane.io/update-test",
			want:   false,
		},
		"ExternalName": {
			reason: "crossplane.io/external-name is a real Crossplane annotation, never uptest's to police.",
			key:    "crossplane.io/external-name",
			want:   false,
		},
		"UnrelatedPrefix": {
			reason: "An unrelated annotation is never owned.",
			key:    "example.org/some-key",
			want:   false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ownsAnnotation(tc.key)
			if got != tc.want {
				t.Errorf("\n%s\nownsAnnotation(%q): -want %v, +got %v", tc.reason, tc.key, tc.want, got)
			}
		})
	}
}

func TestApplySidecar(t *testing.T) {
	cases := map[string]struct {
		reason      string
		path        string
		sidecarText string
		docs        []*unstructured.Unstructured
		wantErr     string
		wantAnns    map[string]map[string]string // object name -> expected annotations
	}{
		"MergesOntoSelectedTarget": {
			reason: "A sidecar targeting the sole matching object merges its annotations onto it.",
			path:   "widget.yaml",
			sidecarText: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "1200"
`,
			docs: mustDecode(t, widgetManifest),
			wantAnns: map[string]map[string]string{
				"test-widget": {"uptest.upbound.io/timeout": "1200"},
			},
		},
		"SelectsNonFirstDocument": {
			reason: "Selection targets the object the sidecar names, not document order — the Secret is first in the file but the sidecar targets the Widget.",
			path:   "bundle.yaml",
			sidecarText: `for: widget.example.org/v1alpha1/Widget
meta.upbound.io/example-id: widget/v1alpha1/widget
`,
			docs: mustDecode(t, secretManifest+"---\n"+widgetManifest),
			wantAnns: map[string]map[string]string{
				"prerequisite-secret": nil,
				"test-widget":         {"meta.upbound.io/example-id": "widget/v1alpha1/widget"},
			},
		},
		"ConflictInlineWithSidecar": {
			reason: "A sidecar REPLACES a manifest's harness annotations. One still present inline is a hard error, not a silent overlay.",
			path:   "widget.yaml",
			sidecarText: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "1200"
`,
			docs: mustDecode(t, `apiVersion: widget.example.org/v1alpha1
kind: Widget
metadata:
  name: test-widget
  annotations:
    uptest.upbound.io/timeout: "600"
`),
			wantErr: "carries annotation",
		},
		"ConflictOnUnselectedDocument": {
			reason: "The switch check runs across every document in the file, not only the ones the sidecar targets — a stray annotation left on a prerequisite Secret must be caught too.",
			path:   "bundle.yaml",
			sidecarText: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "1200"
`,
			docs: mustDecode(t, `apiVersion: v1
kind: Secret
metadata:
  name: prerequisite-secret
  namespace: crossplane-system
  annotations:
    meta.upbound.io/example-id: leftover
---
`+widgetManifest),
			wantErr: "carries annotation",
		},
		"AmbiguousSelector": {
			reason: "A selector matching more than one object without a narrowing name:/namespace: is an error from the sidecar package itself, propagated unchanged.",
			path:   "bundle.yaml",
			sidecarText: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "1200"
`,
			docs: mustDecode(t, widgetManifest+`---
apiVersion: widget.example.org/v1alpha1
kind: Widget
metadata:
  name: other-widget
`),
			wantErr: "ambiguous",
		},
		"MissingForDirective": {
			reason: "A sidecar document with no for: is a parse error.",
			path:   "widget.yaml",
			sidecarText: `uptest.upbound.io/timeout: "1200"
`,
			docs:    mustDecode(t, widgetManifest),
			wantErr: "for:",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := applySidecar(tc.path, tc.sidecarText, tc.docs)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("\n%s\napplySidecar(...): expected an error containing %q, got none", tc.reason, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("\n%s\napplySidecar(...): error %q does not contain %q", tc.reason, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("\n%s\napplySidecar(...): unexpected error: %v", tc.reason, err)
			}
			for _, d := range tc.docs {
				want, ok := tc.wantAnns[d.GetName()]
				if !ok {
					continue
				}
				got := d.GetAnnotations()
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("\n%s\nannotations for %s: -want, +got:\n%s", tc.reason, d.GetName(), diff)
				}
			}
		})
	}
}

func mustDecode(t *testing.T, data string) []*unstructured.Unstructured {
	t.Helper()
	docs, err := decodeDocuments(data)
	if err != nil {
		t.Fatalf("mustDecode: %v", err)
	}
	return docs
}

// TestPrepareManifestsSidecar exercises the full PrepareManifests path
// end-to-end against real files on disk, covering both the un-migrated
// (no sidecar) case — which must behave exactly as it did before sidecars
// existed — and the migrated case.
func TestPrepareManifestsSidecar(t *testing.T) {
	cases := map[string]struct {
		reason      string
		manifest    string
		sidecar     string // "" means no sidecar file is written at all
		writeNoFile bool
		wantAnns    map[string]string
		wantErr     string
	}{
		"NoSidecarUnchanged": {
			reason:   "A manifest with no sidecar file behaves exactly as before sidecars existed: whatever annotations are inline are the ones that land, nothing more.",
			manifest: widgetManifest,
			wantAnns: nil,
		},
		"SidecarMerged": {
			reason:   "A manifest with a sidecar gets the sidecar's annotations merged onto the decoded object.",
			manifest: widgetManifest,
			sidecar: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "900"
meta.upbound.io/example-id: widget/v1alpha1/widget
`,
			wantAnns: map[string]string{
				"uptest.upbound.io/timeout":  "900",
				"meta.upbound.io/example-id": "widget/v1alpha1/widget",
			},
		},
		"SidecarDataSubstitution": {
			reason:   "${data.*} placeholders inside a sidecar are substituted through the same injectValues path as the manifest.",
			manifest: widgetManifest,
			sidecar: `for: widget.example.org/v1alpha1/Widget
uptest.upbound.io/timeout: "${data.timeoutSeconds}"
`,
			wantAnns: map[string]string{
				"uptest.upbound.io/timeout": "1800",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			manifestPath := filepath.Join(dir, "widget.yaml")
			if err := os.WriteFile(manifestPath, []byte(tc.manifest), 0o600); err != nil {
				t.Fatalf("WriteFile(manifest): %v", err)
			}
			if tc.sidecar != "" {
				if err := os.WriteFile(manifestPath+".uptest", []byte(tc.sidecar), 0o600); err != nil {
					t.Fatalf("WriteFile(sidecar): %v", err)
				}
			}

			opts := []PreparerOption{WithTestDirectory(t.TempDir())}
			if strings.Contains(tc.sidecar, "${data.") {
				dsPath := filepath.Join(dir, "datasource.yaml")
				if err := os.WriteFile(dsPath, []byte("timeoutSeconds: \"1800\"\n"), 0o600); err != nil {
					t.Fatalf("WriteFile(datasource): %v", err)
				}
				opts = append(opts, WithDataSource(dsPath))
			}

			p := NewPreparer([]string{manifestPath}, opts...)
			manifests, err := p.PrepareManifests()
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("\n%s\nPrepareManifests(): expected an error containing %q, got none", tc.reason, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("\n%s\nPrepareManifests(): error %q does not contain %q", tc.reason, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("\n%s\nPrepareManifests(): unexpected error: %v", tc.reason, err)
			}
			if len(manifests) != 1 {
				t.Fatalf("\n%s\nPrepareManifests(): -want 1 manifest, +got %d", tc.reason, len(manifests))
			}
			got := manifests[0].Object.GetAnnotations()
			if diff := cmp.Diff(tc.wantAnns, got); diff != "" {
				t.Errorf("\n%s\nannotations: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInjectDataSourceVsInjectRandom(t *testing.T) {
	p := &Preparer{}
	dataSourceMap := map[string]string{"tenantID": "t-1234"}

	t.Run("InjectDataSourceSubstitutesDataPlaceholders", func(t *testing.T) {
		got := p.injectDataSource("tenant: ${data.tenantID}", dataSourceMap)
		want := "tenant: t-1234"
		if got != want {
			t.Errorf("injectDataSource(...): -want %q, +got %q", want, got)
		}
	})

	t.Run("InjectDataSourceLeavesRandPlaceholdersUntouched", func(t *testing.T) {
		// This is the property AC 2 depends on: a sidecar's text must go
		// through ${data.*} substitution without also going through
		// ${Rand.*} substitution, so a ${Rand.*} placeholder used in a
		// name:/namespace: selector is rejected by the sidecar parser
		// instead of being silently replaced with a value that could never
		// equal the manifest's own random suffix.
		in := "name: widget-${Rand.RFC1123Subdomain}"
		got := p.injectDataSource(in, dataSourceMap)
		if got != in {
			t.Errorf("injectDataSource(...): expected ${Rand.*} left untouched, -want %q +got %q", in, got)
		}
	})

	t.Run("InjectValuesSubstitutesBoth", func(t *testing.T) {
		got := p.injectValues("tenant: ${data.tenantID}", dataSourceMap)
		if strings.Contains(got, "${data.") {
			t.Errorf("injectValues(...): ${data.*} placeholder was not substituted: %q", got)
		}
	})
}
