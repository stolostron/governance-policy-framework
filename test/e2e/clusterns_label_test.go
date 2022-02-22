// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test cluster ns creation", func() {
	It("Should contain label on cluster ns", func() {
		ns, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), clusterNamespace, metav1.GetOptions{})
		Expect(err).To(BeNil())
		Expect(ns.GetLabels()["policy.open-cluster-management.io/isClusterNamespace"]).To(Equal("true"))
	})
})
