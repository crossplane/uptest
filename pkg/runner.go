package pkg

import (
	"log"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/uptest/internal"
	"github.com/crossplane/uptest/internal/config"
)

// RunTest runs the specified automated test
func RunTest(o *config.AutomatedTest) error {
	if !o.RenderOnly {
		defer func() {
			if err := os.RemoveAll(o.Directory); err != nil {
				log.Printf("Cannot clean the test directory: %s\n", err.Error())
			}
		}()
	}

	// Read examples and inject data source values to manifests
	manifests, err := internal.NewPreparer(o.ManifestPaths, internal.WithDataSource(o.DataSourcePath), internal.WithTestDirectory(o.Directory)).PrepareManifests()
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := internal.NewTester(manifests, o).ExecuteTests(); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}

func NewAutomatedTestBuilder() *config.Builder {
	return config.NewBuilder()
}
