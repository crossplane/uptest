package config

import (
	"time"
)

type Builder struct {
	test AutomatedTest
}

func NewBuilder() *Builder {
	return &Builder{
		test: AutomatedTest{},
	}
}

func (b *Builder) SetDirectory(directory string) *Builder {
	b.test.Directory = directory
	return b
}

func (b *Builder) SetManifestPaths(manifestPaths []string) *Builder {
	b.test.ManifestPaths = manifestPaths
	return b
}

func (b *Builder) SetDataSourcePath(dataSourcePath string) *Builder {
	b.test.DataSourcePath = dataSourcePath
	return b
}

func (b *Builder) SetSetupScriptPath(setupScriptPath string) *Builder {
	b.test.SetupScriptPath = setupScriptPath
	return b
}

func (b *Builder) SetTeardownScriptPath(teardownScriptPath string) *Builder {
	b.test.TeardownScriptPath = teardownScriptPath
	return b
}

func (b *Builder) SetDefaultTimeout(defaultTimeout time.Duration) *Builder {
	b.test.DefaultTimeout = defaultTimeout
	return b
}

func (b *Builder) SetDefaultConditions(defaultConditions []string) *Builder {
	b.test.DefaultConditions = defaultConditions
	return b
}

func (b *Builder) SetSkipDelete(skipDelete bool) *Builder {
	b.test.SkipDelete = skipDelete
	return b
}

func (b *Builder) SetSkipUpdate(skipUpdate bool) *Builder {
	b.test.SkipUpdate = skipUpdate
	return b
}

func (b *Builder) SetSkipImport(skipImport bool) *Builder {
	b.test.SkipImport = skipImport
	return b
}

func (b *Builder) SetOnlyCleanUptestResources(onlyCleanUptestResources bool) *Builder {
	b.test.OnlyCleanUptestResources = onlyCleanUptestResources
	return b
}

func (b *Builder) SetRenderOnly(renderOnly bool) *Builder {
	b.test.RenderOnly = renderOnly
	return b
}

func (b *Builder) SetLogCollectionInterval(logCollectionInterval time.Duration) *Builder {
	b.test.LogCollectionInterval = logCollectionInterval
	return b
}

func (b *Builder) Build() *AutomatedTest {
	return &b.test
}
