// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test recreateOption", Ordered, Label("BVT"), func() {
	const (
		ns                  = "open-cluster-management-global-set"
		policyInitialYAML   = "../resources/recreate_option/policy-initial-deployment.yaml"
		policyUpdateYAML    = "../resources/recreate_option/policy-update-deployment.yaml"
		policyConfigMapYAML = "../resources/recreate_option/policy-configmap.yaml"
	)

	AfterAll(func(ctx SpecContext) {
		By("Deleting the recreate-option-initial policy")
		_, err := common.OcHub(
			"delete",
			"-f",
			policyInitialYAML,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the recreate-option-update policy")
		_, err = common.OcHub(
			"delete",
			"-f",
			policyUpdateYAML,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the recreate-option-all policy")
		_, err = common.OcHub(
			"delete",
			"-f",
			policyConfigMapYAML,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Updates a Deployment due to immutable fields with recreateOption set to IfRequired", func(ctx context.Context) {
		By("Creating a policy to create the initial Deployment")
		_, err := common.OcHub(
			"apply",
			"-f",
			policyInitialYAML,
		)
		Expect(err).ToNot(HaveOccurred())

		verifyPolicyOnAllClusters(ctx, ns, "recreate-option-initial", "Compliant", defaultTimeoutSeconds)

		By("Deleting the recreate-option-initial policy")
		_, err = common.OcHub(
			"delete",
			"-f",
			policyInitialYAML,
		)
		Expect(err).ToNot(HaveOccurred())

		By("Creating the policy to update immutable fields on the Deployment")
		_, err = common.OcHub(
			"apply",
			"-f",
			policyUpdateYAML,
		)
		Expect(err).ToNot(HaveOccurred())

		clusters := verifyPolicyOnAllClusters(ctx, ns, "recreate-option-update", "NonCompliant", defaultTimeoutSeconds)

		for _, cluster := range clusters {
			Eventually(func(g Gomega) {
				By("Checking the policy message for cluster " + cluster)

				replicatedPolicyName := ns + ".recreate-option-update"
				policyInterface := common.ClientHubDynamic.Resource(common.GvrPolicy).Namespace(cluster)

				policy, err := policyInterface.Get(ctx, replicatedPolicyName, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())

				details, _, err := unstructured.NestedSlice(policy.Object, "status", "details")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(details).To(HaveLen(1))

				templateDetails, ok := details[0].(map[string]interface{})
				g.Expect(ok).To(BeTrue())

				history, _, err := unstructured.NestedSlice(templateDetails, "history")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(history).ToNot(BeEmpty())

				topHistoryItem, ok := history[0].(map[string]interface{})
				g.Expect(ok).To(BeTrue())

				message, _, _ := unstructured.NestedString(topHistoryItem, "message")
				g.Expect(message).To(MatchRegexp(".*cannot be updated, likely due to immutable fields.*"))
			}, defaultTimeoutSeconds, 1).Should(Succeed())
		}

		By("Setting recreateOption IfRequired to update immutable fields on the Deployment")
		_, err = common.OcHub(
			"-n", ns, "patch", "policy", "recreate-option-update", "--type=json", "-p",
			`[{ "op": "add", `+
				`"path": "/spec/policy-templates/0/objectDefinition/spec/object-templates/0/recreateOption", `+
				`"value": 'IfRequired' }]`,
		)
		Expect(err).ToNot(HaveOccurred())

		verifyPolicyOnAllClusters(ctx, ns, "recreate-option-update", "Compliant", defaultTimeoutSeconds)
	})

	It("Recreates a ConfigMap on update with recreateOption set to All", func(ctx context.Context) {
		By("Creating the policy to create the ConfigMap")
		_, err := common.OcHub("apply", "-f", policyConfigMapYAML)
		Expect(err).ToNot(HaveOccurred())

		verifyPolicyOnAllClusters(ctx, ns, "recreate-option-all", "Compliant", defaultTimeoutSeconds)

		configMap, err := clientManaged.CoreV1().ConfigMaps("default").Get(
			ctx, "recreate-option-all", metav1.GetOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		oldUID := configMap.GetUID()

		By("Updating the policy to update the ConfigMap")
		_, err = common.OcHub(
			"-n", ns, "patch", "policy", "recreate-option-all", "--type=json", "-p",
			`[{ "op": "replace", `+
				`"path": "/spec/policy-templates/0/objectDefinition/spec/object-templates/0/`+
				`objectDefinition/data/city", `+
				`"value": 'Durham' }]`,
		)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			configMap, err := clientManaged.CoreV1().ConfigMaps("default").Get(
				ctx, "recreate-option-all", metav1.GetOptions{},
			)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(configMap.UID).ToNot(Equal(oldUID), "expected a new UID on the ConfigMap")
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		// Verify the policy is compliant on all clusters now that the ConfigMap has updated at least on one managed
		// cluster.
		verifyPolicyOnAllClusters(ctx, ns, "recreate-option-all", "Compliant", defaultTimeoutSeconds)
	})
})
