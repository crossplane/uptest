# Integrating Uptest for End to End Testing

In this tutorial, we will integrate [uptest](https://github.com/crossplane/uptest) to a Github repository to automate end to end
testing managed resources. While we will use a `Provider` repository as an example, the process will be almost identical
for a `Configuration` repository.

Starting with a provider repository with no end to end testing capability, we will end up having:
- A make target to locally test examples end to end
- A GitHub action triggered for PRs whenever a comment as `/test-examples=<examples-path>` is left

## Setting up the Make targets

1. Go to the [demo repository](https://github.com/upbound/demo-uptest-integration) which contains a GitHub provider
    generated using upjet and hit the `Use this template` button to initialize your demo repository under your own
	GitHub organization.
1. Clone your demo repository on your local and `cd` into the root directory.
2. Initialize build submodule with

	```bash
	make submodules
	```

4. First we will add a simple setup script that will deploy a secret and a provider config for our provider. 

	```bash
	mkdir -p cluster/test
	touch cluster/test/setup.sh
	chmod +x cluster/test/setup.sh

	cat <<EOF0 > cluster/test/setup.sh
	#!/usr/bin/env bash
	set -aeuo pipefail

	echo "Running setup.sh"
	echo "Creating cloud credential secret..."
	\${KUBECTL} -n upbound-system create secret generic provider-secret --from-literal=credentials="{\"token\":\"\${UPTEST_CLOUD_CREDENTIALS}\"}" \
	--dry-run=client -o yaml | \${KUBECTL} apply -f -

	echo "Waiting until provider is healthy..."
	\${KUBECTL} wait provider.pkg --all --for condition=Healthy --timeout 5m

	echo "Waiting for all pods to come online..."
	\${KUBECTL} -n upbound-system wait --for=condition=Available deployment --all --timeout=5m

	echo "Creating a default provider config..."
	cat <<EOF | \${KUBECTL} apply -f -
	apiVersion: github.upbound.io/v1beta1
	kind: ProviderConfig
	metadata:
	  name: default
	spec:
	  credentials:
	    source: Secret
	    secretRef:
	      name: provider-secret
	      namespace: upbound-system
	      key: credentials
	EOF	  
	EOF0
	```

5. Now, please add the following lines to your Makefile which which will add `uptest` and `e2e` targets:

	```Makefile
	# ====================================================================================
	# End to End Testing
	CROSSPLANE_NAMESPACE = upbound-system
	-include build/makelib/local.xpkg.mk
	-include build/makelib/controlplane.mk

	uptest: $(UPTEST) $(KUBECTL) $(KUTTL)
		@$(INFO) running automated tests
		@KUBECTL=$(KUBECTL) KUTTL=$(KUTTL) $(UPTEST) e2e "${UPTEST_EXAMPLE_LIST}" --setup-script=cluster/test/setup.sh || $(FAIL)
		@$(OK) running automated tests

	e2e: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME) uptest
	```

	> Note: If you're following this tutorial for a `Configuration` repository, you will need to add
	> `local.xpkg.deploy.configuration.$(PROJECT_NAME)` instead of `local.xpkg.deploy.provider.$(PROJECT_NAME)` to the
	> `e2e` target.

6. Commit the changes we did so far.

	```bash
	git add Makefile cluster/test/setup.sh
	git commit -m "Add uptest and e2e targets"
	```

## Testing Locally

1. Generate a [Personal Access Token](https://github.com/settings/tokens/new) for your Github account with
    `repo/public_repo` and `delete_repo` scopes.
2. Run the following:

	```bash
	export UPTEST_CLOUD_CREDENTIALS=<your-token-here>
	UPTEST_EXAMPLE_LIST=examples/repository/repository.yaml make e2e
	```

You should see a `PASS` at the end of logs indicating everything worked fine.

## Adding the GitHub workflow

Now we have things working locally, let's add a GitHub workflow to automate end to end testing with CI.

1. Run the following to add the GitHub workflow definition which will be triggered for `issue_comment` events and will call
uptests reusable workflow:

	```bash
	cat <<EOF > .github/workflows/e2e.yaml
	name: End to End Testing

	on:
	  issue_comment:
	    types: [created]

	jobs:
	  e2e:
	    uses: upbound/uptest/.github/workflows/pr-comment-trigger.yml@main
	    secrets:
	      UPTEST_CLOUD_CREDENTIALS: \${{ secrets.UPTEST_CLOUD_CREDENTIALS }}
	      UPTEST_DATASOURCE: \${{ secrets.UPTEST_DATASOURCE }}
	EOF
	```
	
   > See [Injecting Dynamic Values (and Datasource)](../README.md#injecting-dynamic-values-and-datasource) for more
   > details on `UPTEST_DATASOURCE` secret.

1. Commit and push to the `main` branch of the repository.

	```
	git add .github/workflows/e2e.yaml
	git commit -s -m "Add e2e workflow"
	git push origin main
	```

3. Lastly, we need to add a Repository Secret with our GitHub token.
	1. Go to your repository Settings in GitHub UI.
	2. On the left side, select `Secrets` -> `Actions` under `Security` section.
	3. Hit `New repository secret`
	4. Enter `UPTEST_CLOUD_CREDENTIALS` as `Name` and your GitHub Token as `Secret` and hit `Add secret`

## Testing via CI

We are now ready to test our changes end to end via GitHub Actions. We will try that out by opening a test PR.

1. Go to the `examples/repository/repository.yaml` file and make some wording changes in `description` field, e.g. 
   Add `CI -` as a prefix.
2. Create a PR with that change.
3. Add the following comment on the PR:

	```
	/test-examples="examples/repository/repository.yaml"
	```

4. Check the Actions and follow how end to end testing goes.
