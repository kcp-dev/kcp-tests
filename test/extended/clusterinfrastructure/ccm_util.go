package clusterinfrastructure

import (
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// waitForClusterHealthy check if new machineconfig is applied successfully
func waitForClusterHealthy(oc *exutil.CLI) {
	e2e.Logf("Waiting for the cluster healthy ...")
	pollErr := wait.Poll(1*time.Minute, 25*time.Minute, func() (bool, error) {
		master, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "master", "-o", "jsonpath='{.status.conditions[?(@.type==\"Updated\")].status}'").Output()
		worker, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-o", "jsonpath='{.status.conditions[?(@.type==\"Updated\")].status}'").Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(master, "True") && strings.Contains(worker, "True") {
			e2e.Logf("mc operation is completed on mcp")
			return true, nil
		}
		return false, nil
	})
	if pollErr != nil {
		e2e.Failf("Expected cluster is not healthy after waiting up to 25 minutes ...")
	}
	e2e.Logf("Cluster is healthy ...")
}
