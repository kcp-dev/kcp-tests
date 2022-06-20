package storage

import (
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	o "github.com/onsi/gomega"
)

// Execute command in node
func execCommandInSpecificNode(oc *exutil.CLI, nodeHostName string, command string) (string, error) {
	var output string
	debugPodNamespace := oc.Namespace()
	// Check whether current namespace is Active
	nsState, err := oc.AsAdmin().Run("get").Args("ns/"+oc.Namespace(), "-o=jsonpath={.status.phase}", "--ignore-not-found").Output()
	if nsState != "Active" || err != nil {
		debugPodNamespace = "default"
	}
	argsCmd := []string{"-n", debugPodNamespace, "node/" + nodeHostName, "-q", "--", "chroot", "/host", "bin/sh", "-c", command}
	stdOut, stdErr, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(argsCmd...).Outputs()
	debugLogf("Executed \""+command+"\" on node \"%s\":\n*stdErr* :\"%s\"\n*stdOut* :\"%s\".", nodeHostName, stdErr, stdOut)
	// Adapt Pod Security changed on k8s v1.23+
	// https://kubernetes.io/docs/tutorials/security/cluster-level-pss/
	// Ignore the oc debug node output warning info: "Warning: would violate PodSecurity "restricted:latest": host namespaces (hostNetwork=true, hostPID=true), ..."
	if strings.ContainsAny(stdErr, "warning") {
		output = stdOut
	} else {
		output = strings.TrimSpace(strings.Join([]string{stdErr, stdOut}, "\n"))
	}
	if err != nil {
		e2e.Logf("Execute \""+command+"\" on node \"%s\" *failed with* : \"%v\".", nodeHostName, err)
		return output, err
	}
	debugLogf("Executed \""+command+"\" on node \"%s\" *Output is* : \"%v\".", nodeHostName, output)
	e2e.Logf("Executed \""+command+"\" on node \"%s\" *Successed* ", nodeHostName)
	return output, nil
}

// Check the Volume mounted on the Node
func checkVolumeMountOnNode(oc *exutil.CLI, volumeName string, nodeName string) {
	command := "mount | grep " + volumeName
	err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		_, err := execCommandInSpecificNode(oc, nodeName, command)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check volume: \"%s\" mount on node: \"%s\" failed", volumeName, nodeName))
}

// Check the Volume not mounted on the Node
func checkVolumeNotMountOnNode(oc *exutil.CLI, volumeName string, nodeName string) {
	command := "mount | grep -c \"" + volumeName + "\" || true"
	err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		count, err := execCommandInSpecificNode(oc, nodeName, command)
		if err != nil {
			e2e.Logf("Err Occurred: %v", err)
			return false, err
		}
		if count == "0" {
			e2e.Logf("Volume: \"%s\" umount from node \"%s\" successfully", volumeName, nodeName)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check volume: \"%s\" unmount from node: \"%s\" timeout", volumeName, nodeName))
}

// Check the Volume not detached from the Node
func checkVolumeDetachedFromNode(oc *exutil.CLI, volumeName string, nodeName string) {
	command := "lsblk | grep -c \"" + volumeName + "\" || true"
	err := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
		count, err := execCommandInSpecificNode(oc, nodeName, command)
		if err != nil {
			e2e.Logf("Err Occurred: %v", err)
			return false, err
		}
		if count == "0" {
			e2e.Logf("Volume: \"%s\" detached from node \"%s\" successfully", volumeName, nodeName)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check volume: \"%s\" detached from node: \"%s\" timeout", volumeName, nodeName))
}

// Check the mounted volume on the Node contains content by cmd
func checkVolumeMountCmdContain(oc *exutil.CLI, volumeName string, nodeName string, content string) {
	command := "mount | grep " + volumeName
	err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		msg, err := execCommandInSpecificNode(oc, nodeName, command)
		if err != nil {
			return false, nil
		}
		return strings.Contains(msg, content), nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check volume: \"%s\" mount in node : \"%s\" contains  \"%s\" failed", volumeName, nodeName, content))
}

// Get the Node List for pod with label
func getNodeListForPodByLabel(oc *exutil.CLI, namespace string, labelName string) ([]string, error) {
	podsList, err := getPodsListByLabel(oc, namespace, labelName)
	o.Expect(err).NotTo(o.HaveOccurred())
	var nodeList []string
	for _, pod := range podsList {
		nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", pod, "-n", namespace, "-o=jsonpath={.spec.nodeName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s is on Node:\"%s\"", pod, nodeName)
		nodeList = append(nodeList, nodeName)
	}
	return nodeList, err
}

func getNodeNameByPod(oc *exutil.CLI, namespace string, podName string) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The node name is %s", nodeName)
	return nodeName
}

