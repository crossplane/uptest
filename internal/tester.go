// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/kyverno/chainsaw/pkg/discovery"
	kconfig "github.com/kyverno/chainsaw/pkg/loaders/config"
	"github.com/kyverno/chainsaw/pkg/runner"
	enginecontext "github.com/kyverno/chainsaw/pkg/runner/context"
	runnerflags "github.com/kyverno/chainsaw/pkg/runner/flags"
	restutils "github.com/kyverno/chainsaw/pkg/utils/rest"
	"github.com/kyverno/pkg/ext/output/color"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane/v2/cmd/crank/beta/trace"

	"github.com/crossplane/uptest/v2/internal/config"
	"github.com/crossplane/uptest/v2/internal/templates"
)

var testFiles = []string{
	"00-apply.yaml",
	"01-update.yaml",
	"02-import.yaml",
	"03-delete.yaml",
}

// NewTester returns a Tester object.
func NewTester(ms []config.Manifest, opts *config.AutomatedTest) *Tester {
	return &Tester{
		options:   opts,
		manifests: ms,
	}
}

// Tester is responsible preparing and storing the test data&configurations,
// and executing the tests.
type Tester struct {
	options   *config.AutomatedTest
	manifests []config.Manifest
}

// ExecuteTests execute tests via chainsaw.
func (t *Tester) ExecuteTests() error {
	if err := writeTestFile(t.manifests, t.options.Directory); err != nil {
		return errors.Wrap(err, "cannot write test manifest files")
	}

	resources, timeout, err := t.writeChainsawFiles()
	if err != nil {
		return errors.Wrap(err, "cannot write chainsaw test files")
	}

	log.Printf("Written test files: %s\n", t.options.Directory)

	if t.options.RenderOnly {
		return nil
	}

	log.Println("Running chainsaw tests at " + t.options.Directory)
	startTime := time.Now()
	for _, tf := range testFiles {
		if !checkFileExists(filepath.Join(t.options.Directory, caseDirectory, tf)) {
			log.Println("Skipping test " + tf)
			continue
		}
		if err := executeSingleTestFile(t, tf, timeout-time.Since(startTime), resources); err != nil {
			return errors.Wrap(err, "cannot execute test "+tf)
		}
	}
	return nil
}

func executeSingleTestFile(t *Tester, tf string, timeout time.Duration, resources []config.Resource) error {
	if t.options.UseLibraryMode {
		return executeSingleTestFileLibraryMode(t, tf, timeout, resources)
	}
	return executeSingleTestFileCLIMode(t, tf, timeout, resources)
}

func executeSingleTestFileLibraryMode(t *Tester, tf string, timeout time.Duration, resources []config.Resource) error {
	// Explicitly Set Controller Logger
	// because of log.SetLogger(...) was never called;
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	log.Println("Loading default configuration...")
	configuration, err := kconfig.DefaultConfiguration()
	if err != nil {
		return errors.Wrap(err, "failed to load Chainsaw default configuration")
	}
	configuration.Spec.Discovery.TestFile = tf
	configuration.Spec.Execution.Parallel = ptr.To(1)
	configuration.Spec.Cleanup.SkipDelete = true

	log.Printf("- Using test file: %s\n", configuration.Spec.Discovery.TestFile)
	log.Printf("- ApplyTimeout %v\n", configuration.Spec.Timeouts.Apply.Duration)
	log.Printf("- AssertTimeout %v\n", configuration.Spec.Timeouts.Assert.Duration)
	log.Printf("- CleanupTimeout %v\n", configuration.Spec.Timeouts.Cleanup.Duration)
	log.Printf("- DeleteTimeout %v\n", configuration.Spec.Timeouts.Delete.Duration)
	log.Printf("- ErrorTimeout %v\n", configuration.Spec.Timeouts.Error.Duration)
	log.Printf("- ExecTimeout %v\n", configuration.Spec.Timeouts.Exec.Duration)
	log.Printf("- Parallel %d\n", *configuration.Spec.Execution.Parallel)
	color.Init(false, true)

	log.Println("Loading tests...")
	tests, err := discovery.DiscoverTests(tf, nil, false, t.options.Directory)
	if err != nil {
		return errors.Wrap(err, "failed to discover test cases")
	}

	var testToRun []discovery.Test
	for _, test := range tests {
		if test.Err != nil {
			log.Printf("- %s (%s) - (%s)\n", test.Test.Name, test.BasePath, test.Err)
		} else {
			log.Printf("- %s (%s)\n", test.Test.Name, test.BasePath)
			testToRun = append(testToRun, test)
		}
	}

	log.Println("Running tests...")
	overrides := clientcmd.ConfigOverrides{}
	restConfig, err := restutils.DefaultConfig(overrides)
	if err != nil {
		return errors.Wrap(err, "failed to load Kubernetes config")
	}

	tc, err := enginecontext.InitContext(configuration.Spec, restConfig, nil)
	if err != nil {
		return errors.Wrap(err, "failed to initialize execution context")
	}

	clock := clock.RealClock{}
	onFailure := func() {
		log.Println("Test failed.")
	}

	runner := runner.New(clock, onFailure)

	ticker := time.NewTicker(t.options.LogCollectionInterval)
	done := make(chan bool)
	var mutex sync.Mutex
	defer func() {
		ticker.Stop()
		close(done)
	}()

	go logCollectorLibraryMode(done, ticker, &mutex, resources)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := runnerflags.SetupFlags(configuration.Spec); err != nil {
		return err
	}
	err = runner.Run(ctx, configuration.Spec.Namespace, tc, testToRun...)
	if err != nil {
		return errors.Wrap(err, "test execution failed")
	}

	log.Println("Tests Summary:")
	log.Printf("- Passed: %d\n", tc.Passed())
	log.Printf("- Failed: %d\n", tc.Failed())
	log.Printf("- Skipped: %d\n", tc.Skipped())

	if tc.Failed() > 0 {
		return errors.New("some tests failed")
	}

	return nil
}

