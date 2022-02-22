// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var (
	clusterNamespace = "managed"

	defaultTimeoutSeconds int
	userNamespace         string
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Framework e2e Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {
	By("Setup hub and managed client")
	userNamespace = common.UserNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds

	clientHub = common.NewKubeClient("", common.KubeconfigHub, "")
	clientHubDynamic = common.NewKubeClientDynamic("", common.KubeconfigHub, "")
	clientManaged = common.NewKubeClient("", common.KubeconfigManaged, "")
	clientManagedDynamic = common.NewKubeClientDynamic("", common.KubeconfigManaged, "")

	By("Create Namespace if needed")
	namespaces := clientHub.CoreV1().Namespaces()
	if _, err := namespaces.Get(context.TODO(), userNamespace, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		Expect(namespaces.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: userNamespace,
			},
		}, metav1.CreateOptions{})).NotTo(BeNil())
	}
	Expect(namespaces.Get(context.TODO(), userNamespace, metav1.GetOptions{})).NotTo(BeNil())
})
