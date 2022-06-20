package util

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	o "github.com/onsi/gomega"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// MachineSetwithLabelDescription to create machineset with labels to put pods on specific machines
type MachineSetwithLabelDescription struct {
	Name           string
	Replicas       int
	Metadatalabels string
	Diskparams     string
}

// CreateMachineSet create a new machineset
func (ms *MachineSetwithLabelDescription) CreateMachineSet(oc *CLI) {
	e2e.Logf("Creating a new MachineSets with labels ...")
	machinesetName := GetRandomMachineSetName(oc)
	machineSetJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", machineAPINamespace, "-o=json").OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())

	bytes, _ := ioutil.ReadFile(machineSetJSON)
	machinesetjsonWithName, _ := sjson.Set(string(bytes), "metadata.name", ms.Name)
	machinesetjsonWithSelector, _ := sjson.Set(machinesetjsonWithName, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithTemplateLabel, _ := sjson.Set(machinesetjsonWithSelector, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithReplicas, _ := sjson.Set(machinesetjsonWithTemplateLabel, "spec.replicas", ms.Replicas)
	// Adding labels to machineset so that pods can be scheduled to specific machines
	machinesetjsonWithMetadataLabels, _ := sjson.Set(machinesetjsonWithReplicas, "spec.template.spec.metadata.labels.nodeName", ms.Metadatalabels)
	machinesetjsonWithDiskParams, _ := sjson.Set(machinesetjsonWithMetadataLabels, "spec.template.spec.providerSpec.value.ultraSSDCapability", ms.Diskparams)
	err = ioutil.WriteFile(machineSetJSON, []byte(machinesetjsonWithDiskParams), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
	if err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", machineSetJSON).Execute(); err != nil {
		ms.DeleteMachineSet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		ms.AssertLabelledMachinesRunningDeleteIfNot(oc, ms.Replicas, ms.Name)
	}
}

// DeleteMachineSet delete a machineset
func (ms *MachineSetwithLabelDescription) DeleteMachineSet(oc *CLI) error {
	e2e.Logf("Deleting a MachineSets ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(MapiMachineset, ms.Name, "-n", machineAPINamespace).Execute()
}

// AssertLabelledMachinesRunningDeleteIfNot check labeled machines are running if not delete machineset
func (ms *MachineSetwithLabelDescription) AssertLabelledMachinesRunningDeleteIfNot(oc *CLI, machineNumber int, machineSetName string) {
	e2e.Logf("Waiting for the machines Running ...")
	pollErr := wait.Poll(60*time.Second, 720*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machineSetName, "-o=jsonpath={.status.readyReplicas}", "-n", machineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(msg)
		if machinesRunning != machineNumber {
			e2e.Logf("Expected %v  machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
			return false, nil
		}
		e2e.Logf("Expected %v  machines are Running", machineNumber)
		return true, nil
	})
	if pollErr != nil {
		e2e.Logf("Deleting a MachineSets ...")
		ms.DeleteMachineSet(oc)
		AssertWaitPollNoErr(pollErr, fmt.Sprintf("Expected %v  machines are not Running after waiting up to 12 minutes ...", machineNumber))
	}
	e2e.Logf("All machines are Running ...")
}
