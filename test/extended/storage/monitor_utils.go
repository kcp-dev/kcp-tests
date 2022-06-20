package storage

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	prometheusQueryURL  string = "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query="
	prometheusNamespace string = "openshift-monitoring"
	prometheusK8s       string = "prometheus-k8s"
)

//  Define a monitor object
type monitor struct {
	token    string
	ocClient *exutil.CLI
}

// Init a monitor
func newMonitor(oc *exutil.CLI) *monitor {
	var mo monitor
	mo.ocClient = oc
	mo.token = getSAToken(oc)
	return &mo
}

// Get a specified metric's value from prometheus
func (mo *monitor) getSpecifiedMetricValue(metricName string, valueJSONPath string) (string, error) {
	getCmd := "curl -k -s -H \"" + fmt.Sprintf("Authorization: Bearer %v", mo.token) + "\" " + prometheusQueryURL + metricName
	respsonce, err := execCommandInSpecificPod(mo.ocClient, prometheusNamespace, "statefulsets/"+prometheusK8s, getCmd)
	metricValue := gjson.Get(respsonce, valueJSONPath).String()
	return metricValue, err
}

// Waiting for a specified metric's value update to expected
func (mo *monitor) waitSpecifiedMetricValueAsExpected(metricName string, valueJSONPath string, expectedValue string) {
	err := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		realValue, err := mo.getSpecifiedMetricValue(metricName, valueJSONPath)
		if err != nil {
			e2e.Logf("Can't get %v metrics, error: %s. Trying again", metricName, err)
			return false, nil
		}
		if realValue == expectedValue {
			e2e.Logf("The metric: %s's {%s} value become to expected \"%s\"", metricName, valueJSONPath, expectedValue)
			return true, nil
		}
		e2e.Logf("The metric: %s's {%s} current value is \"%s\"", metricName, valueJSONPath, realValue)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting for metric: metric: %s's {%s} value become to expected timeout", metricName, valueJSONPath))
}

// GetSAToken get a token assigned to prometheus-k8s from openshift-monitoring namespace
func getSAToken(oc *exutil.CLI) string {
	e2e.Logf("Getting a token assgined to prometheus-k8s from openshift-monitoring namespace...")
	var (
		token string
		err   error
	)
	if versionIsAbove(getClientVersion(oc), "4.10") {
		token, err = oc.AsAdmin().WithoutNamespace().Run("create").Args("token", prometheusK8s, "-n", prometheusNamespace).Output()
	} else {
		token, err = oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", prometheusK8s, "-n", prometheusNamespace).Output()
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(token).NotTo(o.BeEmpty())
	return token
}

// Check the alert raied (pengding or firing)
func checkAlertRaised(oc *exutil.CLI, alertName string) {
	token := getSAToken(oc)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", prometheusK8s, "-n", prometheusNamespace, "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	alertCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\")'", token, url, alertName)
	//alertAnnoCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").annotations'", token, url, alertName)
	//alertStateCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").state'", token, url, alertName)
	err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
		result, err := exec.Command("bash", "-c", alertCMD).Output()
		if err != nil {
			e2e.Logf("Error retrieving prometheus alert: %v, retry ...", err)
			return false, nil
		}
		if len(string(result)) == 0 {
			e2e.Logf("Prometheus alert is nil, retry ...")
			return false, nil
		}
		if !strings.Contains(string(result), "firing") && !strings.Contains(string(result), "pending") {
			e2e.Logf(string(result))
			return false, fmt.Errorf("alert state is not firing or pending")
		}
		e2e.Logf("Alert %s found with the status firing or pending", alertName)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "alert state is not firing or pending")
}

// Get metric with metric name
func getStorageMetrics(oc *exutil.CLI, metricName string) string {
	token := getSAToken(oc)
	output, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-k", "-s", "-H", fmt.Sprintf("Authorization: Bearer %v", token), prometheusQueryURL+metricName).Outputs()
	o.Expect(err).NotTo(o.HaveOccurred())
	debugLogf("The metric outout is:\n %s", output)
	return output
}

// Check if metric contains specified content
func checkStorageMetricsContent(oc *exutil.CLI, metricName string, content string) {
	token := getSAToken(oc)
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		output, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", prometheusNamespace, "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), prometheusQueryURL+metricName).Outputs()
		if err != nil {
			e2e.Logf("Can't get %v metrics, error: %s. Trying again", metricName, err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(content, output); matched {
			e2e.Logf("Check the %s in %s metric succeed \n", content, metricName)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Cannot get %s in %s metric via prometheus", content, metricName))
}
