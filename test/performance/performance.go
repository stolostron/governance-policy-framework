package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"k8s.io/klog"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const performanceDir = "test/performance"

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithHeader(rt http.RoundTripper) withHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return withHeader{Header: make(http.Header), rt: rt}
}

func (h withHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(h.Header) == 0 {
		return h.rt.RoundTrip(req)
	}

	req = req.Clone(req.Context())
	for k, v := range h.Header {
		req.Header[k] = v
	}

	return h.rt.RoundTrip(req)
}

func query(host string, token string, query string, insecure bool) (time.Time, float64) {
	tr := &http.Transport{}
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	trHeader := WithHeader(tr)
	trHeader.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	cl := &http.Client{Transport: trHeader}
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("https://%s", host),
		Client:  cl,
	})
	if err != nil {
		klog.Exitf("Error creating client: %v\n", err)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, query, time.Now())
	if err != nil {
		klog.Exitf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		klog.Warningf("Prometheus query warnings: %v\n", warnings)
	}

	val := result.(model.Vector)[0]
	metric, err := strconv.ParseFloat(val.Value.String(), 32)
	if err != nil {
		klog.Exitf("Error parsing metric response: %v\n", err)
	}

	return val.Timestamp.Time(), metric
}

func setupMetrics() (token []byte, thanosHost string) {
	// wait for config map to be applied properly before executing command
	secretOutput, err := exec.Command("kubectl", "get", "secret", "-n", "openshift-user-workload-monitoring").CombinedOutput()
	if err != nil || !strings.Contains(string(secretOutput), "prometheus-user-workload-token") {
		klog.V(2).Info("Setting up secret for k8s metrics")
		_, err = exec.Command("kubectl", "apply", "-f", path.Join(performanceDir, "resources/setup/metrics-configmap.yaml")).CombinedOutput()
		if err != nil {
			klog.Exitf("Error applying metrics configMap: %s", err)
		}
		// wait for secret to be created for 2 minutes
		found := false
		for i := 0; i < 120; i += 5 {
			secretOutput, err = exec.Command("kubectl", "get", "secret", "-n", "openshift-user-workload-monitoring").CombinedOutput()
			if err == nil && strings.Contains(string(secretOutput), "prometheus-user-workload-token") {
				found = true
				klog.V(2).Info("Secret found! Waiting an additional 30 seconds for metrics setup to complete.")
				time.Sleep(5 * time.Second)
				break
			}
			klog.V(2).Info("Secret not found, waiting 5 seconds...")
			time.Sleep(5 * time.Second)
		}

		if !found {
			klog.Fatal("Error: Prometheus secret creation timed out")
		}
	}
	secretFinder := regexp.MustCompile("prometheus-user-workload-token-[a-z0-9]{5}")
	secret := secretFinder.FindString(string(secretOutput))
	if secret == "" {
		klog.Exit("Error getting Prometheus metrics secret: secret not found in openshift-user-workload-monitoring ns", err)
	}

	tokenB64, err := exec.Command("kubectl", "get", "secret", secret, "-n", "openshift-user-workload-monitoring", "-o=jsonpath='{.data.token}'").CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from secret: %s", err)
	}
	token, err = base64.StdEncoding.DecodeString(strings.Trim(string(tokenB64), "'"))

	if err != nil {
		klog.Exitf("Error decoding token from secret %s", secret, err)
	}

	thanosB64, err := exec.Command("kubectl", "get", "route", "thanos-querier", "-n", "openshift-monitoring", "-o=jsonpath='{.spec.host}'").CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from secret: %s", err)
	}

	klog.V(2).Info("Setup complete! Token and metrics route retrieved sucessfully")

	return token, strings.Trim(string(thanosB64), "'")
}

