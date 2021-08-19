package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policy/v1"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
)

const iamPolicyName = "policy-limitclusteradmin"
const iamPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/main/stable/AC-Access-Control/" + iamPolicyName + ".yaml"

// Note that these tests must be run on OpenShift since the tests create an OpenShift group
var _ = Describe("Test the stable IAM policy", func() {
	var getIAMComplianceState func() interface{}
	BeforeEach(func() {
		// Assign this here to avoid using nil pointers as arguments
		getIAMComplianceState = common.GetComplianceState(clientHubDynamic, userNamespace, iamPolicyName, clusterNamespace)
	})

	It("stable/"+iamPolicyName+" should be created on the hub", func() {
		By("Creating the policy on the hub")
		utils.KubectlWithOutput("apply", "-f", iamPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)

		By("Patching the placement rule")
		utils.KubectlWithOutput(
			"patch",
			"-n",
			userNamespace,
			"placementrule.apps.open-cluster-management.io/placement-"+iamPolicyName,
			"--type=json",
			`-p=[{"op": "replace", "path": "/spec/clusterSelector/matchExpressions", "value":[{"key": "name", "operator": "In", "values": [`+clusterNamespace+`]}]}]`,
			"--kubeconfig="+kubeconfigHub,
		)

		By("Checking " + iamPolicyName + " on the hub cluster in the ns " + userNamespace)
		rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, iamPolicyName, userNamespace, true, defaultTimeoutSeconds)
		Expect(rootPlc).NotTo(BeNil())
	})

	It("stable/"+iamPolicyName+" should be created on the managed cluster", func() {
		By("Checking " + iamPolicyName + " on the managed cluster in the ns " + clusterNamespace)
		managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+iamPolicyName, clusterNamespace, true, defaultTimeoutSeconds*2)
		Expect(managedplc).NotTo(BeNil())
	})

	It("stable/"+iamPolicyName+" should be compliant", func() {
		By("Checking if the status of the root policy is compliant")
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
	})

	It("Make the policy noncompliant", func() {
		By("Creating an OpenShift group (RHBZ#1981127)")
		utils.KubectlWithOutput("apply", "-f", "../resources/iam_policy/group.yaml", "-n", userNamespace, "--kubeconfig="+kubeconfigHub)

		By("Creating a cluster role binding")
		utils.KubectlWithOutput("apply", "-f", "../resources/iam_policy/clusterrolebinding.yaml", "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
	})

	It("stable/"+iamPolicyName+" should be noncompliant", func() {
		By("Checking if the status of the root policy is noncompliant")
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
	})

	It("Make stable/"+iamPolicyName+" be compliant", func() {
		By("Deleting the OpenShift group")
		utils.KubectlWithOutput("delete", "-f", "../resources/iam_policy/group.yaml", "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
	})

	It("stable/"+iamPolicyName+" should be compliant", func() {
		By("Checking if the status of the root policy is compliant")
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
	})

	It("Clean up stable/"+iamPolicyName, func() {
		utils.KubectlWithOutput("delete", "-f", iamPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		utils.KubectlWithOutput("delete", "-f", "../resources/iam_policy/clusterrolebinding.yaml", "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		Eventually(
			func() interface{} {
				managedPlc := utils.GetWithTimeout(
					clientManagedDynamic, common.GvrPolicy, userNamespace+"."+iamPolicyName, clusterNamespace, false, defaultTimeoutSeconds,
				)
				return managedPlc
			},
			defaultTimeoutSeconds,
			1,
		).Should(BeNil())
	})
})
