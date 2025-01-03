# Copyright Contributors to the Open Cluster Management project

PWD := $(shell pwd)
LOCAL_BIN ?= $(PWD)/bin
deployOnHub ?= false
RELEASE_BRANCH ?= main
ifneq ($(RELEASE_BRANCH), main)
  VERSION_TAG ?= latest-$(subst release-,,$(RELEASE_BRANCH))
else
  VERSION_TAG ?= latest
endif

OCM_API_COMMIT ?= $(shell awk '/open-cluster-management.io\/api/ {print $$2}' go.mod)
CALLER_REPO ?= stolostron
DOCKER_URI ?= quay.io/stolostron
ifeq ($(CALLER_REPO), open-cluster-management-io)
  DOCKER_URI ?= quay.io/open-cluster-management
endif

CERT_MANAGER_VERSION ?= v1.12.0

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
export PATH := $(LOCAL_BIN):$(GOBIN):$(PATH)

# Handle KinD configuration
KIND_HUB_NAMESPACE ?= open-cluster-management
KIND_MANAGED_NAMESPACE ?= open-cluster-management-agent-addon
MANAGED_CLUSTER_NAME ?= managed
HUB_CLUSTER_NAME ?= hub
KIND_VERSION ?= latest

# Fetch OLM version
OLM_VERSION ?= v0.24.0

# Debugging configuration
KIND_COMPONENTS := config-policy-controller cert-policy-controller governance-policy-framework-addon
KIND_COMPONENT_SELECTOR := name
ACM_COMPONENTS := cert-policy-controller config-policy-controller governance-policy-framework
ACM_COMPONENT_SELECTOR := app
DEBUG_DIR ?= test-output/debug

# Test configuration
TEST_FILE ?=
TEST_ARGS ?=
UPSTREAM_TEST ?= false

ifdef TEST_FILE
	TEST_ARGS += --focus-file=$(TEST_FILE)
endif

ifeq ($(UPSTREAM_TEST), true)
	TEST_ARGS += --skip-file=cert_policy
endif

include build/common/Makefile.common.mk

############################################################
# clean section
############################################################

