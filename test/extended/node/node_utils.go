package node

import (
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

type podModifyDescription struct {
	name          string
	namespace     string
	mountpath     string
	command       string
	args          string
	restartPolicy string
	user          string
	role          string
	level         string
	template      string
}

type podLivenessProbe struct {
	name                  string
	namespace             string
	overridelivenessgrace string
	terminationgrace      int
	failurethreshold      int
	periodseconds         int
	template              string
}

type kubeletCfgMaxpods struct {
	name       string
	labelkey   string
	labelvalue string
	maxpods    int
	template   string
}

type ctrcfgDescription struct {
	namespace  string
	pidlimit   int
	loglevel   string
	overlay    string
	logsizemax string
	command    string
	configFile string
	template   string
}

type objectTableRefcscope struct {
	kind string
	name string
}

type podTerminationDescription struct {
	name      string
	namespace string
	template  string
}

type podOOMDescription struct {
	name      string
	namespace string
	template  string
}

type podInitConDescription struct {
	name      string
	namespace string
	template  string
}

type podUserNSDescription struct {
	name      string
	namespace string 
	template  string
}

type podSleepDescription struct {
	namespace string
	template  string
}

type kubeletConfigDescription struct {
	name       string
	labelkey   string
	labelvalue string
	template   string
}

type memHogDescription struct {
	name       string
	namespace  string
	labelkey   string
	labelvalue string
	template   string
}

type podTwoContainersDescription struct {
	name      string
	namespace string
	template  string
}

func (podUserNS *podUserNSDescription) createPodUserNS(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podUserNS.template, "-p", "NAME="+podUserNS.name, "NAMESPACE="+podUserNS.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podUserNS *podUserNSDescription) deletePodUserNS(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podUserNS.namespace, "pod", podUserNS.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (kubeletConfig *kubeletConfigDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kubeletConfig.template, "-p", "NAME="+kubeletConfig.name, "LABELKEY="+kubeletConfig.labelkey, "LABELVALUE="+kubeletConfig.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (memHog *memHogDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", memHog.template, "-p", "NAME="+memHog.name, "LABELKEY="+memHog.labelkey, "LABELVALUE="+memHog.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podSleep *podSleepDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podSleep.template, "-p", "NAMESPACE="+podSleep.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Delete Namespace with all resources
func (podSleep *podSleepDescription) deleteProject(oc *exutil.CLI) error {
	e2e.Logf("Deleting Project ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", podSleep.namespace).Execute()
}

func (podInitCon *podInitConDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podInitCon.template, "-p", "NAME="+podInitCon.name, "NAMESPACE="+podInitCon.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podInitCon *podInitConDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podInitCon.namespace, "pod", podInitCon.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podOOM *podOOMDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podOOM.template, "-p", "NAME="+podOOM.name, "NAMESPACE="+podOOM.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podOOM *podOOMDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podOOM.namespace, "pod", podOOM.name).Execute()
}
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func (kubeletcfg *kubeletCfgMaxpods) createKubeletConfigMaxpods(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kubeletcfg.template, "-p", "NAME="+kubeletcfg.name, "LABELKEY="+kubeletcfg.labelkey, "LABELVALUE="+kubeletcfg.labelvalue, "MAXPODS="+strconv.Itoa(kubeletcfg.maxpods))
	if err != nil {
		e2e.Logf("the err of createKubeletConfigMaxpods:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (kubeletcfg *kubeletCfgMaxpods) deleteKubeletConfigMaxpods(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", kubeletcfg.name).Execute()
	if err != nil {
		e2e.Logf("the err of deleteKubeletConfigMaxpods:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podLivenessProbe) createPodLivenessProbe(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "OVERRIDELIVENESSGRACE="+pod.overridelivenessgrace, "TERMINATIONGRACE="+strconv.Itoa(pod.terminationgrace), "FAILURETHRESHOLD="+strconv.Itoa(pod.failurethreshold), "PERIODSECONDS="+strconv.Itoa(pod.periodseconds))
	if err != nil {
		e2e.Logf("the err of createPodLivenessProbe:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podLivenessProbe) deletePodLivenessProbe(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", pod.namespace, "pod", pod.name).Execute()
	if err != nil {
		e2e.Logf("the err of deletePodLivenessProbe:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podModify.template, "-p", "NAME="+podModify.name, "NAMESPACE="+podModify.namespace, "MOUNTPATH="+podModify.mountpath, "COMMAND="+podModify.command, "ARGS="+podModify.args, "POLICY="+podModify.restartPolicy, "USER="+podModify.user, "ROLE="+podModify.role, "LEVEL="+podModify.level)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podModify.namespace, "pod", podModify.name).Execute()
}

func (podTermination *podTerminationDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podTermination.template, "-p", "NAME="+podTermination.name, "NAMESPACE="+podTermination.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podTermination *podTerminationDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podTermination.namespace, "pod", podTermination.name).Execute()
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "node-config.json")
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("The resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func podStatusReason(oc *exutil.CLI) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.waiting.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "CrashLoopBackOff") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatusterminatedReason(oc *exutil.CLI) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.terminated.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "Error") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatus(oc *exutil.CLI) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.phase}", "-n", oc.Namespace()).Output()
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "Running") && !strings.Contains(status, "Pending") {
			e2e.Logf("Pod status is : %s", status)
			return true, nil
		}
		return false, nil
	})
}

func podEvent(oc *exutil.CLI, timeout int, keyword string) error {
	return wait.Poll(10*time.Second, time.Duration(timeout)*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", oc.Namespace()).Output()
		if err != nil {
			e2e.Logf("Can't get events from test project, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(keyword, output); matched {
			e2e.Logf(keyword)
			return true, nil
		}
		return false, nil
	})
}

func kubeletNotPromptDupErr(oc *exutil.CLI, keyword string, name string) error {
	return wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
		re := regexp.MustCompile(keyword)
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", name, "-o=jsonpath={.status.conditions[*]}").Output()
		if err != nil {
			e2e.Logf("Can't get kubeletconfig status, error: %s. Trying again", err)
			return false, nil
		}
		found := re.FindAllString(output, -1)
		if lenStr := len(found); lenStr > 1 {
			e2e.Logf("[%s] appear %d times.", keyword, lenStr)
			return false, nil
		} else if lenStr == 1 {
			e2e.Logf("[%s] appear %d times.\nkubeletconfig not prompt duplicate error message", keyword, lenStr)
			return true, nil
		} else {
			e2e.Logf("error: kubelet not prompt [%s]", keyword)
			return false, nil
		}
	})
}

func volStatus(oc *exutil.CLI) error {
	e2e.Logf("check content of volume")
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("init-volume", "-c", "hello-pod", "cat", "/init-test/volume-test", "-n", oc.Namespace()).Output()
		e2e.Logf("The content of the vol is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "This is OCP volume test") {
			e2e.Logf(" Init containers with volume work fine \n")
			return true, nil
		}
		return false, nil
	})
}

