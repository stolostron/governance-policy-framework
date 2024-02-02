package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"flag"
	"fmt"
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

func setupMetrics() (token []byte, thanosHost string) {
	// wait for config map to be applied properly before executing command
	secretOutput, err := exec.Command(
		"kubectl", "get", "secret", "-n",
		"openshift-user-workload-monitoring",
	).CombinedOutput()
	if err != nil || !strings.Contains(string(secretOutput), "prometheus-user-workload-token") {
		klog.V(2).Info("Setting up secret for k8s metrics")

		_, err = exec.Command(
			"kubectl", "apply", "-f",
			path.Join(performanceDir, "resources/setup/metrics-configmap.yaml"),
		).CombinedOutput()
		if err != nil {
			klog.Exitf("Error applying metrics configMap: %s", err)
		}
		// wait for secret to be created for 2 minutes
		found := false

		for i := 0; i < 120; i += 5 {
			secretOutput, err = exec.Command(
				"kubectl", "get", "secret", "-n",
				"openshift-user-workload-monitoring",
			).CombinedOutput()

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
		klog.Exit("Error getting Prometheus metrics secret: secret not found in "+
			"openshift-user-workload-monitoring ns", err)
	}

	tokenB64, err := exec.Command(
		"kubectl", "get", "secret", secret, "-n",
		"openshift-user-workload-monitoring",
		"-o=jsonpath='{.data.token}'",
	).CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from secret: %s", err)
	}

	token, err = base64.StdEncoding.DecodeString(strings.Trim(string(tokenB64), "'"))
	if err != nil {
		klog.Exitf("Error decoding token from secret %s", err)
	}

	thanosB64, err := exec.Command(
		"kubectl", "get", "route", "thanos-querier", "-n",
		"openshift-monitoring", "-o=jsonpath='{.spec.host}'",
	).CombinedOutput()
	if err != nil {
		klog.Exitf("Error getting API token from secret: %s", err)
	}

	klog.V(2).Info("Setup complete! Token and metrics route retrieved successfully")

	return token, strings.Trim(string(thanosB64), "'")
}

