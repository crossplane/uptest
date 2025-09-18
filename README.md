# UPTEST

_Note:_ The `uptest` tool used to live in the https://github.com/upbound/uptest
repository. We have moved it to this repository
(https://github.com/crossplane/uptest) and kept the old repository instead of
renaming it because the provider reusable CI workflows and the other CI tooling
such as `crddiff` or `updoc` still reside in there.

The end to end integration testing tool for Crossplane providers and configurations.

Uptest comes as a binary which can be installed from the releases section. It runs end-to-end tests
by applying the provided examples and waiting for the expected conditions. Other than that, it enables templating to
insert dynamic values into the examples and supports running scripts as hooks just before and right after applying
the examples.

## Usage

```shell
$ uptest e2e --help
usage: uptest e2e [<flags>] [<manifest-list>]

Run e2e tests for manifests by applying them to a control plane and waiting until a given condition is met.

Flags:
  --help                             Show context-sensitive help (also try --help-long and --help-man).
  --data-source=""                   File path of data source that will be used for injection some values.
  --setup-script=""                  Script that will be executed before running tests.
  --teardown-script=""               Script that will be executed after running tests.
  --default-timeout=1200s            Default timeout in seconds for the test. Timeout could be overridden per resource using
                                     "uptest.upbound.io/timeout" annotation.
  --default-conditions="Ready"       Comma separated list of default conditions to wait for a successful test. Conditions could be
                                     overridden per resource using "uptest.upbound.io/conditions" annotation.
  --skip-delete                      Skip the delete step of the test.
  --test-directory="/tmp/uptest-e2e" Directory where chainsaw test case will be generated and executed.
  --only-clean-uptest-resources      While deletion step, only clean resources that were created by uptest
  --render-only                      Only render test files. Do not run the tests.
  --log-collect-interval=30s         Specifies the interval duration for collecting logs. The duration should be provided in a
                                     format understood by the tool, such as seconds (s), minutes (m), or hours (h). For example,
                                     '30s' for 30 seconds, '5m' for 5 minutes, or '1h' for one hour.
  --skip-update                      Skip the update step of the test.
  --skip-import                      Skip the import step of the test.
  --use-library-mode                 Use library mode instead of CLI fork mode. When enabled, chainsaw and crossplane are used as Go
                                     libraries instead of external CLI commands.

Args:
  [<manifest-list>]  List of manifests. Value of this option will be used to trigger/configure the tests.The possible usage:
                     'provider-aws/examples/s3/bucket.yaml,provider-gcp/examples/storage/bucket.yaml': The comma separated resources
                     are used as test inputs. If this option is not set, 'MANIFEST_LIST' env var is used as default.
```

Uptest expects a running control-plane (a.k.a. k8s + crossplane) where required providers are running and/or required
configuration were applied.

Example run:

```shell
uptest e2e examples/user.yaml,examples/bucket.yaml --setup-script="test/hooks/setup.sh"
```

## Execution Modes

Uptest supports two execution modes: CLI Fork Mode (default) and Library Mode. For detailed information about these modes, their differences, and usage examples, see [Execution Modes Documentation](docs/execution-modes.md).

### Injecting Dynamic Values (and Datasource)

Uptest supports injecting dynamic values into the examples by using a data source. The data source is a yaml file
storing key-value pairs. The values can be used in the examples by using the following syntax:

```
${data.key}
```

Example data source file content:

```yaml
aws_account_id: 123456789012
aws_region: us-east-1
```

Example manifest:

```yaml
apiVersion: athena.aws.upbound.io/v1beta1
kind: DataCatalog
metadata:
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    description: Example Athena data catalog
    parameters:
      function: arn:aws:lambda:${data.aws_region}:${data.aws_account_id}:function:upbound-example-function
    region: us-west-1
    tags:
      Name: example-athena-data-catalog
    type: LAMBDA
```

Uptest also supports generating random strings as follows:

```
${Rand.RFC1123Subdomain}
```

Example Manifest:

```yaml
apiVersion: s3.aws.upbound.io/v1beta1
kind: Bucket
metadata:
  name: ${Rand.RFC1123Subdomain}
  labels:
    testing.upbound.io/example-name: s3
spec:
  forProvider:
    region: us-west-1
    objectLockEnabled: true
    tags:
      Name: SampleBucket
```

### Hooks

There are 6 types of hooks that can be used to customize the test flow:

1. `setup-script`: This hook will be executed before running the tests case. It is useful to set up the control plane
   before running the tests. For example, you can use it to create a provider config and your cloud credentials. This
   can be configured via `--setup-script` flag as a relative path to where uptest is executed.
2. `teardown-script`: This hook will be executed after running the tests case. This can be configured via
   `--teardown-script` flag as a relative path to where uptest is executed.
3. `pre-assert-hook`: This hook will be executed before running the assertions and after applying a specific manifest.
    This can be configured via `uptest.upbound.io/pre-assert-hook` annotation on the manifest as a relative path to the
    manifest file.
4. `post-assert-hook`: This hook will be executed after running the assertions. This can be configured via
    `uptest.upbound.io/post-assert-hook` annotation on the manifest as a relative path to the manifest file.
5. `pre-delete-hook`: This hook will be executed just before deleting the resource. This can be configured via
    `uptest.upbound.io/pre-delete-hook` annotation on the manifest as a relative path to the manifest file.
6. `post-delete-hook`: This hook will be executed right after the resource is deleted. This can be configured via
   `uptest.upbound.io/post-delete-hook` annotation on the manifest as a relative path to the manifest file.

> All hooks need to be executables, please make sure to set the executable bit on your scripts, e.g. with `chmod +x`.

### Troubleshooting

Uptest uses [Chainsaw](https://github.com/kyverno/chainsaw) under the hood and generates a `chainsaw` test cases based on the provided input.
You can render and inspect the generated chainsaws test cases by using uptest
`--render-only` flag and checking the output directory. For example:

```shell
uptest e2e examples/kcl/network-xr.yaml --setup-script=test/setup.sh --render-only

2024/11/01 22:20:46 Skipping update step because the root resource does not exist
2024/11/01 22:20:46 Written test files: /var/folders/sx/0tlfb9ys20bbqnszv3lw12m40000gn/T/uptest-e2e

ls -1 /var/folders/sx/0tlfb9ys20bbqnszv3lw12m40000gn/T/uptest-e2e/case/
00-apply.yaml
02-import.yaml
03-delete.yaml
test-input.yaml
```

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/crossplane/uptest/issues).

## Licensing

Uptest is under the Apache 2.0 license.
