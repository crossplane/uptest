// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

// main package for the uptest tooling.
package main

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/crossplane/uptest/internal"
	"github.com/crossplane/uptest/internal/config"
)

var (
	app = kingpin.New("uptest", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()
	// e2e command (single command is preserved for backward compatibility)
	// and we may have further commands in the future.
	e2e = app.Command("e2e", "Run e2e tests for manifests by applying them to a control plane and waiting until a given condition is met.")
)

var (
	manifestList = e2e.Arg("manifest-list", "List of manifests. Value of this option will be used to trigger/configure the tests."+
		"The possible usage:\n"+
		"'provider-aws/examples/s3/bucket.yaml,provider-gcp/examples/storage/bucket.yaml': "+
		"The comma separated resources are used as test inputs.\n"+
		"If this option is not set, 'MANIFEST_LIST' env var is used as default.").Envar("MANIFEST_LIST").String()
	dataSourcePath = e2e.Flag("data-source", "File path of data source that will be used for injection some values.").Envar("UPTEST_DATASOURCE_PATH").Default("").String()
	setupScript    = e2e.Flag("setup-script", "Script that will be executed before running tests.").Default("").String()
	teardownScript = e2e.Flag("teardown-script", "Script that will be executed after running tests.").Default("").String()

	defaultTimeout = e2e.Flag("default-timeout", "Default timeout in seconds for the test.\n"+
		"Timeout could be overridden per resource using \"uptest.upbound.io/timeout\" annotation.").Default("1200s").Duration()
	defaultConditions = e2e.Flag("default-conditions", "Comma separated list of default conditions to wait for a successful test.\n"+
		"Conditions could be overridden per resource using \"uptest.upbound.io/conditions\" annotation.").Default("Ready").String()

	skipDelete               = e2e.Flag("skip-delete", "Skip the delete step of the test.").Default("false").Bool()
	testDir                  = e2e.Flag("test-directory", "Directory where chainsaw test case will be generated and executed.").Envar("UPTEST_TEST_DIR").Default(filepath.Join(os.TempDir(), "uptest-e2e")).String()
	onlyCleanUptestResources = e2e.Flag("only-clean-uptest-resources", "While deletion step, only clean resources that were created by uptest").Default("false").Bool()

	renderOnly         = e2e.Flag("render-only", "Only render test files. Do not run the tests.").Default("false").Bool()
	logCollectInterval = e2e.Flag("log-collect-interval", "Specifies the interval duration for collecting logs. "+
		"The duration should be provided in a format understood by the tool, such as seconds (s), minutes (m), or hours (h). For example, '30s' for 30 seconds, '5m' for 5 minutes, or '1h' for one hour.").Default("30s").Duration()
	skipUpdate = e2e.Flag("skip-update", "Skip the update step of the test.").Default("false").Bool()
	skipImport = e2e.Flag("skip-import", "Skip the import step of the test.").Default("false").Bool()
)

func main() {
	if kingpin.MustParse(app.Parse(os.Args[1:])) == e2e.FullCommand() {
		e2eTests()
	}
}

func e2eTests() {
	cd, err := os.Getwd()
	if err != nil {
		kingpin.FatalIfError(err, "cannot get current directory")
	}

	list := strings.Split(*manifestList, ",")
	examplePaths := make([]string, 0, len(list))
	for _, e := range list {
		if e == "" {
			continue
		}
		examplePaths = append(examplePaths, filepath.Join(cd, filepath.Clean(e)))
	}
	if len(examplePaths) == 0 {
		kingpin.Fatalf("No manifest to test provided.")
	}

	setupPath := ""
	if *setupScript != "" {
		setupPath, err = filepath.Abs(*setupScript)
		if err != nil {
			kingpin.FatalIfError(err, "cannot get absolute path of setup script")
		}
	}

	teardownPath := ""
	if *teardownScript != "" {
		teardownPath, err = filepath.Abs(*teardownScript)
		if err != nil {
			kingpin.FatalIfError(err, "cannot get absolute path of teardown script")
		}
	}
	o := &config.AutomatedTest{
		ManifestPaths:            examplePaths,
		DataSourcePath:           *dataSourcePath,
		SetupScriptPath:          setupPath,
		TeardownScriptPath:       teardownPath,
		DefaultConditions:        strings.Split(*defaultConditions, ","),
		DefaultTimeout:           *defaultTimeout,
		Directory:                *testDir,
		SkipDelete:               *skipDelete,
		SkipUpdate:               *skipUpdate,
		SkipImport:               *skipImport,
		OnlyCleanUptestResources: *onlyCleanUptestResources,
		RenderOnly:               *renderOnly,
		LogCollectionInterval:    *logCollectInterval,
	}

	kingpin.FatalIfError(internal.RunTest(o), "cannot run e2e tests successfully")
}
