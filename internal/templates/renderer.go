// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

// Package templates contains utilities for rendering chainsaw test cases using
// the templates contained in the package.
package templates

import (
	"strings"
	"text/template"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/uptest/v2/internal/config"
)

// File names of the rendered chainsaw test case steps, shared with callers
// that need to know a step's file name without rendering it (e.g. to build
// the ordered list of files a test case is expected to produce).
const (
	ApplyFilename  = "00-apply.yaml"
	UpdateFilename = "01-update.yaml"
	ImportFilename = "02-import.yaml"
	DeleteFilename = "03-delete.yaml"
)

var fileTemplates = map[string]string{
	ApplyFilename:  inputFileTemplate,
	UpdateFilename: updateFileTemplate,
	ImportFilename: importFileTemplate,
	DeleteFilename: deleteFileTemplate,
}

// Render renders the specified list of resources as a test case
// with the specified configuration.
func Render(tc *config.TestCase, resources []config.Resource, skipDelete bool) (map[string]string, error) {
	data := struct {
		Resources []config.Resource
		TestCase  config.TestCase
	}{
		Resources: resources,
		TestCase:  *tc,
	}

	res := make(map[string]string, len(fileTemplates))
	for name, tmpl := range fileTemplates {
		// Skip templates with names starting with "01-" if skipUpdate is true
		if tc.SkipUpdate && strings.HasPrefix(name, "01-") {
			continue
		}
		// Skip templates with names starting with "02-" if skipImport is true
		if tc.SkipImport && strings.HasPrefix(name, "02-") {
			continue
		}
		// Skip templates with names starting with "03-" if skipDelete is true
		if skipDelete && strings.HasPrefix(name, "03-") {
			continue
		}

		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse template %q", name)
		}

		var b strings.Builder
		if err := t.Execute(&b, data); err != nil {
			return nil, errors.Wrapf(err, "cannot execute template %q", name)
		}
		res[name] = b.String()
	}

	return res, nil
}