func executeSingleTestFileCLIMode(t *Tester, tf string, timeout time.Duration, resources []config.Resource) error {
	chainsawCommand := fmt.Sprintf(`"${CHAINSAW}" test --test-dir %s --test-file %s --skip-delete --parallel 1 2>&1`,
		filepath.Clean(filepath.Join(t.options.Directory, caseDirectory)),
		filepath.Clean(tf))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", chainsawCommand) // #nosec G204
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "cannot start chainsaw: %s", chainsawCommand)
	}

	// Start ticker for kubectl command every 30 seconds
	ticker := time.NewTicker(t.options.LogCollectionInterval)
	done := make(chan bool)
	defer func() {
		ticker.Stop()
		close(done)
	}()

	var mutex sync.Mutex
	go logCollectorCLIMode(done, ticker, &mutex, resources)

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		mutex.Lock()
		log.Println(sc.Text())
		mutex.Unlock()
	}
	if sc.Err() != nil {
		return errors.Wrap(sc.Err(), "cannot scan output")
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrapf(err, "cannot wait for chainsaw: %s", chainsawCommand)
	}

	return nil
}

func logCollectorLibraryMode(done chan bool, ticker *time.Ticker, mutex sync.Locker, resources []config.Resource) {
	logger := logging.NewNopLogger()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			mutex.Lock()

			kongParser := kong.Must(&trace.Cmd{})
			kongCtx := &kong.Context{
				Kong: kongParser,
			}

			for _, r := range resources {
				traceCmd := trace.Cmd{
					Resource: r.KindGroup,
					Name:     r.Name,
					Output:   "wide",
				}

				if err := traceCmd.Run(kongCtx, logger); err != nil {
					continue
				}
			}
			mutex.Unlock()
		}
	}
}

func logCollectorCLIMode(done chan bool, ticker *time.Ticker, mutex sync.Locker, resources []config.Resource) {
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			mutex.Lock()
			for _, r := range resources {
				// During the setup script is running, the crossplane command
				// is failing because of the resource not found error.
				// We do not want to show this error to the user because it
				// is a noise and temporary one.
				// The error output was redirected to a file.
				traceCmd := exec.Command("bash", "-c", fmt.Sprintf(`"${CROSSPLANE_CLI}" beta trace %s %s -o wide 2>>/tmp/uptest_crossplane_temp_errors.log`, r.KindGroup, r.Name)) //nolint:gosec // Disabling gosec to allow dynamic shell command execution
				output, err := traceCmd.CombinedOutput()
				if err == nil {
					log.Printf("crossplane trace logs %s\n%s\n", time.Now(), string(output))
				}
			}
			mutex.Unlock()
		}
	}
}

