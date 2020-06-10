module github.com/open-cluster-management/governance-policy-framework

go 1.14

require (
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.10.0
	github.com/open-cluster-management/governance-policy-propagator v0.0.0-20200610154314-d8f18e1cd95e
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
)

replace howett.net/plist => github.com/DHowett/go-plist v0.0.0-20181124034731-591f970eefbb
