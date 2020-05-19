TRAVIS_BUILD ?= 1

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))

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
kind-bootstrap-cluster: kind-create-cluster install-crds install-resources kind-deploy-controller

.PHONY: kind-bootstrap-cluster-dev
kind-bootstrap-cluster-dev: kind-create-cluster install-crds install-resources

check-env:
ifndef DOCKER_USER
	$(error DOCKER_USER is undefined)
endif
ifndef DOCKER_PASS
	$(error DOCKER_PASS is undefined)
endif

kind-deploy-controller: check-env
	@echo installing policy-propagator on hub
	kubectl create ns governance --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl create secret -n governance docker-registry multiclusterhub-operator-pull-secret --docker-server=quay.io --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS} --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/propagator -n governance --kubeconfig=$(PWD)/kubeconfig_hub
	@echo creating secrets on managed
	kubectl create ns multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed
	kubectl create secret -n multicluster-endpoint docker-registry multiclusterhub-operator-pull-secret --docker-server=quay.io --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS} --kubeconfig=$(PWD)/kubeconfig_managed
	kubectl create secret -n multicluster-endpoint generic endpoint-connmgr-hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_hub_internal --kubeconfig=$(PWD)/kubeconfig_managed
	@echo installing policy-spec-sync on managed
	kubectl apply -f deploy/spec-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed
	@echo installing policy-status-sync on managed
	kubectl apply -f deploy/status-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed
	@echo installing policy-template-sync on managed
	kubectl apply -f deploy/template-sync -n multicluster-endpoint --kubeconfig=$(PWD)/kubeconfig_managed

kind-create-cluster:
	@echo "creating cluster"
	kind create cluster --name test-hub
	kind get kubeconfig --name test-hub > $(PWD)/kubeconfig_hub
	# needed for mangaed -> hub communication
	kind get kubeconfig --name test-hub --internal > $(PWD)/kubeconfig_hub_internal
	kind create cluster --name test-managed
	kind get kubeconfig --name test-managed > $(PWD)/kubeconfig_managed

kind-delete-cluster:
	kind delete cluster --name test-hub
	kind delete cluster --name test-managed

install-crds:
	@echo installing crds on hub
	kubectl apply -f deploy/crds/apps.open-cluster-management.io_placementrules_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/policies.open-cluster-management.io_placementbindings_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/policies.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f deploy/crds/cluster-registry-crd.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	@echo installing crds on managed
	kubectl apply -f deploy/crds/policies.open-cluster-management.io_policies_crd.yaml --kubeconfig=$(PWD)/kubeconfig_managed

install-resources:
	@echo creating user namespace on hub
	kubectl create ns policy-test --kubeconfig=$(PWD)/kubeconfig_hub
	@echo creating cluster namespace on hub 
	kubectl create ns managed --kubeconfig=$(PWD)/kubeconfig_hub
	kubectl apply -f test/resources/managed-cluster.yaml --kubeconfig=$(PWD)/kubeconfig_hub
	
e2e-test:
	ginkgo -v --slowSpecThreshold=10 test/e2e