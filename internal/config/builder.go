// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

// Package config contains configuration options for configuring uptest runtime.
package config

import (
	"time"
)

// Builder is a struct that helps construct an AutomatedTest instance step-by-step.
type Builder struct {
	test AutomatedTest
}

// NewBuilder initializes and returns a new Builder instance.
func NewBuilder() *Builder {
	return &Builder{
		test: AutomatedTest{},
	}
}

// SetDirectory sets the directory path for the AutomatedTest and returns the Builder.
func (b *Builder) SetDirectory(directory string) *Builder {
	b.test.Directory = directory
	return b
}

// SetManifestPaths sets the paths of the manifest files for the AutomatedTest and returns the Builder.
func (b *Builder) SetManifestPaths(manifestPaths []string) *Builder {
	b.test.ManifestPaths = manifestPaths
	return b
}

// SetDataSourcePath sets the data source path for the AutomatedTest and returns the Builder.
func (b *Builder) SetDataSourcePath(dataSourcePath string) *Builder {
	b.test.DataSourcePath = dataSourcePath
	return b
}

// SetSetupScriptPath sets the setup script path for the AutomatedTest and returns the Builder.
func (b *Builder) SetSetupScriptPath(setupScriptPath string) *Builder {
	b.test.SetupScriptPath = setupScriptPath
	return b
}

// SetTeardownScriptPath sets the teardown script path for the AutomatedTest and returns the Builder.
func (b *Builder) SetTeardownScriptPath(teardownScriptPath string) *Builder {
	b.test.TeardownScriptPath = teardownScriptPath
	return b
}

// SetDefaultTimeout sets the default timeout duration for the AutomatedTest and returns the Builder.
func (b *Builder) SetDefaultTimeout(defaultTimeout time.Duration) *Builder {
	b.test.DefaultTimeout = defaultTimeout
	return b
}

// SetDefaultConditions sets the default conditions for the AutomatedTest and returns the Builder.
func (b *Builder) SetDefaultConditions(defaultConditions []string) *Builder {
	b.test.DefaultConditions = defaultConditions
	return b
}

// SetSkipDelete sets whether the AutomatedTest should skip resource deletion and returns the Builder.
func (b *Builder) SetSkipDelete(skipDelete bool) *Builder {
	b.test.SkipDelete = skipDelete
	return b
}

// SetSkipUpdate sets whether the AutomatedTest should skip resource updates and returns the Builder.
func (b *Builder) SetSkipUpdate(skipUpdate bool) *Builder {
	b.test.SkipUpdate = skipUpdate
	return b
}

// SetSkipImport sets whether the AutomatedTest should skip resource imports and returns the Builder.
func (b *Builder) SetSkipImport(skipImport bool) *Builder {
	b.test.SkipImport = skipImport
	return b
}

// SetOnlyCleanUptestResources sets whether the AutomatedTest should clean up only test-specific resources and returns the Builder.
func (b *Builder) SetOnlyCleanUptestResources(onlyCleanUptestResources bool) *Builder {
	b.test.OnlyCleanUptestResources = onlyCleanUptestResources
	return b
}

// SetRenderOnly sets whether the AutomatedTest should only render outputs without execution and returns the Builder.
func (b *Builder) SetRenderOnly(renderOnly bool) *Builder {
	b.test.RenderOnly = renderOnly
	return b
}

// SetLogCollectionInterval sets the interval for log collection during the AutomatedTest and returns the Builder.
func (b *Builder) SetLogCollectionInterval(logCollectionInterval time.Duration) *Builder {
	b.test.LogCollectionInterval = logCollectionInterval
	return b
}

// SetUseLibraryMode sets whether the AutomatedTest should use library mode instead of CLI fork mode and returns the Builder.
func (b *Builder) SetUseLibraryMode(useLibraryMode bool) *Builder {
	b.test.UseLibraryMode = useLibraryMode
	return b
}

// Build finalizes and returns the constructed AutomatedTest instance.
func (b *Builder) Build() *AutomatedTest {
	return &b.test
}
