# Copyright Contributors to the Open Cluster Management project

TRAVIS_BUILD ?= 1

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
deployOnHub ?= false

# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
	ifeq ($(TRAVIS_BUILD),1)
	-include $(shell curl -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
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

check-env:
ifndef DOCKER_USER
	$(error DOCKER_USER is undefined)
endif
ifndef DOCKER_PASS
	$(error DOCKER_PASS is undefined)
endif

kind-deploy-policy-framework: check-env
	@echo installing policy-propagator on hub
	kubectl create ns governance --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl create secret -n governance docker-registry multiclusterhub-operator-pull-secret --docker-server=quay.io --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS} --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/propagator -n governance --kubeconfig=$(PWD)/kubeconfig_hub
	@echo creating secrets on managed
	kubectl create ns multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed
	kubectl create secret -n multicluster-endpoint docker-registry multiclusterhub-operator-pull-secret --docker-server=quay.io --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS} --kubeconfig=$(PWD)/kubeconfig_managed
	kubectl create secret -n multicluster-endpoint generic endpoint-connmgr-hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_hub_internal --kubeconfig=$(PWD)/kubeconfig_managed
	if [ "$(deployOnHub)" = "true" ]; then\
		echo skipping installing policy-spec-sync on managed;\
	else\
		echo installing policy-spec-sync on managed;\
		kubectl apply -f deploy/spec-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed;\
	fi
	if [ "$(deployOnHub)" = "true" ]; then\
		echo installing policy-status-sync with ON_MULTICLUSTERHUB;\
		kubectl apply -k deploy/status-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed;\
	else\
		echo installing policy-status-sync on managed;\
		kubectl apply -f deploy/status-sync/yamls -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed;\
	fi
	@echo installing policy-template-sync on managed
	kubectl apply -f deploy/template-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed

kind-deploy-config-policy-controller: check-env
	@echo installing config-policy-controller on managed
	kubectl apply -f deploy/config-policy-controller -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed

kind-deploy-cert-policy-controller: check-env
	@echo installing cert-manager on managed
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.1/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_managed
	@echo installing cert-policy-controller on managed
	kubectl apply -f deploy/cert-policy-controller -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed

kind-deploy-iam-policy-controller: check-env
	@echo installing iam-policy-controller on managed
	kubectl apply -f deploy/iam-policy-controller -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed

kind-deploy-olm: check-env
	@echo installing OLM on managed
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/crds.yaml --kubeconfig=$(PWD)/kubeconfig_managed
	sleep 5
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/olm.yaml --kubeconfig=$(PWD)/kubeconfig_managed
	# @echo installing gatekeeper on managed
	# kubectl apply -f deploy/gatekeeper --kubeconfig=$(PWD)/kubeconfig_managed

kind-create-cluster:
	@echo "creating cluster hub"
	kind create cluster --name test-hub
	kind get kubeconfig --name test-hub > $(PWD)/kubeconfig_hub
	# needed for mangaed -> hub communication
	kind get kubeconfig --name test-hub --internal > $(PWD)/kubeconfig_hub_internal
	if [ "$(deployOnHub)" = "true" ]; then\
		echo import cluster hub as managed;\
		kind get kubeconfig --name test-hub > $(PWD)/kubeconfig_managed;\
	else\
		echo creating cluster managed;\
		kind create cluster --name test-managed;\
		kind get kubeconfig --name test-managed > $(PWD)/kubeconfig_managed;\
	fi

kind-delete-cluster:
	kind delete cluster --name test-hub
	kind delete cluster --name test-managed

install-crds:
	@echo installing crds on hub
	kubectl apply -f deploy/crds/apps.open-cluster-management.io_placementrules_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_placementbindings_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policyautomations_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/cluster.open-cluster-management.io_managedclusters.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	@echo installing crds on managed
	kubectl apply -f deploy/crds/policy.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_managed

install-resources:
	@echo creating user namespace on hub
	kubectl create ns policy-test --kubeconfig=$(PWD)/kubeconfig_hub
	@echo creating cluster namespace on hub 
	kubectl create ns managed --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f test/resources/managed-cluster.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	
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
		quay.io/open-cluster-management/grc-ui-tests:latest-dev node ./tests/utils/slack-reporter.js
