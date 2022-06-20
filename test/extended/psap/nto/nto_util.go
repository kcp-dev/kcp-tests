package nto

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// isPodInstalled will return true if any pod is found in the given namespace, and false otherwise
func isPodInstalled(oc *exutil.CLI, namespace string) bool {
	e2e.Logf("Checking if pod is found in namespace %s...", namespace)
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(podList.Items) == 0 {
		e2e.Logf("No pod found in namespace %s :(", namespace)
		return false
	}
	e2e.Logf("Pod found in namespace %s!", namespace)
	return true
}

// getNTOPodName checks all pods in a given namespace and returns the first NTO pod name found
func getNTOPodName(oc *exutil.CLI, namespace string) (string, error) {
	podList, err := exutil.GetAllPods(oc, namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	podListSize := len(podList)
	for i := 0; i < podListSize; i++ {
		if strings.Contains(podList[i], "cluster-node-tuning-operator") {
			return podList[i], nil
		}
	}
	return "", fmt.Errorf("NTO pod was not found in namespace %s", namespace)
}

// getTunedState returns a string representation of the spec.managementState of the specified tuned in a given namespace
func getTunedState(oc *exutil.CLI, namespace string, tunedName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", tunedName, "-n", namespace, "-o=jsonpath={.spec.managementState}").Output()
}

// patchTunedState will patch the state of the specified tuned to that specified if supported, will throw an error if patch fails or state unsupported
func patchTunedState(oc *exutil.CLI, namespace string, tunedName string, state string) error {
	state = strings.ToLower(state)
	if state == "unmanaged" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type", "merge", "-n", namespace).Execute()
	} else if state == "managed" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Managed"}}`, "--type", "merge", "-n", namespace).Execute()
	} else if state == "removed" {
		return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "-p", `{"spec":{"managementState":"Removed"}}`, "--type", "merge", "-n", namespace).Execute()
	} else {
		return fmt.Errorf("specified state %s is unsupported", state)
	}
}

// getTunedPriority returns a string representation of the spec.recommend.priority of the specified tuned in a given namespace
func getTunedPriority(oc *exutil.CLI, namespace string, tunedName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", tunedName, "-n", namespace, "-o=jsonpath={.spec.recommend[*].priority}").Output()
}

// patchTunedPriority will patch the priority of the specified tuned to that specified in a given YAML or JSON file
// we cannot directly patch the value since it is nested within a list, thus the need for a patch file for this function
func patchTunedProfile(oc *exutil.CLI, namespace string, tunedName string, patchFile string) error {
	return oc.AsAdmin().WithoutNamespace().Run("patch").Args("tuned", tunedName, "--patch-file="+patchFile, "--type", "merge", "-n", namespace).Execute()
}

// getTunedRender returns a string representation of the rendered for tuned in the given namespace
func getTunedRender(oc *exutil.CLI, namespace string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "tuned", "rendered", "-o", "yaml").Output()
}

// getTunedProfile returns a string representation of the status.tunedProfile of the given node in the given namespace
func getTunedProfile(oc *exutil.CLI, namespace string, tunedNodeName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("profile", tunedNodeName, "-n", namespace, "-o=jsonpath={.status.tunedProfile}").Output()
}

// assertIfTunedProfileApplied checks the logs for a given tuned pod in a given namespace to see if the expected profile was applied
func assertIfTunedProfileApplied(oc *exutil.CLI, namespace string, tunedPodName string, profile string) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		podLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, "--tail=9", tunedPodName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isApplied := strings.Contains(podLogs, "tuned.daemon.daemon: static tuning from profile '"+profile+"' applied")
		if !isApplied {
			e2e.Logf("Profile '%s' has not yet been applied to %s - retrying...", profile, tunedPodName)
			return false, nil
		}
		e2e.Logf("Profile '%s' has been applied to %s - continuing...", profile, tunedPodName)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Profile was not applied to %s within timeout limit (30 seconds)", tunedPodName))
}

