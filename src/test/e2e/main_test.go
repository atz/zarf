// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
)

var (
	e2e ZarfE2ETest //nolint:gochecknoglobals
)

const (
	applianceModeEnvVar = "APPLIANCE_MODE"
	skipK8sEnvVar       = "SKIP_K8S"
)

// TestMain lets us customize the test run. See https://medium.com/goingogo/why-use-testmain-for-testing-in-go-dafb52b406bc.
func TestMain(m *testing.M) {
	// Work from the root directory of the project
	os.Chdir("../../../")

	// K3d use the intern package, which requires this to be set in go 1.19
	os.Setenv("ASSUME_NO_MOVING_GC_UNSAFE_RISK_IT_WITH", "go1.19")

	retCode, err := doAllTheThings(m)
	if err != nil {
		fmt.Println(err) //nolint:forbidigo
	}
	os.Exit(retCode)
}

// doAllTheThings just wraps what should go in TestMain. It's in its own function so it can
// [a] Not have a bunch of `os.Exit()` calls in it
// [b] Do defers properly
// [c] Handle errors cleanly
//
// It returns the return code passed from `m.Run()` and any error thrown.
func doAllTheThings(m *testing.M) (int, error) {
	var err error

	// Set up constants in the global variable that all the tests are able to access
	e2e.arch = config.GetArch()
	e2e.zarfBinPath = path.Join("build", GetCLIName())
	e2e.applianceMode = os.Getenv(applianceModeEnvVar) == "true"
	e2e.runClusterTests = os.Getenv(skipK8sEnvVar) != "true"

	// Validate that the Zarf binary exists. If it doesn't that means the dev hasn't built it, usually by running
	// `make build-cli`
	_, err = os.Stat(e2e.zarfBinPath)
	if err != nil {
		return 1, fmt.Errorf("zarf binary %s not found", e2e.zarfBinPath)
	}

	// Run the tests, with the cluster cleanup being deferred to the end of the function call
	returnCode := m.Run()

	return returnCode, nil
}
