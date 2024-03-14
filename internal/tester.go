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

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/uptest/internal/config"
	"github.com/crossplane/uptest/internal/templates"
)

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
	if err := t.writeKuttlFiles(); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	fmt.Println("Running kuttl tests at " + t.options.Directory)
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`"${KUTTL}" test --start-kind=false --skip-cluster-delete %s --timeout %d 2>&1`, t.options.Directory, t.options.DefaultTimeout)) // #nosec G204
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "cannot start kuttl")
	}
	sc := bufio.NewScanner(stdout)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	return errors.Wrap(cmd.Wait(), "kuttl failed")
}

func (t *tester) prepareConfig() (*config.TestCase, []config.Resource, error) { //nolint:gocyclo // TODO: can we break this?
	tc := &config.TestCase{
		Timeout:                  t.options.DefaultTimeout,
		SetupScriptPath:          t.options.SetupScriptPath,
		TeardownScriptPath:       t.options.TeardownScriptPath,
		OnlyCleanUptestResources: t.options.OnlyCleanUptestResources,
	}
	examples := make([]config.Resource, 0, len(t.manifests))

	for _, m := range t.manifests {
		obj := m.Object
		groupVersionKind := obj.GroupVersionKind()
		kg := strings.ToLower(groupVersionKind.Kind + "." + groupVersionKind.Group)

		example := config.Resource{
			Name:       obj.GetName(),
			Namespace:  obj.GetNamespace(),
			KindGroup:  kg,
			YAML:       m.YAML,
			Timeout:    t.options.DefaultTimeout,
			Conditions: t.options.DefaultConditions,
		}

		var err error
		annotations := obj.GetAnnotations()
		if v, ok := annotations[config.AnnotationKeyTimeout]; ok {
			example.Timeout, err = strconv.Atoi(v)
			if err != nil {
				return nil, nil, errors.Wrap(err, "timeout value is not valid")
			}
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
					tc.SkipImport = true
				}
				if updateParameter == "" {
					tc.SkipUpdate = true
				}
				example.Root = true
			}
		}

		examples = append(examples, example)
	}

	return tc, examples, nil
}

func (t *tester) writeKuttlFiles() error {
	tc, examples, err := t.prepareConfig()
	if err != nil {
		return errors.Wrap(err, "cannot build examples config")
	}

	files, err := templates.Render(tc, examples, t.options.SkipDelete)
	if err != nil {
		return errors.Wrap(err, "cannot render kuttl templates")
	}

	for k, v := range files {
		if err := os.WriteFile(filepath.Join(filepath.Join(t.options.Directory, caseDirectory), k), []byte(v), fs.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot write file %q", k)
		}
	}

	return nil
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
