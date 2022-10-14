package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// Note that these tests must be run on OpenShift since the tests create an OpenShift group
var _ = Describe("GRC: [P1][Sev1][policy-grc] Test "+
	"the policy-limitclusteradmin policy", Ordered, Label("policy-collection", "stable", "BVT"), func() {
	const (
		iamPolicyName             = "policy-limitclusteradmin"
		iamPolicyURL              = policyCollectACURL + iamPolicyName + ".yaml"
		iamPolicyManagedNamespace = "iam-policy-test"
	)

	var getIAMComplianceState func(Gomega) interface{}
	BeforeEach(func() {
		// Assign this here to avoid using nil pointers as arguments
		getIAMComplianceState = common.GetComplianceState(userNamespace, iamPolicyName, clusterNamespace)
	})

	It("stable/"+iamPolicyName+" should be created on the hub", func() {
		By("Creating the policy on the hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f",
			iamPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching the placement rule")
		err = common.PatchPlacementRule(
			userNamespace,
			"placement-"+iamPolicyName,
			clusterNamespace,
			kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking " + iamPolicyName + " on the hub cluster in the ns " + userNamespace)
		rootPlc := utils.GetWithTimeout(
			clientHubDynamic,
			common.GvrPolicy,
			iamPolicyName,
			userNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(rootPlc).NotTo(BeNil())
	})

	It("stable/"+iamPolicyName+" should be created on the managed cluster", func() {
		By("Checking " + iamPolicyName + " on the managed cluster in the ns " + clusterNamespace)
		managedplc := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+iamPolicyName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds*2,
		)
		Expect(managedplc).NotTo(BeNil())
	})

	It("stable/"+iamPolicyName+" should be compliant", func() {
		By("Checking if the status of the root policy is compliant")
		// Increasing the time out for now to wait for the iam policy test from GRC-UI to complete.
		// iam policy test from GRC-UI takes around 5 minutes to complete.
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*10, 1).Should(Equal(policiesv1.Compliant))
	})

	It("Make the policy noncompliant", func() {
		By("Creating the" + iamPolicyManagedNamespace + " namespace on the managed cluster")
		namespaces := clientManaged.CoreV1().Namespaces()
		_, err := namespaces.Create(
			context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: iamPolicyManagedNamespace,
				},
			},
			metav1.CreateOptions{},
		)
		if err != nil {
			Expect(k8serrors.IsAlreadyExists(err)).Should(BeTrue())
		}

		Expect(namespaces.Get(context.TODO(), iamPolicyManagedNamespace, metav1.GetOptions{})).NotTo(BeNil())

		By("Creating an OpenShift group (RHBZ#1981127)")
		_, err = utils.KubectlWithOutput(
			"apply", "-f",
			"../resources/iam_policy/group.yaml",
			"-n",
			iamPolicyManagedNamespace,
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())

		By("Creating a cluster role binding")
		_, err = utils.KubectlWithOutput(
			"apply", "-f",
			"../resources/iam_policy/clusterrolebinding.yaml",
			"-n",
			iamPolicyManagedNamespace,
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+iamPolicyName+" should be noncompliant", func() {
		By("Checking if the status of the root policy is noncompliant")
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
	})

	It("Make stable/"+iamPolicyName+" be compliant", func() {
		By("Deleting the OpenShift group")
		_, err := utils.KubectlWithOutput(
			"delete", "-f",
			"../resources/iam_policy/group.yaml",
			"-n",
			iamPolicyManagedNamespace,
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+iamPolicyName+" should be compliant", func() {
		By("Checking if the status of the root policy is compliant")
		// Increasing the time out for now to wait for the iam policy test from GRC-UI to complete.
		// iam policy test from GRC-UI takes around 5 minutes to complete.
		Eventually(getIAMComplianceState, defaultTimeoutSeconds*10, 1).Should(Equal(policiesv1.Compliant))
	})

	AfterAll(func() {
		err := clientManaged.CoreV1().Namespaces().Delete(
			context.TODO(),
			iamPolicyManagedNamespace,
			metav1.DeleteOptions{},
		)
		Expect(err).Should(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "-f",
			iamPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		Eventually(
			func() interface{} {
				managedPlc := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrPolicy,
					userNamespace+"."+iamPolicyName,
					clusterNamespace,
					false,
					defaultTimeoutSeconds,
				)

				return managedPlc
			},
			defaultTimeoutSeconds,
			1,
		).Should(BeNil())
	})
})
