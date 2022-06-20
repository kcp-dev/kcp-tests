package clusterinfrastructure

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// get a token assigned to prometheus-k8s from openshift-monitoring namespace
func getPrometheusSAToken(oc *exutil.CLI) string {
	e2e.Logf("Getting a token assgined to prometheus-k8s from openshift-monitoring namespace...")
	token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(token).NotTo(o.BeEmpty())
	return token
}

// check if alert raised (pengding or firing)
func checkAlertRaised(oc *exutil.CLI, alertName string) {
	token := getPrometheusSAToken(oc)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "prometheus-k8s", "-n", "openshift-monitoring", "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	alertCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\")'", token, url, alertName)
	err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
		result, err := exec.Command("bash", "-c", alertCMD).Output()
		if err != nil {
			e2e.Logf("Error '%v' retrieving prometheus alert, retry ...", err)
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

// check if metrics shown
func checkMetricsShown(oc *exutil.CLI, metricsName string, args ...string) {
	e2e.Logf("Check metrics " + metricsName)
	token := getPrometheusSAToken(oc)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "prometheus-k8s", "-n", "openshift-monitoring", "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	metricsCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/query?query=%s", token, url, metricsName)
	var queryResult string
	errQuery := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		result, err := exec.Command("bash", "-c", metricsCMD).Output()
		if err != nil {
			e2e.Logf("Error '%v' retrieving metrics, retry ...", err)
			return false, nil
		}
		queryResult = string(result)
		if !strings.Contains(queryResult, metricsName) {
			e2e.Logf("Metrics not contain '%s', retry ...", metricsName)
			return false, nil
		}
		for _, arg := range args {
			if !strings.Contains(queryResult, arg) {
				e2e.Logf("Metrics not contain '%s', retry ...", arg)
				return false, nil
			}
		}
		e2e.Logf(metricsName + " metrics is shown right")
		return true, nil
	})
	if errQuery != nil {
		e2e.Logf("the failing query result is %s", queryResult)
	}
	exutil.AssertWaitPollNoErr(errQuery, "Check metrics "+metricsName+" failed")
}
