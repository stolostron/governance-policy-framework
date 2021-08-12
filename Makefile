# Copyright Contributors to the Open Cluster Management project

TRAVIS_BUILD ?= 1

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
deployOnHub ?= false

# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

# Handle KinD configuration
KIND_HUB_NAMESPACE ?= open-cluster-management
KIND_MANAGED_NAMESPACE ?= open-cluster-management-agent-addon
MANAGED_CLUSTER_NAME ?= cluster1
HUB_CLUSTER_NAME ?= hub

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
	ifeq ($(TRAVIS_BUILD),1)
		ifndef GITHUB_TOKEN
		-include $(shell curl -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
		else
		-include $(shell curl -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
		endif
	endif
else
-include vbh/.build-harness-vendorized
endif

default::
	@echo "Build Harness Bootstrapped"

############################################################
# e2e test section
############################################################
.PHONY: kind-bootstrap-cluster
kind-bootstrap-cluster: kind-create-cluster install-crds install-resources kind-deploy-policy-framework kind-deploy-policy-controllers

.PHONY: kind-bootstrap-cluster-dev
kind-bootstrap-cluster-dev: kind-create-cluster install-crds install-resources

.PHONY: kind-deploy-policy-controllers
kind-deploy-policy-controllers: kind-deploy-cert-policy-controller kind-deploy-olm kind-deploy-config-policy-controller kind-deploy-iam-policy-controller

kind-policy-framework-hub-setup:
	kubectl config use-context kind-$(HUB_CLUSTER_NAME)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

deploy-policy-framework-hub-crd-operator:
	kubectl create ns $(KIND_HUB_NAMESPACE) || true
	@echo installing Policy CRDs on hub
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies_crd.yaml
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_placementbindings_crd.yaml
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policyautomations_crd.yaml
	@echo installing policy-propagator on hub
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-propagator/main/deploy/operator.yaml -n $(KIND_HUB_NAMESPACE)

deploy-policy-framework-hub: kind-policy-framework-hub-setup deploy-policy-framework-hub-crd-operator

deploy-community-policy-framework-hub: deploy-policy-framework-hub-crd-operator

kind-policy-framework-managed-setup:
	kubectl config use-context kind-$(MANAGED_CLUSTER_NAME)
	kubectl create ns $(KIND_MANAGED_NAMESPACE) || true
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

deploy-policy-framework-managed-crd-operator:
	@echo installing Policy CRD on managed
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies_crd.yaml
	@echo installing policy-spec-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-spec-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-spec-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-spec-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"
	@echo installing policy-status-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-status-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-status-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-status-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"
	@echo installing policy-template-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/governance-policy-template-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-template-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-template-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"

deploy-policy-framework-managed: kind-policy-framework-managed-setup deploy-policy-framework-managed-crd-operator

deploy-community-policy-framework-managed: deploy-policy-framework-managed-crd-operator

kind-deploy-policy-framework:
	@echo installing policy-propagator on hub
	kubectl create ns $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f deploy/propagator -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating secrets on managed
	kubectl create ns $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	if [ "$(deployOnHub)" = "true" ]; then\
		echo skipping installing policy-spec-sync on managed;\
	else\
		echo installing policy-spec-sync on managed;\
		kubectl apply -f deploy/spec-sync -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi
	if [ "$(deployOnHub)" = "true" ]; then\
		echo installing policy-status-sync with ON_MULTICLUSTERHUB;\
		kubectl apply -k deploy/status-sync -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	else\
		echo installing policy-status-sync on managed;\
		kubectl apply -f deploy/status-sync/yamls -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi
	@echo installing policy-template-sync on managed
	kubectl apply -f deploy/template-sync -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-config-policy-controller:
	@echo installing config-policy-controller on managed
	kubectl apply -f deploy/config-policy-controller -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-cert-policy-controller:
	@echo installing cert-manager on managed
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.1/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	@echo installing cert-policy-controller on managed
	kubectl apply -f deploy/cert-policy-controller -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-iam-policy-controller:
	@echo installing iam-policy-controller on managed
	kubectl apply -f deploy/iam-policy-controller -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-olm:
	@echo installing OLM on managed
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/crds.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	sleep 5
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/olm.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-create-cluster:
	@echo "creating cluster hub"
	kind create cluster --name $(HUB_CLUSTER_NAME)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	# needed for mangaed -> hub communication
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal
	if [ "$(deployOnHub)" = "true" ]; then\
		echo import cluster hub as managed;\
		kind get kubeconfig --name $(HUB_CLUSTER_NAME) > $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	else\
		echo creating cluster managed;\
		kind create cluster --name $(MANAGED_CLUSTER_NAME);\
		kind get kubeconfig --name $(MANAGED_CLUSTER_NAME) > $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi

kind-delete-cluster:
	kind delete cluster --name $(HUB_CLUSTER_NAME)
	kind delete cluster --name $(MANAGED_CLUSTER_NAME)

install-crds:
	@echo installing crds on hub
	kubectl apply -f deploy/crds/apps.open-cluster-management.io_placementrules_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f deploy/crds/cluster.open-cluster-management.io_managedclusters.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_placementbindings_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policyautomations_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo installing crds on managed
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

install-resources:
	@echo creating user namespace on hub
	kubectl create ns policy-test --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating cluster namespace on hub 
	kubectl create ns managed --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f test/resources/managed-cluster.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	
e2e-test:
	ginkgo -v --slowSpecThreshold=10 test/e2e

policy-collection-test:
	ginkgo -v --slowSpecThreshold=10 test/policy-collection

travis-slack-reporter:
	docker run --volume $(PWD)/results:/opt/app-root/src/grc-ui/test-output/e2e \
		--volume $(PWD)/results-cypress:/opt/app-root/src/grc-ui/test-output/cypress \
		--env SLACK_TOKEN=$(SLACK_TOKEN) \
		--env TRAVIS_REPO_SLUG=$(TRAVIS_REPO_SLUG) \
  	  	--env TRAVIS_PULL_REQUEST=$(TRAVIS_PULL_REQUEST) \
   		--env TRAVIS_BRANCH=$(TRAVIS_BRANCH) \
		--env TRAVIS_BUILD_WEB_URL=$(TRAVIS_BUILD_WEB_URL) \
		quay.io/open-cluster-management/grc-ui-tests:latest-2.3 node ./tests/utils/slack-reporter.js
