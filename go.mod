module github.com/open-cluster-management/governance-policy-framework

go 1.14

require (
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/open-cluster-management/governance-policy-propagator v0.0.0-20210330170457-7ccd8538eb95
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	howett.net/plist => github.com/DHowett/go-plist v0.0.0-20181124034731-591f970eefbb
	k8s.io/client-go => k8s.io/client-go v0.18.3 // Required by prometheus-operator
)