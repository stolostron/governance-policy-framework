// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/stolostron/governance-policy-framework/test"
	"github.com/stolostron/governance-policy-framework/test/common"
)

var (
	userNamespace         string
	clusterNamespace      string
	clusterNamespaceOnHub string
	kubeconfigHub         string
	kubeconfigManaged     string
	defaultTimeoutSeconds int
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface
	clientHosting         kubernetes.Interface
	clientHostingDynamic  dynamic.Interface
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Framework e2e Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	common.InitFlags(nil)
}

var _ = test.ConfigPruneBehavior()

var _ = test.TemplateSyncErrors()

var _ = test.PolicyOrdering()

var _ = BeforeSuite(func(ctx SpecContext) {
	By("Setup hub and managed client")

	common.InitInterfaces(common.KubeconfigHub, common.KubeconfigManaged, common.IsHosted)

	kubeconfigHub = common.KubeconfigHub
	kubeconfigManaged = common.KubeconfigManaged
	userNamespace = common.UserNamespace
	clusterNamespace = common.ClusterNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds
	clusterNamespaceOnHub = common.ClusterNamespaceOnHub
	clientHub = common.ClientHub
	clientHubDynamic = common.ClientHubDynamic
	clientManaged = common.ClientManaged
	clientManagedDynamic = common.ClientManagedDynamic
	clientHosting = common.ClientHosting
	clientHostingDynamic = common.ClientHostingDynamic

	By("Create Namespace if needed")
	namespaces := clientHub.CoreV1().Namespaces()
	if _, err := namespaces.Get(
		ctx,
		userNamespace,
		metav1.GetOptions{},
	); err != nil && errors.IsNotFound(err) {
		Expect(namespaces.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: userNamespace,
			},
		}, metav1.CreateOptions{})).NotTo(BeNil())
	}
	Expect(namespaces.Get(ctx, userNamespace, metav1.GetOptions{})).NotTo(BeNil())

	if !common.ManuallyPatchDecisions {
		By("Create ManagedClusterSetBinding")
		err := common.ApplyManagedClusterSetBinding(ctx)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = AfterSuite(func(ctx SpecContext) {
	By("Cleaning up generated PlacementDecisions")
	Expect(clientHubDynamic.Resource(common.GvrPlacementDecision).Namespace(userNamespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: "generated-by-policy-test",
		})).To(Succeed())
})