// Get the cluster wokernodes info
func getWorkersInfo(oc *exutil.CLI) string {
	workersInfo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return workersInfo
}

func getWorkersList(oc *exutil.CLI) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(output, " ")
}

// Get the cluster schedulable woker nodes names with the same avaiable zone
func getSchedulableWorkersWithSameAz(oc *exutil.CLI) (schedulableWorkersWithSameAz []string, azName string) {
	var (
		workersInfo              = getWorkersInfo(oc)
		workers                  = strings.Split(strings.Trim(strings.Trim(gjson.Get(workersInfo, "items.#.metadata.name").String(), "["), "]"), ",")
		schedulableWorkersWithAz = make(map[string]string)
		zonePath                 = `metadata.labels.topology\.kubernetes\.io\/zone`
	)
	for _, worker := range workers {
		readyStatus := gjson.Get(workersInfo, "items.#(metadata.name="+worker+").status.conditions.#(type=Ready).status").String()
		scheduleFlag := gjson.Get(workersInfo, "items.#(metadata.name="+worker+").spec.unschedulable").String()
		workerOS := gjson.Get(workersInfo, "items.#(metadata.name="+worker+").metadata.labels.kubernetes\\.io\\/os").String()
		if readyStatus == "True" && scheduleFlag != "true" && workerOS == "linux" {
			azName = gjson.Get(workersInfo, "items.#(metadata.name="+worker+")."+zonePath).String()
			if azName == "" {
				azName = "noneAzCluster"
			}
			if _, ok := schedulableWorkersWithAz[azName]; ok {
				e2e.Logf("Schedulable workers %s,%s in the same az %s", worker, schedulableWorkersWithAz[azName], azName)
				return append(schedulableWorkersWithSameAz, worker, schedulableWorkersWithAz[azName]), azName
			}
			schedulableWorkersWithAz[azName] = worker
		}
	}
	e2e.Logf("*** The test cluster has less than two schedulable linux workers in each avaiable zone! ***")
	return nil, azName
}

// Drain specified node
func drainSpecificNode(oc *exutil.CLI, nodeName string) {
	e2e.Logf("oc adm drain nodes/" + nodeName + " --ignore-daemonsets --delete-emptydir-data --force --timeout=600s")
	err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("drain", "nodes/"+nodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force", "--timeout=600s").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Uncordon specified node
func uncordonSpecificNode(oc *exutil.CLI, nodeName string) error {
	e2e.Logf("oc adm uncordon nodes/" + nodeName)
	return oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", "nodes/"+nodeName).Execute()
}

// Waiting specified node avaiable: scheduleable and ready
func waitNodeAvaiable(oc *exutil.CLI, nodeName string) {
	err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		nodeInfo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+nodeName, "-o", "json").Output()
		if err != nil {
			e2e.Logf("Get node status Err Occurred: \"%v\", try next round", err)
			return false, nil
		}
		if !gjson.Get(nodeInfo, `spec.unschedulable`).Exists() && gjson.Get(nodeInfo, `status.conditions.#(type=Ready).status`).String() == "True" {
			e2e.Logf("Node: \"%s\" is ready to use", nodeName)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting Node: \"%s\" become ready to use timeout", nodeName))
}

// Get Region info
func getClusterRegion(oc *exutil.CLI) string {
	node := getWorkersList(oc)[0]
	region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o=jsonpath={.metadata.labels.failure-domain\\.beta\\.kubernetes\\.io\\/region}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return region
}