// assertIfNodeSchedulingDisabled checks all nodes in a cluster to see if 'SchedulingDisabled' status is present on any node
func assertIfNodeSchedulingDisabled(oc *exutil.CLI) string {
	var nodeNames []string
	err := wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		nodeCheck, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		isNodeSchedulingDisabled := strings.Contains(nodeCheck, "SchedulingDisabled")
		if isNodeSchedulingDisabled {
			e2e.Logf("'SchedulingDisabled' status found!")
			nodeNameReg := regexp.MustCompile(".*SchedulingDisabled.*")
			nodeNameList := nodeNameReg.FindAllString(nodeCheck, -1)
			nodeNamestr := nodeNameList[0]
			nodeNames = strings.Split(nodeNamestr, " ")
			e2e.Logf("Node Names is %v", nodeNames)
			return true, nil
		}
		e2e.Logf("'SchedulingDisabled' status not found - retrying...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "No node was found with 'SchedulingDisabled' status within timeout limit (3 minutes)")
	e2e.Logf("Node Name is %v", nodeNames[0])
	return nodeNames[0]
}

// assertIfMasterNodeChangesApplied checks all nodes in a cluster with the master role to see if 'default_hugepagesz=2M' is present on every node in /proc/cmdline
func assertIfMasterNodeChangesApplied(oc *exutil.CLI, masterNodeName string) {

	err := wait.Poll(1*time.Minute, 5*time.Minute, func() (bool, error) {
		output, err := exutil.DebugNode(oc, masterNodeName, "cat", "/proc/cmdline")
		o.Expect(err).NotTo(o.HaveOccurred())

		isMasterNodeChanged := strings.Contains(output, "default_hugepagesz=2M")
		if isMasterNodeChanged {
			e2e.Logf("Node %v has expected changes:\n%v", masterNodeName, output)
			return true, nil
		}
		e2e.Logf("Node %v does not have expected changes - retrying...", masterNodeName)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Node"+masterNodeName+"did not have expected changes within timeout limit")
}

// assertIfMCPChangesApplied checks the MCP of a given oc client and determines if the machine counts are as expected
func assertIfMCPChangesAppliedByName(oc *exutil.CLI, mcpName string, timeDurationMin int) {
	err := wait.Poll(1*time.Minute, time.Duration(timeDurationMin)*time.Minute, func() (bool, error) {
		var (
			mcpCheckMachineCount        string
			mcpCheckReadyMachineCount   string
			mcpCheckUpdatedMachineCount string
			mcpDegradedMachineCount     string
			err                         error
		)

		//For master node, only make sure one of master is ready.
		if strings.Contains(mcpName, "master") {
			mcpCheckMachineCount = "1"
		} else {
			mcpCheckMachineCount, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.machineCount}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		//Do not check master err due to sometimes SNO can not accesss api server when server rebooted
		if strings.Contains(mcpName, "master") {
			mcpCheckReadyMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.readyMachineCount}").Output()
			mcpCheckUpdatedMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.updatedMachineCount}").Output()
			mcpDegradedMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.degradedMachineCount}").Output()
		} else {
			mcpCheckReadyMachineCount, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.readyMachineCount}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			mcpCheckUpdatedMachineCount, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.updatedMachineCount}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			mcpDegradedMachineCount, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.degradedMachineCount}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		if mcpCheckMachineCount == mcpCheckReadyMachineCount && mcpCheckMachineCount == mcpCheckUpdatedMachineCount && mcpDegradedMachineCount == "0" {
			e2e.Logf("MachineConfigPool checks succeeded!")
			return true, nil
		}
		e2e.Logf("MachineConfigPool %v checks failed, the following values were found (all should be '%v'):\nmachineCount: %v\nreadyMachineCount: %v\nupdatedMachineCount: %v\nmcpDegradedMachine:%v\nRetrying...", mcpName, mcpCheckMachineCount, mcpCheckMachineCount, mcpCheckReadyMachineCount, mcpCheckUpdatedMachineCount, mcpDegradedMachineCount)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "MachineConfigPool checks were not successful within timeout limit")
}