func getMetrics(
	thanosHost string, token string,
	perBatchSleep int, numPolicies int,
	insecure bool,
) (metricData, saturationData) {
	// print metrics
	ts, avgController := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"avg_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])",
			perBatchSleep,
		),
		insecure,
	)
	_, avgApiserver := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(avg_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))",
			perBatchSleep,
		),
		insecure,
	)
	_, maxController := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"max_over_time(pod:container_cpu_usage:sum{pod=~'config-policy-controller-.*'}[%dm:30s])",
			perBatchSleep,
		),
		insecure,
	)
	_, maxApiserver := query(
		thanosHost,
		token,
		fmt.Sprintf(
			"sum(max_over_time(pod:container_cpu_usage:sum{pod=~'kube-apiserver-ip-.*'}[%dm:30s]))",
			perBatchSleep,
		),
		insecure,
	)

	satTS, satLE1 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="1"}`,
		insecure,
	)

	_, satLE3 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="3"}`,
		insecure,
	)

	_, satLE9 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="9"}`,
		insecure,
	)

	_, satLE10 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="10.5"}`,
		insecure,
	)

	_, satLE15 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="15"}`,
		insecure,
	)

	_, satLE30 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="30"}`,
		insecure,
	)

	_, satLE60 := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="60"}`,
		insecure,
	)

	_, satLEinf := query(
		thanosHost,
		token,
		`config_policies_evaluation_duration_seconds_bucket{le="+Inf"}`,
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
		}, saturationData{
			timestamp:   satTS.Format("15:04:05"),
			numPolicies: numPolicies,
			le1:         int(satLE1),
			g1le3:       int(satLE3 - satLE1),
			g3le9:       int(satLE9 - satLE3),
			g9le10:      int(satLE10 - satLE9),
			g10le15:     int(satLE15 - satLE10),
			g15le30:     int(satLE30 - satLE15),
			g30le60:     int(satLE60 - satLE30),
			g60:         int(satLEinf - satLE60),
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
	timestamp     string
	numPolicies   int
	controllerAvg float64
	controllerMax float64
	apiserverAvg  float64
	apiserverMax  float64
}

type saturationData struct {
	timestamp   string
	numPolicies int
	le1         int
	g1le3       int
	g3le9       int
	g9le10      int
	g10le15     int
	g15le30     int
	g30le60     int
	g60         int
}

// pretty print table of results to stdout
func printCPUTable(data []metricData) {
	table := tabwriter.NewWriter(os.Stdout, 1, 4, 1, ' ', tabwriter.Debug|tabwriter.AlignRight)
	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")
	fmt.Fprintln(table, "time\t# policies\tavg cpu (controller)\t"+
		"max cpu (controller)\tavg cpu (apiserver)\tmax cpu (apiserver)\t")
	fmt.Fprintln(table, "========\t==========\t====================\t"+
		"====================\t===================\t===================\t")

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

	klog.V(5).Infof("============================================" +
		"================================================================")
	klog.V(5).Infof("CPU Data (core usage):")
	table.Flush()
}

// pretty print table of saturation results to stdout
func printSaturationTable(data []saturationData) {
	table := tabwriter.NewWriter(os.Stdout, 1, 4, 1, ' ', tabwriter.Debug|tabwriter.AlignRight)
	fmt.Fprintln(table, "========\t==========\t=============\t=======\t"+
		"=======\t==========\t===========\t=========\t=========\t===========\t")
	fmt.Fprintln(table, "time\t# policies\t1 sec or less\t1-3 sec\t3-9 sec\t"+
		"9-10.5 sec\t10.5-15 sec\t15-30 sec\t30-60 sec\tover 60 sec\t")
	fmt.Fprintln(table, "========\t==========\t=============\t=======\t"+
		"=======\t==========\t===========\t=========\t=========\t===========\t")

	for i := 0; i < len(data); i++ {
		fmt.Fprintf(table, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t\n",
			data[i].timestamp,
			data[i].numPolicies,
			data[i].le1,
			data[i].g1le3,
			data[i].g3le9,
			data[i].g9le10,
			data[i].g10le15,
			data[i].g15le30,
			data[i].g30le60,
			data[i].g60,
		)
	}

	klog.V(5).Infof("======================================================" +
		"=============================================================")
	klog.V(5).Infof("Saturation Data (config policy evaluation loops / second):")
	klog.V(5).Infof("Note: saturation occurs when the evaluation time " +
		"starts to exceed the 10 second interval between loops")
	table.Flush()
}

// translate saturation metrics from cumulative to per-batch
func normalizeSaturationData(data []saturationData) []saturationData {
	// start baseline at 0 policies evaluated
	result := []saturationData{{
		timestamp:   data[0].timestamp,
		numPolicies: 0,
		le1:         0,
		g1le3:       0,
		g3le9:       0,
		g9le10:      0,
		g10le15:     0,
		g15le30:     0,
		g30le60:     0,
		g60:         0,
	}}

	// counts are cumulative, so subtract counts from previous batch
	for i := 1; i < len(data); i++ {
		baseline := data[i-1]

		result = append(result, saturationData{
			timestamp:   data[i].timestamp,
			numPolicies: data[i].numPolicies,
			le1:         data[i].le1 - baseline.le1,
			g1le3:       data[i].g1le3 - baseline.g1le3,
			g3le9:       data[i].g3le9 - baseline.g3le9,
			g9le10:      data[i].g9le10 - baseline.g9le10,
			g10le15:     data[i].g10le15 - baseline.g10le15,
			g15le30:     data[i].g15le30 - baseline.g15le30,
			g30le60:     data[i].g30le60 - baseline.g30le60,
			g60:         data[i].g60 - baseline.g60,
		})
	}

	return result
}

// export table of results to a csv file
func exportTable(cpuData []metricData, satData []saturationData, filename string) {
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
		"time", "numPolicies", "avg_cpu_controller", "max_cpu_controller", "avg_cpu_apiserver",
		"max_cpu_apiserver", "<1", "1-3", "3-9", "9-10.5", "10.5-15", "15-30", ">30",
	}
	if err := w.Write(line); err != nil {
		klog.Exitf("Error writing headers to file; %s", err)
	}

	for i, entry := range cpuData {
		line = []string{
			entry.timestamp,
			fmt.Sprintf("%d", entry.numPolicies),
			fmt.Sprintf("%.5f", entry.controllerAvg),
			fmt.Sprintf("%.5f", entry.controllerMax),
			fmt.Sprintf("%.5f", entry.apiserverAvg),
			fmt.Sprintf("%.5f", entry.apiserverMax),
			fmt.Sprint(satData[i].le1),
			fmt.Sprint(satData[i].g1le3),
			fmt.Sprint(satData[i].g3le9),
			fmt.Sprint(satData[i].g9le10),
			fmt.Sprint(satData[i].g10le15),
			fmt.Sprint(satData[i].g15le30),
			fmt.Sprint(satData[i].g30le60),
			fmt.Sprint(satData[i].g60),
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
	satTableData := []saturationData{}

	cpuMetrics, satMetrics := getMetrics(thanosHost, string(token), perBatchSleep, 0, insecure)

	tableData = append(tableData, cpuMetrics)
	satTableData = append(satTableData, satMetrics)

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

		for batchPlcs := 0; batchPlcs < nPerBatch; batchPlcs++ {
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
						os.Exit(1)
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

		cpuMetrics, satMetrics := getMetrics(thanosHost, string(token), perBatchSleep, totalPlcs, insecure)

		tableData = append(tableData, cpuMetrics)
		satTableData = append(satTableData, satMetrics)
	}

	satTableData = normalizeSaturationData(satTableData)

	printCPUTable(tableData)
	printSaturationTable(satTableData)

	wd, err := os.Getwd()
	if err != nil {
		klog.Errorf("Error getting working directory: %s", err)
	}

	outpath := path.Join(wd, performanceDir, "output")

	err = os.MkdirAll(outpath, os.ModePerm)
	if err != nil {
		klog.Errorf("Error creating output directory: %s", err)
	}

	exportTable(tableData, satTableData, path.Join(performanceDir, "output", outputFilename))

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
