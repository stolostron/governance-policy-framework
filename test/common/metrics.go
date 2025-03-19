// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// GetWithToken makes a GET request to the given target, and puts the
// token in an Authorization header if non-empty. The HTTP client has
// a sane timeout, and will skip verifying the target certificate.
func GetWithToken(url, authToken string) (body, status string, err error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}

	if authToken != "" {
		req.Header.Add("Authorization", "Bearer "+authToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)

	return string(bodyBytes), resp.Status, err
}

// MatchMetricValue returns a GomegaMatcher to look through the full
// response from a metrics endpoint and check for a specific data point.
func MatchMetricValue(name, label, value string) types.GomegaMatcher {
	regex := `(?m)`              // multiline mode (makes ^ and $ work)
	regex += "^" + name + "{"    // full name of metric at start of line
	regex += ".*" + label + ".*" // label somewhere inside the {...}
	regex += "} " + value + "$"  // value at the end of line

	return MatchRegexp(regex)
}
