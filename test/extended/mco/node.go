package mco

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"time"
)

// Node is used to handle node OCP resources
type Node struct {
	Resource
}

// NodeList handles list of nodes
type NodeList struct {
	ResourceList
}

// NewNode construct a new node struct
func NewNode(oc *exutil.CLI, name string) *Node {
	return &Node{*NewResource(oc, "node", name)}
}

// NewNodeList construct a new node list struct to handle all existing nodes
func NewNodeList(oc *exutil.CLI) *NodeList {
	return &NodeList{*NewResourceList(oc, "node")}
}

// String implements the Stringer interface
func (n Node) String() string {
	return n.GetName()
}

// DebugNodeWithChroot creates a debugging session of the node with chroot
func (n *Node) DebugNodeWithChroot(cmd ...string) (string, error) {
	return exutil.DebugNodeWithChroot(n.oc, n.name, cmd...)
}

// DebugNodeWithOptions launch debug container with options e.g. --image
func (n *Node) DebugNodeWithOptions(options []string, cmd ...string) (string, error) {
	return exutil.DebugNodeWithOptions(n.oc, n.name, options, cmd...)
}

// DebugNode creates a debugging session of the node
func (n *Node) DebugNode(cmd ...string) (string, error) {
	return exutil.DebugNode(n.oc, n.name, cmd...)
}

// AddLabel add the given label to the node
func (n *Node) AddLabel(label string, value string) (string, error) {
	return exutil.AddLabelToNode(n.oc, n.name, label, value)

}

// DeleteLabel removes the given label from the node
func (n *Node) DeleteLabel(label string) (string, error) {
	e2e.Logf("Delete label %s from node %s", label, n.GetName())
	return exutil.DeleteLabelFromNode(n.oc, n.name, label)
}

// WaitForLabelRemoved waits until the given label is not present in the node.
func (n *Node) WaitForLabelRemoved(label string) error {
	e2e.Logf("Waiting for label %s to be removed from node %s", label, n.GetName())
	waitErr := wait.Poll(1*time.Minute, 10*time.Minute, func() (bool, error) {
		labels, err := n.Get(`{.metadata.labels}`)
		if err != nil {
			e2e.Logf("Error waiting for labels to be removed:%v, and try next round", err)
			return false, nil
		}
		labelsMap := JSON(labels)
		label, err := labelsMap.GetSafe(label)
		if err == nil && !label.Exists() {
			e2e.Logf("Label %s has been removed from node %s", label, n.GetName())
			return true, nil
		}
		return false, nil
	})

	if waitErr != nil {
		e2e.Logf("Timeout while waiting for label %s to be delete from node %s. Error: %s",
			label,
			n.GetName(),
			waitErr)
	}

	return waitErr
}

