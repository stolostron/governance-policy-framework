// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test standalone hub templating", Ordered, Serial, Label("SVT"), func() {
	const (
		standaloneConfigPolYAML    = "../resources/standalone_hubtemplates/standalone-hubtemplates-test.yaml"
		standaloneConfigPolUpdated = "../resources/standalone_hubtemplates/standalone-hubtemplates-test-updated.yaml"
		standaloneConfigPolName    = "standalone-hubtemplates-test"
		standaloneConfigPolNS      = "open-cluster-management-policies"

		addonPolYAML = "../resources/standalone_hubtemplates/config-standalone-addon.yaml"
		addonPolName = "config-standalone-addon"

		rbacPolYAML = "../resources/standalone_hubtemplates/config-standalone-rbac.yaml"
		rbacPolName = "config-standalone-rbac"

		hubClusterNamespace = "local-cluster"
	)

	AfterAll(func(ctx SpecContext) {
		By("Deleting policies")

		_, err := common.OcHub("delete", "-f", standaloneConfigPolYAML, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
			standaloneConfigPolName, standaloneConfigPolNS, false, defaultTimeoutSeconds)

		common.DoCleanupPolicy(addonPolYAML, common.GvrConfigurationPolicy)
		common.DoCleanupPolicy(rbacPolYAML, common.GvrConfigurationPolicy)
	})

	It("Cannot resolve the policy when the addon is not enabled", func(ctx context.Context) {
		_, err := common.OcHub("create", "-f", standaloneConfigPolYAML)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("NonCompliant"))

		Consistently(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			details, _, _ := unstructured.NestedSlice(plc.Object, "status", "compliancyDetails")
			Expect(details).To(HaveLen(1))

			conds, _, _ := unstructured.NestedSlice(details[0].(map[string]interface{}), "conditions")
			Expect(conds).To(HaveLen(1))

			msg, _, _ := unstructured.NestedString(conds[0].(map[string]interface{}), "message")

			return msg
		}, 10, 2).Should(ContainSubstring("governance-standalone-hub-templating addon must be enabled"))
	})

	It("Is able to successfully enforce the addon policy", func(ctx context.Context) {
		_, err := common.OcHub("create", "-f", addonPolYAML)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				addonPolName, hubClusterNamespace, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("Compliant"))
	})

	It("Resolves the template after the addon is enabled", func(ctx context.Context) {
		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("Compliant"))
	})

	It("Cannot resolve other resources without additional RBAC", func(ctx context.Context) {
		_, err := common.OcHub("apply", "-f", standaloneConfigPolUpdated)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("NonCompliant"))

		Consistently(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			details, _, _ := unstructured.NestedSlice(plc.Object, "status", "compliancyDetails")
			Expect(details).To(HaveLen(1))

			conds, _, _ := unstructured.NestedSlice(details[0].(map[string]interface{}), "conditions")
			Expect(conds).To(HaveLen(1))

			msg, _, _ := unstructured.NestedString(conds[0].(map[string]interface{}), "message")

			return msg
		}, 10, 2).Should(ContainSubstring(`cannot list resource "configmaps"`))
	})

	It("Is able to successfully configure additional RBAC permissions", func(ctx context.Context) {
		_, err := common.OcHub("create", "-f", rbacPolYAML)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				rbacPolName, hubClusterNamespace, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("Compliant"))
	})

	It("Resolves the template after RBAC is configured", func(ctx context.Context) {
		Eventually(func() string {
			plc := utils.GetWithTimeout(clientHubDynamic, common.GvrConfigurationPolicy,
				standaloneConfigPolName, standaloneConfigPolNS, true, defaultTimeoutSeconds)

			compliance, _, _ := unstructured.NestedString(plc.Object, "status", "compliant")

			return compliance
		}, defaultTimeoutSeconds, 2).Should(Equal("Compliant"))
	})
})
