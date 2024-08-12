// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: CC0-1.0

package templates

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
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

	namespaceManifest = `apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
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
					Timeout: 10,
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": "# This file belongs to the resource apply step.\n---\n" + bucketManifest,
					"00-assert.yaml": `# This assert file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -n "${CROSSPLANE_CLI}" ]; then ${KUBECTL} get composite --no-headers -o name | while read -r comp; do [ -n "$comp" ] && ${CROSSPLANE_CLI} beta trace "$comp"; done; fi
- script: echo "Dump MR manifests for the apply assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the apply assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
`,
					"01-assert.yaml": `# This assert file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the update assertion step:"; ${KUBECTL} get managed -o yaml
`,
					"02-assert.yaml": `# This assert file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the import assertion step:"; ${KUBECTL} get managed -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: new_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.status.atProvider.id}')" && old_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.metadata.annotations.uptest-old-id}')" && [ "$new_id" = "$old_id" ]
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=true --overwrite
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=0 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=0
- command: sleep 10
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=1 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=1
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
- script: /tmp/check_endpoints.sh
- script: /tmp/patch.sh s3.aws.upbound.io example-bucket
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=false --overwrite
`,

					"03-assert.yaml": `# This assert file belongs to the resource delete step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the delete assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the delete assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=delete --timeout 10s
- command: ${KUBECTL} wait managed --all --for=delete --timeout 10s
`,
					"03-delete.yaml": `# This file belongs to the resource delete step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
`,
				},
			},
		},
		"SuccessMultipleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
					{
						YAML:      namespaceManifest,
						Name:      "test-namespace",
						KindGroup: "namespace.",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: /tmp/setup.sh
` + "---\n" + bucketManifest + "---\n" + claimManifest + "---\n" + secretManifest + "---\n" + namespaceManifest,
					"00-assert.yaml": `# This assert file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -n "${CROSSPLANE_CLI}" ]; then ${KUBECTL} get composite --no-headers -o name | while read -r comp; do [ -n "$comp" ] && ${CROSSPLANE_CLI} beta trace "$comp"; done; fi
- script: echo "Dump MR manifests for the apply assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the apply assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: /tmp/bucket/pre-assert.sh
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Ready --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Synced --timeout 10s --namespace upbound-system
- command: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
`,
					"01-assert.yaml": `# This assert file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the update assertion step:"; ${KUBECTL} get managed -o yaml
`,
					"02-assert.yaml": `# This assert file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the import assertion step:"; ${KUBECTL} get managed -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: new_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.status.atProvider.id}')" && old_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.metadata.annotations.uptest-old-id}')" && [ "$new_id" = "$old_id" ]
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=true --overwrite
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=0 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=0
- command: sleep 10
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=1 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=1
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
- script: /tmp/check_endpoints.sh
- script: /tmp/patch.sh s3.aws.upbound.io example-bucket
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=false --overwrite
`,
					"03-assert.yaml": `# This assert file belongs to the resource delete step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the delete assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the delete assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=delete --timeout 10s
- script: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=delete --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait managed --all --for=delete --timeout 10s
- command: /tmp/teardown.sh
`,
					"03-delete.yaml": `# This file belongs to the resource delete step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
- command: /tmp/bucket/post-delete.sh
- command: /tmp/claim/pre-delete.sh
- command: ${KUBECTL} delete cluster.gcp.platformref.upbound.io/test-cluster-claim --wait=false --namespace upbound-system --ignore-not-found
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
					Timeout: 10,
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": "# This file belongs to the resource apply step.\n---\n" + bucketManifest,
					"00-assert.yaml": `# This assert file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -n "${CROSSPLANE_CLI}" ]; then ${KUBECTL} get composite --no-headers -o name | while read -r comp; do [ -n "$comp" ] && ${CROSSPLANE_CLI} beta trace "$comp"; done; fi
- script: echo "Dump MR manifests for the apply assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the apply assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
`,
					"01-assert.yaml": `# This assert file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the update assertion step:"; ${KUBECTL} get managed -o yaml
