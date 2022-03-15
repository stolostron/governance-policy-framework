# Copyright Contributors to the Open Cluster Management project

TRAVIS_BUILD ?= 1

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
deployOnHub ?= false

# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
GOARCH = $(shell go env GOARCH)
GOOS = $(shell go env GOOS)
export PATH=$(shell echo $$PATH):$(PWD)/bin

# Handle KinD configuration
KIND_HUB_NAMESPACE ?= open-cluster-management
KIND_MANAGED_NAMESPACE ?= open-cluster-management-agent-addon
MANAGED_CLUSTER_NAME ?= managed
HUB_CLUSTER_NAME ?= hub

# Fetch Ginkgo/Gomega versions from go.mod
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' go.mod)
GOMEGA_VERSION := $(shell awk '/github.com\/onsi\/gomega/ {print $$2}' go.mod)

# Fetch OLM version
OLM_VERSION ?= $(shell curl -s https://api.github.com/repos/operator-framework/operator-lifecycle-manager/releases/latest | jq -r '.tag_name')

# Debugging configuration
KIND_COMPONENTS := config-policy-controller cert-policy-controller iam-policy-controller governance-policy-spec-sync governance-policy-status-sync governance-policy-template-sync
KIND_COMPONENT_SELECTOR := name
ACM_COMPONENTS := cert-policy-controller klusterlet-addon-iampolicyctrl policy-config-policy policy-framework
ACM_COMPONENT_SELECTOR := app
DEBUG_DIR ?= test-output/debug

# Test configuration
TEST_FILE ?=
TEST_ARGS ?=

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
	ifeq ($(TRAVIS_BUILD),1)
		ifndef GITHUB_TOKEN
		-include $(shell curl -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/stolostron/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
		else
		-include $(shell curl -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/stolostron/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
		endif
	endif
else
-include vbh/.build-harness-vendorized
endif

default::
	@echo "Build Harness Bootstrapped"

############################################################
# format section
############################################################

fmt-dependencies:
	$(call go-get-tool,$(PWD)/bin/gci,github.com/daixiang0/gci@v0.2.9)
	$(call go-get-tool,$(PWD)/bin/gofumpt,mvdan.cc/gofumpt@v0.2.0)

# All available format: format-go format-protos format-python
# Default value will run all formats, override these make target with your requirements:
#    eg: fmt: format-go format-protos
fmt: fmt-dependencies
	find . -not \( -path "./.go" -prune \) -name "*.go" | xargs gofmt -s -w
	find . -not \( -path "./.go" -prune \) -name "*.go" | xargs gofumpt -l -w
	find . -not \( -path "./.go" -prune \) -name "*.go" | xargs gci -w -local "$(shell cat go.mod | head -1 | cut -d " " -f 2)"

############################################################
# e2e test section
############################################################
.PHONY: kind-bootstrap-cluster
kind-bootstrap-cluster: kind-create-cluster install-crds install-resources kind-deploy-policy-framework kind-deploy-policy-controllers

.PHONY: kind-bootstrap-cluster-dev
kind-bootstrap-cluster-dev: kind-create-cluster install-crds install-resources

.PHONY: kind-deploy-policy-controllers
kind-deploy-policy-controllers: kind-deploy-cert-policy-controller kind-deploy-config-policy-controller kind-deploy-iam-policy-controller kind-deploy-olm

kind-policy-framework-hub-setup:
	kubectl config use-context kind-$(HUB_CLUSTER_NAME)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

.PHONY: kustomize
KUSTOMIZE = $(PWD)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

deploy-policy-framework-hub-crd-operator:
	kubectl create ns $(KIND_HUB_NAMESPACE) || true
	@echo installing Policy CRDs on hub
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies.yaml
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_placementbindings.yaml
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policyautomations.yaml
	@echo installing policy-propagator on hub
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/operator.yaml -n $(KIND_HUB_NAMESPACE)

deploy-policy-framework-hub: kind-policy-framework-hub-setup deploy-policy-framework-hub-crd-operator

deploy-community-policy-framework-hub: deploy-policy-framework-hub-crd-operator

kind-policy-framework-managed-setup:
	kubectl config use-context kind-$(MANAGED_CLUSTER_NAME)
	kubectl create ns $(KIND_MANAGED_NAMESPACE) || true
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

deploy-policy-framework-managed-crd-operator:
	@echo installing Policy CRD on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies.yaml
	@echo installing policy-spec-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-spec-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-spec-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-spec-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"
	@echo installing policy-status-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-status-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-status-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-status-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"
	@echo installing policy-template-sync on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-template-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-template-sync -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-template-sync\",\"env\":[{\"name\":\"WATCH_NAMESPACE\",\"value\":\"$(MANAGED_CLUSTER_NAME)\"}]}]}}}}"

deploy-policy-framework-managed: kind-policy-framework-managed-setup deploy-policy-framework-managed-crd-operator

deploy-community-policy-framework-managed: deploy-policy-framework-managed-crd-operator

kind-deploy-policy-framework: kustomize
	@echo installing policy-propagator on hub
	kubectl create ns $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/operator.yaml -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating secrets on managed
	kubectl create ns $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	@if [ "$(deployOnHub)" = "true" ]; then\
		echo skipping installing policy-spec-sync on managed;\
	else\
		echo installing policy-spec-sync on managed;\
		kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-spec-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi
	@if [ "$(deployOnHub)" = "true" ]; then\
		echo installing policy-status-sync with ON_MULTICLUSTERHUB;\
		kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-status-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
		kubectl set env deployment/governance-policy-status-sync ON_MULTICLUSTERHUB=true -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	else\
		echo installing policy-status-sync on managed;\
		kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-status-sync/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi
	@echo installing policy-template-sync on managed
	kustomize build deploy/template-sync | kubectl apply -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) -f -

kind-deploy-config-policy-controller:
	@echo installing config-policy-controller on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/config-policy-controller/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/config-policy-controller/main/deploy/crds/policy.open-cluster-management.io_configurationpolicies.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-cert-policy-controller:
	@echo installing cert-manager on managed
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.5.5/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	@echo installing cert-policy-controller on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/cert-policy-controller/main/deploy/crds/policy.open-cluster-management.io_certificatepolicies.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/cert-policy-controller/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl patch deployment cert-policy-controller \
		-n $(KIND_MANAGED_NAMESPACE) -p '{"spec": {"template": {"spec": {"containers": [{"name":"cert-policy-controller", "args": ["--enable-lease=true", "--hubconfig-secret-name=hub-kubeconfig"]}]}}}}' \
		--kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-iam-policy-controller:
	@echo installing iam-policy-controller on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/iam-policy-controller/main/deploy/crds/policy.open-cluster-management.io_iampolicies.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/iam-policy-controller/main/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl patch deployment iam-policy-controller \
		-n $(KIND_MANAGED_NAMESPACE) -p '{"spec": {"template": {"spec": {"containers": [{"name":"iam-policy-controller", "args": ["--enable-lease=true", "--hubconfig-secret-name=hub-kubeconfig"]}]}}}}' \
		--kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

kind-deploy-olm:
	@echo installing OLM on managed
	export KUBECONFIG=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/install.sh -o install.sh
	chmod +x install.sh
	./install.sh $(OLM_VERSION)

kind-create-cluster:
	@echo "creating cluster hub"
	kind create cluster --name $(HUB_CLUSTER_NAME)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	# needed for managed -> hub communication
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal
	@if [ "$(deployOnHub)" = "true" ]; then\
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
	rm $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) || true
	rm $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal || true
	rm $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) || true

