// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/open-cluster-management/governance-policy-framework/test/common"
)

var (
	userNamespace         string
	clusterNamespace      string
	kubeconfigHub         string
	kubeconfigManaged     string
	defaultTimeoutSeconds int
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface
	getComplianceState    func(policyName string) func() interface{}
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("../../test-output/integration.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Policy Framework repo integration Suite", []Reporter{junitReporter})
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {
	By("Setup hub and managed client")
	kubeconfigHub = common.KubeconfigHub
	kubeconfigManaged = common.KubeconfigManaged
	userNamespace = common.UserNamespace
	clusterNamespace = common.ClusterNamespace
	defaultTimeoutSeconds = common.DefaultTimeoutSeconds

	clientHub = common.NewKubeClient("", kubeconfigHub, "")
	clientHubDynamic = common.NewKubeClientDynamic("", kubeconfigHub, "")
	clientManaged = common.NewKubeClient("", kubeconfigManaged, "")
	clientManagedDynamic = common.NewKubeClientDynamic("", kubeconfigManaged, "")

	getComplianceState = func(policyName string) func() interface{} {
		return common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace)
	}

	By("Create Namespace if needed")
	_, err := clientHub.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: userNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		Expect(errors.IsAlreadyExists(err)).Should(BeTrue())
	}
	Expect(clientHub.CoreV1().Namespaces().Get(context.TODO(), userNamespace, metav1.GetOptions{})).NotTo(BeNil())
})
