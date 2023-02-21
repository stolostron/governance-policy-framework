//go:build e2e
// +build e2e

// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package main

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// TestRunMain wraps the main() function in order to build a test binary and collection coverage for
// E2E/Integration tests. Controller CLI flags are also passed in here.
func TestRunMain(t *testing.T) {
	// Modifying the args seems necessary to make the test binary and cobra happy.
	os.Args = append([]string{
		"governance-policy-addon-controller",
		"controller",
		"--namespace=governance-policy-addon-controller-system",
	}, os.Args[1:]...)

	// Run main in a separate goroutine
	go main()

	// The test will end (and pass) via an external signal
	stopHandler := make(chan os.Signal, 1)
	signal.Notify(stopHandler, syscall.SIGUSR1)
	<-stopHandler
}