install-crds:
	@echo installing crds on hub
	kubectl apply -f https://raw.githubusercontent.com/stolostron/multicloud-operators-placementrule/main/deploy/crds/apps.open-cluster-management.io_placementrules_crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/api/main/cluster/v1/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/main/cluster/v1beta1/0000_02_clusters.open-cluster-management.io_placements.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/main/cluster/v1beta1/0000_03_clusters.open-cluster-management.io_placementdecisions.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_placementbindings.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policyautomations.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policysets.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo installing crds on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-propagator/main/deploy/crds/policy.open-cluster-management.io_policies.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

install-resources:
	@echo creating user namespace on hub
	kubectl create ns policy-test --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating cluster namespace on hub 
	kubectl create ns managed --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f test/resources/managed-cluster.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)

e2e-dependencies:
	go get github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
	go get github.com/onsi/gomega/...@$(GOMEGA_VERSION)

e2e-test:
	@if [ -z "$(TEST_FILE)" ]; then\
		$(GOPATH)/bin/ginkgo -v --no-color $(TEST_ARGS) --fail-fast test/e2e;\
	else\
		$(GOPATH)/bin/ginkgo -v --no-color $(TEST_ARGS) --fail-fast --focus-file=$(TEST_FILE) test/e2e;\
	fi

