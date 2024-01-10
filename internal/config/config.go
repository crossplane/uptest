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

// Package config contains configuration options for configuring uptest runtime.
package config

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	// AnnotationKeyTimeout defines a test time for the annotated resource.
	AnnotationKeyTimeout = "uptest.upbound.io/timeout"
	// AnnotationKeyConditions defines the list of status conditions to
	// assert on the tested resource.
	AnnotationKeyConditions = "uptest.upbound.io/conditions"
	// AnnotationKeyPreAssertHook defines the path to a pre-assert
	// hook script to be executed before the resource is tested.
	AnnotationKeyPreAssertHook = "uptest.upbound.io/pre-assert-hook"
	// AnnotationKeyPostAssertHook defines the path to a post-assert
	// hook script to be executed after the resource is tested.
	AnnotationKeyPostAssertHook = "uptest.upbound.io/post-assert-hook"
	// AnnotationKeyPreDeleteHook defines the path to a pre-delete
	// hook script to be executed before the tested resource is deleted.
	AnnotationKeyPreDeleteHook = "uptest.upbound.io/pre-delete-hook"
	// AnnotationKeyPostDeleteHook defines the path to a post-delete
	// hook script to be executed after the tested resource is deleted.
	AnnotationKeyPostDeleteHook = "uptest.upbound.io/post-delete-hook"
	// AnnotationKeyUpdateParameter defines the update parameter that will be
	// used during the update step
	AnnotationKeyUpdateParameter = "uptest.upbound.io/update-parameter"
	// AnnotationKeyExampleID is id of example that populated from example
	// manifest. This information will be used for determining the root resource
	AnnotationKeyExampleID = "meta.upbound.io/example-id"
	// AnnotationKeyDisableImport defines that determines whether the Import
	// step of the resource to be tested will be executed or not.
	AnnotationKeyDisableImport = "uptest.upbound.io/disable-import"
)

// AutomatedTest represents an automated test of resource example
// manifests to be run with uptest.
type AutomatedTest struct {
	Directory string

	ManifestPaths  []string
	DataSourcePath string

	SetupScriptPath    string
	TeardownScriptPath string

	DefaultTimeout    int
	DefaultConditions []string

	SkipDelete bool

	OnlyCleanUptestResources bool
}

// Manifest represents a resource loaded from an example resource manifest file.
type Manifest struct {
	FilePath string
	Object   *unstructured.Unstructured
	YAML     string
}

// TestCase represents a test-case to be run by kuttl.
type TestCase struct {
	Timeout            int
	SetupScriptPath    string
	TeardownScriptPath string
	SkipUpdate         bool
	SkipImport         bool

	OnlyCleanUptestResources bool
}

// Resource represents a Kubernetes object to be tested and asserted
// by uptest.
type Resource struct {
	Name      string
	Namespace string
	KindGroup string
	YAML      string

	Timeout              int
	Conditions           []string
	PreAssertScriptPath  string
	PostAssertScriptPath string
	PreDeleteScriptPath  string
	PostDeleteScriptPath string

	UpdateParameter   string
	UpdateAssertKey   string
	UpdateAssertValue string

	SkipImport bool

	Root bool
}