func getMetrics(thanosHost string, token string, perBatchSleep int, numPolicies int, insecure bool) metricData {
	// print metrics
	ts, avgController := query(
		thanosHost,
		token,
		fmt.Sprintf("avg_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])", perBatchSleep),
		insecure,
	)
	_, avgApiserver := query(
		thanosHost,
		token,
		fmt.Sprintf("sum(avg_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))", perBatchSleep),
		insecure,
	)
	_, maxController := query(
		thanosHost,
		token,
		fmt.Sprintf("max_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])", perBatchSleep),
		insecure,
	)
	_, maxApiserver := query(
		thanosHost,
		token,
		fmt.Sprintf("sum(max_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))", perBatchSleep),
		insecure,
	)

	// log metrics
	klog.V(2).Infof("CPU utilization over the past %d minutes:", perBatchSleep)
	klog.V(2).Infof("config policy controller: %.5f / %.5f (avg/max)", avgController, maxController)
	klog.V(2).Infof("kube API server: %.5f / %.5f (avg/max)", avgApiserver, maxApiserver)

	return metricData{
		timestamp:     ts.Format("15:04:05"),
		numPolicies:   numPolicies,
		controllerAvg: avgController,
		controllerMax: maxController,
		apiserverAvg:  avgApiserver,
		apiserverMax:  maxApiserver,
	}
}

func genUniquePolicy(inFilename string, outFilename string) error {
	input, err := ioutil.ReadFile(inFilename)
	if err != nil {
		return err
	}

	output := strings.ReplaceAll(string(input), "[ID]", uuid.New().String())

	err = ioutil.WriteFile(outFilename, []byte(output), 0644)
	if err != nil {
		return err
	}

	return nil
}

type metricData struct {
	timestamp     string
	numPolicies   int
	controllerAvg float64
	controllerMax float64
	apiserverAvg  float64
	apiserverMax  float64
}

// pretty print table of results to stdout
func printTable(data []metricData) {
	table := tabwriter.NewWriter(os.Stdout, 1, 4, 1, ' ', tabwriter.Debug|tabwriter.AlignRight)
	fmt.Fprintln(table, "========\t==========\t====================\t====================\t===================\t===================\t")
	fmt.Fprintln(table, "time\t# policies\tavg cpu (controller)\tmax cpu (controller)\tavg cpu (apiserver)\tmax cpu (apiserver)\t")
	fmt.Fprintln(table, "========\t==========\t====================\t====================\t===================\t===================\t")

	for i := 0; i < len(data); i++ {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\t\n",
			data[i].timestamp,
			data[i].numPolicies,
			fmt.Sprintf("%.5f", data[i].controllerAvg),
			fmt.Sprintf("%.5f", data[i].controllerMax),
			fmt.Sprintf("%.5f", data[i].apiserverAvg),
			fmt.Sprintf("%.5f", data[i].apiserverMax),
		)
	}

	table.Flush()
}

// export table of results to a csv file
func exportTable(data []metricData, filename string) {
	f, err := os.Create(filename)
	if err != nil {
		klog.Exitf("Error: failed to create %s; %s", filename, err)
	}

	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	w := csv.NewWriter(f)
	defer w.Flush()

	line := []string{"time", "numPolicies", "avg_cpu_controller", "max_cpu_controller", "avg_cpu_apiserver", "max_cpu_apiserver"}
	if err := w.Write(line); err != nil {
		klog.Exitf("Error writing headers to file; %s", err)
	}

	for _, entry := range data {
		line = []string{
			entry.timestamp,
			fmt.Sprintf("%d", entry.numPolicies),
			fmt.Sprintf("%.5f", entry.controllerAvg),
			fmt.Sprintf("%.5f", entry.controllerMax),
			fmt.Sprintf("%.5f", entry.apiserverAvg),
			fmt.Sprintf("%.5f", entry.apiserverMax),
		}
		if err := w.Write(line); err != nil {
			klog.Exitf("Error writing data to file; %s", err)
		}
	}
}

func init() {
	klog.SetOutput(os.Stdout)
	klog.InitFlags(flag.CommandLine)
}

