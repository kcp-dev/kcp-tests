package monitoring

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	withoutNamespace = true
	requireNS        = true
)

type monitoringConfig struct {
	name               string
	namespace          string
	enableUserWorkload string
	template           string
}

func (cm *monitoringConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cm.template, "-p", "NAME="+cm.name, "NAMESPACE="+cm.namespace, "ENABLEUSERWORKLOAD="+cm.enableUserWorkload)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

//the method is to create one resource with template
func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "cluster-monitoring.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func labelNameSpace(oc *exutil.CLI, namespace string, label string) {
	err := oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", namespace, label, "--overwrite").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The namespace %s is labeled by %q", namespace, label)

}

func getSAToken(oc *exutil.CLI, account string, namespace string) string {
	token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", account, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(token).NotTo(o.BeEmpty())
	return token
}

//check data by running curl on a pod
func checkMetric(oc *exutil.CLI, url string, metricString string, timeout time.Duration) error {
	var metrics string
	var err error
	token := getSAToken(oc, "prometheus-k8s", "openshift-monitoring")
	err = wait.Poll(3*time.Second, timeout*time.Second, func() (bool, error) {
		metrics, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("prometheus-k8s-0", "-c", "prometheus", "-n", "openshift-monitoring", "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), url).Output()
		if err != nil || !strings.Contains(metrics, metricString) {
			e2e.Logf("the err:%v, the metrics: %s and try next round", err, metrics)
			return false, nil
		}
		return true, err
	})
	e2e.Logf("The metrics is %s. expect to contain %s", metrics, metricString)
	return err
}