func ContainerSccStatus(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "hello-pod", "-o=jsonpath={.spec.securityContext.seLinuxOptions.*}", "-n", oc.Namespace()).Output()
		e2e.Logf("The Container SCC Content is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "unconfined_u unconfined_r s0:c25,c968") {
			e2e.Logf("SeLinuxOptions in pod applied to container Sucessfully \n")
			return true, nil
		}
		return false, nil
	})
}

func (ctrcfg *ctrcfgDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ctrcfg.template, "-p", "LOGLEVEL="+ctrcfg.loglevel, "OVERLAY="+ctrcfg.overlay, "LOGSIZEMAX="+ctrcfg.logsizemax)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func cleanupObjectsClusterScope(oc *exutil.CLI, objs ...objectTableRefcscope) {
	for _, v := range objs {
		e2e.Logf("\n Start to remove: %v", v)
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, v.name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (ctrcfg *ctrcfgDescription) checkCtrcfgParameters(oc *exutil.CLI) error {
	return wait.Poll(10*time.Minute, 11*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)

			if nodeStatus == "Ready" {
				criostatus, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "crio", "config").OutputToFile("crio.conf")
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf(`\nCRI-O PARAMETER ON THE WORKER NODE :` + fmt.Sprintf("%s", v))
				e2e.Logf("\ncrio config file path is  %v", criostatus)

				wait.Poll(2*time.Second, 1*time.Minute, func() (bool, error) {
					result, err1 := exec.Command("bash", "-c", "cat "+criostatus+" | egrep 'pids_limit|log_level'").Output()
					if err != nil {
						e2e.Failf("the result of ReadFile:%v", err1)
						return false, nil
					}
					e2e.Logf("\nCtrcfg Parameters is %s", result)
					if strings.Contains(string(result), "debug") && strings.Contains(string(result), "2048") {
						e2e.Logf("\nCtrcfg parameter pod limit and log_level configured successfully")
						return true, nil
					}
					return false, nil
				})
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
			}
		}
		return true, nil
	})
}

