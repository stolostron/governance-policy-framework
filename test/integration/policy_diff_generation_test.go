// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test diff generation",
	Ordered, Label("BVT"), func() {
		const (
			policyConfigMapName = "policy-diff-gen-configmap"
			policyConfigMapPath = "../resources/diff_generation/diff_generation.yaml"
			configMapName       = "diff-gen-configmap"
		)

		It("ConfigMap "+configMapName+" should be created", func() {
			_, err := common.OcManaged(
				"create", "configmap", configMapName, "-n=default", "--from-literal=fish=tuna",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(policyConfigMapName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := common.OcHub(
				"apply", "-f", policyConfigMapPath, "-n", userNamespace,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Patching placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+policyConfigMapName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking that " + policyConfigMapName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyConfigMapName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It(policyConfigMapName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyConfigMapName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It(policyConfigMapName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyConfigMapName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("should log the diff in the config-policy-controller", func() {
			By("Parsing the logs of the config-policy-controller on the managed cluster")
			configPolicyPodName, err := common.OcManaged("get", "pod", "-n", common.OCMAddOnNamespace,
				"-l=app=config-policy-controller", "-o=name")
			Expect(err).ToNot(HaveOccurred())

			controllerLogs, err := common.OcManaged("logs", "-n", common.OCMAddOnNamespace,
				strings.TrimSpace(configPolicyPodName))
			Expect(err).ToNot(HaveOccurred())

			diffLogRegEx := regexp.MustCompile("(?m)Logging the diff:\n([^\t].*\n)+\t{\"policy\":.*")
			diffLogs := diffLogRegEx.FindAllString(controllerLogs, -1)
			Expect(diffLogs).ToNot(BeEmpty(), "config-policy-controller logs should contain a diff")

			diffLog := `Logging the diff:
--- default/` + configMapName + ` : existing
+++ default/` + configMapName + ` : updated
@@ -2,3 +2,4 @@
 data:
-  fish: tuna
+  cephalopod: squid
+  fish: marlin
 kind: ConfigMap
	{"policy": "` + policyConfigMapName + `", "name": "` + configMapName +
				`", "namespace": "default", "resource": "configmaps"}`
			Expect(diffLogs[0]).To(Equal(diffLog))
		})

		AfterAll(func() {
			_, err := common.OcManaged(
				"delete", "configmap", configMapName, "-n=default",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcHub(
				"delete", "-f", policyConfigMapPath, "-n", userNamespace,
			)
			Expect(err).ToNot(HaveOccurred())
		})
	})
