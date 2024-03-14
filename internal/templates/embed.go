// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package templates

import _ "embed"

// inputFileTemplate is the template for the input file.
//
//go:embed 00-apply.yaml.tmpl
var inputFileTemplate string

// assertFileTemplate is the template for the assert file.
//
//go:embed 00-assert.yaml.tmpl
var assertFileTemplate string

// updateFileTemplate is the template for the update file.
//
//go:embed 01-update.yaml.tmpl
var updateFileTemplate string

// assertUpdatedFileTemplate is the template for update assert file.
//
//go:embed 01-assert.yaml.tmpl
var assertUpdatedFileTemplate string

// deleteFileTemplate is the template for the import file.
//
//go:embed 02-import.yaml.tmpl
var importFileTemplate string

// assertDeletedFileTemplate is the template for import assert file.
//
//go:embed 02-assert.yaml.tmpl
var assertImportedFileTemplate string

// deleteFileTemplate is the template for the delete file.
//
//go:embed 03-delete.yaml.tmpl
var deleteFileTemplate string

// assertDeletedFileTemplate is the template for delete assert file.
//
//go:embed 03-assert.yaml.tmpl
var assertDeletedFileTemplate string
