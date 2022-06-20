package etcd

import (
	o "github.com/onsi/gomega"

	"fmt"
	"strings"
	"time"
	"math/rand"
	"regexp"

	"encoding/json"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)


type PrometheusQueryResult struct {
	Status string `json:"status"`
	Data struct {
		ResultType string `json:"resultType"`
		Result []struct {
			Metric struct {
				To		string `json:"To"`
				Endpoint	string `json:"endpoint"`
				Instance	string `json:"instance"`
				Job		string `json:"job"`
				Namespace	string `json:"namespace"`
				Pod		string `json:"pod"`
				Service		string `json:"service"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
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

func getNodeListByLabel(oc *exutil.CLI, labelKey string) []string {
        output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
        o.Expect(err).NotTo(o.HaveOccurred())
        nodeNameList := strings.Fields(output)
        return nodeNameList
}

func getPodListByLabel(oc *exutil.CLI, labelKey string) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-etcd", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podNameList := strings.Fields(output)
	return podNameList
}

func runDRBackup(oc *exutil.CLI, nodeNameList []string) (nodeName string, etcddb string) {
	var nodeN,etcdDb string
	for nodeindex, node := range nodeNameList {
		backupout, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", oc.Namespace(), "node/"+node, "--", "chroot", "/host", "/usr/local/bin/cluster-backup.sh", "/home/core/assets/backup").Output()
		if strings.Contains(backupout, "Snapshot saved at") && err == nil {
			e2e.Logf("backup on master %v ", node)
			regexp,_ := regexp.Compile("/home/core/assets/backup/snapshot.*db")
			etcdDb = regexp.FindString(backupout)
			nodeN = node
			break
		} else if err != nil && nodeindex < len(nodeNameList)  {
			e2e.Logf("Try for next master!")
		} else {
			e2e.Failf("Failed to run the backup!")
		}
	}
	return nodeN,etcdDb
}

func doPrometheusQuery(oc *exutil.CLI, token string, url string, query string) PrometheusQueryResult {
	var data PrometheusQueryResult
	msg, _, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
		"-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "-i", "--",
		"curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token),
		fmt.Sprintf("%s%s", url, query)).Outputs()
	if err != nil {
		e2e.Failf("Failed Prometheus query, error: %v", err)
	}
	o.Expect(msg).NotTo(o.BeEmpty())
	json.Unmarshal([]byte(msg), &data)
	logPrometheusResult(data)
	return data
}

func logPrometheusResult(data PrometheusQueryResult) {
	if len(data.Data.Result) > 0 {
		e2e.Logf("Unexpected metric values.")
		for i, v := range data.Data.Result {
			e2e.Logf(fmt.Sprintf("index: %d value: %s", i, v.Value[1].(string)))
		}
	}
}
