// Copyright Contributors to the Open Cluster Management project

package common

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	GvrPod                   = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	GvrNS                    = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	GvrRole                  = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	GvrCRD                   = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1beta1", Resource: "customresourcedefinitions"}
	GvrPolicy                = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies"}
	GvrConfigurationPolicy   = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "configurationpolicies"}
	GvrCertPolicy            = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "certificatepolicies"}
	GvrIamPolicy             = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "iampolicies"}
	GvrPlacementBinding      = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "placementbindings"}
	GvrPlacementRule         = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "placementrules"}
	GvrK8sRequiredLabels     = schema.GroupVersionResource{Group: "constraints.gatekeeper.sh", Version: "v1beta1", Resource: "k8srequiredlabels"}
	GvrClusterVersion        = schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "clusterversions"}
	GvrComplianceScan        = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancescans"}
	GvrComplianceSuite       = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancesuites"}
	GvrComplianceCheckResult = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancecheckresults"}
	GvrRoute                 = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
)