`,
					"02-assert.yaml": `# This assert file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the import assertion step:"; ${KUBECTL} get managed -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: new_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.status.atProvider.id}')" && old_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.metadata.annotations.uptest-old-id}')" && [ "$new_id" = "$old_id" ]
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=true --overwrite
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=0 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=0
- command: sleep 10
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=1 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=1
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
- script: /tmp/check_endpoints.sh
- script: /tmp/patch.sh s3.aws.upbound.io example-bucket
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=false --overwrite
`,
				},
			},
		},
		"SkipImport": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
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
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
					{
						YAML:      namespaceManifest,
						Name:      "test-namespace",
						KindGroup: "namespace.",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: /tmp/setup.sh
` + "---\n" + bucketManifest + "---\n" + claimManifest + "---\n" + secretManifest + "---\n" + namespaceManifest,
					"00-assert.yaml": `# This assert file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -n "${CROSSPLANE_CLI}" ]; then ${KUBECTL} get composite --no-headers -o name | while read -r comp; do [ -n "$comp" ] && ${CROSSPLANE_CLI} beta trace "$comp"; done; fi
- script: echo "Dump MR manifests for the apply assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the apply assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: /tmp/bucket/pre-assert.sh
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Ready --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Synced --timeout 10s --namespace upbound-system
- command: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
`,
					"01-assert.yaml": `# This assert file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the update assertion step:"; ${KUBECTL} get managed -o yaml
`,
					"02-assert.yaml": `# This assert file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the import assertion step:"; ${KUBECTL} get managed -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=true --overwrite
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=0 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=0
- command: sleep 10
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=1 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=1
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
- script: /tmp/check_endpoints.sh
- script: /tmp/patch.sh s3.aws.upbound.io example-bucket
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=false --overwrite
`,
				},
			},
		},
		"SuccessMultipleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
					{
						YAML:      namespaceManifest,
						Name:      "test-namespace",
						KindGroup: "namespace.",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `# This file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: /tmp/setup.sh
` + "---\n" + bucketManifest + "---\n" + claimManifest + "---\n" + secretManifest + "---\n" + namespaceManifest,
					"00-assert.yaml": `# This assert file belongs to the resource apply step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -n "${CROSSPLANE_CLI}" ]; then ${KUBECTL} get composite --no-headers -o name | while read -r comp; do [ -n "$comp" ] && ${CROSSPLANE_CLI} beta trace "$comp"; done; fi
- script: echo "Dump MR manifests for the apply assertion step:"; ${KUBECTL} get managed -o yaml
- script: echo "Dump Claim manifests for the apply assertion step:" || ${KUBECTL} get claim --all-namespaces -o yaml
- command: /tmp/bucket/pre-assert.sh
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Ready --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Synced --timeout 10s --namespace upbound-system
- command: /tmp/claim/post-assert.sh
`,
					"01-update.yaml": `# This file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
`,
					"01-assert.yaml": `# This assert file belongs to the resource update step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the update assertion step:"; ${KUBECTL} get managed -o yaml
`,
					"02-assert.yaml": `# This assert file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- script: echo "Dump MR manifests for the import assertion step:"; ${KUBECTL} get managed -o yaml
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: new_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.status.atProvider.id}')" && old_id="$(${KUBECTL} get s3.aws.upbound.io/example-bucket -o=jsonpath='{.metadata.annotations.uptest-old-id}')" && [ "$new_id" = "$old_id" ]
`,
					"02-import.yaml": `# This file belongs to the resource import step.
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=true --overwrite
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=0 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=0
- command: sleep 10
- command: ${KUBECTL} scale deployment crossplane -n ${CROSSPLANE_NAMESPACE} --replicas=1 --timeout 10s
- script: ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} get deploy --no-headers -o custom-columns=":metadata.name" | grep "provider-" | xargs ${KUBECTL} -n ${CROSSPLANE_NAMESPACE} scale deploy --replicas=1
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh -o /tmp/check_endpoints.sh && chmod +x /tmp/check_endpoints.sh
- script: curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/patch.sh -o /tmp/patch.sh && chmod +x /tmp/patch.sh
- script: /tmp/check_endpoints.sh
- script: /tmp/patch.sh s3.aws.upbound.io example-bucket
- command: ${KUBECTL} annotate managed --all crossplane.io/paused=false --overwrite
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
