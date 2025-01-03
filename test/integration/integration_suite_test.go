// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"fmt"
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

var (
	policyCollectBaseURL      string
	policyCollectCommunityURL string
	policyCollectStableURL    string
	policyCollectACURL        string
	policyCollectCAURL        string
	policyCollectCMURL        string
	policyCollectSCURL        string
	policyCollectSIURL        string
)

var (
	userNamespace         string
	clusterNamespace      string
	ocmNS                 string
	ocmAddonNS            string
	kubeconfigHub         string
	kubeconfigManaged     string
	defaultTimeoutSeconds int
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface
	gitopsUser            common.OCPUser

	canCreateOpenshiftNamespacesInitialized bool
	canCreateOpenshiftNamespacesResult      bool
)

func TestIntegration(t *testing.T) {
	policyCollectBaseURL = fmt.Sprintf(
		"https://raw.githubusercontent.com/stolostron/policy-collection/%s/", common.PolicyCollectionBranch,
	)
	policyCollectCommunityURL = policyCollectBaseURL + "community/"
	policyCollectStableURL = policyCollectBaseURL + "stable/"
	policyCollectACURL = policyCollectStableURL + "AC-Access-Control/"
	policyCollectCAURL = policyCollectStableURL + "CA-Security-Assessment-and-Authorization/"
	policyCollectCMURL = policyCollectStableURL + "CM-Configuration-Management/"
	policyCollectSCURL = policyCollectStableURL + "SC-System-and-Communications-Protection/"
	policyCollectSIURL = policyCollectStableURL + "SI-System-and-Information-Integrity/"

	RegisterFailHandler(Fail)
	RunSpecs(t, "GRC framework integration test suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	common.InitFlags(nil)
}

var _ = BeforeSuite(func(ctx SpecContext) {
	By("Setup hub and managed client")
	common.InitInterfaces(common.KubeconfigHub, common.KubeconfigManaged, common.IsHosted)
	kubeconfigHub = common.KubeconfigHub
	kubeconfigManaged = common.KubeconfigManaged
	userNamespace = common.UserNamespace
	clusterNamespace = common.ClusterNamespace
	ocmNS = common.OCMNamespace
	ocmAddonNS = common.OCMAddOnNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds

	clientHub = common.ClientHub
	clientHubDynamic = common.ClientHubDynamic
	clientManaged = common.ClientManaged
	clientManagedDynamic = common.ClientManagedDynamic

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

	By("Create ManagedClusterSetBinding")
	err := common.ApplyManagedClusterSetBinding(ctx)
	Expect(err).ToNot(HaveOccurred())

	By("Setting up GitOps user")
	common.GitOpsUserSetup(ctx, &gitopsUser)
})

var _ = AfterSuite(func(ctx SpecContext) {
	By("Cleaning up generated PlacementDecisions")
	Expect(clientHubDynamic.Resource(common.GvrPlacementDecision).Namespace(userNamespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: "generated-by-policy-test",
		})).To(Succeed())

	if userNamespace != "open-cluster-management-global-set" {
		By("Delete Namespace if needed")
		_, err := common.OcHub(
			"delete", "namespace", userNamespace,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = common.OcHub(
			"delete", "managedclustersetbinding", "global", "-n",
			userNamespace, "--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())
	}

	_, err := common.OcHub(
		"delete", "pod", "default",
		"pod-that-does-not-exist", "--ignore-not-found",
	)
	Expect(err).ToNot(HaveOccurred())

	common.GitOpsCleanup(ctx, gitopsUser)
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
	out, _ := utils.KubectlWithOutput(
		"create", "ns", "openshift-grc-test",
		"--kubeconfig="+kubeconfigManaged,
		"--dry-run=server",
		"--as=system:serviceaccount:"+ocmAddonNS+":config-policy-controller-sa",
	)

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
