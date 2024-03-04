// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// Note that this is set to Serial since the cleanup involves restarting the propagator to clear its cache of database
// IDs.
var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the compliance history API", Ordered, Serial, Label("BVT"), func() {
	const policyNS = "open-cluster-management-global-set"
	var eventsEndpoint string
	var token string
	const saName = "compliance-history-user"

	httpClient := http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	listComplianceEvents := func(ctx context.Context, queryArgs ...string) ([]interface{}, error) {
		url := eventsEndpoint

		if len(queryArgs) > 0 {
			url += "?" + strings.Join(queryArgs, "&")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Got a %d status code", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respJSON := map[string]any{}

		err = json.Unmarshal(body, &respJSON)
		if err != nil {
			return nil, err
		}

		return respJSON["data"].([]interface{}), nil
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
		By("Deleting Postgres")
		_, err := common.OcHub(
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
		const policyName = "compliance-api-configpolicy"

		startOfTest := time.Now().UTC().Format(time.RFC3339Nano)

		By("Creating the policy")
		_, err := common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		clusters := verifyPolicyOnAllClusters(ctx, policyNS, policyName, "NonCompliant", defaultTimeoutSeconds*2)

		expectedEvents := len(clusters) * 2

		By(fmt.Sprintf("Verifying that there are %d compliance events for the parent policy", expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+startOfTest,
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(2), fmt.Sprintf("expected cluster %s to have two events", cluster))

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

		now := time.Now().UTC().Format(time.RFC3339Nano)

		By("Deleting a policy template to verify the disabled event")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(policyNS).Patch(
			ctx,
			policyName,
			k8stypes.JSONPatchType,
			[]byte(
				`[{"op": "remove", "path": "/spec/policy-templates/1"}]`,
			),
			metav1.PatchOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that there is a single disabled compliance events for the parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx,
					"cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+now,
					"event.compliance=Disabled",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one disabled events", cluster))

				event := events[0].(map[string]interface{})
				g.Expect(event["policy"].(map[string]interface{})["name"]).To(Equal(
					"does-not-exist-namespace-must-exist",
				))
				eventDetails := event["event"].(map[string]interface{})

				g.Expect(eventDetails["compliance"]).To(Equal("Disabled"))
				g.Expect(eventDetails["message"]).To(Equal("The policy was removed from the parent policy"))
			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		now = time.Now().UTC().Format(time.RFC3339Nano)

		_, err = common.OcHub(
			"delete",
			"-f",
			"../resources/compliance_history/policy.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that there is a single disabled compliance event for the parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx,
					"cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+now,
					"event.compliance=Disabled",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one disabled events", cluster))

				event := events[0].(map[string]interface{})
				g.Expect(event["policy"].(map[string]interface{})["name"]).To(Equal("default-namespace-must-exist"))
				eventDetails := event["event"].(map[string]interface{})

				g.Expect(eventDetails["compliance"]).To(Equal("Disabled"))
				g.Expect(eventDetails["message"]).To(Equal(
					"The policy was removed because the parent policy no longer applies to this cluster",
				))
			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())
	})

	It("Creates a policy with a Gatekeeper constraint", func(ctx context.Context) {
		const installGKPolicyName = "compliance-api-install-gk"
		const uninstallGKPolicyName = "compliance-api-uninstall-gk"
		const gkTargetNS = "compliance-api-test"
		const invalidConfigMapName = "compliance-api-test"
		const gkPolicyName = "compliance-api-gk"

		By("Creating the " + installGKPolicyName + " policy to install Gatekeeper")
		_, err := common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-install-gk.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + installGKPolicyName + " policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-install-gk.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the " + uninstallGKPolicyName + " policy to uninstall Gatekeeper")
			_, err = common.OcHub(
				"apply",
				"-f",
				"../resources/compliance_history/policy-uninstall-gk.yaml",
			)
			Expect(err).ToNot(HaveOccurred())

			_ = verifyPolicyOnAllClusters(ctx, policyNS, uninstallGKPolicyName, "Compliant", defaultTimeoutSeconds*2)

			By("Deleting the " + uninstallGKPolicyName + " policy")
			_, err = common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-uninstall-gk.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		By("Creating the " + gkTargetNS + " namespace")
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: gkTargetNS,
			},
		}
		_, err = clientHub.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Creating an invalid ConfigMap")
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: invalidConfigMapName,
			},
		}
		configmap, err = clientHub.CoreV1().ConfigMaps(gkTargetNS).Create(ctx, configmap, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + gkTargetNS + " namespace")
			err := clientHub.CoreV1().Namespaces().Delete(ctx, gkTargetNS, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		// The audit pod can take a while to become healthy.
		_ = verifyPolicyOnAllClusters(ctx, policyNS, installGKPolicyName, "Compliant", defaultTimeoutSeconds*6)

		now := time.Now().UTC().Format(time.RFC3339Nano)

		By("Creating the " + gkPolicyName + " policy to add Gatekeeper constraints")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-gk.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + gkPolicyName + " policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-gk.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		clusters := verifyPolicyOnAllClusters(ctx, policyNS, gkPolicyName, "NonCompliant", defaultTimeoutSeconds*2)

		expectedEvents := len(clusters) * 3

		By(fmt.Sprintf("Verifying that there are %d compliance events for the parent policy", expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+gkPolicyName, "event.timestamp_after="+now,
				)
				g.Expect(err).ToNot(HaveOccurred())

				policyToEventDetails := map[string][]map[string]interface{}{}

				for _, event := range events {
					eventTyped := event.(map[string]interface{})

					policyName := eventTyped["policy"].(map[string]interface{})["name"].(string)
					eventDetails := eventTyped["event"].(map[string]interface{})

					policyToEventDetails[policyName] = append(policyToEventDetails[policyName], eventDetails)
				}

				// Ensure the ConstraintTemplate has 1 event
				g.Expect(policyToEventDetails["complianceapitest"]).To(HaveLen(1))
				g.Expect(policyToEventDetails["complianceapitest"][0]["compliance"]).To(Equal("Compliant"))
				expectedMsg := "ConstraintTemplate complianceapitest was created successfully"
				g.Expect(policyToEventDetails["complianceapitest"][0]["message"]).To(Equal(expectedMsg))

				// Ensure the constraint has 2 or more events. More than one template-error compliance event can
				// be set based on race conditions.
				lenOk := len(policyToEventDetails["compliance-api"]) >= 2
				g.Expect(lenOk).To(
					BeTrue(),
					fmt.Sprintf(
						"Expected the compliance-api policy to have 2 or more compliance events, got %d",
						len(policyToEventDetails["compliance-api"]),
					),
				)
				// Sorted by timestamp in descending order
				g.Expect(policyToEventDetails["compliance-api"][0]["compliance"]).To(Equal("NonCompliant"))
				expectedMsg = "warn - All configmaps must have a 'my-gk-test' label (on ConfigMap " +
					"compliance-api-test/compliance-api-test)"
				g.Expect(policyToEventDetails["compliance-api"][0]["message"]).To(Equal(expectedMsg))

				// All other compliance events should be a template-error
				for _, eventDetails := range policyToEventDetails["compliance-api"][1:] {
					g.Expect(eventDetails["compliance"]).To(Equal("NonCompliant"))
					expectedMsg = "template-error; Mapping not found, check if the required ConstraintTemplate has " +
						"been deployed: the resource version was not found: constraints.gatekeeper.sh/v1beta1, " +
						"Kind=ComplianceAPITest"
					g.Expect(eventDetails["message"]).To(Equal(expectedMsg))
				}
			}
			// It can take a while for the Gatekeeper audit pod to produce audit results.
		}, defaultTimeoutSeconds*4, 1).Should(Succeed())

		By("Updating the ConfigMap to be valid")
		configmap.Labels = map[string]string{"my-gk-test": "a-value"}
		_, err = clientHub.CoreV1().ConfigMaps(gkTargetNS).Update(ctx, configmap, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that a compliant event was sent")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+gkPolicyName, "event.timestamp_after="+now,
					"policy.name=compliance-api", "event.compliance=Compliant",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one compliant event", cluster))

				eventDetails := events[0].(map[string]interface{})["event"].(map[string]interface{})
				g.Expect(eventDetails["compliance"]).To(Equal("Compliant"))
				g.Expect(eventDetails["message"]).To(Equal("The constraint has no violations"))
			}
		}, defaultTimeoutSeconds*2, 1).Should(Succeed())
	})
})

func verifyPolicyOnAllClusters(
	ctx context.Context, namespace string, policy string, compliance string, timeout int, //nolint: unparam
) (
	clusters []string,
) {
	By(fmt.Sprintf("Verifying that the policy %s/%s is %s", namespace, policy, compliance))

	EventuallyWithOffset(1, func(g Gomega) {
		clusters = []string{}

		parentPolicy, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(namespace).Get(
			ctx, policy, metav1.GetOptions{},
		)
		g.Expect(err).ToNot(HaveOccurred())

		perClusterStatus, _, _ := unstructured.NestedSlice(parentPolicy.Object, "status", "status")
		g.Expect(perClusterStatus).ToNot(BeEmpty(), "no cluster status was available on the parent policy")

		for _, clusterStatus := range perClusterStatus {
			clusterStatus, ok := clusterStatus.(map[string]interface{})
			g.Expect(ok).To(BeTrue(), "the cluster status was not the right type")

			g.Expect(clusterStatus["compliant"]).To(Equal(compliance))
			g.Expect(clusterStatus["clustername"]).ToNot(BeEmpty())
			clusters = append(clusters, clusterStatus["clustername"].(string))
		}
	}, timeout, 1).Should(Succeed())

	return
}
