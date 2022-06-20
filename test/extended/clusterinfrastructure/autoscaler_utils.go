package clusterinfrastructure

import (
	"strconv"

	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type clusterAutoscalerDescription struct {
	maxNode              int
	minCore              int
	maxCore              int
	minMemory            int
	maxMemory            int
	utilizationThreshold string
	template             string
}

type machineAutoscalerDescription struct {
	name           string
	namespace      string
	maxReplicas    int
	minReplicas    int
	template       string
	machineSetName string
}

type workLoadDescription struct {
	name      string
	namespace string
	template  string
}

func (clusterAutoscaler *clusterAutoscalerDescription) createClusterAutoscaler(oc *exutil.CLI) {
	e2e.Logf("Creating clusterautoscaler ...")
	var err error
	if clusterAutoscaler.utilizationThreshold == "" {
		err = applyResourceFromTemplate(oc, "-f", clusterAutoscaler.template, "-p", "MAXNODE="+strconv.Itoa(clusterAutoscaler.maxNode), "MINCORE="+strconv.Itoa(clusterAutoscaler.minCore), "MAXCORE="+strconv.Itoa(clusterAutoscaler.maxCore), "MINMEMORY="+strconv.Itoa(clusterAutoscaler.minMemory), "MAXMEMORY="+strconv.Itoa(clusterAutoscaler.maxMemory))

	} else {
		err = applyResourceFromTemplate(oc, "-f", clusterAutoscaler.template, "-p", "MAXNODE="+strconv.Itoa(clusterAutoscaler.maxNode), "MINCORE="+strconv.Itoa(clusterAutoscaler.minCore), "MAXCORE="+strconv.Itoa(clusterAutoscaler.maxCore), "MINMEMORY="+strconv.Itoa(clusterAutoscaler.minMemory), "MAXMEMORY="+strconv.Itoa(clusterAutoscaler.maxMemory), "UTILIZATIONTHRESHOLD="+clusterAutoscaler.utilizationThreshold)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (clusterAutoscaler *clusterAutoscalerDescription) deleteClusterAutoscaler(oc *exutil.CLI) error {
	e2e.Logf("Deleting clusterautoscaler ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterautoscaler", "default").Execute()
}

func (machineAutoscaler *machineAutoscalerDescription) createMachineAutoscaler(oc *exutil.CLI) {
	e2e.Logf("Creating machineautoscaler ...")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", machineAutoscaler.template, "-p", "NAME="+machineAutoscaler.name, "NAMESPACE="+machineAPINamespace, "MAXREPLICAS="+strconv.Itoa(machineAutoscaler.maxReplicas), "MINREPLICAS="+strconv.Itoa(machineAutoscaler.minReplicas), "MACHINESETNAME="+machineAutoscaler.machineSetName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (machineAutoscaler *machineAutoscalerDescription) deleteMachineAutoscaler(oc *exutil.CLI) error {
	e2e.Logf("Deleting a machineautoscaler ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("machineautoscaler", machineAutoscaler.name, "-n", machineAPINamespace).Execute()
}

func (workLoad *workLoadDescription) createWorkLoad(oc *exutil.CLI) {
	e2e.Logf("Creating workLoad ...")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", workLoad.template, "-p", "NAME="+workLoad.name, "NAMESPACE="+workLoad.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (workLoad *workLoadDescription) deleteWorkLoad(oc *exutil.CLI) error {
	e2e.Logf("Deleting workload ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("job", workLoad.name, "-n", machineAPINamespace).Execute()
}
