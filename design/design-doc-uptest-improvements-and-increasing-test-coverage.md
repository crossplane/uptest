# Uptest Improvements and Increasing Test Coverage

* Owner: Sergen Yalcin (@sergenyalcin)
* Reviewers: Uptest Maintainers
* Status: Draft

## Background

Uptest is a tool for testing and validating Crossplane providers. This tool is
utilized to test the behavior of Crossplane controllers, resource management
operations, and other Crossplane components. Uptest can simulate the creation,
update, import, deletion, and other operations of Crossplane resources.

The primary goal of Uptest is to facilitate the testing process of Crossplane
resources. It integrates seamlessly with Crossplane and provides a testing
infrastructure that enables users to create and run test scenarios for
validating the reliability, functionality, and performance of Crossplane
resources.

Uptest was developed as a tool designed to run e2e tests in the GitHub Actions
environment. Then, various improvements were made to ensure it ran in local
environments. However, the tool was not considered a standalone project in the
first place. Uptest was designed to run tests in more controlled environments
(for example, a Kind cluster created from scratch) rather than running tests on
any arbitrary Kubernetes cluster, and has evolved to become a standalone project
over time. Today Uptest is being evaluated as a tool for users to integrate into
their Crossplane development pipelines.

As a result, it is necessary to work on various enhancements for Uptest to
continue its development as a more powerful and independent tool. In this
document, evaluations will be made on the present and future of Uptest and the
aspects to be developed.

In its current form, Uptest offers its users an extensive test framework. This
framework allows us to test and validate many different MRs simultaneously and
seamlessly.

For Crossplane providers, it is not enough to see `Ready: True` in the status of
an MR. `Late-initialization` that occurs after the resource is `Ready` or the
resource is not stable and is subjected to a continuous update loop, are
actually situations that do not affect the `Ready` state of the resource but
affect its lifecycle.

To overcome some of the problems hidden behind this `Ready` condition, we use
the `UpToDate` condition, which is only used in tests and activated by a
specific annotation. This condition makes sure that after the resource is
`Ready`, the `Late-Initialization` step is done, and the resource is not stuck
in any update loop.

For some MRs, it is vital that their names (or some identifier field values) are
randomized. If the name of a resource is not unique, this will cause a conflict,
and the resource cannot be created, or an existing resource could be updated.
Two main issues need to be addressed to avoid this conflict. One is that some
resources expect a universally unique name to be identified. In other words, we
are talking about a name that will be unique on all cloud providers regardless
of account or organization. The other issue is the need for uniqueness on a
smaller scale, that is, within the account or organization used. At the end of
the day, there is a need to generate random strings for some fields of some
resources. Currently, there is only support for this in one format and only for
some fields.

Test cases that initially included only the `Apply` and `Delete` steps now also
include the `Update` and `Import` steps. These new steps are very important in
terms of increasing the test coverage of the lifecycle of resources. The
`Import` step is also an important coverage point in terms of testing whether
the external name configuration of the resources is done correctly. The
validations performed during the transition to the new provider architecture,
especially the new `Import` step, played a critical role in detecting many bugs
and problems early.

## Goals

- Increase Uptest's test capabilities by accommodating different test scenarios.
- Make Uptest capable of running tests on arbitrary clusters.
- Improve uptest documentation, such as user guides, technical documents, etc.
- In cases where Uptest tests fail, debugging/logging should be improved to
allow the user to understand the problem easily.
- Increasing the configurability of Uptest by introducing parametric options to
the CLI.

## Proposal

The proposals below are suggested to achieve the above goals by making various
improvements to the existing Uptest tool.  The main goal is to make Uptest a
more stable and inclusive standalone tool at the end of the day.

## Increased test capabilities

Increasing the capabilities of Uptest will help us in the process of evolving it
into a standalone tool both for test coverage and future use cases.

### Provider upgrade testing

When Uptest first appeared, it was mainly concerned with testing whether a
resource worked properly. Today, it has become an end-to-end testing solution.
Especially when many validations are needed, such as the transition to a new
architecture or before a release. In this context, it is valuable that Uptest
also handles some end-to-end cases.

Uptest tests some basic general steps in each case, such as setting up
Crossplane and provider and testing ProviderConfig (Secret source). However, the
`Upgrade` test, which is one of the main paths, can also be considered an
end-to-end test in this context. Running an automated test along with the
manual tests will increase the confidence level of developers. Roughly, the
`Upgrade` test can look like this:

- Installation of Source Package (parameterized)
- Provisioning of Resources (several centralized resources and packages can be 
selected)
- Upgrade to Target Package (parameterized)
- Check that installed Resources are not affected
- Provisioning of new resources (several centralized resources and packages can
be selected)

A subcommand called `upgrade` can be used to run these tests:

```shell
uptest upgrade --source v1.0.0 --target v1.1.0 --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test"
```

Testing a scenario like this would be especially valuable in terms of increasing
coverage and improving confidence levels ahead of the provider releases.

