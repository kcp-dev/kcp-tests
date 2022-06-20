package util

import (
	"strings"
)

// GetFirstLinuxWorkerNode returns the first linux worker node in the cluster
func GetFirstLinuxWorkerNode(oc *CLI) (string, error) {
	var (
		workerNode string
		err        error
	)
	workerNode, err = getFirstNodeByOsID(oc, "worker", "rhcos")
	if len(workerNode) == 0 {
		workerNode, err = getFirstNodeByOsID(oc, "worker", "rhel")
	}
	return workerNode, err
}

// GetAllNodesbyOSType returns a list of the names of all linux/windows nodes in the cluster have both linux and windows node
func GetAllNodesbyOSType(oc *CLI, ostype string) ([]string, error) {
	var nodesArray []string
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "kubernetes.io/os="+ostype, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	nodesStr := strings.Trim(nodes, "'")
	//If split an empty string to string array, the default length string array is 1
	//So need to check if string is empty.
	if len(nodesStr) == 0 {
		return nodesArray, err
	}
	nodesArray = strings.Split(nodesStr, " ")
	return nodesArray, err
}

// GetAllNodes returns a list of the names of all nodes in the cluster
func GetAllNodes(oc *CLI) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// GetFirstWorkerNode returns a first worker node
func GetFirstWorkerNode(oc *CLI) (string, error) {
	workerNodes, err := GetClusterNodesBy(oc, "worker")
	return workerNodes[0], err
}

// GetFirstMasterNode returns a first master node
func GetFirstMasterNode(oc *CLI) (string, error) {
	masterNodes, err := GetClusterNodesBy(oc, "master")
	return masterNodes[0], err
}

// GetClusterNodesBy returns the cluster nodes by role
func GetClusterNodesBy(oc *CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// DebugNodeWithChroot creates a debugging session of the node with chroot
func DebugNodeWithChroot(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, []string{}, true, cmd...)
}

// DebugNodeWithOptions launch debug container with options e.g. --image
func DebugNodeWithOptions(oc *CLI, nodeName string, options []string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, options, false, cmd...)
}

// DebugNodeWithOptionsAndChroot launch debug container using chroot and with options e.g. --image
func DebugNodeWithOptionsAndChroot(oc *CLI, nodeName string, options []string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, options, true, cmd...)
}

// DebugNode creates a debugging session of the node
func DebugNode(oc *CLI, nodeName string, cmd ...string) (string, error) {
	return debugNode(oc, nodeName, []string{}, false, cmd...)
}

func debugNode(oc *CLI, nodeName string, cmdOptions []string, needChroot bool, cmd ...string) (string, error) {
	var cargs []string
	cargs = []string{"node/" + nodeName}
	if len(cmdOptions) > 0 {
		cargs = append(cargs, cmdOptions...)
	}
	if needChroot {
		cargs = append(cargs, "--", "chroot", "/host")
	} else {
		cargs = append(cargs, "--")
	}
	cargs = append(cargs, cmd...)
	return oc.AsAdmin().Run("debug").Args(cargs...).Output()
}

// DeleteLabelFromNode delete the custom label from the node
func DeleteLabelFromNode(oc *CLI, node string, label string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, label+"-").Output()
}

// AddLabelToNode add the custom label to the node
func AddLabelToNode(oc *CLI, node string, label string, value string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, label+"="+value).Output()
}

// GetFirstCoreOsWorkerNode returns the first CoreOS worker node
func GetFirstCoreOsWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhcos")
}

// GetFirstRhelWorkerNode returns the first rhel worker node
func GetFirstRhelWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhel")
}

// getFirstNodeByOsID returns the cluster node by role and os id
func getFirstNodeByOsID(oc *CLI, role string, osID string) (string, error) {
	nodes, err := GetClusterNodesBy(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osID {
			return node, err
		}
	}
	return "", err
}

// GetNodeHostname returns the cluster node hostname
func GetNodeHostname(oc *CLI, node string) (string, error) {
	hostname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o", "jsonpath='{..kubernetes\\.io/hostname}'").Output()
	return strings.Trim(hostname, "'"), err
}
