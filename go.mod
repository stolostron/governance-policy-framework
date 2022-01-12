module github.com/stolostron/governance-policy-framework

go 1.16

require (
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/stolostron/governance-policy-propagator v0.0.0-20220112144621-e92336a3af99
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
)

replace (
	github.com/open-cluster-management/api => open-cluster-management.io/api v0.0.0-20200610161514-939cead3902c
	k8s.io/client-go => k8s.io/client-go v0.20.5
)