func (podTermination *podTerminationDescription) getTerminationGrace(oc *exutil.CLI) error {
	e2e.Logf("check terminationGracePeriodSeconds period")
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podTermination.namespace).Output()
		e2e.Logf("The nodename is %v", nodename)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", nodename), "-o=jsonpath={.status.conditions[3].type}").Output()
		e2e.Logf("The Node state is %v", nodeStatus)
		o.Expect(err).NotTo(o.HaveOccurred())
		containerID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.containerStatuses[0].containerID}", "-n", podTermination.namespace).Output()
		e2e.Logf("The containerID is %v", containerID)
		o.Expect(err).NotTo(o.HaveOccurred())
		if nodeStatus == "Ready" {
			terminationGrace, err := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", nodename), "--", "chroot", "/host", "systemctl", "show", fmt.Sprintf("%s", containerID)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(terminationGrace), "TimeoutStopUSec=1min 30s") {
				e2e.Logf("\nTERMINATION GRACE PERIOD IS SET CORRECTLY")
				return true, nil
			} else {
				e2e.Logf("\ntermination grace is NOT Updated")
				return false, nil
			}
		}
		return false, nil
	})
}

func (podOOM *podOOMDescription) podOOMStatus(oc *exutil.CLI) error {
	return wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
		podstatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.containerStatuses[0].lastState.terminated.reason}", "-n", podOOM.namespace).Output()
		e2e.Logf("The podstatus shows %v", podstatus)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(podstatus), "OOMKilled") {
			e2e.Logf("\nPOD TERMINATED WITH OOM KILLED SITUATION")
			return true, nil
		} else {
			e2e.Logf("\nWaiting for status....")
			return false, nil
		}
		return false, nil
	})
}

func (podInitCon *podInitConDescription) containerExit(oc *exutil.CLI) error {
	return wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
		initConStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.initContainerStatuses[0].state.terminated.reason}", "-n", podInitCon.namespace).Output()
		e2e.Logf("The initContainer status is %v", initConStatus)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(initConStatus), "Completed") {
			e2e.Logf("The initContainer exit normally")
			return true, nil
		} else {
			e2e.Logf("The initContainer not exit!")
			return false, nil
		}
		return false, nil
	})
}

func (podInitCon *podInitConDescription) deleteInitContainer(oc *exutil.CLI) error {
	nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podInitCon.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	containerID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.initContainerStatuses[0].containerID}", "-n", podInitCon.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The containerID is %v", containerID)
	initContainerID := string(containerID)[8:]
	e2e.Logf("The initContainerID is %s", initContainerID)
	return oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", nodename), "--", "chroot", "/host", "crictl", "rm", initContainerID).Execute()
}

func (podInitCon *podInitConDescription) initContainerNotRestart(oc *exutil.CLI) error {
	return wait.Poll(3*time.Minute, 6*time.Minute, func() (bool, error) {
		re := regexp.MustCompile("running")
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", podInitCon.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", podInitCon.namespace, "--", "cat", "/mnt/data/test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		found := re.FindAllString(output, -1)
		if lenStr := len(found); lenStr > 1 {
			e2e.Logf("initContainer restart %d times.", (lenStr - 1))
			return false, nil
		} else if lenStr == 1 {
			e2e.Logf("initContainer not restart")
			return true, nil
		}
		return false, nil
	})
}

func checkNodeStatus(oc *exutil.CLI, workerNodeName string) error {
	return wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
		nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", workerNodeName, "-o=jsonpath={.status.conditions[3].type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Status is %s\n", nodeStatus)
		if nodeStatus == "Ready" {
			e2e.Logf("\n WORKER NODE IS READY\n ")
		} else {
			e2e.Logf("\n WORKERNODE IS NOT READY\n ")
			return false, nil
		}
		return true, nil
	})
}

func getSingleWorkerNode(oc *exutil.CLI) string {
	workerNodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nWorker Node Name is %v", workerNodeName)
	return workerNodeName
}

func getSingleMasterNode(oc *exutil.CLI) string {
	masterNodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[1].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nMaster Node Name is %v", masterNodeName)
	return masterNodeName
}

func addLabelToNode(oc *exutil.CLI, label string, workerNodeName string, resource string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args(resource, workerNodeName, label).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nLabel Added")
}

func removeLabelFromNode(oc *exutil.CLI, label string, workerNodeName string, resource string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args(resource, workerNodeName, label).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nLabel Removed")
}

func rebootNode(oc *exutil.CLI, workerNodeName string) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		e2e.Logf("\nRebooting....")
		_, err1 := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+workerNodeName, "--", "chroot", "/host", "reboot").Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		return true, nil
	})
}