// GetMachineConfigDaemon returns the name of the ConfigDaemon pod for this node
func (n *Node) GetMachineConfigDaemon() string {
	machineConfigDaemon, err := exutil.GetPodName(n.oc, "openshift-machine-config-operator", "k8s-app=machine-config-daemon", n.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	return machineConfigDaemon
}

// GetNodeHostname returns the cluster node hostname
func (n *Node) GetNodeHostname() (string, error) {
	return exutil.GetNodeHostname(n.oc, n.name)
}

// ForceReapplyConfiguration create the file `/run/machine-config-daemon-force` in the node
//  in order to force MCO to reapply the current configuration
func (n *Node) ForceReapplyConfiguration() error {
	e2e.Logf("Forcing reapply configuration in node %s", n.GetName())
	_, err := n.DebugNodeWithChroot("touch", "/run/machine-config-daemon-force")

	return err
}

// GetUnitStatus executes `systemctl status` command on the node and returns the output
func (n *Node) GetUnitStatus(unitName string) (string, error) {
	return n.DebugNodeWithChroot("systemctl", "status", unitName)
}

// UnmaskService executes `systemctl unmask` command on the node and returns the output
func (n *Node) UnmaskService(svcName string) (string, error) {
	return n.DebugNodeWithChroot("systemctl", "unmask", svcName)
}

// PollIsCordoned returns a function that can be used by Gomega to poll the if the node is cordoned (with Eventually/Consistently)
func (n *Node) PollIsCordoned() func() bool {
	return func() bool {
		key, err := n.Get(`{.spec.taints[?(@.effect=="NoSchedule")].key}`)
		if err != nil {
			return false
		}
		return key == "node.kubernetes.io/unschedulable"
	}
}

// GetCurrentMachineConfig returns the ID of the current machine config used in the node
func (n *Node) GetCurrentMachineConfig() string {
	return n.GetOrFail(`{.metadata.annotations.machineconfiguration\.openshift\.io/currentConfig}`)
}

// GetDesiredMachineConfig returns the ID of the machine config that we want the node to use
func (n *Node) GetDesiredMachineConfig() string {
	return n.GetOrFail(`{.metadata.annotations.machineconfiguration\.openshift\.io/desiredConfig}`)
}

// GetMachineConfigState returns the State of machineconfiguration process
func (n *Node) GetMachineConfigState() string {
	return n.GetOrFail(`{.metadata.annotations.machineconfiguration\.openshift\.io/state}`)
}

// IsUpdated returns if the node is pending for machineconfig configuration or it is up to date
func (n *Node) IsUpdated() bool {
	return (n.GetCurrentMachineConfig() == n.GetDesiredMachineConfig()) && (n.GetMachineConfigState() == "Done")
}

// IsTainted returns if the node hast taints or not
func (n *Node) IsTainted() bool {
	taint, err := n.Get("{.spec.taints}")
	return err == nil && taint != ""
}

// IsUpdating returns if the node is currently updating the machine configuration
func (n *Node) IsUpdating() bool {
	return n.GetMachineConfigState() == "Working"
}

// IsReady returns boolean 'true' if the node is ready. Else it retruns 'false'.
func (n Node) IsReady() bool {
	readyCondition := JSON(n.GetOrFail(`{.status.conditions[?(@.type=="Ready")]}`))
	return "True" == readyCondition.Get("status").ToString()
}

//GetAll returns a []Node list with all existing nodes
func (nl *NodeList) GetAll() ([]Node, error) {
	allNodeResources, err := nl.ResourceList.GetAll()
	if err != nil {
		return nil, err
	}
	allNodes := make([]Node, 0, len(allNodeResources))

	for _, nodeRes := range allNodeResources {
		allNodes = append(allNodes, *NewNode(nl.oc, nodeRes.name))
	}

	return allNodes, nil
}

// GetAllMasterNodes returns a list of master Nodes
func (nl NodeList) GetAllMasterNodes() ([]Node, error) {
	nl.ByLabel("node-role.kubernetes.io/master=")

	return nl.GetAll()
}

// GetAllWorkerNodes returns a list of worker Nodes
func (nl NodeList) GetAllWorkerNodes() ([]Node, error) {
	nl.ByLabel("node-role.kubernetes.io/worker=")

	return nl.GetAll()
}

// GetAllMasterNodesOrFail returns a list of master Nodes
func (nl NodeList) GetAllMasterNodesOrFail() []Node {
	masters, err := nl.GetAllMasterNodes()
	o.Expect(err).NotTo(o.HaveOccurred())
	return masters
}

// GetAllWorkerNodesOrFail returns a list of worker Nodes. Fail the test case if an error happens.
func (nl NodeList) GetAllWorkerNodesOrFail() []Node {
	workers, err := nl.GetAllWorkerNodes()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}

// GetAllRhelWokerNodesOrFail returns a list with all RHEL nodes in the cluster. Fail the test if an error happens.
func (nl NodeList) GetAllRhelWokerNodesOrFail() []Node {
	nl.ByLabel("node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhel")

	workers, err := nl.GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}

// GetAllCoreOsWokerNodesOrFail returns a list with all CoreOs nodes in the cluster. Fail the test case if an error happens.
func (nl NodeList) GetAllCoreOsWokerNodesOrFail() []Node {
	nl.ByLabel("node-role.kubernetes.io/worker=,node.openshift.io/os_id=rhcos")

	workers, err := nl.GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workers
}

// GetTaintedNodes returns a list with all tainted nodes in the cluster. Fail the test if an error happens.
func (nl *NodeList) GetTaintedNodes() []Node {
	allNodes, err := nl.GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())

	taintedNodes := []Node{}
	for _, node := range allNodes {
		if node.IsTainted() {
			taintedNodes = append(taintedNodes, node)
		}
	}

	return taintedNodes
}
