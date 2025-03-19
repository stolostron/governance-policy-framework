package main

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/spf13/pflag"
	"k8s.io/klog"
)

const performanceDir = "test/performance"

type WithHeader struct {
	http.Header
	rt http.RoundTripper
}

func NewWithHeader(rt http.RoundTripper) WithHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return WithHeader{Header: make(http.Header), rt: rt}
}

func (h WithHeader) RoundTrip(req *http.Request) (*http.Response, error) {
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

	trHeader := NewWithHeader(tr)
	trHeader.Set("Authorization", "Bearer "+token)

	cl := &http.Client{Transport: trHeader}

	client, err := api.NewClient(api.Config{
		Address: "https://" + host,
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

	if len(result.(model.Vector)) > 0 {
		val := result.(model.Vector)[0]

		metric, err := strconv.ParseFloat(val.Value.String(), 32)
		if err != nil {
			klog.Exitf("Error parsing metric response: %v\n", err)
		}

		return val.Timestamp.Time(), metric
	}

	klog.Exitf("Error: metrics response is empty: %v\n", result)

	return time.Time{}, -1
}

func setupMetrics() (token string, thanosHost string) {
	tokenBytes, err := exec.Command("oc", "whoami", "--show-token").CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from `oc whoami --show-token`: %v", err)
	}

	thanosB64, err := exec.Command(
		"kubectl", "get", "route", "thanos-querier", "-n",
		"openshift-monitoring", "-o=jsonpath='{.spec.host}'",
	).CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from secret: %s", err)
	}

	klog.V(2).Info("Setup complete! Token and metrics route retrieved successfully")

	return strings.TrimSpace(string(tokenBytes)), strings.Trim(string(thanosB64), "'")
}

func getMetrics(
	thanosHost string, token string,
	perBatchSleep int, numPolicies int,
	insecure bool,
) metricData {
	// print metrics
	ts, controllerCPUAvg := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"avg_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])",
			perBatchSleep,
		),
		insecure,
	)
	_, apiServerCPUAvg := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(avg_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))",
			perBatchSleep,
		),
		insecure,
	)
	_, controllerCPUMax := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"max_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])",
			perBatchSleep,
		),
		insecure,
	)
	_, apiServerCPUMax := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(max_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))",
			perBatchSleep,
		),
		insecure,
	)

	_, controllerMemAvg := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(avg_over_time(container_memory_working_set_bytes{container='config-policy-controller'}[%dm:30s])) "+
				"* 0.000001",
			perBatchSleep,
		),
		insecure,
	)
	_, apiServerMemAvg := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(avg_over_time(container_memory_working_set_bytes{container='kube-apiserver'}[%dm:30s])) * 0.000001",
			perBatchSleep,
		),
		insecure,
	)
	_, controllerMemMax := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(max_over_time(container_memory_working_set_bytes{container='config-policy-controller'}[%dm:30s])) "+
				"* 0.000001",
			perBatchSleep,
		),
		insecure,
	)
	_, apiServerMemMax := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(max_over_time(container_memory_working_set_bytes{container='kube-apiserver'}[%dm:30s])) * 0.000001",
			perBatchSleep,
		),
		insecure,
	)

	// log metrics
	klog.V(2).Infof("CPU utilization over the past %d minutes:", perBatchSleep)
	klog.V(2).Infof("config policy controller: %.5f / %.5f (avg/max)", controllerCPUAvg, controllerCPUMax)
	klog.V(2).Infof("kube API server: %.5f / %.5f (avg/max)", apiServerCPUAvg, apiServerCPUMax)

	klog.V(2).Infof("Memory utilization over the past %d minutes:", perBatchSleep)
	klog.V(2).Infof("config policy controller: %.5f MB / %.5f MB (avg/max)", controllerMemAvg, controllerMemMax)
	klog.V(2).Infof("kube API server: %.5f MB / %.5f MB (avg/max)", apiServerMemAvg, apiServerMemMax)

	return metricData{
		timestamp:        ts.Format("15:04:05"),
		numPolicies:      numPolicies,
		controllerCPUAvg: controllerCPUAvg,
		controllerCPUMax: controllerCPUMax,
		apiServerCPUAvg:  apiServerCPUAvg,
		apiServerCPUMax:  apiServerCPUMax,
		controllerMemAvg: controllerMemAvg,
		controllerMemMax: controllerMemMax,
		apiServerMemAvg:  apiServerMemAvg,
		apiServerMemMax:  apiServerMemMax,
	}
}

func genUniquePolicy(inFilename string, outFilename string) error {
	input, err := os.ReadFile(inFilename)
	if err != nil {
		return err
	}

	output := strings.ReplaceAll(string(input), "[ID]", uuid.New().String())

	err = os.WriteFile(outFilename, []byte(output), 0o644)
	if err != nil {
		return err
	}

	return nil
}

type metricData struct {
	timestamp        string
	numPolicies      int
	controllerCPUAvg float64
	controllerCPUMax float64
	apiServerCPUAvg  float64
	apiServerCPUMax  float64
	controllerMemAvg float64
	controllerMemMax float64
	apiServerMemAvg  float64
	apiServerMemMax  float64
}

