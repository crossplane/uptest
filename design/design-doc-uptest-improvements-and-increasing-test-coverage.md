# Uptest Improvements and Increasing Test Coverage

* Owner: Sergen Yalcin (@sergenyalcin)
* Reviewers: Uptest Maintainers
* Status: Draft

## Background

Uptest is a tool for testing and validating Crossplane providers. This tool is
utilized to test the behavior of Crossplane controllers, resource management
operations, and other Crossplane components. Uptest can simulate the creation,
update, import, deletion, and other operations of Crossplane resources.

The primary goal of Uptest is to facilitate the testing process of Crossplane.
It integrates seamlessly with Crossplane and provides a testing infrastructure
that enables users to create and run test scenarios for validating the
reliability, functionality, and performance of Crossplane.

Uptest first came to the forefront as a tool designed to run e2e tests on GitHub
environment. Then, various improvements were made in order to run easily in
local environments. However, the tool was not considered a standalone project
in the first place. Uptest, which was designed to run tests in more controlled
environments (for example, a Kind cluster that was created from scratch) rather
than running tests on any arbitrary Kubernetes cluster, has progressed to become
a standalone project day by day. The main reason for this is that Uptest is now
also on the agenda in the context of user integrations.

As a result, it is necessary to work on various enhancements for Uptest to
continue its development as a more powerful and independent tool. In this
document, evaluations will be made on the present and future of Uptest and the
aspects to be developed.

In its current form, Uptest offers its users a very large test framework.
Thanks to this framework, we have the ability to test and validate many
different MRs simultaneously and seamlessly.

For Crossplane providers, it is not enough for us to see `Ready: True` in the
status of an MR. `Late-initialization` that occurs after the resource is `Ready`
or the resource is not stable and is subjected to a continuous update loop are
actually situations that do not affect the Ready state of the resource but
affect its lifecycle.

Therefore, to overcome some of the problems hidden behind this `Ready`
condition, we use the `UpToDate` condition, which is only used in tests and
activated by a specific annotation. This condition makes sure that after the
resource is `Ready`, the `Late-Initialization` step is done and the resource is
not stuck in any update loop.

For some MRs, it is vital that their names (or some identifier field values) are
randomized. If the name of a resource is not unique, this will cause a conflict
and the resource cannot be created or an existing resource can be updated. There
are two main issues for this conflict to occur. One is that some resources
expect a universally unique name to be identified. In other words, we are
talking about a name that will be unique on all cloud providers regardless of
account or organization. The other issue is the need for uniqueness on a smaller
scale, that is, within the account or organization used. At the end of the day,
there is a need to generate random strings for some fields of some resources.
Currently, there is only support for this in one format and only for some
fields.

Test cases that initially included only the `Apply` and `Delete` steps now also
include the `Update` and `Import` steps. These new steps are very important in
terms of increasing the test coverage of the lifecycle of resources. The
`Import` step is also an important coverage point in terms of testing whether
the external name configuration of the resources is done correctly. The
validations performed during the transition to the new provider architecture,
especially the new `Import` step played a critical role in the early detection
of many bugs and problems.

## Goals

- Increasing Uptest's test coverage by adding different test scenarios.
- Making Uptest capable of running tests on arbitrary clusters.
- Improvement of Uptest Documents: User Guide, Technical Docs, etc.
- In cases where Uptest fails, debugging/logging should be improved to
understand what the problem is.
- Increasing the client's configuration ability by making some features of
Uptest parametric at the CLI level.

## Proposal

In this document, it is suggested to achieve the above goals by making various
improvements on the existing Uptest tool. These improvements will be analyzed
item by item starting from the next section. The main goal is to make Uptest a
more stable and inclusive standalone tool at the end of the day.

### Increasing Coverage

Increasing the capacities of the tool prepared so far will help us in the
process of evolving the tool into a standalone tool both in test coverage and
in the long run.

#### Release Tests for Providers

It is important to use various automation in the tests performed before
releases. Today there are no customized tests for this. Usually, the usual tests
from Uptest are used for a few random resources. In this context, various
end-to-end tests prepared to work before the releases will increase the level of
confidence in the releases.

#### Upgrade Tests

When Uptest first appeared, it was mainly concerned with testing whether a
resource was working properly. Today, it has become a more end-to-end testing
infrastructure. Especially in periods when many validations are needed, such as
the transition to a new architecture, before the release, everyone knows how
urgent such a need is. In this context, it is valuable that Uptest also handles
some end-to-end cases.