.PHONY: clean
clean::
	-rm bin/*
	-rm kubeconfig_*
	-rm -r test-output/
	-rm -r vendor/
	-rm -rf $(ADDON_CONTROLLER)
	-rm -r test/resources/policy_generator/helm-kustomization/base/charts/
	find . -type d \( -name "stolostron" -o -name "open-cluster-management-io" \) -exec rm -rf {} +

############################################################
# lint section
############################################################

.PHONY: lint
lint:

.PHONY: fmt
fmt:

############################################################
# e2e test section
############################################################
.PHONY: e2e-setup-kube
e2e-setup-kube: KUBECONFIG ?= $(HOME)/.kube/config
e2e-setup-kube:
	cp $(KUBECONFIG) kubeconfig_hub
	cp $(KUBECONFIG) kubeconfig_managed

.PHONY: kind-bootstrap-cluster
kind-bootstrap-cluster: kind-create-clusters install-crds install-resources kind-deploy-policy-framework kind-deploy-policy-controllers

.PHONY: kind-bootstrap-cluster-dev
kind-bootstrap-cluster-dev: kind-create-clusters install-crds install-resources

.PHONY: kind-deploy-policy-controllers
kind-deploy-policy-controllers: kind-deploy-cert-policy-controller kind-deploy-config-policy-controller kind-deploy-olm

.PHONY: kind-policy-framework-hub-setup
kind-policy-framework-hub-setup:
	kubectl config use-context kind-$(HUB_CLUSTER_NAME)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

.PHONY: kustomize
kustomize:

.PHONY: deploy-policy-framework-hub-crd-operator
deploy-policy-framework-hub-crd-operator:
	kubectl create ns $(KIND_HUB_NAMESPACE) || true
	@echo installing Policy CRDs on hub
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policies.yaml
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_placementbindings.yaml
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policyautomations.yaml
	@echo installing policy-propagator on hub
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_HUB_NAMESPACE)
	kubectl patch deployment governance-policy-propagator -n $(KIND_HUB_NAMESPACE) \
	  -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-propagator\",\"image\":\"${DOCKER_URI}/governance-policy-propagator:${VERSION_TAG}\"}]}}}}"
	kubectl rollout status deployment/governance-policy-propagator -n $(KIND_HUB_NAMESPACE) --timeout=180s

.PHONY: deploy-policy-framework-hub
deploy-policy-framework-hub: kind-policy-framework-hub-setup deploy-policy-framework-hub-crd-operator

.PHONY: deploy-community-policy-framework-hub
deploy-community-policy-framework-hub: deploy-policy-framework-hub-crd-operator

.PHONY: kind-policy-framework-managed-setup
kind-policy-framework-managed-setup:
	kubectl config use-context kind-$(MANAGED_CLUSTER_NAME)
	kubectl create ns $(KIND_MANAGED_NAMESPACE) || true
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal

.PHONY: deploy-policy-framework-managed-crd-operator
deploy-policy-framework-managed-crd-operator:
	@echo installing Policy CRD on managed
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policies.yaml
	@echo installing governance-policy-framework-addon on managed
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-framework-addon/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE)
	kubectl patch deployment governance-policy-framework-addon -n $(KIND_MANAGED_NAMESPACE) -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-framework-addon\",\"args\":[\"--hub-cluster-configfile=/var/run/klusterlet/kubeconfig\", \"--cluster-namespace=$(MANAGED_CLUSTER_NAME)\", \"--enable-lease=true\", \"--log-level=2\", \"--disable-spec-sync=$(deployOnHub)\"]}]}}}}"
	kubectl rollout status deployment/governance-policy-framework-addon -n $(KIND_MANAGED_NAMESPACE) --timeout=180s

.PHONY: deploy-policy-framework-managed
deploy-policy-framework-managed: kind-policy-framework-managed-setup deploy-policy-framework-managed-crd-operator

.PHONY: deploy-community-policy-framework-managed
deploy-community-policy-framework-managed: deploy-policy-framework-managed-crd-operator

.PHONY: kind-deploy-policy-propagator
kind-deploy-policy-propagator:
	# Installing policy-propagator on hub
	-kubectl create ns $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	# Installing cert-manager (prerequisite for webhook)
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl wait deployment -n cert-manager -l app.kubernetes.io/instance=cert-manager --for condition=Available=True --timeout=180s --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	curl https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/${RELEASE_BRANCH}/deploy/webhook.yaml | \
		sed 's/namespace: open-cluster-management/namespace: $(KIND_HUB_NAMESPACE)/g' | \
	 	kubectl apply -f - --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl patch deployment governance-policy-propagator -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) \
	  -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-propagator\",\"image\":\"${DOCKER_URI}/governance-policy-propagator:${VERSION_TAG}\"}]}}}}"
	kubectl rollout status deployment/governance-policy-propagator -n $(KIND_HUB_NAMESPACE) --timeout=180s --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)

.PHONY: kind-deploy-policy-framework
kind-deploy-policy-framework: kind-deploy-policy-propagator
	@echo creating secrets on managed
	kubectl create ns $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl create secret -n $(KIND_MANAGED_NAMESPACE) generic hub-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	@echo installing governance-policy-framework-addon on managed
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-framework-addon/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	echo patching governance-policy-framework-addon to set the managed cluster
	kubectl patch deployment governance-policy-framework-addon -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) \
		-p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-framework-addon\",\"image\":\"${DOCKER_URI}/governance-policy-framework-addon:${VERSION_TAG}\",\"args\":[\"--hub-cluster-configfile=/var/run/klusterlet/kubeconfig\", \"--cluster-namespace=$(MANAGED_CLUSTER_NAME)\", \"--enable-lease=true\", \"--log-level=2\", \"--disable-spec-sync=$(deployOnHub)\"]}]}}}}"
	kubectl rollout status deployment/governance-policy-framework-addon -n $(KIND_MANAGED_NAMESPACE) --timeout=180s --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

.PHONY: kind-deploy-config-policy-controller
kind-deploy-config-policy-controller:
	@echo installing config-policy-controller on managed
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/config-policy-controller/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/config-policy-controller/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_configurationpolicies.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/config-policy-controller/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_operatorpolicies.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl patch deployment config-policy-controller -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) \
		-p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"config-policy-controller\",\"image\":\"${DOCKER_URI}/config-policy-controller:${VERSION_TAG}\"}]}}}}"
	kubectl rollout status deployment/config-policy-controller -n $(KIND_MANAGED_NAMESPACE) --timeout=180s --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

.PHONY: kind-deploy-cert-policy-controller
kind-deploy-cert-policy-controller:
	@echo installing cert-manager on managed
	kubectl apply --validate=false -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	@echo installing cert-policy-controller on managed
	kubectl apply -f https://raw.githubusercontent.com/stolostron/cert-policy-controller/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_certificatepolicies.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/stolostron/cert-policy-controller/$(RELEASE_BRANCH)/deploy/operator.yaml -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl patch deployment cert-policy-controller -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) \
		-p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"cert-policy-controller\",\"image\":\"quay.io/stolostron/cert-policy-controller:${VERSION_TAG}\"}]}}}}"
	kubectl patch deployment cert-policy-controller \
		-n $(KIND_MANAGED_NAMESPACE) -p '{"spec": {"template": {"spec": {"containers": [{"name":"cert-policy-controller", "args": ["--enable-lease=true"]}]}}}}' \
		--kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	kubectl rollout status deployment/cert-policy-controller -n $(KIND_MANAGED_NAMESPACE) --timeout=180s --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

.PHONY: kind-deploy-olm
kind-deploy-olm:
	@echo installing OLM on managed
	export KUBECONFIG=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)
	curl --fail -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/install.sh -o ./bin/install.sh
	chmod +x ./bin/install.sh
	./bin/install.sh $(OLM_VERSION)

.PHONY: kind-create-clusters
kind-create-clusters:
	@echo "creating cluster hub"
	kind create cluster --name $(HUB_CLUSTER_NAME) $(KIND_ARGS)
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	cp $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_e2e
	# needed for managed -> hub communication
	kind get kubeconfig --name $(HUB_CLUSTER_NAME) --internal > $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal
	@if [ "$(deployOnHub)" = "true" ]; then\
		echo import cluster hub as managed;\
		kind get kubeconfig --name $(HUB_CLUSTER_NAME) > $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	else\
		echo creating cluster managed;\
		kind create cluster --name $(MANAGED_CLUSTER_NAME) $(KIND_ARGS);\
		kind get kubeconfig --name $(MANAGED_CLUSTER_NAME) > $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME);\
	fi
	cp $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)_e2e

.PHONY: kind-delete-cluster
kind-delete-cluster:
	kind delete cluster --name $(HUB_CLUSTER_NAME)
	kind delete cluster --name $(MANAGED_CLUSTER_NAME)
	rm $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) || true
	rm $(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)_internal || true
	rm $(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) || true

.PHONY: install-crds
install-crds:
	@echo installing crds on hub
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/$(OCM_API_COMMIT)/cluster/v1/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/$(OCM_API_COMMIT)/cluster/v1beta1/0000_02_clusters.open-cluster-management.io_placements.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/api/$(OCM_API_COMMIT)/cluster/v1beta1/0000_03_clusters.open-cluster-management.io_placementdecisions.crd.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_placementbindings.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policies.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policyautomations.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policysets.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo installing crds on managed
	kubectl apply -f https://raw.githubusercontent.com/$(CALLER_REPO)/governance-policy-propagator/$(RELEASE_BRANCH)/deploy/crds/policy.open-cluster-management.io_policies.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

.PHONY: install-resources
install-resources:
	@echo creating user namespace on hub
	kubectl create ns policy-test --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating cluster namespace on hub 
	kubectl create ns $(MANAGED_CLUSTER_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f test/resources/managed-cluster.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	@echo creating cluster namespace on managed 
	kubectl create ns $(MANAGED_CLUSTER_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) || true

.PHONY: e2e-dependencies
e2e-dependencies:

K8SCLIENT ?= oc
GINKGO = $(LOCAL_BIN)/ginkgo
IS_HOSTED ?= false
PATCH_DECISIONS ?= true
MANAGED_CLUSTER_NAMESPACE ?= $(MANAGED_CLUSTER_NAME)

.PHONY: e2e-test
e2e-test: e2e-dependencies
	$(GINKGO) -v $(TEST_ARGS) test/e2e -- -cluster_namespace=$(MANAGED_CLUSTER_NAMESPACE) -k8s_client=$(K8SCLIENT) -is_hosted=$(IS_HOSTED) -patch_decisions=$(PATCH_DECISIONS) -cluster_namespace_on_hub=$(CLUSTER_NAMESPACE_ON_HUB)

.PHONY: e2e-test-hosted
e2e-test-hosted: CLUSTER_NAMESPACE_ON_HUB=cluster2
e2e-test-hosted: IS_HOSTED=true
e2e-test-hosted: PATCH_DECISIONS=false
e2e-test-hosted: MANAGED_CLUSTER_NAMESPACE=cluster2-hosted
e2e-test-hosted: e2e-test

.PHONY: e2e-debug
e2e-debug: e2e-debug-hub e2e-debug-managed

.PHONY: e2e-debug-hub
e2e-debug-hub:
	# Collecting hub cluster debug logs
	mkdir -p $(DEBUG_DIR)
	-kubectl get namespaces --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_namespaces.log
	-kubectl get all -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_all_$(KIND_HUB_NAMESPACE).log
	-kubectl get all -n $(MANAGED_CLUSTER_NAME) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_all_$(MANAGED_CLUSTER_NAME).log
	-kubectl get leases -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_get_leases_$(KIND_HUB_NAMESPACE).log
	-kubectl describe pods -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_describe_pods_$(KIND_HUB_NAMESPACE).log
	-kubectl get managedclusteraddon -A -o yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_managedclusteraddon.yaml
	-for POD in $$(kubectl get pods -n $(KIND_HUB_NAMESPACE) -l "app=multicluster-operators-hub-subscription" -o name --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)); do \
		PODNAME=$${POD##"pod/"}; \
	  	kubectl logs $${PODNAME} -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_logs_$${PODNAME}.log; \
	done
	-for POD in $$(kubectl get pods -n $(KIND_HUB_NAMESPACE) -l "name in (governance-policy-propagator,governance-policy-addon-controller)" -o name --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)); do \
		PODNAME=$${POD##"pod/"}; \
	  	kubectl logs $${PODNAME} -n $(KIND_HUB_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME) > $(DEBUG_DIR)/hub_logs_$${PODNAME}.log; \
	done

.PHONY: e2e-debug-managed
e2e-debug-managed:
	# Collecting managed cluster debug logs
	mkdir -p $(DEBUG_DIR)
	-kubectl get namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_namespaces.log
	-kubectl get all -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_all_$(KIND_MANAGED_NAMESPACE).log
	-kubectl get leases -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_leases_$(KIND_MANAGED_NAMESPACE).log
	-kubectl get configurationpolicies.policy.open-cluster-management.io --all-namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_configurationpolicies.log
	-kubectl get certificatepolicies.policy.open-cluster-management.io --all-namespaces --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_get_certificatepolicies.log
	-kubectl describe pods -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) > $(DEBUG_DIR)/managed_describe_pods_$(KIND_MANAGED_NAMESPACE).log

.PHONY: e2e-debug-kind
e2e-debug-kind: e2e-debug
	-@for APP in $(KIND_COMPONENTS); do\
		for CONTAINER in $$(kubectl get pod -l $(KIND_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -o jsonpath={.items[*].spec.containers[*].name}  --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)); do\
			echo "* Logs for Label: $(KIND_COMPONENT_SELECTOR)=$${APP}, Container: $${CONTAINER}" > $(DEBUG_DIR)/managed_logs_$${CONTAINER}.log;\
			kubectl logs -l $(KIND_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -c $${CONTAINER} --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME) --tail=-1 >> $(DEBUG_DIR)/managed_logs_$${CONTAINER}.log;\
		done;\
	done

.PHONY: e2e-debug-acm
e2e-debug-acm: e2e-debug
	-@for CLUSTER in $(HUB_CLUSTER_NAME) $(MANAGED_CLUSTER_NAME); do \
		echo "# Collecting ACM $${CLUSTER} cluster pod logs"; \
		for APP in $(ACM_COMPONENTS); do \
			echo "ADDON: $${APP}"; \
			for POD in $$(kubectl get pods -l $(ACM_COMPONENT_SELECTOR)=$${APP} -n $(KIND_MANAGED_NAMESPACE) -o name --kubeconfig=$(PWD)/kubeconfig_$${CLUSTER}); do \
				PODNAME=$${POD##"pod/"}; \
				echo "* POD: $${PODNAME}"; \
				for CONTAINER in $$(kubectl get pod $${PODNAME} -n $(KIND_MANAGED_NAMESPACE) -o jsonpath='{.spec.containers[*].name}'  --kubeconfig=$(PWD)/kubeconfig_$${CLUSTER}); do\
						echo "  * CONTAINER: $${CONTAINER}"; \
					kubectl logs $${PODNAME} -c $${CONTAINER} -n $(KIND_MANAGED_NAMESPACE) --kubeconfig=$(PWD)/kubeconfig_$${CLUSTER} > $(DEBUG_DIR)/$${CLUSTER}_logs_$${PODNAME}_$${CONTAINER}.log; \
				done;\
			done;\
		done;\
	done

.PHONY: e2e-debug-dump
e2e-debug-dump:
	@echo "* DEBUG LOG DUMP..."
	@echo "====="
	@for FILE in $$(ls ./$(DEBUG_DIR)/*_get_*.log); do\
			echo "* Log file: $${FILE}";\
			cat $${FILE};\
			echo "=====";\
	done
	@for FILE in $$(ls ./$(DEBUG_DIR)/*_logs_*.log); do\
			echo "* Log file: $${FILE}";\
			tail -n 50 $${FILE};\
			echo "=====";\
	done

.PHONY: integration-test
integration-test: e2e-dependencies
	$(GINKGO) -v $(TEST_ARGS) test/integration -- -cluster_namespace=$(MANAGED_CLUSTER_NAMESPACE) -k8s_client=$(K8SCLIENT) -is_hosted=$(IS_HOSTED) -cluster_namespace_on_hub=$(MANAGED_CLUSTER_NAMESPACE) -patch_decisions=false -policy_collection_branch=$(RELEASE_BRANCH)

#hosted
ADDON_CONTROLLER = $(PWD)/.go/governance-policy-addon-controller

.PHONY: kind-bootstrap-hosted
kind-bootstrap-hosted: kind-install-hosted install-crds install-resources kind-deploy-policy-propagator kind-deploy-cert-manager setup-managedcluster
	@echo "Restarting propagator and addon-controller"
	kubectl delete pod -l name=governance-policy-propagator -A --kubeconfig=./kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl delete pod -l app=governance-policy-addon-controller -A --kubeconfig=./kubeconfig_$(HUB_CLUSTER_NAME)

.PHONY: kind-install-hosted
kind-install-hosted: $(ADDON_CONTROLLER)
	@cd $(ADDON_CONTROLLER) && KIND_VERSION=$(KIND_VERSION) HOSTED_MODE=true ./build/manage-clusters.sh
	@cp $(ADDON_CONTROLLER)/kubeconfig_cluster1_e2e ./kubeconfig_$(HUB_CLUSTER_NAME)
	@cp $(ADDON_CONTROLLER)/kubeconfig_cluster1_e2e-internal ./kubeconfig_$(HUB_CLUSTER_NAME)_internal
	@cp $(ADDON_CONTROLLER)/kubeconfig_cluster2_e2e ./kubeconfig_$(MANAGED_CLUSTER_NAME)
	@cp $(ADDON_CONTROLLER)/kubeconfig_cluster2_e2e-internal ./kubeconfig_$(MANAGED_CLUSTER_NAME)_internal
	kubectl apply -f https://raw.githubusercontent.com/stolostron/governance-policy-addon-controller/$(RELEASE_BRANCH)/test/resources/cert_policy_clustermanagementaddon.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f $(ADDON_CONTROLLER)/test/resources/config_policy_clustermanagementaddon.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)
	kubectl apply -f $(ADDON_CONTROLLER)/test/resources/framework_clustermanagementaddon.yaml --kubeconfig=$(PWD)/kubeconfig_$(HUB_CLUSTER_NAME)

$(ADDON_CONTROLLER):
	git clone --depth=1 -b $(RELEASE_BRANCH) https://github.com/$(CALLER_REPO)/governance-policy-addon-controller.git $(ADDON_CONTROLLER)

.PHONY: setup-managedcluster
setup-managedcluster: MANAGED_CLUSTER_NAMESPACE=cluster2-hosted
setup-managedcluster: 
	-kubectl create ns $(MANAGED_CLUSTER_NAMESPACE) --kubeconfig=kubeconfig_$(HUB_CLUSTER_NAME)
	-kubectl -n $(MANAGED_CLUSTER_NAMESPACE) create secret generic config-policy-controller-managed-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_managed_internal --kubeconfig=kubeconfig_$(HUB_CLUSTER_NAME)
	-kubectl -n $(MANAGED_CLUSTER_NAMESPACE) create secret generic cert-policy-controller-managed-kubeconfig --from-file=kubeconfig=$(PWD)/kubeconfig_managed_internal --kubeconfig=kubeconfig_$(HUB_CLUSTER_NAME)
	-sed 's/imagetag/$(VERSION_TAG)/g' test/resources/hosted_mode/managed-cluster-addon.yaml | kubectl apply -f- --kubeconfig=kubeconfig_$(HUB_CLUSTER_NAME) -n cluster2

kind-delete-hosted: $(ADDON_CONTROLLER)
	@cd $(ADDON_CONTROLLER) && make kind-bootstrap-delete-clusters 
	@rm kubeconfig_hub kubeconfig_managed kubeconfig_hub_internal kubeconfig_managed_internal

.PHONY: 
kind-deploy-cert-manager:
	@echo installing cert-manager on managed
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml --kubeconfig=$(PWD)/kubeconfig_$(MANAGED_CLUSTER_NAME)

