// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// Note that this is set to Serial since the cleanup involves restarting the propagator to clear its cache of database
// IDs.
var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the compliance history API", Ordered, Serial, Label("BVT"), func() {
	var eventsEndpoint string
	var token string
	const saName = "compliance-history-user"

	httpClient := http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	BeforeAll(func(ctx context.Context) {
		By("Setting up Postgres in " + common.OCMNamespace)
		_, err := common.OcHub(
			"apply",
			"-n",
			common.OCMNamespace,
			"-f",
			"../resources/compliance_history/compliance-api-prerequisites.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Creating a service account in the default namespace with access to all managed clusters")
		_, err = common.OcHub("apply", "-f", "../resources/compliance_history/service_account.yaml")
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			By("Getting the service account token")
			secret, err := common.ClientHub.CoreV1().Secrets("default").Get(ctx, saName, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(secret.Data["token"]).ToNot(BeNil())

			token = string(secret.Data["token"])
		}, common.DefaultTimeoutSeconds, 1).Should(Succeed())

		var routeHost string

		Eventually(func(g Gomega) {
			By("Getting the compliance history API route")
			route, err := common.ClientHubDynamic.Resource(common.GvrRoute).Namespace(common.OCMNamespace).Get(
				ctx, "governance-history-api", metav1.GetOptions{},
			)
			g.Expect(err).ToNot(HaveOccurred())

			routeHost, _, _ = unstructured.NestedString(route.Object, "spec", "host")

			g.Expect(routeHost).ToNot(BeEmpty())
		}, common.DefaultTimeoutSeconds, 1).Should(Succeed())

		eventsEndpoint = fmt.Sprintf("https://%s/api/v1/compliance-events", routeHost)

		By("Creating the AddOnDeploymentConfig with the complianceHistoryAPIURL variable")
		addonDeploymentConfig := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "addon.open-cluster-management.io/v1alpha1",
				"kind":       "AddOnDeploymentConfig",
				"metadata": map[string]interface{}{
					"name": "governance-policy-framework",
				},
				"spec": map[string]interface{}{
					"customizedVariables": []interface{}{
						map[string]interface{}{
							"name":  "complianceHistoryAPIURL",
							"value": "https://" + routeHost,
						},
					},
				},
			},
		}

		_, err = clientHubDynamic.Resource(common.GvrAddonDeploymentConfig).Namespace(common.OCMNamespace).Create(
			ctx, &addonDeploymentConfig, metav1.CreateOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		By("Setting the governance-policy-framework ClusterManagementAddOn to use the AddOnDeploymentConfig")
		cma, err := clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Get(
			ctx, "governance-policy-framework", metav1.GetOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		supportedConfigs := []interface{}{
			map[string]interface{}{
				"group":    "addon.open-cluster-management.io",
				"resource": "addondeploymentconfigs",
				"defaultConfig": map[string]interface{}{
					"name":      "governance-policy-framework",
					"namespace": common.OCMNamespace,
				},
			},
		}

		err = unstructured.SetNestedField(cma.Object, supportedConfigs, "spec", "supportedConfigs")
		Expect(err).ToNot(HaveOccurred())

		_, err = clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Update(ctx, cma, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the compliance endpoint is up")
		Eventually(func(g Gomega) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventsEndpoint, nil)
			g.Expect(err).ToNot(HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := httpClient.Do(req)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(resp.StatusCode).To(
				Equal(http.StatusOK), "expected the compliance API to return the 200 status code",
			)
		}, common.DefaultTimeoutSeconds*2, 1).Should(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		By("Deleting the policy")
		_, err := common.OcHub(
			"delete",
			"-f",
			"../resources/compliance_history/policy.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting Postgres")
		_, err = common.OcHub(
			"delete",
			"-n",
			common.OCMNamespace,
			"-f",
			"../resources/compliance_history/compliance-api-prerequisites.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the service account in the default namespace")
		_, err = common.OcHub(
			"delete", "-f", "../resources/compliance_history/service_account.yaml", "--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the AddOnDeploymentConfig")
		err = clientHubDynamic.Resource(common.GvrAddonDeploymentConfig).Namespace(common.OCMNamespace).Delete(
			ctx, "governance-policy-framework", metav1.DeleteOptions{},
		)
		if !k8serrors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("Creates a policy with a compliant and noncompliant configuration policy", func(ctx context.Context) {
		const policyName = "compliance-history-test"
		const policyNS = "open-cluster-management-global-set"

		now := time.Now().Add(-1 * time.Second).Format(time.RFC3339)

		By("Creating the policy")
		_, err := common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		var parentPolicy *unstructured.Unstructured
		var clusters []string

		Eventually(func(g Gomega) {
			By("Checking to see if the policy is noncompliant on all clusters")
			var err error
			clusters = []string{}

			parentPolicy, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(policyNS).Get(
				ctx, policyName, metav1.GetOptions{},
			)
			g.Expect(err).ToNot(HaveOccurred())

			perClusterStatus, _, _ := unstructured.NestedSlice(parentPolicy.Object, "status", "status")
			g.Expect(perClusterStatus).ToNot(BeEmpty(), "no cluster status was available on the parent policy")

			for _, clusterStatus := range perClusterStatus {
				clusterStatus, ok := clusterStatus.(map[string]interface{})
				g.Expect(ok).To(BeTrue(), "the cluster status was not the right type")

				g.Expect(clusterStatus["compliant"]).To(Equal("NonCompliant"))
				g.Expect(clusterStatus["clustername"]).ToNot(BeEmpty())
				clusters = append(clusters, clusterStatus["clustername"].(string))
			}
		}, common.DefaultTimeoutSeconds, 1).Should(Succeed())

		expectedEvents := len(clusters) * 2

		By(fmt.Sprintf("Verifying that there are %d compliance events for the parent policy", expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				url := fmt.Sprintf(
					"%s?cluster.name=%s&parent_policy.name=%s&event.timestamp_after=%s",
					eventsEndpoint,
					cluster,
					policyName,
					now,
				)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				g.Expect(err).ToNot(HaveOccurred())

				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := httpClient.Do(req)
				g.Expect(err).ToNot(HaveOccurred())

				defer resp.Body.Close()

				g.Expect(resp.StatusCode).To(
					Equal(http.StatusOK), "expected the compliance API to return the 200 status code",
				)

				body, err := io.ReadAll(resp.Body)
				g.Expect(err).ToNot(HaveOccurred())

				respJSON := map[string]any{}

				err = json.Unmarshal(body, &respJSON)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(respJSON["data"]).To(
					HaveLen(2), fmt.Sprintf("expected cluster %s to have two events", cluster),
				)

				events := respJSON["data"].([]interface{})
				event1 := events[0].(map[string]interface{})
				event2 := events[1].(map[string]interface{})

				if event1["policy"].(map[string]interface{})["name"].(string) == "default-namespace-must-exist" {
					g.Expect(event1["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
					g.Expect(event2["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
				} else {
					g.Expect(event1["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
					g.Expect(event2["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
				}

			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())
	})
})