In fact, Uptest tests some basic general steps in each case, such as setting up
Crossplane and provider and testing ProviderConfig (Secret source). However, the
`Upgrade` test, which is one of the main paths, can also be considered as an
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

#### Diff Tests
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

### Connection Details Included Tests

`Connection Details`, one of the key points that Crossplane provides is that
when a provider creates a managed resource, the resource can create
resource-specific details. These details can include usernames, passwords, or
connection details such as IP address. Such details are vital for the user to
access and use the provisioned resource. Uptest does not perform any tests on
`Connection Details` of such resources today.

For example, when a `Cluster` is provisioned on a `provider-gcp`, the
connection details of this cluster are in a secret. Whether the value in this
secret is properly populated or not is actually of the same importance as
whether the resource is Ready or not.

For this reason, a test step that checks the `Connection Details` can be added
for some resources. These test steps can be manipulated with various hooks.
Basically, the CLI of the relevant provider can be used here. At this point,
it should be noted that this will be a custom step for frequently used resources
rather than a generic step, but it can be a good point in terms of increasing
test coverage.

The `Import` and `Update` steps are managed through the annotations in the
example manifests. It would be appropriate to consider Connection Details as a
similar step and manage it through annotations for the resources that are
desired to run. As a default behavior, the `Connection-Details` step will not
run. This step can be executed if the annotation is set in the related example.
The secret field values to be checked in the related annotation can be
specified. For example:

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

### Real-Life Scenarios / Composition Testing

Although Uptest has the ability to perform Claim tests, MRs are tested on a
provider basis. However, Crossplane users often use higher-level abstractions
such as `Compositions` and `Claims`. At the end of the day, these can be reduced
to MRs, and it can even be said that providers are only responsible for the
lifecycle of MRs. However, some situations are only observed when such
higher-level abstractions are used. Therefore, it may make sense to use
real-life scenarios in order to catch some bugs and problems in advance and
increase test convergence.

Here, imitating various user environments or creating configurations similar
to reference platforms can be a start.

### ProviderConfig Coverage

As it is known, Uptest uses only `Secret` source `ProviderConfig` in its tests.
However, crossplane providers allow many different provider configuration
mechanisms (`IRSA`, `WebIdentitiy`, etc.). For this reason, especially the
changes made in this context are tested manually and there is difficulty in
preparing the environments locally. At this point, testing different
`ProviderConfig` sources will significantly increase provider test coverage. It
will also make things easier for this and similar cases that are difficult to
test locally.

By default, `Secret` source is still used, but a specific provider config
manifest can be applied to the cluster via a CLI flag:

```shell
uptest e2e --provider-config="examples/irsa-config.yaml" --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test"
```

### More Comprehensive Test Assertions

`Uptest` focuses on the status conditions of the crossplane resources. For
example, during a test of MR, the `Uptest` checks the `UpToDate` condition and
does not look to the fields of the created resources. Doing more comprehensive
assertions like comparing the values of the fields in spec and status of MRs and
validating patch steps for Compositions will increase the test coverage.

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

`Uptest` provisions physical resources while running tests on providers. In some
cases, users may want to run their tests on a mock system. Mocking providers is
not directly the subject of `Uptest`, but enabling some existing mock
infrastructures is the point of `Uptest`.

For example, [localstack](https://github.com/localstack/localstack) is a cloud
service emulator that runs in a single container on your laptop or in your CI
environment. With LocalStack, you can run your AWS applications or Lambdas
entirely on your local machine without connecting to a remote cloud provider.

Currently, there is an infrastructure for localstack in `provider-aws`. If
`ProviderConfig` is configured properly, it will be possible to perform the
relevant tests in a mock way. Therefore, increasing the `ProviderConfig`
coverage mentioned before and even allowing custom configurations is at a
critical point in this context.

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

A detailed user document in which Uptest's use cases are introduced and usages
are summarized will be found very useful by users. The main purpose of this type
of document is not to talk about detailed technical discussions but to explain
how to use the tool. A guide prepared in this context will also be very useful
for teams that are in direct communication with users.

## Conclusion

The Uptest tool has reached this form as a result of all the stages it has gone
through. It will become stronger with the items mentioned above and become a
more stable tool. This is essential to increase both test coverage and quality.
It is also valuable in terms of shortening manual processes and reducing manual
time spent on tests in the long term.