// getMaxUserWatchesValue parses out the line determining max_user_watches in inotify.conf
func getMaxUserWatchesValue(inotify string) string {
	reLine := regexp.MustCompile(`fs.inotify.max_user_watches = \d+`)
	reValue := regexp.MustCompile(`\d+`)
	maxUserWatches := reLine.FindString(inotify)
	maxUserWatchesValue := reValue.FindString(maxUserWatches)
	return maxUserWatchesValue
}

// getMaxUserInstancesValue parses out the line determining max_user_instances in inotify.conf
func getMaxUserInstancesValue(inotify string) string {
	reLine := regexp.MustCompile(`fs.inotify.max_user_instances = \d+`)
	reValue := regexp.MustCompile(`\d+`)
	maxUserInstances := reLine.FindString(inotify)
	maxUserInstancesValue := reValue.FindString(maxUserInstances)
	return maxUserInstancesValue
}

// getKernelPidMaxValue parses out the line determining pid_max in the kernel
func getKernelPidMaxValue(kernel string) string {
	reLine := regexp.MustCompile(`kernel.pid_max = \d+`)
	reValue := regexp.MustCompile(`\d+`)
	pidMax := reLine.FindString(kernel)
	pidMaxValue := reValue.FindString(pidMax)
	return pidMaxValue
}

//Compare if the sysctl parameter is equal to specified value on all the node
func compareSpecifiedValueByNameOnLabelNode(oc *exutil.CLI, labelNodeName, sysctlparm, specifiedvalue string) {

	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	output, err := exutil.DebugNodeWithChroot(oc, labelNodeName, "sysctl", sysctlparm)
	conntrackMax := regexpstr.FindString(output)
	e2e.Logf("The value is %v on %v", conntrackMax, labelNodeName)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))

}

