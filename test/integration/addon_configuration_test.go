// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test add-on configuration", Serial, Label("SVT"), func() {
	BeforeEach(func(ctx SpecContext) {
		DeferCleanup(func() {
			_, _ = common.OcHub("delete", "namespace", "grc-addon-config-test")
		})

		By("Deploying AddonDeploymentConfig grc-addon-config")
		_, _ = common.OcHub("create", "namespace", "grc-addon-config-test")
		_, err := common.OcHub(
			"apply",
			"-n=grc-addon-config-test",
			"-f=../resources/addon_configuration/addondeploymentconfig.yaml")
		Expect(err).ToNot(HaveOccurred())
	})

	DescribeTable("",
		addonTest,
		Entry(nil, "governance-policy-framework"),
		Entry(nil, "config-policy-controller"),
		Entry(nil, "cert-policy-controller"),
	)
})

var addonTest = func(ctx SpecContext, addOn string) {
	var (
		expectedResources    = `{"limits":{"memory":"1Gi"},"requests":{"memory":"512Mi"}}`
		expectedNodeSelector = `{"kubernetes.io/os":"linux"}`
		expectedTolerations  = `[{"key":"dedicated","operator":"Equal","value":"something-else","effect":"NoSchedule"}]`
	)

	// Restore AddOns regardless of test result
	DeferCleanup(func() {
		_, _ = common.OcHub(
			"patch",
			"managedclusteraddon",
			"-n", clusterNamespace,
			addOn,
			"--type=json",
			"--patch-file=../resources/addon_configuration/mcao_restore_patch.json",
		)
		_, _ = common.OcHub(
			"patch",
			"clustermanagementaddon",
			addOn,
			"--type=json",
			"--patch-file=../resources/addon_configuration/cmao_restore_patch.json",
		)
	})

	fetchDeployment := func(g Gomega) (*appsv1.Deployment, corev1.Container) {
		deployment, err := clientManaged.AppsV1().Deployments(ocmAddonNS).Get(ctx, addOn, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		container := deployment.Spec.Template.Spec.Containers
		g.Expect(container).To(HaveLen(1))

		return deployment, container[0]
	}

	By("Fetching Deployment to determine default configuration")
	baseDeployment, baseContainer := fetchDeployment(Default)

	By("Attaching the AddOnDeploymentConfig to the ManagedClusterAddOn for " + addOn)
	_, err := common.OcHub(
		"patch",
		"managedclusteraddon",
		"-n", clusterNamespace,
		addOn,
		"--type=json",
		"--patch-file=../resources/addon_configuration/mcao_config_patch.json",
	)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func(g Gomega) {
		deployment, container := fetchDeployment(g)
		// Check Resources
		g.Expect(json.Marshal(container.Resources)).Should(BeEquivalentTo(expectedResources))
		// Check NodeSelector
		g.Expect(json.Marshal(deployment.Spec.Template.Spec.NodeSelector)).Should(BeEquivalentTo(expectedNodeSelector))
		// Check Tolerations
		g.Expect(json.Marshal(deployment.Spec.Template.Spec.Tolerations)).Should(BeEquivalentTo(expectedTolerations))
	}, defaultTimeoutSeconds*2, 1).Should(Succeed())

	By("Restoring the ManagedClusterAddOn for " + addOn)
	_, err = common.OcHub(
		"patch",
		"managedclusteraddon",
		"-n", clusterNamespace,
		addOn,
		"--type=json",
		"--patch-file=../resources/addon_configuration/mcao_restore_patch.json",
	)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func(g Gomega) {
		deployment, container := fetchDeployment(g)
		g.Expect(
			deployment.Spec.Template.Spec.NodeSelector,
		).Should(Equal(
			baseDeployment.Spec.Template.Spec.NodeSelector,
		))
		g.Expect(container.Resources).Should(Equal(baseContainer.Resources))
	}, defaultTimeoutSeconds*2, 1).Should(Succeed())

	By("Attaching the AddOnDeploymentConfig to the ClusterManagementAddOn for " + addOn)
	_, err = common.OcHub(
		"patch",
		"clustermanagementaddon",
		addOn,
		"--type=json",
		"--patch-file=../resources/addon_configuration/cmao_config_patch.json",
	)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func(g Gomega) {
		deployment, container := fetchDeployment(g)
		// Check Resources
		g.Expect(json.Marshal(container.Resources)).Should(BeEquivalentTo(expectedResources))
		// Check NodeSelector
		g.Expect(json.Marshal(deployment.Spec.Template.Spec.NodeSelector)).Should(BeEquivalentTo(expectedNodeSelector))
		// Check Tolerations
		g.Expect(json.Marshal(deployment.Spec.Template.Spec.Tolerations)).Should(BeEquivalentTo(expectedTolerations))
	}, defaultTimeoutSeconds*2, 1).Should(Succeed())

	By("Restoring the ManagedClusterAddOn for " + addOn)
	_, err = common.OcHub(
		"patch",
		"clustermanagementaddon",
		addOn,
		"--type=json",
		"--patch-file=../resources/addon_configuration/cmao_restore_patch.json",
	)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func(g Gomega) {
		deployment, container := fetchDeployment(g)
		g.Expect(
			deployment.Spec.Template.Spec.NodeSelector,
		).Should(Equal(
			baseDeployment.Spec.Template.Spec.NodeSelector,
		))
		g.Expect(container.Resources).Should(Equal(baseContainer.Resources))
	}, defaultTimeoutSeconds*2, 1).Should(Succeed())
}
