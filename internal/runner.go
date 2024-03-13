// Copyright 2023 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"fmt"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/uptest/internal/config"
)

// RunTest runs the specified automated test
func RunTest(o *config.AutomatedTest) error {
	defer func() {
		if err := os.RemoveAll(o.Directory); err != nil {
			fmt.Println(fmt.Sprint(err, "cannot clean the test directory"))
		}
	}()

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
