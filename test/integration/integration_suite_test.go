// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

const (
	policyCollectBaseURL      = "https://raw.githubusercontent.com/stolostron/policy-collection/main/"
	policyCollectCommunityURL = policyCollectBaseURL + "community/"
	policyCollectStableURL    = policyCollectBaseURL + "stable/"
	policyCollectACURL        = policyCollectStableURL + "AC-Access-Control/"
	policyCollectCAURL        = policyCollectStableURL + "CA-Security-Assessment-and-Authorization/"
	policyCollectCMURL        = policyCollectStableURL + "CM-Configuration-Management/"
	policyCollectSCURL        = policyCollectStableURL + "SC-System-and-Communications-Protection/"
	policyCollectSIURL        = policyCollectStableURL + "SI-System-and-Information-Integrity/"
)

var (
	userNamespace                           string
	clusterNamespace                        string
	kubeconfigHub                           string
	kubeconfigManaged                       string
	defaultTimeoutSeconds                   int
	clientHub                               kubernetes.Interface
	clientHubDynamic                        dynamic.Interface
	clientManaged                           kubernetes.Interface
	clientManagedDynamic                    dynamic.Interface
	getComplianceState                      func(policyName string) func() interface{}
	canCreateOpenshiftNamespacesInitialized bool
	canCreateOpenshiftNamespacesResult      bool
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GRC framework integration test suite")
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

	getComplianceState = func(policyName string) func() interface{} {
		return common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace)
	}

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

var _ = AfterSuite(func() {
	By("Delete Namespace if needed")
	_, err := common.OcHub("delete", "namespace", userNamespace)
	Expect(err).Should(BeNil())
	common.OcHub("delete", "pod", "default", "pod-that-does-not-exist")
})

func canCreateOpenshiftNamespaces() bool {
	// Only check once - this makes it faster and prevents the answer changing mid-suite.
	if canCreateOpenshiftNamespacesInitialized {
		return canCreateOpenshiftNamespacesResult
	}

	canCreateOpenshiftNamespacesInitialized = true
	canCreateOpenshiftNamespacesResult = false

	// A server-side dry run will check the admission webhooks, but it needs to use the right
	// serviceaccount because the kubeconfig might have superuser privileges to get around them.
	out, _ := utils.KubectlWithOutput("create", "ns", "openshift-grc-test", "--kubeconfig="+kubeconfigManaged,
		"--dry-run=server", "--as=system:serviceaccount:open-cluster-management-agent-addon:config-policy-controller-sa")
	if strings.Contains(out, "namespace/openshift-grc-test created") {
		canCreateOpenshiftNamespacesResult = true
	}
	if strings.Contains(out, "namespaces \"openshift-grc-test\" already exists") {
		// Weird situation, but probably means it could make the namespace
		canCreateOpenshiftNamespacesResult = true
	}
	if strings.Contains(out, "admission webhook \"mutation.gatekeeper.sh\" does not support dry run") {
		// Gatekeeper is installed, so assume the namespace could be created
		canCreateOpenshiftNamespacesResult = true
	}
	return canCreateOpenshiftNamespacesResult
}
