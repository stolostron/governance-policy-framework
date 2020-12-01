module github.com/open-cluster-management/governance-policy-framework

go 1.14

require (
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/open-cluster-management/governance-policy-propagator v0.0.0-20201201163302-eb54c6802d3d
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
