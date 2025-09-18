// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package templates

import (
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/uptest/internal/config"
)

const (
	bucketManifest = `apiVersion: s3.aws.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: test-bucket
spec:
  deletionPolicy: Delete
`

	claimManifest = `apiVersion: gcp.platformref.upbound.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-claim
  namespace: upbound-system
spec:
  parameters:
    nodes:
      count: 1
      size: small
`

	secretManifest = `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: upbound-system
type: Opaque
data:
  key: dmFsdWU=
`
)

func TestRender(t *testing.T) {
	type args struct {
		tc        *config.TestCase
		resources []config.Resource
	}
	type want struct {
		out map[string]string
		err error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"SuccessSingleResource": {
			args: args{
				tc: &config.TestCase{
					SetupScriptPath: "/tmp/setup.sh",
					Timeout:         10 * time.Minute,
					TestDirectory:   "/tmp/test-input.yaml",
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						APIVersion: "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:       "Bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: import
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Remove State
    description: |
      Removes the resource statuses from MRs and controllers. For controllers
      the scale down&up was applied. For MRs status conditions are patched.
      Also, for the assertion step, the ID before import was stored in the
      uptest-old-id annotation.
    try:
    - script:
        content: |
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket crossplane.io/paused=true --overwrite
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 0}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale down"
          fi
    - sleep:
        duration: 10s
    - script:
        content: |
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 1}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale up"
          fi
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch-ns.sh -o /tmp/patch-ns.sh && chmod +x /tmp/patch-ns.sh
          /tmp/check_endpoints.sh
          /tmp/patch.sh s3.aws.upbound.io example-bucket
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket --all crossplane.io/paused=false --overwrite
  - name: Assert Status Conditions and IDs
    description: |
      Assert imported resources. Firstly check the status conditions. Then
      compare the stored ID and the new populated ID. For successful test,
      the ID must be the same.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        timeout: 1m
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          ("status.atProvider.id" == "metadata.annotations.uptest-old-id"): true
`,
					"03-delete.yaml": `# This file belongs to the resource delete step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: delete
spec:
  timeouts:
    exec: 10m0s
  steps:
  - name: Delete Resources
    description: Delete resources. If needs ordered deletion, the pre-delete scripts were used.
    try:
    - script:
        content: |
          ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
  - name: Assert Deletion
    description: Assert deletion of resources.
    try:
    - script:
        content: |
          ${KUBECTL} wait --for=delete s3.aws.upbound.io/example-bucket --timeout 10m0s
    - script:
        content: |
          ${KUBECTL} wait managed --all --for=delete --timeout -1s
`,
				},
			},
		},
		"SuccessSingleResourceWithNoSetupScript": {
			args: args{
				tc: &config.TestCase{
					Timeout:       10 * time.Minute,
					TestDirectory: "/tmp/test-input.yaml",
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						APIVersion: "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:       "Bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: import
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Remove State
    description: |
      Removes the resource statuses from MRs and controllers. For controllers
      the scale down&up was applied. For MRs status conditions are patched.
      Also, for the assertion step, the ID before import was stored in the
      uptest-old-id annotation.
    try:
    - script:
        content: |
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket crossplane.io/paused=true --overwrite
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 0}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale down"
          fi
    - sleep:
        duration: 10s
    - script:
        content: |
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 1}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale up"
          fi
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch-ns.sh -o /tmp/patch-ns.sh && chmod +x /tmp/patch-ns.sh
          /tmp/check_endpoints.sh
          /tmp/patch.sh s3.aws.upbound.io example-bucket
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket --all crossplane.io/paused=false --overwrite
  - name: Assert Status Conditions and IDs
    description: |
      Assert imported resources. Firstly check the status conditions. Then
      compare the stored ID and the new populated ID. For successful test,
      the ID must be the same.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        timeout: 1m
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          ("status.atProvider.id" == "metadata.annotations.uptest-old-id"): true
`,
					"03-delete.yaml": `# This file belongs to the resource delete step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: delete
spec:
  timeouts:
    exec: 10m0s
  steps:
  - name: Delete Resources
    description: Delete resources. If needs ordered deletion, the pre-delete scripts were used.
    try:
    - script:
        content: |
          ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
  - name: Assert Deletion
    description: Assert deletion of resources.
    try:
    - script:
        content: |
          ${KUBECTL} wait --for=delete s3.aws.upbound.io/example-bucket --timeout 10m0s
    - script:
        content: |
          ${KUBECTL} wait managed --all --for=delete --timeout -1s
`,
				},
			},
		},
		"SuccessMultipleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10 * time.Minute,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
					TestDirectory:      "/tmp/test-input.yaml",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						APIVersion:           "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:                 "Bucket",
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						APIVersion:           "cluster.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "Cluster",
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
						SkipImport:           true,
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - command:
        entrypoint: /tmp/bucket/pre-assert.sh
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        resource:
          apiVersion: cluster.gcp.platformref.upbound.io/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster-claim
            namespace: upbound-system
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: import
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Remove State
    description: |
      Removes the resource statuses from MRs and controllers. For controllers
      the scale down&up was applied. For MRs status conditions are patched.
      Also, for the assertion step, the ID before import was stored in the
      uptest-old-id annotation.
    try:
    - script:
        content: |
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket crossplane.io/paused=true --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim crossplane.io/paused=true --overwrite
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 0}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale down"
          fi
    - sleep:
        duration: 10s
    - script:
        content: |
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 1}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale up"
          fi
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch-ns.sh -o /tmp/patch-ns.sh && chmod +x /tmp/patch-ns.sh
          /tmp/check_endpoints.sh
          /tmp/patch.sh s3.aws.upbound.io example-bucket
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket --all crossplane.io/paused=false --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim --all crossplane.io/paused=false --overwrite
  - name: Assert Status Conditions and IDs
    description: |
      Assert imported resources. Firstly check the status conditions. Then
      compare the stored ID and the new populated ID. For successful test,
      the ID must be the same.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        timeout: 1m
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          ("status.atProvider.id" == "metadata.annotations.uptest-old-id"): true
`,
					"03-delete.yaml": `# This file belongs to the resource delete step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: delete
