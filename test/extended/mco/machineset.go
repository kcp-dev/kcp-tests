package mco

import (
	"fmt"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"strconv"
)

type MachineSet struct {
	Resource
}

type MachineSetList struct {
	ResourceList
}

// NewMachineSet constructs a new MachineSet struct
func NewMachineSet(oc *exutil.CLI, namespace string, name string) *MachineSet {
	return &MachineSet{*NewNamespacedResource(oc, "MachineSet", namespace, name)}
}

// NewMachineSetList constructs a new MachineSetListist struct to handle all existing MachineSets
func NewMachineSetList(oc *exutil.CLI, namespace string) *MachineSetList {
	return &MachineSetList{*NewNamespacedResourceList(oc, "MachineSet", namespace)}
}

// String implements the Stringer interface
func (ms MachineSet) String() string {
	return ms.GetName()
}

// ScaleTo scales the MachineSet to the exact given value
func (ms MachineSet) ScaleTo(scale int) error {
	return ms.Patch("merge", fmt.Sprintf(`{"spec": {"replicas": %d}}`, scale))
}

// AddToScale scales the MachineSet adding the given value (positive or negative).
func (ms MachineSet) AddToScale(delta int) error {
	currentReplicas, err := strconv.Atoi(ms.GetOrFail(`{.spec.replicas}`))
	if err != nil {
		return err
	}

	return ms.ScaleTo(currentReplicas + delta)
}

// PollIsReady returns a function that can be used by Gomega "Eventually" and "Consistently" to check that the MachineSet instances ready
func (ms MachineSet) PollIsReady() func() bool {
	return func() bool {
		status := JSON(ms.GetOrFail(`{.status}`))
		replicasData := status.Get("replicas")
		readyReplicasData := status.Get("readyReplicas")

		if !replicasData.Exists() {
			return false
		}
		replicas := replicasData.ToInt()
		if replicas == 0 {
			// We cant check the ready status when there is 0 replica configured
			return true
		}
		if !readyReplicasData.Exists() {
			return false
		}
		readyReplicas := readyReplicasData.ToInt()
		return replicas == readyReplicas
	}
}

//GetAll returns a []node list with all existing nodes
func (msl *MachineSetList) GetAll() ([]MachineSet, error) {
	allMSResources, err := msl.ResourceList.GetAll()
	if err != nil {
		return nil, err
	}
	allMS := make([]MachineSet, 0, len(allMSResources))

	for _, msRes := range allMSResources {
		allMS = append(allMS, *NewMachineSet(msl.oc, msRes.GetNamespace(), msRes.GetName()))
	}

	return allMS, nil
}

// PollAllMachineSetsReady returns a function that can be used by Gomega "Eventually" and "Consistently" to check that all MachineSet instances are ready
func (msl MachineSetList) PollAllMachineSetsReady() func() bool {
	return func() bool {
		allMS, err := msl.GetAll()
		if err != nil {
			return false
		}

		for _, ms := range allMS {
			if !ms.PollIsReady()() {
				return false
			}
		}

		return true
	}
}
