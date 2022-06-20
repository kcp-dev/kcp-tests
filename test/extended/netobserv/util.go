package netobserv

import (
	"regexp"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type goflowKubeDescription struct {
	serviceNs string
	name      string
	cmname    string
	template  string
}

func (goflowkube *goflowKubeDescription) create(oc *exutil.CLI, ns string, goflowDeploymenTemplate string) {
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", goflowDeploymenTemplate, "-p", "NAMESPACE="+ns)
}

func waitPodReady(oc *exutil.CLI, ns string, label string) {
	podName := getGoflowPod(oc, ns, label)
	exutil.AssertPodToBeReady(oc, podName, ns)
}

func patchResourceAsAdmin(oc *exutil.CLI, ns, resource, rsname, patch string) {
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(resource, rsname, "--type=json", "-p", patch, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getGoflowCollector(oc *exutil.CLI, resource string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("flowCollector", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("get flowCollector: %v", output)
	return output
}

// get name of goflow pod by label
func getGoflowPod(oc *exutil.CLI, ns string, name string) string {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", ns, "-l", "app="+name, "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of podname:%v", podName)
	return podName
}

// Verify some key and deterministic fields and their values
func verifyFlowRecord(podLog string) {
	re := regexp.MustCompile(`{\"BiFlowDirection\":.*}`)
	//e2e.Logf("the logs of goflow-kube pods are: %v", podLog)
	flowRecords := re.FindAllString(podLog, -1)
	e2e.Logf("The flowRecords %v\n\n\n", flowRecords)
	for i, flow := range flowRecords {
		e2e.Logf("The %d th flow record is: %v\n\n\n", i, flow)
		o.Expect(flow).Should(o.And(
			o.MatchRegexp("BiFlowDirection.:[0-9]"),
			o.MatchRegexp("Bytes.:[0-9]+"),
			o.MatchRegexp("DstAS.:[0-9]+"),
			o.MatchRegexp("DstMac.:\"[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}."),
			o.MatchRegexp("DstNet.:[0-9]+"),
			o.MatchRegexp("DstPort.:[0-9]+"),
			o.MatchRegexp("SrcAS.:[0-9]+"),
			o.MatchRegexp("SrcMac.:\"[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}:[a-zA-Z0-9]{2}\""),
			o.MatchRegexp("SrcNet.:[0-9]+"),
			o.MatchRegexp("SrcPort.:[0-9]+"),
			o.MatchRegexp("TimeFlowEnd.:[1-9][0-9]+"),
			o.MatchRegexp("TimeFlowStart.:[1-9][0-9]+"),
			o.MatchRegexp("TimeReceived.:[1-9][0-9]+")))
	}
}
