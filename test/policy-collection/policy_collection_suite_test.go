// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/open-cluster-management/governance-policy-framework/test/common"
)

var (
	userNamespace         string
	clusterNamespace      string
	kubeconfigHub         string
	kubeconfigManaged     string
	defaultTimeoutSeconds int
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface

	gvrClusterVersion        = schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "clusterversions"}
	gvrComplianceScan        = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancescans"}
	gvrComplianceSuite       = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancesuites"}
	gvrComplianceCheckResult = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancecheckresults"}
	gvrPolicy                = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies"}
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("../../test-output/policy-collection.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Policy collection repo e2e Suite", []Reporter{junitReporter})
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {
	By("Setup hub and managed client")
	kubeconfigHub = common.KubeconfigHub
	kubeconfigManaged = common.KubeconfigManaged
	userNamespace = common.UserNamespace
	clusterNamespace = common.ClusterNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds

	clientHub = common.NewKubeClient("", kubeconfigHub, "")
	clientHubDynamic = common.NewKubeClientDynamic("", kubeconfigHub, "")
	clientManaged = common.NewKubeClient("", kubeconfigManaged, "")
	clientManagedDynamic = common.NewKubeClientDynamic("", kubeconfigManaged, "")

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