//Compare if the sysctl parameter is not equal to specified value on all the node
func compareSysctlDifferentFromSpecifiedValueByName(oc *exutil.CLI, sysctlparm, specifiedvalue string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	for i := 0; i < nodeListSize; i++ {
		output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", sysctlparm)
		conntrackMax := regexpstr.FindString(output)
		e2e.Logf("The value is %v on %v", conntrackMax, nodeList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
	}

}

//Compare the sysctl parameter's value on specified node, it should different than other node
func compareSysctlValueOnSepcifiedNodeByName(oc *exutil.CLI, tunedNodeName, sysctlparm, defaultvalue, specifiedvalue string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	// tuned nodes should have value of 1048578, others should be 1048576
	regexpstr, _ := regexp.Compile(sysctlparm + ".*")
	for i := 0; i < nodeListSize; i++ {
		output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "sysctl", sysctlparm)
		conntrackMax := regexpstr.FindString(output)
		e2e.Logf("The value is %v on %v", conntrackMax, nodeList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		if nodeList[i] == tunedNodeName {
			o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
		} else {
			if len(defaultvalue) == 0 {
				o.Expect(output).NotTo(o.ContainSubstring(sysctlparm + " = " + specifiedvalue))
			} else {
				o.Expect(output).To(o.ContainSubstring(sysctlparm + " = " + defaultvalue))
			}
		}
	}
}

func getTunedPodNamebyNodeName(oc *exutil.CLI, tunedNodeName, namespace string) string {

	podNames, err := exutil.GetPodName(oc, namespace, "", tunedNodeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	//Get Pod name based on node name, and filter tuned pod name when mulitple pod return on the same node
	regexpstr, err := regexp.Compile(`tuned-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	tunedPodName := regexpstr.FindString(podNames)
	e2e.Logf("The Tuned Pod Name is: %v", tunedPodName)
	return tunedPodName
}

type ntoResource struct {
	name        string
	namespace   string
	template    string
	sysctlparm  string
	sysctlvalue string
}

func (ntoRes *ntoResource) createTunedProfileIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) createDebugTunedProfileIfNotExist(oc *exutil.CLI, isDebug bool) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue, "ISDEBUG="+strconv.FormatBool(isDebug))
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) createIRQSMPAffinityProfileIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", ntoRes.name, "-n", ntoRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No tuned in project: %s, create one: %s", ntoRes.namespace, ntoRes.name))
		exutil.CreateNsResourceFromTemplate(oc, ntoRes.namespace, "--ignore-unknown-parameters=true", "-f", ntoRes.template, "-p", "TUNED_NAME="+ntoRes.name, "SYSCTLPARM="+ntoRes.sysctlparm, "SYSCTLVALUE="+ntoRes.sysctlvalue)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", ntoRes.name, ntoRes.namespace))
	}
}

func (ntoRes *ntoResource) delete(oc *exutil.CLI) {
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ntoRes.namespace, "tuned", ntoRes.name, "--ignore-not-found").Execute()
}

func (ntoRes ntoResource) assertTunedProfileApplied(oc *exutil.CLI) {

	err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoRes.namespace, "profile").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, ntoRes.name) {
			//Check if the new profiles name applied on a node
			e2e.Logf("Current profile for each node: \n%v", output)
			return true, nil
		}
		e2e.Logf("The profile %v is not applied on node, try next around \n", ntoRes.name)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "New tuned profile isn't applied correctly, please check")
}

func assertNTOOperatorLogs(oc *exutil.CLI, namespace string, ntoOperatorPod string, profileName string) {
	ntoOperatorLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, ntoOperatorPod, "--tail=3").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ntoOperatorLogs).To(o.ContainSubstring(profileName))
}

func isOneMasterNode(oc *exutil.CLI) bool {
	masterNodes, _ := exutil.GetClusterNodesBy(oc, "master")
	if len(masterNodes) == 1 {
		return true
	}
	return false
}

func isSNOCluster(oc *exutil.CLI) bool {

	//Only 1 master, 1 worker node and with the same hostname.
	masterNodes, _ := exutil.GetClusterNodesBy(oc, "master")
	workerNodes, _ := exutil.GetClusterNodesBy(oc, "worker")
	if len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
		return true
	}
	return false
}

func assertAffineDefaultCPUSets(oc *exutil.CLI, tunedPodName, namespace string) bool {

	tunedCpusAllowedList, err := exutil.RemoteShPodWithBash(oc, namespace, tunedPodName, "grep ^Cpus_allowed_list /proc/`pgrep openshift-tuned`/status")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Tuned's Cpus_allowed_list is: \n%v", tunedCpusAllowedList)

	chronyCpusAllowedList, err := exutil.RemoteShPodWithBash(oc, namespace, tunedPodName, "grep Cpus_allowed_list /proc/`pidof chronyd`/status")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Chrony's Cpus_allowed_list is: \n%v", chronyCpusAllowedList)

	regTunedCpusAllowedList0, err := regexp.Compile(`.*0-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	regChronyCpusAllowedList1, err := regexp.Compile(`.*0$`)
	o.Expect(err).NotTo(o.HaveOccurred())

	regChronyCpusAllowedList2, err := regexp.Compile(`.*0,2-.*`)
	o.Expect(err).NotTo(o.HaveOccurred())

	isMatch0 := regTunedCpusAllowedList0.MatchString(tunedCpusAllowedList)
	isMatch1 := regChronyCpusAllowedList1.MatchString(chronyCpusAllowedList)
	isMatch2 := regChronyCpusAllowedList2.MatchString(chronyCpusAllowedList)

	if isMatch0 && (isMatch1 || isMatch2) {
		e2e.Logf("assert affine default cpusets result: %v", true)
		return true
	}
	e2e.Logf("assert affine default cpusets result: %v", false)
	return false
}

func assertDebugSettings(oc *exutil.CLI, tunedNodeName string, ntoNamespace string, isDebug string) bool {
	nodeProfile, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("profile", tunedNodeName, "-n", ntoNamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	regDebugCheck, err := regexp.Compile(".*Debug:.*" + isDebug)
	o.Expect(err).NotTo(o.HaveOccurred())

	isMatch := regDebugCheck.MatchString(nodeProfile)
	loglines := regDebugCheck.FindAllString(nodeProfile, -1)
	e2e.Logf("The result is: %v", loglines[0])
	return isMatch
}

func getDefaultSMPAffinityBitMaskbyCPUCores(oc *exutil.CLI, workerNodeName string) string {
	//Currently support 48core cpu worker nodes
	smpbitMask := 0xffffffffffff
	smpbitMaskIntStr := fmt.Sprintf("%d", smpbitMask)

	//Get CPU number in specified worker nodes
	cpuNum, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workerNodeName, "-ojsonpath={.status.capacity.cpu}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	cpuNumStr := string(cpuNum)
	cpuNumInt, err := strconv.Atoi(cpuNumStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the total cpu numbers in worker nodes %v is : %v", workerNodeName, cpuNumStr)

	//Get corresponding smpMask
	rightMoveBit := 48 - cpuNumInt
	smpMaskInt, err := strconv.Atoi(smpbitMaskIntStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	smpMaskStr := fmt.Sprintf("%x", smpMaskInt>>rightMoveBit)
	e2e.Logf("the bit mask for cpu numbers in worker nodes %v is : %v", workerNodeName, smpMaskStr)
	return smpMaskStr
}

//Convert hex into int string
func hexToInt(x string) string {
	base, err := strconv.ParseInt(x, 16, 64)
	o.Expect(err).NotTo(o.HaveOccurred())
	return strconv.FormatInt(base, 10)
}

func assertIsolateCPUCoresAffectedBitMask(defaultSMPBitMask string, isolatedCPU string) string {

	defaultSMPBitMaskStr := hexToInt(defaultSMPBitMask)
	isolatedCPUStr := hexToInt(isolatedCPU)

	defaultSMPBitMaskInt, err := strconv.Atoi(defaultSMPBitMaskStr)
	o.Expect(err).NotTo(o.HaveOccurred())
	isolatedCPUInt, err := strconv.Atoi(isolatedCPUStr)
	o.Expect(err).NotTo(o.HaveOccurred())

	SMPBitMask := fmt.Sprintf("%x", defaultSMPBitMaskInt^isolatedCPUInt)
	return SMPBitMask
}

func assertDefaultIRQSMPAffinityAffectedBitMask(defaultSMPBitMask string, isolatedCPU string) bool {

	var isMatch bool
	defaultSMPBitMaskStr := hexToInt(defaultSMPBitMask)
	isolatedCPUStr := hexToInt(isolatedCPU)

	defaultSMPBitMaskInt, _ := strconv.Atoi(defaultSMPBitMaskStr)
	isolatedCPUInt, err := strconv.Atoi(isolatedCPUStr)
	o.Expect(err).NotTo(o.HaveOccurred())

	if defaultSMPBitMaskInt == isolatedCPUInt {
		isMatch = true
	} else {
		isMatch = false
	}
	return isMatch
}

//AssertTunedAppliedMC Check if customed tuned applied via MCP
func AssertTunedAppliedMC(oc *exutil.CLI, mcpName string, filter string) {
	mcNameList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", "--no-headers", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	mcNameReg, _ := regexp.Compile(".*" + mcpName + ".*")
	mcName := mcNameReg.FindAllString(mcNameList, -1)

	mcOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mcName[0], "-oyaml").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(mcOutput).To(o.ContainSubstring(filter))

	//Print machineconfig content by filter
	mccontent, _ := regexp.Compile(".*" + filter + ".*")
	contentLines := mccontent.FindAllString(mcOutput, -1)
	e2e.Logf("The result is: %v", contentLines[0])
	o.Expect(mcOutput).To(o.ContainSubstring(filter))
}

//AssertTunedAppliedToNode Check if customed tuned applied to a certain node
func AssertTunedAppliedToNode(oc *exutil.CLI, tunedNodeName string, filter string) bool {
	cmdLineOutput, err := exutil.DebugNode(oc, tunedNodeName, "cat", "/proc/cmdline")
	o.Expect(err).NotTo(o.HaveOccurred())
	var isMatch bool
	if strings.Contains(cmdLineOutput, filter) {
		//Print machineconfig content by filter
		cmdLineReg, _ := regexp.Compile(".*" + filter + ".*")
		contentLines := cmdLineReg.FindAllString(cmdLineOutput, -1)
		e2e.Logf("The result is: %v", contentLines[0])
		isMatch = true
	} else {
		isMatch = false
	}
	return isMatch
}

func assertNTOPodLogsLastLines(oc *exutil.CLI, namespace string, ntoPod string, lineN string, timeDurationSec int, filter string) {
	err := wait.Poll(15*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {

		//Remove err assert for SNO, the OCP will can not access temporily when master node restart or certificate key removed
		ntoPodLogs, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", namespace, ntoPod, "--tail="+lineN).Output()

		regNTOPodLogs, err := regexp.Compile(".*" + filter + ".*")
		o.Expect(err).NotTo(o.HaveOccurred())
		isMatch := regNTOPodLogs.MatchString(ntoPodLogs)
		if isMatch {
			loglines := regNTOPodLogs.FindAllString(ntoPodLogs, -1)
			e2e.Logf("The logs of nto pod %v is: \n%v", ntoPod, loglines[0])
			return true, nil
		}
		e2e.Logf("The keywords of nto pod isn't found, try next ...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The tuned pod's log doesn't contain the keywords, please check")
}

func getServiceENDPoint(oc *exutil.CLI, namespace string) string {
	serviceOutput, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("-n", namespace, "service/node-tuning-operator").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	endPointReg, _ := regexp.Compile(".*Endpoints.*")
	endPointIPStr := endPointReg.FindString(serviceOutput)
	endPointIPStrNoSpace := strings.ReplaceAll(endPointIPStr, " ", "")
	endPointIPArr := strings.Split(endPointIPStrNoSpace, ":")
	endPointIP := endPointIPArr[1] + ":" + endPointIPArr[2]
	return endPointIP
}

//AssertNTOCertificateRotate used for check if NTO certificate rotate
func AssertNTOCertificateRotate(oc *exutil.CLI, ntoNamespace string, tunedNodeName string, encodeBase64OpenSSLOutputBefore string, encodeBase64OpenSSLExpireDateBefore string) {

	metricEndpoint := getServiceENDPoint(oc, ntoNamespace)
	err := wait.Poll(15*time.Second, 300*time.Second, func() (bool, error) {

		openSSLOutputAfter, err := exutil.DebugNodeWithOptions(oc, tunedNodeName, []string{"--quiet=true"}, "/bin/bash", "-c", "/host/bin/openssl s_client -connect "+metricEndpoint+" 2>/dev/null </dev/null")
		o.Expect(err).NotTo(o.HaveOccurred())

		openSSLExpireDateAfter, err := exutil.DebugNodeWithOptions(oc, tunedNodeName, []string{"--quiet=true"}, "/bin/bash", "-c", "/host/bin/openssl s_client -connect "+metricEndpoint+" 2>/dev/null </dev/null  | /host/bin/openssl x509 -noout -dates")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("The openSSL Expired Date information of NTO openSSL after rotate as below: \n%v", openSSLExpireDateAfter)

		encodeBase64OpenSSLOutputAfter := exutil.StringToBASE64(openSSLOutputAfter)
		encodeBase64OpenSSLExpireDateAfter := exutil.StringToBASE64(openSSLExpireDateAfter)

		if encodeBase64OpenSSLOutputBefore != encodeBase64OpenSSLOutputAfter && encodeBase64OpenSSLExpireDateBefore != encodeBase64OpenSSLExpireDateAfter {
			e2e.Logf("The certificate has been updated ...")
			return true, nil
		}
		e2e.Logf("The certificate isn't updated, try next round ...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The NTO certificate isn't rotate, please check")
}

func compareCertificateBetweenOpenSSLandTLSSecret(oc *exutil.CLI, ntoNamespace string, tunedNodeName string) {

	metricEndpoint := getServiceENDPoint(oc, ntoNamespace)
	err := wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {

		//Extract certificate from openssl that nto operator service endpoint
		openSSLOutputAfter, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", ntoNamespace, "--quiet=true", "node/"+tunedNodeName, "--", "/bin/bash", "-c", "/host/bin/openssl s_client -connect "+metricEndpoint+" 2>/dev/null </dev/null | sed -ne '/-BEGIN CERTIFICATE-/,/-END CERTIFICATE-/p'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		//Extract tls.crt from secret node-tuning-operator-tls
		encodeBase64tlsCertOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "secret", "node-tuning-operator-tls", `-ojsonpath='{ .data.tls\.crt }'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		tmpTLSCertOutput := strings.Trim(encodeBase64tlsCertOutput, "'")
		tlsCertOutput := exutil.BASE64DecodeStr(tmpTLSCertOutput)

		if strings.Contains(tlsCertOutput, openSSLOutputAfter) {
			e2e.Logf("The certificate is the same ...")
			return true, nil
		}
		e2e.Logf("The certificate is different, try next round ...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The certificate is different, please check")
}

func assertIFChannel(oc *exutil.CLI, namespace string, tunedNodeName string) bool {

	ifName, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", namespace, "--quiet=true", "node/"+tunedNodeName, "--", "find", "/sys/class/net", "-type", "l", "-not", "-lname", "*virtual*", "-a", "-not", "-name", "enP*", "-printf", "%f").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	ethToolsOutput, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", namespace, "--quiet=true", "node/"+tunedNodeName, "--", "ethtool", "-l", ifName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("ethtool -l %v:, \n%v", ifName, ethToolsOutput)

	regChannel, err := regexp.Compile("Combined:.*1")
	o.Expect(err).NotTo(o.HaveOccurred())
	isMatch := regChannel.MatchString(ethToolsOutput)
	if isMatch {
		return true
	}
	return false
}

func compareSpecifiedValueByNameOnLabelNodewithRetry(oc *exutil.CLI, ntoNamespace, nodeName, sysctlparm, specifiedvalue string) {

	err := wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {

		sysctlOutput, err := exutil.DebugNodeWithChroot(oc, nodeName, "sysctl", sysctlparm)
		o.Expect(err).NotTo(o.HaveOccurred())

		regexpstr, _ := regexp.Compile(sysctlparm + " = " + specifiedvalue)
		matchStr := regexpstr.FindString(sysctlOutput)
		e2e.Logf("The value is %v on %v", matchStr, nodeName)

		isMatch := regexpstr.MatchString(sysctlOutput)
		if isMatch {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The certificate is different, please check")
}

func skipDeployPAO(oc *exutil.CLI) bool {

	skipPAO := true
	clusterVersion, _, err := exutil.GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cluster Version: %v", clusterVersion)
	paoDeployOCPVersionList := []string{"4.6", "4.7", "4.8", "4.9", "4.10"}

	for _, v := range paoDeployOCPVersionList {
		if strings.Contains(clusterVersion, v) {
			skipPAO = false
			break
		}
	}
	return skipPAO
}

func assertIOTimeOutandMaxRetries(oc *exutil.CLI, ntoNamespace string) {
	nodeList, err := exutil.GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeListSize := len(nodeList)

	for i := 0; i < nodeListSize; i++ {
		timeoutOutput, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", ntoNamespace, "--quiet=true", "node/"+nodeList[i], "--", "chroot", "/host", "cat", "/sys/module/nvme_core/parameters/io_timeout").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The value of io_timeout is : %v on node %v", timeoutOutput, nodeList[i])
		o.Expect(timeoutOutput).To(o.ContainSubstring("4294967295"))
	}
}

func confirmedTunedReady(oc *exutil.CLI, ntoNamespace string, tunedName string, timeDurationSec int) {
	err := wait.Poll(10*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {

		tunedStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", "-n", ntoNamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if strings.Contains(tunedStatus, tunedName) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "tuned is not ready")
}
