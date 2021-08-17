// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

var (
	clusterNamespace = "managed"

	defaultTimeoutSeconds int
	userNamespace         string
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface

	gvrPod                 = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	gvrNS                  = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	gvrRole                = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	gvrCRD                 = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1beta1", Resource: "customresourcedefinitions"}
	gvrPolicy              = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies"}
	gvrConfigurationPolicy = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "configurationpolicies"}
	gvrCertPolicy          = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "certificatepolicies"}
	gvrIamPolicy           = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "iampolicies"}
	gvrPlacementBinding    = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "placementbindings"}
	gvrPlacementRule       = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "placementrules"}
	gvrK8sRequiredLabels   = schema.GroupVersionResource{Group: "constraints.gatekeeper.sh", Version: "v1beta1", Resource: "k8srequiredlabels"}
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Framework e2e Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {
	By("Setup hub and managed client")
	userNamespace = common.UserNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds

	clientHub = common.NewKubeClient("", common.KubeconfigHub, "")
	clientHubDynamic = common.NewKubeClientDynamic("", common.KubeconfigHub, "")
	clientManaged = common.NewKubeClient("", common.KubeconfigManaged, "")
	clientManagedDynamic = common.NewKubeClientDynamic("", common.KubeconfigManaged, "")

	By("Create Namespace if needed")
	namespaces := clientHub.CoreV1().Namespaces()
	if _, err := namespaces.Get(context.TODO(), userNamespace, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		Expect(namespaces.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: userNamespace,
			},
		}, metav1.CreateOptions{})).NotTo(BeNil())
	}
	Expect(namespaces.Get(context.TODO(), userNamespace, metav1.GetOptions{})).NotTo(BeNil())
})