func (t *Tester) prepareConfig() (*config.TestCase, []config.Resource, error) { //nolint:gocyclo // TODO: can we break this?
	tc := &config.TestCase{
		Timeout:                  t.options.DefaultTimeout,
		SetupScriptPath:          t.options.SetupScriptPath,
		TeardownScriptPath:       t.options.TeardownScriptPath,
		OnlyCleanUptestResources: t.options.OnlyCleanUptestResources,
		TestDirectory:            "test-input.yaml",
	}
	examples := make([]config.Resource, 0, len(t.manifests))

	rootFound := false
	for _, m := range t.manifests {
		obj := m.Object
		groupVersionKind := obj.GroupVersionKind()
		apiVersion, kind := groupVersionKind.ToAPIVersionAndKind()
		kg := strings.ToLower(groupVersionKind.Kind + "." + groupVersionKind.Group)

		example := config.Resource{
			Name:       obj.GetName(),
			Namespace:  obj.GetNamespace(),
			KindGroup:  kg,
			YAML:       m.YAML,
			Timeout:    t.options.DefaultTimeout,
			Conditions: t.options.DefaultConditions,
			APIVersion: apiVersion,
			Kind:       kind,
		}

		var err error
		annotations := obj.GetAnnotations()
		if v, ok := annotations[config.AnnotationKeyTimeout]; ok {
			d, err := strconv.Atoi(v)
			if err != nil {
				return nil, nil, errors.Wrap(err, "timeout value is not valid")
			}
			example.Timeout = time.Duration(d) * time.Second
			if example.Timeout > tc.Timeout {
				tc.Timeout = example.Timeout
			}
		}

		if v, ok := annotations[config.AnnotationKeyConditions]; ok {
			example.Conditions = strings.Split(v, ",")
		}

		if v, ok := annotations[config.AnnotationKeyPreAssertHook]; ok {
			example.PreAssertScriptPath, err = filepath.Abs(filepath.Join(filepath.Dir(m.FilePath), filepath.Clean(v)))
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot find absolute path for pre assert hook")
			}
		}

		if v, ok := annotations[config.AnnotationKeyPostAssertHook]; ok {
			example.PostAssertScriptPath, err = filepath.Abs(filepath.Join(filepath.Dir(m.FilePath), filepath.Clean(v)))
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot find absolute path for post assert hook")
			}
		}

		if v, ok := annotations[config.AnnotationKeyPreDeleteHook]; ok {
			example.PreDeleteScriptPath, err = filepath.Abs(filepath.Join(filepath.Dir(m.FilePath), filepath.Clean(v)))
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot find absolute path for pre delete hook")
			}
		}

		if v, ok := annotations[config.AnnotationKeyPostDeleteHook]; ok {
			example.PostDeleteScriptPath, err = filepath.Abs(filepath.Join(filepath.Dir(m.FilePath), filepath.Clean(v)))
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot find absolute path for post delete hook")
			}
		}

		updateParameter, ok := annotations[config.AnnotationKeyUpdateParameter]
		if !ok {
			updateParameter = os.Getenv("UPTEST_UPDATE_PARAMETER")
		}
		if updateParameter != "" {
			example.UpdateParameter = updateParameter
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(updateParameter), &data); err != nil {
				return nil, nil, errors.Wrapf(err, "cannot unmarshal JSON object: %s", updateParameter)
			}
			example.UpdateAssertKey, example.UpdateAssertValue = convertToJSONPath(data, "")
		}
		disableImport, ok := annotations[config.AnnotationKeyDisableImport]
		if ok && disableImport == "true" {
			example.SkipImport = true
		}

		if exampleID, ok := annotations[config.AnnotationKeyExampleID]; ok {
			if exampleID == strings.ToLower(fmt.Sprintf("%s/%s/%s", strings.Split(groupVersionKind.Group, ".")[0], groupVersionKind.Version, groupVersionKind.Kind)) {
				if disableImport == "true" {
					log.Println("Skipping import step because the root resource has disable import annotation")
					tc.SkipImport = true
				}
				if updateParameter == "" {
					log.Println("Skipping update step because the root resource does not have the update parameter")
					tc.SkipUpdate = true
				}
				example.Root = true
				rootFound = true
			}
		}

		examples = append(examples, example)
	}

	if !rootFound {
		log.Println("Skipping update step because the root resource does not exist")
		tc.SkipUpdate = true
	}
	if t.options.SkipUpdate {
		log.Println("Skipping update step because the skip-delete option is set to true")
		tc.SkipUpdate = true
	}
	if t.options.SkipImport {
		log.Println("Skipping import step because the skip-import option is set to true")
		tc.SkipImport = true
	}

	return tc, examples, nil
}

func (t *Tester) writeChainsawFiles() ([]config.Resource, time.Duration, error) {
	tc, examples, err := t.prepareConfig()
	if err != nil {
		return nil, 0, errors.Wrap(err, "cannot build examples config")
	}

	files, err := templates.Render(tc, examples, t.options.SkipDelete)
	if err != nil {
		return nil, 0, errors.Wrap(err, "cannot render chainsaw templates")
	}

	for k, v := range files {
		if err := os.WriteFile(filepath.Join(filepath.Join(t.options.Directory, caseDirectory), k), []byte(v), fs.ModePerm); err != nil {
			return nil, 0, errors.Wrapf(err, "cannot write file %q", k)
		}
	}

	return examples, tc.Timeout, nil
}

func writeTestFile(manifests []config.Manifest, directory string) error {
	file, err := os.Create(filepath.Clean(filepath.Join(directory, caseDirectory, "test-input.yaml")))
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // Ignoring error on file close as any failures do not impact the functionality and are logged at a higher level.

	writer := bufio.NewWriter(file)
	for _, manifest := range manifests {
		if _, err := writer.WriteString("---\n"); err != nil {
			return errors.Wrap(err, "cannot write the manifest delimiter")
		}
		if _, err = writer.WriteString(manifest.YAML + "\n"); err != nil {
			return errors.Wrap(err, "cannot write the manifest content")
		}
	}
	return writer.Flush()
}

func convertToJSONPath(data map[string]interface{}, currentPath string) (string, string) {
	for key, value := range data {
		newPath := currentPath + "." + key
		switch v := value.(type) {
		case map[string]interface{}:
			return convertToJSONPath(v, newPath)
		default:
			return newPath, fmt.Sprintf("%v", v)
		}
	}
	return currentPath, ""
}

func checkFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}
