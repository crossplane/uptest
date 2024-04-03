# Considerations for Changing Test Framework of Uptest

* Owner: Sergen Yalcin (@sergenyalcin)
* Reviewers: Uptest Maintainers
* Status: Draft

## Background

Uptest is a tool for testing and validating Crossplane providers. This tool is
utilized to test the behavior of Crossplane controllers, resource management
operations, and other Crossplane components. Uptest can simulate the creation,
update, import, deletion, and other operations of Crossplane resources.

When `Uptest` was first written, it used [kuttl](https://github.com/kudobuilder/kuttl)
as the underlying test framework in order to have good declarative testing
capabilities. Over time, it was realized that there are better and more
compatible alternatives and a perception that `kuttl` isn't being actively
maintained, so it was decided to evaluate an alternative underlying framework.

[kuttl](https://github.com/kudobuilder/kuttl) provides a declarative approach to
test Kubernetes Operators. `kuttl` is designed for testing operators, however it
can declaratively test any kubernetes objects.

When starting the `Uptest` effort, we considered a few different alternatives
and `kuttl`'s capabilities were appropriate for our assertion aims even though
it missed some points. Today, we consider changing the underlying test framework
tool because of the perception of `kuttl` not being actively maintained and
other frameworks offering superior capabilities.

## Goals

Decide on a more comprehensive underlying test framework to meet the current
and [future requirements](https://github.com/crossplane/uptest/pull/10/files) of
Uptest.

## Proposal - Switching to `chainsaw`

[chainsaw](https://github.com/kyverno/chainsaw) provides a declarative approach
to test Kubernetes operators and controllers. While Chainsaw is designed for
testing operators and controllers, it can declaratively test any Kubernetes
objects. Chainsaw is an open-source tool that was initially developed for
defining and running Kyverno end-to-end tests. The tool has Apache-2.0 license.

In addition to providing similar functionality provided by `kuttl`,  it also
offers better logs, config maps assertions,
[assertions trees](https://kyverno.io/blog/2023/12/13/kyverno-chainsaw-exploring-the-power-of-assertion-trees/)
and many more things. The fact that it is well-maintained, and has the
capability for migration from `kuttl` makes it an attractive option.

`chainsaw` shares similar concepts with `kuttl`. In this way, we do not have to
make major changes to the templates.

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - try:
    # ...
    - apply:
        file: my-configmap.yaml
    # ...
```

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - try:
    # ...
    - command:
        entrypoint: echo
        args:
        - hello chainsaw
    # ...
```

Also provides logical assertion statements:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - try:
    # ...
    - assert:
        resource:
          apiVersion: v1
          kind: Deployment
          metadata:
            name: foo
          spec:
            (replicas > 3): true
    # ...
```

Resource Template support is another important requirement for Uptest:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: template
spec:
  template: true
  steps:
  - assert:
      resource:
        # apiVersion, kind, name, namespace and labels are considered for templating
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ($namespace)
        # other fields are not (they are part of the assertion tree)
        data:
          foo: ($namespace)
```

Related Issue: https://github.com/upbound/official-providers-ci/issues/179

In short, `chainsaw` is a more capable and well-maintained framework than
`kuttl` and switching to it will better suit Uptest's future requirements.

## Alternative Considered

### Using the `crossplane-e2e-framework`

[crossplane-e2e-framework](https://github.com/crossplane/crossplane/tree/master/test/e2e)
is a [k8s-e2e-framework](https://pkg.go.dev/sigs.k8s.io/e2e-framework)-based
test framework that provides a baseline for simulating the real-world use to
exercise all of `Crossplane`'s functionality.

`e2e-framework` is a tool that allows tests to be written in Go. Additionally,
one of its advantages is that it works with familiar conventions in the
environment we use. On the other hand, these types of utilities can be used when
writing tests, thanks to their functions specific to the crossplane ecosystem.
However, this will mean changing the entire `Uptest` code-base currently used.
In this context, it should be taken into consideration that such a change would
be quite large.

As mentioned in the [discussion](https://github.com/crossplane-contrib/provider-argocd/pull/89#issuecomment-2016655783),
to clarify the use cases of `Uptest` and `e2e-framework`, it might be good to
strengthen the documentation of the tools. One could also write a guideline that
directly compares the two tools and discusses their capabilities and use cases.
This way, the end user can more easily decide when to use `Uptest` and when to
use `e2e-framework`.

### Writing a Underlying Test Framework From Scratch

Writing such a tool where all the steps in the test pipeline are modular, using
the existing Go libraries, has advantages and disadvantages. One of the most
important advantages of writing such a tool is that it can be developed
completely according to our environment and requirements, and since it is
written in accordance with our test scenarios, it can be easily integrated into
Github Actions and other pipeline elements (example generation). With this tool,
which can run with different configurations for different test scenarios, it
will be possible to handle cases that we can predict for now(and will come to
us in the future).

However, the time it takes to write such a tool is also important. Maybe it
won't take long to reveal the general outline of the tool, but as I mentioned
above, it may take some time for it to be configurable for different scenarios.
In this context, if this option is selected, it would be appropriate to first
create the tool in general outline and then integrate it into various scenarios
(iteratively) to speed up the process.