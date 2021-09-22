// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
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
})
