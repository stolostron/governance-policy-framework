// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

func verifyPolicyOnAllClusters(
	ctx context.Context, namespace string, policy string, compliance string, timeout int,
) (
	clusters []string,
) {
	GinkgoHelper()

	By(fmt.Sprintf("Verifying that the policy %s/%s is %s", namespace, policy, compliance))

	Eventually(func(g Gomega) {
		clusters = confirmComplianceOnAllClusters(ctx, namespace, policy, compliance)(g)
	}, timeout, 1).Should(Succeed())

	return clusters
}

func confirmComplianceOnAllClusters(
	ctx context.Context, namespace string, policy string, compliance string,
) func(g Gomega) []string {
	return func(g Gomega) []string {
		GinkgoHelper()

		clusters := []string{}

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

		return clusters
	}
}