func masterNodeLog(oc *exutil.CLI, masterNode string) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+masterNode, "--", "chroot", "/host", "journalctl", "-u", "crio").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(status, "layer not known") {
			e2e.Logf("\nTest successfully executed")
		} else {
			e2e.Logf("\nTest fail executed, and try next")
			return false, nil
		}
		return true, nil
	})
}

func getmcpStatus(oc *exutil.CLI, nodeName string) error {
	return wait.Poll(10*time.Second, 15*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeName, "-ojsonpath={.status.conditions[4].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nCurrent mcp UPDATING Status is %s\n", status)
		if strings.Contains(status, "False") {
			e2e.Logf("\nmcp updated successfully ")
		} else {
			e2e.Logf("\nmcp is still in UPDATING state")
			return false, nil
		}
		return true, nil
	})
}

func getWorkerNodeDescribe(oc *exutil.CLI, workerNodeName string) error {
	return wait.Poll(3*time.Second, 1*time.Minute, func() (bool, error) {
		nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node", workerNodeName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(nodeStatus, "EvictionThresholdMet") {
			e2e.Logf("\n WORKER NODE MET EVICTION THRESHOLD\n ")
		} else {
			e2e.Logf("\n WORKER NODE DO NOT HAVE MEMORY PRESSURE\n ")
			return false, nil
		}
		return true, nil
	})
}

func (podTwoContainers *podTwoContainersDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podTwoContainers.template, "-p", "NAME="+podTwoContainers.name, "NAMESPACE="+podTwoContainers.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}
func (podTwoContainers *podTwoContainersDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podTwoContainers.namespace, "pod", podTwoContainers.name).Execute()
}

func (podUserNS *podUserNSDescription) crioWorkloadConfigExist(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name	
		workloadString, err := oc.AsAdmin().Run("debug").Args(`node/`+nodename, "--", "chroot", "/host", "cat", "/etc/crio/crio.conf.d/00-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(workloadString), "crio.runtime.workloads.openshift-builder") && strings.Contains(string(workloadString), "io.kubernetes.cri-o.userns-mode") && strings.Contains(string(workloadString), "io.kubernetes.cri-o.Devices"){
			e2e.Logf("the crio workload exist in /etc/crio/crio.conf.d/00-default")
		} else {
			e2e.Logf("the crio workload not exist in /etc/crio/crio.conf.d/00-default")
			return false, nil
		}
		return true, nil
	})
}

func (podUserNS *podUserNSDescription) userContainersExistForNS(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		userContainers, err := oc.AsAdmin().Run("debug").Args(`node/`+nodename, "--", "chroot", "/host", "cat", "/etc/subuid").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		groupContainers, err := oc.AsAdmin().Run("debug").Args(`node/`+nodename, "--", "chroot", "/host", "cat", "/etc/subgid").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(userContainers), "containers") && strings.Contains(string(groupContainers), "containers"){
			e2e.Logf("the user containers exist in /etc/subuid and /etc/subgid")
		} else {
			e2e.Logf("the user containers not exist in /etc/subuid and /etc/subgid")
			return false, nil
		}
		return true, nil
	})
}

func (podUserNS *podUserNSDescription) podRunInUserNS(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", podUserNS.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		idString, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", podUserNS.namespace, podName, "id").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(idString), "uid=0(root) gid=0(root) groups=0(root)"){
			e2e.Logf("the user id in pod is root")
			podUserNSstr, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", podUserNS.namespace, podName, "lsns", "-o", "NS", "-t", "user").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("string(podUserNS) is : %s", string(podUserNSstr))
			podNS := strings.Fields(string(podUserNSstr))
			e2e.Logf("pod user namespace : %s",podNS[1])

			nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podUserNS.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeUserNS, err := oc.AsAdmin().Run("debug").Args(`node/`+string(nodename), "--", "chroot", "/host", "lsns", "-t", "user").Output() 
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeNSstr := strings.Split(string(nodeUserNS), "\n")
			e2e.Logf("host ns string : %s",nodeNSstr[3])
			if strings.Contains(nodeNSstr[3], "/usr/lib/systemd/systemd") {
				nodeNS := strings.Fields(nodeNSstr[3])
				e2e.Logf("host user namespace : %s",nodeNS[0])
				if nodeNS[0] == podNS[1] {
					e2e.Logf("pod run in the same user namespace with host")
					return false, nil
				}
			} else {
				e2e.Logf("root user not found from cmd 'lsns -t user' on host")
				return false, nil
			}
			e2e.Logf("pod run in different user namespace with host")
			return true, nil
		} else {
			e2e.Logf("the user id in pod is not root")
			return false, nil
		}
	})
}