e2e-debug: e2e-debug-hub e2e-debug-managed

e2e-debug-hub:
	mkdir -p $(DEBUG_DIR)
	kubectl get namespaces --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_namespaces.log
	kubectl get all -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_all_$(KIND_HUB_NAMESPACE).log
	kubectl get all -n $(MANAGED_CLUSTER_NAME) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_all_$(MANAGED_CLUSTER_NAME).log
	kubectl get leases -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_leases_$(KIND_HUB_NAMESPACE).log
	kubectl describe pods -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_describe_pods_$(KIND_HUB_NAMESPACE).log
	for POD in $$(kubectl get pods -n $(KIND_HUB_NAMESPACE) -l name=governance-policy-propagator -o name --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)); do \
		PODNAME=$${POD##"pod/"}; \
	  	kubectl logs $${PODNAME} -c governance-policy-propagator -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_logs_$${PODNAME}.log; \
	done

e2e-debug-managed:
	mkdir -p $(DEBUG_DIR)
	kubectl get namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_namespaces.log
	kubectl get all -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_all_$(KIND_MANAGED_NAMESPACE).log
	kubectl get leases -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_leases_$(KIND_MANAGED_NAMESPACE).log
	kubectl get configurationpolicies.policy.open-cluster-management.io --all-namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_configurationpolicies.log
	kubectl get certificatepolicies.policy.open-cluster-management.io --all-namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_certificatepolicies.log
	kubectl get iampolicies.policy.open-cluster-management.io --all-namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_iampolicies.log
	kubectl describe pods -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_describe_pods_$(KIND_MANAGED_NAMESPACE).log

e2e-debug-kind: e2e-debug
	@for APP in $(KIND_COMPONENTS); do\
		for CONTAINER in $$(kubectl get pod -l $(KIND_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -o jsonpath={.items[*].spec.containers[*].name}  --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)); do\
			echo "* Logs for Label: $(KIND_COMPONENT_SELECTOR)=$${APP}, Container: $${CONTAINER}" > $(DEBUG_DIR)/managed_logs_$${CONTAINER}.log;\
			kubectl logs -l $(KIND_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -c $${CONTAINER} --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) >> $(DEBUG_DIR)/managed_logs_$${CONTAINER}.log;\
		done;\
	done

e2e-debug-acm: e2e-debug
	@for APP in $(ACM_COMPONENTS); do\
		echo "* Collecting logs for:"; \
		echo "ADDON: $${APP}"; \
		for POD in $$(kubectl get pods -l $(ACM_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -o name --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)); do \
			PODNAME=$${POD##"pod/"}; \
			echo "POD: $${PODNAME}"; \
			for CONTAINER in $$(kubectl get pod $${PODNAME} -n $(KIND_MANAGED_NAMESPACE) -o jsonpath='{.spec.containers[*].name}'  --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)); do\
	  			echo "CONTAINER: $${CONTAINER}"; \
				kubectl logs $${PODNAME} -c $${CONTAINER} -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_logs_$${PODNAME}_$${CONTAINER}.log; \
			done;\
		done;\
	done

e2e-debug-dump:
	@echo -e "* DEBUG LOG DUMP..."
	@echo -e "\n=====\n"
	@for FILE in $$(ls ./$(DEBUG_DIR)/*); do\
			echo -e "* Log file: $${FILE}\n";\
			cat $${FILE};\
			echo -e "\n=====\n";\
	done

integration-test:
	@if [ -z "$(TEST_FILE)" ]; then\
		$(GOPATH)/bin/ginkgo -v $(TEST_ARGS) --fail-fast test/integration;\
	else\
		$(GOPATH)/bin/ginkgo -v $(TEST_ARGS) --fail-fast --focus-file=$(TEST_FILE) test/integration;\
	fi

policy-collection-test:
	@if [ -z "$(TEST_FILE)" ]; then\
		$(GOPATH)/bin/ginkgo -v $(TEST_ARGS) --fail-fast test/policy-collection;\
	else\
		$(GOPATH)/bin/ginkgo -v $(TEST_ARGS) --fail-fast --focus-file=$(TEST_FILE) test/policy-collection;\
	fi

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
