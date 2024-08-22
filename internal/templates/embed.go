// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package templates

import _ "embed"

// inputFileTemplate is the template for the input file.
//
//go:embed 00-apply.yaml.tmpl
var inputFileTemplate string

// updateFileTemplate is the template for the update file.
//
//go:embed 01-update.yaml.tmpl
var updateFileTemplate string

// deleteFileTemplate is the template for the import file.
//
//go:embed 02-import.yaml.tmpl
var importFileTemplate string

// deleteFileTemplate is the template for the delete file.
//
//go:embed 03-delete.yaml.tmpl
var deleteFileTemplate string
