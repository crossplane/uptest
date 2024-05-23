// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package internal

import (
	"fmt"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/uptest/internal/config"
)

// RunTest runs the specified automated test
func RunTest(o *config.AutomatedTest) error {
	if !o.RenderOnly {
		defer func() {
			if err := os.RemoveAll(o.Directory); err != nil {
				fmt.Println(fmt.Sprint(err, "cannot clean the test directory"))
			}
		}()
	}

	// Read examples and inject data source values to manifests
	manifests, err := newPreparer(o.ManifestPaths, withDataSource(o.DataSourcePath), withTestDirectory(o.Directory)).prepareManifests()
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := newTester(manifests, o).executeTests(); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}
