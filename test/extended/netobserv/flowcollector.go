package netobserv

import (
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type flowcollector struct {
	Namespace     string
	GoflowImage   string
	ConsolePlugin string
	GoflowKind    string
	Template      string
}

// create flowcollector CRD for a given manifest file
func (flow *flowcollector) createFlowcollector(oc *exutil.CLI) {
	parameters := []string{"--ignore-unknown-parameters=true", "-f", flow.Template, "-p", "NAMESPACE=" + flow.Namespace}

	if flow.GoflowImage != "" {
		parameters = append(parameters, "GOFLOW_IMAGE="+flow.GoflowImage)
	}

	if flow.ConsolePlugin != "" {
		parameters = append(parameters, "CONSOLEPLUGIN_IMAGE="+flow.ConsolePlugin)
	}

	if flow.GoflowKind != "" {
		parameters = append(parameters, "KIND="+flow.GoflowKind)
	}

	exutil.ApplyNsResourceFromTemplate(oc, flow.Namespace, parameters...)
}

// delete flowcollector CRD from a cluster
func (flow *flowcollector) deleteFlowcollector(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("flowcollector", "cluster").Execute()
}

// get flow collector port
func getCollectorPort(oc *exutil.CLI) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("flowcollector", "cluster", "-n", oc.Namespace()).Template("{{.spec.goflowkube.port}}").Output()
}

// returns service IP or error for goflow-kube deployment
func getGoflowServiceIP(oc *exutil.CLI) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("svc", "goflow-kube", "-n", oc.Namespace()).Template("{{.spec.clusterIP}}").Output()
}

// returns true/false if flow collection is enabled on cluster
func checkFlowcollectionEnabled(oc *exutil.CLI) string {
	collectorName, err, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("flowcollector").Template("{{range .items}}{{.metadata.name}}{{end}}").Outputs()

	if err != "" {
		return ""
	}

	return collectorName

}

// polls to check ovs-flows-config is created or deleted given shouldExist is true or false
func waitCnoConfigMapUpdate(oc *exutil.CLI, shouldExist bool) {
	err := wait.Poll(20*time.Second, 10*time.Minute, func() (bool, error) {

		// check whether ovs-flows-config config map exists in openshift-network-operator NS
		_, stderr, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "ovs-flows-config", "-n", "openshift-network-operator").Outputs()

		if stderr == "" && shouldExist {
			return true, nil
		}

		if stderr != "" && !shouldExist {
			return true, nil
		}
		return false, nil
	})

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf(" ovs-flows-config ConfigMap is not updated"))
}

// returns target configured in ovs-flows-config config map
func getOVSFlowsConfigTarget(oc *exutil.CLI, goflowDeployedAs string) (string, error) {

	var template string
	if goflowDeployedAs == "Deployment" {
		template = "{{.data.sharedTarget}}"
	}

	if goflowDeployedAs == "DaemonSet" {
		template = "{{.data.nodePort}}"
	}

	stdout, stderr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "ovs-flows-config", "-n", "openshift-network-operator").Template(template).Outputs()

	if stderr != "" || err != nil {
		e2e.Logf("Fetching ovs-flows-config configmap return err %s", stderr)
		return stdout, err
	}
	return stdout, err
}

// get flow collector IPs configured in OVS
func getOVSCollectorIP(oc *exutil.CLI) ([]string, error) {
	jsonpath := "{.items[*].spec.containers[*].env[?(@.name==\"IPFIX_COLLECTORS\")].value}"

	var collectors []string
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-ovn-kubernetes", "-o", "jsonpath="+jsonpath).Output()

	if err != nil {
		return collectors, err
	}
	collectors = strings.Split(stdout, " ")

	return collectors, nil
}

// returns ture/false if flowcollector API exists.
func isFlowCollectorAPIExists(oc *exutil.CLI) (bool, error) {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "-o", "jsonpath='{.items[*].spec.names.kind}'").Output()

	if err != nil {
		return false, err
	}
	return strings.Contains(stdout, "FlowCollector"), nil
}