func main() {
	// pull in test variables from flags
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	var plcFilename, outputFilename string
	var nPerBatch, nTotal, perBatchSleep int
	var insecure bool

	pflag.StringVarP(&plcFilename, "policy", "p",
		"resources/templates/cfgmap-plc.yaml", "path to test policy YAML, relative to performance directory")
	pflag.IntVarP(&nPerBatch, "policies-per-batch", "b", 100, "number of policies to create per batch")
	pflag.IntVarP(&nTotal, "total-policies", "t", 1000, "total number of policies created")
	pflag.IntVarP(&perBatchSleep, "sleep", "s", 20, "time (min) to sleep after creating a batch of policies")
	pflag.StringVar(&outputFilename, "csv", "results.csv", "path to CSV to export results to")
	pflag.BoolVar(&insecure, "insecure-skip-verify", false, "skip certificate verification on metrics requests")

	pflag.Parse()

	token, thanosHost := setupMetrics()

	klog.Info("Starting the Config Policy Controller performance test :)")
	tableData := []metricData{}

	tableData = append(tableData, getMetrics(thanosHost, string(token), perBatchSleep, 0, insecure))

	//setup temp directory for auto-generated policy YAML to live in
	policyDir, err := os.MkdirTemp(path.Join(performanceDir, "resources"), "policies")
	if err != nil {
		klog.Exitf("Error creating temp policy directory: %s", err)
	}
	defer os.RemoveAll(policyDir) // clean up

	totalPlcs := 0
	for totalPlcs < nTotal {
		start := time.Now()

		// create a batch of policies
		klog.V(2).Infof("Creating %d copies of %s on the managed cluster...\n", nPerBatch, plcFilename)
		batchFails := 0
		for batchPlcs := 0; batchPlcs < nPerBatch; batchPlcs += 1 {
			err = genUniquePolicy(path.Join(performanceDir, plcFilename), path.Join(policyDir, "current_policy.yaml"))
			if err != nil {
				klog.Exitf("Error patching policy with unique name: %s", err)
			}

			tries := 2
			for tries > 0 {
				creationOutput, err := exec.Command("kubectl", "apply", "-f", path.Join(policyDir, "current_policy.yaml")).CombinedOutput()
				if err != nil {
					klog.Error("Error creating policy: %s, %s", err, string(creationOutput))
					tries -= 1
					if tries == 0 {
						os.Exit(1)
					}
				} else {
					break
				}
			}
			totalPlcs += 1
		}

		// sleep for the remainder of perBatchSleep
		end := time.Now()
		elapsed := end.Sub(start)
		bonusTime := time.Duration(perBatchSleep)*time.Minute - elapsed

		klog.V(2).Infof(
			"%d policies created in %.2f seconds! Waiting an additional %.2f seconds for policies to process...\n",
			nPerBatch-batchFails,
			elapsed.Seconds(),
			bonusTime.Seconds(),
		)

		time.Sleep(bonusTime)

		tableData = append(tableData, getMetrics(thanosHost, string(token), perBatchSleep, totalPlcs, insecure))
	}

	printTable(tableData)

	wd, err := os.Getwd()
	if err != nil {
		klog.Errorf("Error getting working directory: %s", err)
	}
	outpath := path.Join(wd, performanceDir, "output")
	err = os.MkdirAll(outpath, os.ModePerm)
	if err != nil {
		klog.Errorf("Error creating output directory: %s", err)
	}
	exportTable(tableData, path.Join(performanceDir, "output", outputFilename))

	klog.Info("Performance test completed! Cleaning up...")
	deletionOutput, err := exec.Command("kubectl", "delete", "policies.policy.open-cluster-management.io", "-l", "grc-test=config-policy-performance").CombinedOutput()
	if err != nil {
		klog.Error("Error deleting policies: %s, %s", err, string(deletionOutput))
	}
	prDeletionOutput, err := exec.Command("kubectl", "delete", "placementrules", "-l", "grc-test=config-policy-performance").CombinedOutput()
	if err != nil {
		klog.Error("Error deleting config maps: %s, %s", err, string(prDeletionOutput))
	}
	pbDeletionOutput, err := exec.Command("kubectl", "delete", "placementbindings", "-l", "grc-test=config-policy-performance").CombinedOutput()
	if err != nil {
		klog.Error("Error deleting config maps: %s, %s", err, string(pbDeletionOutput))
	}
}
