// Copyright Contributors to the Open Cluster Management project

package common

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	GvrPod = schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	}
	GvrNS = schema.GroupVersionResource{
		Version:  "v1",
		Resource: "namespaces",
	}
	GvrConfigMap = schema.GroupVersionResource{
		Version:  "v1",
		Resource: "configmaps",
	}
	GvrRole = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}
	GvrCRD = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	GvrPolicy = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1",
		Resource: "policies",
	}
	GvrPolicySet = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1beta1",
		Resource: "policysets",
	}
	GvrConfigurationPolicy = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1",
		Resource: "configurationpolicies",
	}
	GvrCertPolicy = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1",
		Resource: "certificatepolicies",
	}
	GvrDeployment = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	GvrIamPolicy = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1",
		Resource: "iampolicies",
	}
	GvrPlacementBinding = schema.GroupVersionResource{
		Group:    "policy.open-cluster-management.io",
		Version:  "v1",
		Resource: "placementbindings",
	}
	GvrPlacementRule = schema.GroupVersionResource{
		Group:    "apps.open-cluster-management.io",
		Version:  "v1",
		Resource: "placementrules",
	}
	GvrSubscription = schema.GroupVersionResource{
		Group:    "apps.open-cluster-management.io",
		Version:  "v1",
		Resource: "subscriptions",
	}
	GvrK8sRequiredLabels = schema.GroupVersionResource{
		Group:    "constraints.gatekeeper.sh",
		Version:  "v1beta1",
		Resource: "k8srequiredlabels",
	}
	GvrClusterVersion = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusterversions",
	}
	GvrAPIServer = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "apiservers",
	}
	GvrComplianceScan = schema.GroupVersionResource{
		Group:    "compliance.openshift.io",
		Version:  "v1alpha1",
		Resource: "compliancescans",
	}
	GvrComplianceSuite = schema.GroupVersionResource{
		Group:    "compliance.openshift.io",
		Version:  "v1alpha1",
		Resource: "compliancesuites",
	}
	GvrComplianceCheckResult = schema.GroupVersionResource{
		Group:    "compliance.openshift.io",
		Version:  "v1alpha1",
		Resource: "compliancecheckresults",
	}
	GvrSCC = schema.GroupVersionResource{
		Group:    "security.openshift.io",
		Version:  "v1",
		Resource: "securitycontextconstraints",
	}
	GvrRoute = schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}
	GvrOAuth = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "oauths",
	}
	GvrUser = schema.GroupVersionResource{
		Group:    "user.openshift.io",
		Version:  "v1",
		Resource: "users",
	}
	GvrIdentity = schema.GroupVersionResource{
		Group:    "user.openshift.io",
		Version:  "v1",
		Resource: "identities",
	}
	GvrManagedClusterSet = schema.GroupVersionResource{
		Group:    "cluster.open-cluster-management.io",
		Version:  "v1beta2",
		Resource: "managedclustersets",
	}
	GvrAddonDeploymentConfig = schema.GroupVersionResource{
		Group:    "addon.open-cluster-management.io",
		Version:  "v1alpha1",
		Resource: "addondeploymentconfigs",
	}
	GvrClusterManagementAddOn = schema.GroupVersionResource{
		Group:    "addon.open-cluster-management.io",
		Version:  "v1alpha1",
		Resource: "clustermanagementaddons",
	}
)