spec:
  timeouts:
    exec: 10m0s
  steps:
  - name: Delete Resources
    description: Delete resources. If needs ordered deletion, the pre-delete scripts were used.
    try:
    - script:
        content: |
          ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
          /tmp/bucket/post-delete.sh
          /tmp/claim/pre-delete.sh
          ${KUBECTL} delete cluster.gcp.platformref.upbound.io/test-cluster-claim --wait=false --namespace upbound-system --ignore-not-found
  - name: Assert Deletion
    description: Assert deletion of resources.
    try:
    - script:
        content: |
          ${KUBECTL} wait --for=delete s3.aws.upbound.io/example-bucket --timeout 10m0s
    - script:
        content: |
          ${KUBECTL} wait --namespace upbound-system --for=delete cluster.gcp.platformref.upbound.io/test-cluster-claim --timeout 10m0s
    - script:
        content: |
          ${KUBECTL} wait managed --all --for=delete --timeout -1s
    - command:
        entrypoint: /tmp/teardown.sh
`,
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Render(tc.args.tc, tc.args.resources, false)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Render(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("Render(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestRenderWithSkipDelete(t *testing.T) {
	type args struct {
		tc        *config.TestCase
		resources []config.Resource
	}
	type want struct {
		out map[string]string
		err error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"SuccessSingleResource": {
			args: args{
				tc: &config.TestCase{
					SetupScriptPath: "/tmp/setup.sh",
					Timeout:         10 * time.Minute,
					TestDirectory:   "/tmp/test-input.yaml",
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						APIVersion: "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:       "Bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: import
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Remove State
    description: |
      Removes the resource statuses from MRs and controllers. For controllers
      the scale down&up was applied. For MRs status conditions are patched.
      Also, for the assertion step, the ID before import was stored in the
      uptest-old-id annotation.
    try:
    - script:
        content: |
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket crossplane.io/paused=true --overwrite
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 0}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale down"
          fi
    - sleep:
        duration: 10s
    - script:
        content: |
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 1}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale up"
          fi
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch-ns.sh -o /tmp/patch-ns.sh && chmod +x /tmp/patch-ns.sh
          /tmp/check_endpoints.sh
          /tmp/patch.sh s3.aws.upbound.io example-bucket
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket --all crossplane.io/paused=false --overwrite
  - name: Assert Status Conditions and IDs
    description: |
      Assert imported resources. Firstly check the status conditions. Then
      compare the stored ID and the new populated ID. For successful test,
      the ID must be the same.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        timeout: 1m
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          ("status.atProvider.id" == "metadata.annotations.uptest-old-id"): true
`,
				},
			},
		},
		"SkipImport": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10 * time.Minute,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
					TestDirectory:      "/tmp/test-input.yaml",
					SkipImport:         true,
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						APIVersion:           "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:                 "Bucket",
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						SkipImport:           true,
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						Name:                 "test-cluster-claim",
						APIVersion:           "cluster.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "Cluster",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
						SkipImport:           true,
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - command:
        entrypoint: /tmp/bucket/pre-assert.sh
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        resource:
          apiVersion: cluster.gcp.platformref.upbound.io/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster-claim
            namespace: upbound-system
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
				},
			},
		},
		"SuccessMultipleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10 * time.Minute,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
					TestDirectory:      "/tmp/test-input.yaml",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						APIVersion:           "bucket.s3.aws.upbound.io/v1alpha1",
						Kind:                 "Bucket",
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						APIVersion:           "cluster.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "Cluster",
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
						SkipImport:           true,
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket upjet.upbound.io/test=true --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - command:
        entrypoint: /tmp/bucket/pre-assert.sh
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        resource:
          apiVersion: cluster.gcp.platformref.upbound.io/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster-claim
            namespace: upbound-system
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: update
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Update Root Resource
    description: |
      Update the root resource by using the specified update-parameter in annotation.
      Before updating the resources, the status conditions are cleaned.
    try:
  - name: Assert Updated Resource
    description: |
      Assert update operation. Firstly check the status conditions. Then assert
      the updated field in status.atProvider.
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: import
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Remove State
    description: |
      Removes the resource statuses from MRs and controllers. For controllers
      the scale down&up was applied. For MRs status conditions are patched.
      Also, for the assertion step, the ID before import was stored in the
      uptest-old-id annotation.
    try:
    - script:
        content: |
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket crossplane.io/paused=true --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim crossplane.io/paused=true --overwrite
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 0}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale down"
          fi
    - sleep:
        duration: 10s
    - script:
        content: |
          PROVIDER_CONFIGS=$(${KUBECTL} get deploymentruntimeconfig --no-headers -o custom-columns=":metadata.name" | grep "provider-" || true)
          if [ -n "$PROVIDER_CONFIGS" ]; then
            echo "$PROVIDER_CONFIGS" | xargs ${KUBECTL} patch deploymentruntimeconfig --type='json' -p='[{"op": "replace", "path": "/spec/deploymentTemplate/spec/replicas", "value": 1}]'
          else
            echo "No provider DeploymentRuntimeConfigs found to scale up"
          fi
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
          curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch-ns.sh -o /tmp/patch-ns.sh && chmod +x /tmp/patch-ns.sh
          /tmp/check_endpoints.sh
          /tmp/patch.sh s3.aws.upbound.io example-bucket
          ${KUBECTL} annotate  s3.aws.upbound.io/example-bucket --all crossplane.io/paused=false --overwrite
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim --all crossplane.io/paused=false --overwrite
  - name: Assert Status Conditions and IDs
    description: |
      Assert imported resources. Firstly check the status conditions. Then
      compare the stored ID and the new populated ID. For successful test,
      the ID must be the same.
    try:
    - assert:
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          status:
            ((conditions[?type == 'Test'])[0]):
              status: "True"
    - assert:
        timeout: 1m
        resource:
          apiVersion: bucket.s3.aws.upbound.io/v1alpha1
          kind: Bucket
          metadata:
            name: example-bucket
          ("status.atProvider.id" == "metadata.annotations.uptest-old-id"): true
