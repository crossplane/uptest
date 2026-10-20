// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package templates

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/crossplane/uptest/v2/internal/config"
)

// TestUpdateScriptPreservesParameter runs the rendered update script with
// sh and a stub kubectl to make sure the update-parameter reaches kubectl
// unmodified.
func TestUpdateScriptPreservesParameter(t *testing.T) {
	param := `{"description":"x y","tags":{"a":"b"}}`
	out, err := Render(&config.TestCase{Timeout: time.Minute}, []config.Resource{{
		Name:            "example",
		KindGroup:       "s3.aws.upbound.io",
		Root:            true,
		UpdateParameter: param,
	}}, false)
	if err != nil {
		t.Fatal(err)
	}

	var test struct {
		Spec struct {
			Steps []struct {
				Try []struct {
					Script *struct {
						Content string `json:"content"`
					} `json:"script"`
				} `json:"try"`
			} `json:"steps"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal([]byte(out["01-update.yaml"]), &test); err != nil {
		t.Fatal(err)
	}
	script := test.Spec.Steps[0].Try[0].Script.Content

	dir := t.TempDir()
	stub := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nfor a in \"$@\"; do printf '<%s>\\n' \"$a\"; done\n"), 0o755); err != nil { //nolint:gosec // Test stub must be executable.
		t.Fatal(err)
	}

	cmd := exec.Command("sh", "-c", script) //nolint:gosec // Script is rendered from a template under test.
	cmd.Env = append(os.Environ(), "KUBECTL="+stub)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, got)
	}
	want := `<{"spec":{"forProvider":` + param + `}}>`
	if !strings.Contains(string(got), want) {
		t.Errorf("kubectl did not receive %s; output:\n%s", want, got)
	}
}