// pretty print table of results to stdout
func printTable(data []metricData) {
	table := tabwriter.NewWriter(os.Stdout, 1, 4, 1, ' ', tabwriter.Debug|tabwriter.AlignRight)
	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")
	fmt.Fprintln(table, "time\t# policies\tavg cpu (controller)\t"+
		"max cpu (controller)\tavg cpu (apiserver)\tmax cpu (apiserver)\t")
	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")

	for i := range data {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\t\n",
			data[i].timestamp,
			data[i].numPolicies,
			fmt.Sprintf("%.5f", data[i].controllerCPUAvg),
			fmt.Sprintf("%.5f", data[i].controllerCPUMax),
			fmt.Sprintf("%.5f", data[i].apiServerCPUAvg),
			fmt.Sprintf("%.5f", data[i].apiServerCPUMax),
		)
	}

	klog.V(5).Infof("============================================" +
		"================================================================")
	klog.V(5).Infof("CPU Data (core usage):")
	table.Flush()

	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")
	fmt.Fprintln(table, "time\t# policies\tavg memory (controller)\t"+
		"max memory (controller)\tavg memory (apiserver)\tmax memory (apiserver)\t")
	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")

	for i := range data {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\t\n",
			data[i].timestamp,
			data[i].numPolicies,
			fmt.Sprintf("%.5f MB", data[i].controllerMemAvg),
			fmt.Sprintf("%.5f MB", data[i].controllerMemMax),
			fmt.Sprintf("%.5f MB", data[i].apiServerMemAvg),
			fmt.Sprintf("%.5f MB", data[i].apiServerMemMax),
		)
	}

	klog.V(5).Infof("============================================" +
		"================================================================")
	klog.V(5).Infof("Memory Data:")
	table.Flush()
}

// export table of results to a csv file
func exportTable(cpuData []metricData, filename string) {
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

	line := []string{
		"time", "numPolicies", "avg_cpu_controller", "max_cpu_controller", "avg_cpu_apiserver", "max_cpu_apiserver",
		"avg_memory_controller", "max_memory_controller", "avg_memory_apiserver", "max_memory_apiserver",
	}
	if err := w.Write(line); err != nil {
		klog.Exitf("Error writing headers to file; %s", err)
	}

	for _, entry := range cpuData {
		line = []string{
			entry.timestamp,
			strconv.Itoa(entry.numPolicies),
			fmt.Sprintf("%.5f", entry.controllerCPUAvg),
			fmt.Sprintf("%.5f", entry.controllerCPUMax),
			fmt.Sprintf("%.5f", entry.apiServerCPUAvg),
			fmt.Sprintf("%.5f", entry.apiServerCPUMax),
			fmt.Sprintf("%.5f MB", entry.controllerMemAvg),
			fmt.Sprintf("%.5f MB", entry.controllerMemMax),
			fmt.Sprintf("%.5f MB", entry.apiServerMemAvg),
			fmt.Sprintf("%.5f MB", entry.apiServerMemMax),
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

	allMetrics := getMetrics(thanosHost, token, perBatchSleep, 0, insecure)

	tableData = append(tableData, allMetrics)

	// setup temp directory for auto-generated policy YAML to live in
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

		for range nPerBatch {
			err = genUniquePolicy(path.Join(performanceDir, plcFilename), path.Join(policyDir, "current_policy.yaml"))
			if err != nil {
				klog.Exitf("Error patching policy with unique name: %s", err)
			}

			tries := 2
			for tries > 0 {
				creationOutput, err := exec.Command(
					"kubectl", "apply", "-f",
					path.Join(policyDir, "current_policy.yaml"),
				).CombinedOutput()
				if err != nil {
					klog.Errorf("Error creating policy: %s, %s", err, string(creationOutput))

					tries--
					if tries == 0 {
						klog.Fatal()
					}
				} else {
					break
				}
			}

			totalPlcs++
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

		allMetrics := getMetrics(thanosHost, token, perBatchSleep, totalPlcs, insecure)

		tableData = append(tableData, allMetrics)
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

	deletionOutput, err := exec.Command(
		"kubectl", "delete",
		"policies.policy.open-cluster-management.io", "-l",
		"grc-test=config-policy-performance",
		"--ignore-not-found",
	).CombinedOutput()
	if err != nil {
		klog.Errorf("Error deleting policies: %s, %s", err, string(deletionOutput))
	}

	prDeletionOutput, err := exec.Command(
		"kubectl", "delete", "placementrules", "-l",
		"grc-test=config-policy-performance",
		"--ignore-not-found",
	).CombinedOutput()
	if err != nil {
		klog.Errorf("Error deleting config maps: %s, %s", err, string(prDeletionOutput))
	}

	pbDeletionOutput, err := exec.Command(
		"kubectl", "delete", "placementbindings", "-l",
		"grc-test=config-policy-performance",
		"--ignore-not-found",
	).CombinedOutput()
	if err != nil {
		klog.Errorf("Error deleting config maps: %s, %s", err, string(pbDeletionOutput))
	}
}