### Diff tests - Release testing for providers
Before release, it may be valuable to look at the differences with the previous
version and test the resources that are thought to affect the lifecycle. Some
different tools can be used to automate this process. For example, the output of
the [crd-diff](https://github.com/upbound/official-providers-ci/tree/main/cmd/crddiff)
tool can be used to detect field changes in CRDs. Additionally, resources that
have undergone configuration changes can be examined by parsing the git output.

For example, before the `1.3.0` release, the difference between the `1.2.0`
release and the `1.3.0` release can be examined as described above, the affected
resources can be identified, and Uptest jobs can be triggered on them.

```shell
# Different diff sources can be supported
uptest diff-test --source v1.2.0 --diff-source git --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test"
```

### Connection details tests

`Connection Details`, are one of the key points Crossplane provides when a
provider creates a managed resource. The resource can create resource-specific
details including usernames, passwords, or connection details such as an IP
address. Such details are vital for the user to access and use the provisioned
resource. Uptest does not perform any tests on `Connection Details` of such
resources today.

For example, when a `Cluster` is provisioned through `provider-gcp`, the
connection details of this cluster are stored in a secret. Whether the value in
this secret is properly populated or not is of the same importance as whether
the resource is Ready or not.

For this reason, a test step that checks the `Connection Details` can be added
for resources. These test steps can be manipulated with various hooks.
Basically, the CLI of the relevant provider can be used here. At this point, it
should be noted that this will be a custom step for frequently used resources
rather than a generic step.

The annotations in the example manifests manage the `Import` and `Update` steps.
It would be appropriate to consider `Connection Details` as a similar step and
manage it through annotations for the desired resources. As a default behavior,
the `Connection Details` step will not run. This step can be executed if the
annotation is set in the related example. The secret field values to be checked
in the related annotation can be specified. For example:

```yaml
apiVersion: rds.aws.upbound.io/v1beta1
kind: Cluster
metadata:
  name: example
  annotations:
    uptest.upbound.io/connection-details: "endpoint,master_username,port"
    meta.upbound.io/example-id: rds/v1beta1/cluster
spec:
  forProvider:
    region: us-west-1
    engine: aurora-postgresql
    masterUsername: cpadmin
    autoGeneratePassword: true
    masterPasswordSecretRef:
      name: sample-cluster-password
      namespace: upbound-system
      key: password
    skipFinalSnapshot: true
  writeConnectionSecretToRef:
    name: sample-rds-cluster-secret
    namespace: upbound-system
```

Related Issue: https://github.com/upbound/official-providers-ci/issues/82

### ProviderConfig Coverage

Uptest only uses the `Secret` source from the `ProviderConfig` in its tests.
However, Crossplane providers allow many different provider configuration
mechanisms (`IRSA`, `WebIdentitiy`, etc.). For this reason, changes made in
this context are tested manually and there is difficulty in preparing the
environments locally. Testing different `ProviderConfig` sources will
significantly increase provider test coverage. It will also improve the ability
to test changes locally.

By default, `Secret` source is still used, but a specific provider config
manifest can be applied to the cluster via a CLI flag:

```shell
uptest e2e --provider-config="examples/irsa-config.yaml" --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test"
```

### More Comprehensive Test Assertions

`Uptest` focuses on the status conditions of the Crossplane resources. For
example, during a test of MR, Uptest checks the `UpToDate` condition and does
not look at the fields of the created resources. Doing more comprehensive
assertions like comparing the values of the fields in the spec and status of MRs
and validating patch steps for Compositions will increase the test coverage.

Comparisons can be made here using Crossplane's `fieldpath` library. The set of
fields in `status.AtProvider` has, with recent changes, become a set that
includes those in `spec.ForProvider`. In this context, comparisons can be made
using a go tool written using the capabilities of the `fieldpath` library.

```go
// ...

pv, err := fieldpath.PaveObject(mg)
if err != nil {
    return nil, errors.Wrap(err, "cannot pave the managed resource")
}

specV, err := pv.GetValue("spec.forProvider")
if err != nil {
    return nil, errors.Wrap(err, "cannot get spec.forProvider value from paved object")
}
specM, ok := specV.(map[string]any)
if !ok {
    return nil, errors.Wrap(err, "spec.forProvider must be a map")
}

statusV, err := pv.GetValue("status.atProvider")
if err != nil {
return nil, errors.Wrap(err, "cannot get status.atProvider value from paved object")
}
statusM, ok := statusV.(map[string]any)
if !ok {
return nil, errors.Wrap(err, "status.atProvider must be a map")
}

for key, value := range specM {
    // Recursively compare the spec fields with status fields
	// ...
}

// ...
```

Related Issue: https://github.com/upbound/official-providers-ci/issues/175

### Mocking Providers

Uptest provisions physical resources while running tests on providers. In some
cases, users may want to run their tests on a mock system. Mocking providers is
not directly the subject of Uptest, but enabling Uptest to run against existing
mock infrastructures will be beneficial.

For example, [Localstack](https://github.com/localstack/localstack) is a cloud
service emulator that runs in a single container on your laptop or in your CI
environment. With LocalStack, you can run your AWS applications or Lambdas
entirely on your local machine without connecting to a remote cloud provider.

Currently, there is an ability to use LocalStack in `provider-aws`. If
`ProviderConfig` is configured properly, it will be possible to perform the
relevant tests in a mocked way. Therefore, increasing the `ProviderConfig`
coverage mentioned before and even allowing custom configurations is key to
unlocking this capability.

```shell
uptest e2e --provider-config="examples/localstack-config.yaml" --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test"
```

### Debugging Improvements

Debugging is of great importance for tests that fail. Uptest has added many
debugging stages with the latest developments. For example, printing resource
yaml outputs to the screen at regular intervals during testing is an example of
this. However, this also creates noise from time to time. It is important to
regulate the log frequency and review the logs collected after and during the
test. In this way, it is valuable to both easily understand the situation that
caused the perpetrator of the test and to provide the opportunity for rapid
intervention without waiting for the test to be completed. There are some open
issues about this debugging:

- Using `crossplane beta trace` instead of `kubectl` for collecting debugging
information:
https://github.com/upbound/official-providers-ci/issues/177
- Decreasing the log noise in Import step:
https://github.com/upbound/official-providers-ci/issues/168
- Exposing Kind cluster for faster development cycle:
https://github.com/upbound/official-providers-ci/issues/4

### Creating End User Documentation

Detailed documentation in which Uptest's use cases and instructions are
summarized is needed by end users. The main purpose of this type of document is
to explain how to use the tool.
