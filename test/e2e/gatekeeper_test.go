// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// GetClusterLevelWithTimeout keeps polling to get the object for timeout seconds
// until wantFound is met (true for found, false for not found)
func GetClusterLevelWithTimeout(
	clientHubDynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	name string,
	wantFound bool,
	timeout int,
) *unstructured.Unstructured {
	if timeout < 1 {
		timeout = 1
	}
	var obj *unstructured.Unstructured

	Eventually(func() error {
		var err error
		namespace := clientHubDynamic.Resource(gvr)
		obj, err = namespace.Get(context.TODO(), name, metav1.GetOptions{})
		if wantFound && err != nil {
			return err
		}
		if !wantFound && err == nil {
			return fmt.Errorf("expected to return IsNotFound error")
		}
		if !wantFound && err != nil && !errors.IsNotFound(err) {
			return err
		}

		return nil
	}, timeout, 1).Should(BeNil())

	if wantFound {
		return obj
	}

	return nil
}

const GKOPolicyYaml string = "../resources/gatekeeper/policy-gatekeeper-operator.yaml"

var _ = Describe("Test gatekeeper", Ordered, func() {
	const gatekeeperNS = "gatekeeper-system"

	Describe("Test gatekeeper operator", func() {
		const GKOPolicyName string = "policy-gatekeeper-operator"
		It("gatekeeper operator policy should be created on managed", func() {
			common.DoCreatePolicyTest(GKOPolicyYaml)
		})
		It("should create gatekeeper pods on managed cluster", func() {
			By("Checking number of pods in gatekeeper-system ns")
			utils.ListWithTimeoutByNamespace(
				clientManagedDynamic,
				common.GvrPod,
				metav1.ListOptions{},
				"gatekeeper-system",
				6,
				true,
				240,
			)
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				_, err := utils.KubectlWithOutput(
					"-n", gatekeeperNS,
					"get", "pods",
					"--kubeconfig="+kubeconfigManaged,
				)
				Expect(err).To(BeNil())
				_, err = utils.KubectlWithOutput(
					"-n", gatekeeperNS,
					"logs",
					"deployment/gatekeeper-operator-controller",
					"-c", "manager",
				)
				Expect(err).To(BeNil())
				common.OutputDebugInfo("gatekeeper operator", kubeconfigHub)
			}
		})
	})
})
