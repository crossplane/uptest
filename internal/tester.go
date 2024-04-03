// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/uptest/internal/config"
	"github.com/crossplane/uptest/internal/templates"
)

var testFiles = []string{
	"00-apply.yaml",
	"01-update.yaml",
	"02-import.yaml",
	"03-delete.yaml",
}

func newTester(ms []config.Manifest, opts *config.AutomatedTest) *tester {
	return &tester{
		options:   opts,
		manifests: ms,
	}
}

type tester struct {
	options   *config.AutomatedTest
	manifests []config.Manifest
}

func (t *tester) executeTests() error {
	if err := writeTestFile(t.manifests, t.options.Directory); err != nil {
		return err
	}

	resources, err := t.writeChainsawFiles()
	if err != nil {
		return errors.Wrap(err, "cannot write chainsaw test files")
	}

	fmt.Printf("Written test files: %s\n", t.options.Directory)

	if t.options.RenderOnly {
		return nil
	}

	fmt.Println("Running chainsaw tests at " + t.options.Directory)
	for _, tf := range testFiles {
		if !checkFileExists(filepath.Join(t.options.Directory, caseDirectory, tf)) {
			continue
		}
		cmd := exec.Command("bash", "-c", fmt.Sprintf(`"${CHAINSAW}" test --test-dir %s --test-file %s --skip-delete --parallel 1 2>&1`, filepath.Join(t.options.Directory, caseDirectory), tf)) // #nosec G204
		stdout, _ := cmd.StdoutPipe()
		if err := cmd.Start(); err != nil {
			return errors.Wrap(err, "cannot start chainsaw")
		}

		var mutex sync.Mutex
		// Start ticker for kubectl command every 30 seconds
		ticker := time.NewTicker(t.options.LogCollectionInterval)
		done := make(chan bool)
		go logCollector(done, ticker, &mutex, resources)()

		sc := bufio.NewScanner(stdout)
		sc.Split(bufio.ScanLines)
		for sc.Scan() {
			mutex.Lock()
			fmt.Println(sc.Text())
			mutex.Unlock()
		}
		if err := cmd.Wait(); err != nil {
			return errors.Wrap(err, "cannot wait for chainsaw")
		}

		ticker.Stop()
		done <- true
	}
	return nil
}

func logCollector(done chan bool, ticker *time.Ticker, mutex sync.Locker, resources []config.Resource) func() {
	return func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mutex.Lock()
				fmt.Printf("crossplane trace logs %s\n", time.Now())
				for _, r := range resources {
					kubectlCmd := exec.Command("bash", "-c", fmt.Sprintf(`"${CROSSPLANE_CLI}" beta trace %s %s -o wide`, r.KindGroup, r.Name)) //nolint:gosec // Disabling gosec to allow dynamic shell command execution
					output, err := kubectlCmd.CombinedOutput()
					if err != nil {
						fmt.Println("Error executing crossplane:", err)
					} else {
						fmt.Println(string(output))
					}
				}
				mutex.Unlock()
			}
		}
	}
}

func (t *tester) prepareConfig() (*config.TestCase, []config.Resource, error) { //nolint:gocyclo // TODO: can we break this?
	tc := &config.TestCase{
		Timeout:                  t.options.DefaultTimeout.Duration,
		SetupScriptPath:          t.options.SetupScriptPath,
		TeardownScriptPath:       t.options.TeardownScriptPath,
		OnlyCleanUptestResources: t.options.OnlyCleanUptestResources,
		TestDirectory:            "test-input.yaml",
	}
	examples := make([]config.Resource, 0, len(t.manifests))

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
			example.Timeout = v1.Duration{Duration: time.Duration(d)}
			if example.Timeout.Duration > tc.Timeout {
				tc.Timeout = example.Timeout.Duration
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
					tc.SkipImport = true
				}
				if updateParameter == "" || obj.GetNamespace() != "" {
					tc.SkipUpdate = true
				}
				example.Root = true
			}
		}

		examples = append(examples, example)
	}

	return tc, examples, nil
}

func (t *tester) writeChainsawFiles() ([]config.Resource, error) {
	tc, examples, err := t.prepareConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot build examples config")
	}

	files, err := templates.Render(tc, examples, t.options.SkipDelete)
	if err != nil {
		return nil, errors.Wrap(err, "cannot render chainsaw templates")
	}

	for k, v := range files {
		if err := os.WriteFile(filepath.Join(filepath.Join(t.options.Directory, caseDirectory), k), []byte(v), fs.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "cannot write file %q", k)
		}
	}

	return examples, nil
}

func writeTestFile(manifests []config.Manifest, directory string) error {
	file, err := os.Create(filepath.Join(directory, caseDirectory, "test-input.yaml")) //nolint:gosec // Safe disable of gosec: path components are controlled, mitigating directory traversal risks.
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // Ignoring error on file close as any failures do not impact the functionality and are logged at a higher level.

	writer := bufio.NewWriter(file)
	for _, manifest := range manifests {
		if _, err := writer.WriteString("---\n"); err != nil {
			return err
		}
		if _, err = writer.WriteString(manifest.YAML + "\n"); err != nil {
			return err
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