// Check zoned or unzonded nodes in cluster, currently works for azure only
func checkNodeZoned(oc *exutil.CLI) bool {
	// https://kubernetes-sigs.github.io/cloud-provider-azure/topics/availability-zones/#node-labels
	if cloudProvider == "azure" {
		node := getWorkersList(oc)[0]
		zone, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o=jsonpath={.metadata.labels.failure-domain\\.beta\\.kubernetes\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		region := getClusterRegion(oc)
		e2e.Logf("The zone is %s", zone)
		e2e.Logf("The region is %s", region)
		//if len(zone) == 1 {
		if !strings.Contains(zone, region) {
			return false
		}
	}
	return true
}

type node struct {
	name         string
	instanceID   string
	avaiableZone string
	osType       string
	osImage      string
	osID         string
	role         string
	scheduleable bool
	readyStatus  string // "True", "Unknown"(Node is poweroff or disconnect), "False"
	architecture string
}

// Get cluster all node information
func getAllNodesInfo(oc *exutil.CLI) []node {
	var (
		// nodes []node
		nodes    = make([]node, 0, 10)
		zonePath = `metadata.labels.topology\.kubernetes\.io\/zone`
		nodeRole string
	)
	nodesInfoJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodesList := strings.Split(strings.Trim(strings.Trim(gjson.Get(nodesInfoJSON, "items.#.metadata.name").String(), "["), "]"), ",")
	for _, nodeName := range nodesList {
		nodeName = strings.Trim(nodeName, "\"")
		if gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").metadata.labels.node-role\\.kubernetes\\.io\\/master").Exists() {
			if gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").metadata.labels.node-role\\.kubernetes\\.io\\/worker").Exists() {
				nodeRole = "masterAndworker"
			} else {
				nodeRole = "master"
			}
		} else {
			nodeRole = "worker"
		}
		nodeAvaiableZone := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+")."+zonePath).String()
		// Enchancemant: It seems sometimes aws worker node miss kubernetes az label, maybe caused by other parallel cases
		if nodeAvaiableZone == "" && cloudProvider == "aws" {
			e2e.Logf("The node \"%s\" kubernetes az label not exist, retry get from csi az label", nodeName)
			zonePath = `metadata.labels.topology\.ebs\.csi\.aws\.com\/zone`
			nodeAvaiableZone = gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+")."+zonePath).String()
		}
		readyStatus := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").status.conditions.#(type=Ready).status").String()
		scheduleFlag := !gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").spec.unschedulable").Exists()
		nodeOsType := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").metadata.labels.kubernetes\\.io\\/os").String()
		nodeOsID := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").metadata.labels.node\\.openshift\\.io\\/os_id").String()
		nodeOsImage := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").status.nodeInfo.osImage").String()
		nodeArch := gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+").status.nodeInfo.architecture").String()
		tempSlice := strings.Split(gjson.Get(nodesInfoJSON, "items.#(metadata.name="+nodeName+")."+"spec.providerID").String(), "/")
		nodeInstanceID := tempSlice[len(tempSlice)-1]
		nodes = append(nodes, node{
			name:         nodeName,
			instanceID:   nodeInstanceID,
			avaiableZone: nodeAvaiableZone,
			osType:       nodeOsType,
			osID:         nodeOsID,
			osImage:      nodeOsImage,
			role:         nodeRole,
			scheduleable: scheduleFlag,
			architecture: nodeArch,
			readyStatus:  readyStatus,
		})
	}
	e2e.Logf("*** The \"%s\" Cluster nodes info is ***:\n \"%+v\"", cloudProvider, nodes)
	return nodes
}

// Get all schedulable linux wokers
func getSchedulableLinuxWorkers(allNodes []node) (linuxWorkers []node) {
	linuxWorkers = make([]node, 0, 6)
	for _, myNode := range allNodes {
		if myNode.scheduleable && myNode.osType == "linux" && strings.Contains(myNode.role, "worker") && myNode.readyStatus == "True" {
			linuxWorkers = append(linuxWorkers, myNode)
		}
	}
	e2e.Logf("The schedulable linux workers are: \"%+v\"", linuxWorkers)
	return linuxWorkers
}

// Get all schedulable rhel wokers
func getSchedulableRhelWorkers(allNodes []node) []node {
	schedulableRhelWorkers := make([]node, 0, 6)
	for _, myNode := range allNodes {
		if myNode.scheduleable && myNode.osID == "rhel" && strings.Contains(myNode.role, "worker") && myNode.readyStatus == "True" {
			schedulableRhelWorkers = append(schedulableRhelWorkers, myNode)
		}
	}
	e2e.Logf("The schedulable RHEL workers are: \"%+v\"", schedulableRhelWorkers)
	return schedulableRhelWorkers
}

// Get one cluster schedulable linux woker, rhel linux worker first
func getOneSchedulableWorker(allNodes []node) (expectedWorker node) {
	schedulableRhelWorkers := getSchedulableRhelWorkers(allNodes)
	if len(schedulableRhelWorkers) != 0 {
		expectedWorker = schedulableRhelWorkers[0]
	} else {
		for _, myNode := range allNodes {
			if myNode.scheduleable && myNode.osType == "linux" && strings.Contains(myNode.role, "worker") && myNode.readyStatus == "True" {
				expectedWorker = myNode
				break
			}
		}
	}
	e2e.Logf("Get the schedulableWorker is \"%+v\"", expectedWorker)
	o.Expect(expectedWorker.name).NotTo(o.BeEmpty())
	return expectedWorker
}

// Get one cluster schedulable master woker
func getOneSchedulableMaster(allNodes []node) (expectedMater node) {
	for _, myNode := range allNodes {
		if myNode.scheduleable && myNode.osType == "linux" && strings.Contains(myNode.role, "master") && myNode.readyStatus == "True" {
			expectedMater = myNode
			break
		}
	}
	e2e.Logf("Get the schedulableMaster is \"%+v\"", expectedMater)
	o.Expect(expectedMater.name).NotTo(o.BeEmpty())
	return expectedMater
}
