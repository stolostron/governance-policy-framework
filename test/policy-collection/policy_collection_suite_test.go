// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	userNamespace            string
	clusterNamespace         string
	clientHub                kubernetes.Interface
	clientHubDynamic         dynamic.Interface
	clientManaged            kubernetes.Interface
	clientManagedDynamic     dynamic.Interface
	gvrClusterVersion        schema.GroupVersionResource
	gvrComplianceSuite       schema.GroupVersionResource
	gvrComplianceScan        schema.GroupVersionResource
	gvrComplianceCheckResult schema.GroupVersionResource
	gvrPolicy                schema.GroupVersionResource
	gvrPlacementBinding      schema.GroupVersionResource
	kubeconfigHub            string
	kubeconfigManaged        string
	defaultTimeoutSeconds    int
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("../../test-output/policy-collection.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Policy collection repo e2e Suite", []Reporter{junitReporter})
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfigHub, "kubeconfig_hub", "../../kubeconfig_hub", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&kubeconfigManaged, "kubeconfig_managed", "../../kubeconfig_managed", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&userNamespace, "user_namespace", "policy-test", "ns on hub to create root policy")
	flag.StringVar(&clusterNamespace, "cluster_namespace", "local-cluster", "cluster ns name")
	flag.IntVar(&defaultTimeoutSeconds, "timeout_seconds", 30, "Timeout seconds for assertion")
}

var _ = BeforeSuite(func() {
	By("Setup hub and managed client")
	gvrClusterVersion = schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "clusterversions"}
	gvrComplianceScan = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancescans"}
	gvrComplianceSuite = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancesuites"}
	gvrComplianceCheckResult = schema.GroupVersionResource{Group: "compliance.openshift.io", Version: "v1alpha1", Resource: "compliancecheckresults"}
	gvrPolicy = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies"}
	gvrPlacementBinding = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "placementbindings"}
	clientHub = NewKubeClient("", kubeconfigHub, "")
	clientHubDynamic = NewKubeClientDynamic("", kubeconfigHub, "")
	clientManaged = NewKubeClient("", kubeconfigManaged, "")
	clientManagedDynamic = NewKubeClientDynamic("", kubeconfigManaged, "")
	By("Create Namesapce if needed")
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

func NewKubeClient(url, kubeconfig, context string) kubernetes.Interface {
	klog.V(5).Infof("Create kubeclient for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func NewKubeClientDynamic(url, kubeconfig, context string) dynamic.Interface {
	klog.V(5).Infof("Create kubeclient dynamic for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func LoadConfig(url, kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	klog.V(5).Infof("Kubeconfig path %s\n", kubeconfig)
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		if context == "" {
			return clientcmd.BuildConfigFromFlags(url, kubeconfig)
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{
				CurrentContext: context,
			}).ClientConfig()
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		klog.V(5).Infof("clientcmd.BuildConfigFromFlags for url %s using %s\n", url, filepath.Join(usr.HomeDir, ".kube", "config"))
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")

}