`,
				},
			},
		},
		"SuccessClaim": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10 * time.Minute,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
					TestDirectory:      "/tmp/test-input.yaml",
					SkipUpdate:         true,
					SkipImport:         true,
				},
				resources: []config.Resource{
					{
						YAML:                 claimManifest,
						APIVersion:           "cluster.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "Cluster",
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - assert:
        resource:
          apiVersion: cluster.gcp.platformref.upbound.io/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster-claim
            namespace: upbound-system
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
`,
				},
			},
		},
		"SuccessClaimAndXR": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10 * time.Minute,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
					TestDirectory:      "/tmp/test-input.yaml",
					SkipUpdate:         true,
					SkipImport:         true,
				},
				resources: []config.Resource{
					{
						YAML:                 claimManifest,
						APIVersion:           "cluster.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "Cluster",
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
					{
						YAML:                 claimManifest,
						APIVersion:           "xnetwork.gcp.platformref.upbound.io/v1alpha1",
						Kind:                 "XNetwork",
						Name:                 "test-network-xr",
						KindGroup:            "xnetwork.gcp.platformref.upbound.io",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: apply
spec:
  timeouts:
    apply: 10m0s
    assert: 10m0s
    exec: 10m0s
  steps:
  - name: Run Setup Script
    description: Setup the test environment by running the setup script.
    try:
    - command:
        entrypoint: /tmp/setup.sh
  - name: Apply Resources
    description: Apply resources to the cluster.
    try:
    - apply:
        file: /tmp/test-input.yaml
    - script:
        content: |
          echo "Runnning annotation script"
          ${KUBECTL} annotate --namespace upbound-system  cluster.gcp.platformref.upbound.io/test-cluster-claim upjet.upbound.io/test=true --overwrite
          ${KUBECTL} annotate  xnetwork.gcp.platformref.upbound.io/test-network-xr upjet.upbound.io/test=true --overwrite
  - name: Assert Status Conditions
    description: |
      Assert applied resources. First, run the pre-assert script if exists.
      Then, check the status conditions. Finally run the post-assert script if it
      exists.
    try:
    - assert:
        resource:
          apiVersion: cluster.gcp.platformref.upbound.io/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster-claim
            namespace: upbound-system
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
    - assert:
        resource:
          apiVersion: xnetwork.gcp.platformref.upbound.io/v1alpha1
          kind: XNetwork
          metadata:
            name: test-network-xr
          status:
            ((conditions[?type == 'Ready'])[0]):
              status: "True"
            ((conditions[?type == 'Synced'])[0]):
              status: "True"
    - command:
        entrypoint: /tmp/claim/post-assert.sh
`,
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Render(tc.args.tc, tc.args.resources, true)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Render(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("Render(...): -want, +got:\n%s", diff)
			}
		})
	}
}
