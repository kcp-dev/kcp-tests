package networking

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//struct for sriovnetworknodepolicy and sriovnetwork
type sriovNetResource struct {
	name      string
	namespace string
	tempfile  string
	kind      string
}

type sriovNetworkNodePolicy struct {
	policyName   string
	deviceType   string
	pfName       string
	deviceID     string
	vondor       string
	numVfs       int
	resourceName string
	template     string
	namespace    string
}

type sriovNetwork struct {
	name             string
	resourceName     string
	networkNamespace string
	template         string
	namespace        string
}

type sriovTestPod struct {
	name        string
	namespace   string
	networkName string
	template    string
}

//struct for sriov pod
type sriovPod struct {
	name         string
	tempfile     string
	namespace    string
	ipv4addr     string
	ipv6addr     string
	intfname     string
	intfresource string
}

//delete sriov resource
func (rs *sriovNetResource) delete(oc *exutil.CLI) {
	e2e.Logf("delete %s %s in namespace %s", rs.kind, rs.name, rs.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args(rs.kind, rs.name, "-n", rs.namespace).Execute()
}

//create sriov resource
func (rs *sriovNetResource) create(oc *exutil.CLI, parameters ...string) {
	var configFile string
	cmd := []string{"-f", rs.tempfile, "--ignore-unknown-parameters=true", "-p"}
	for _, para := range parameters {
		cmd = append(cmd, para)
	}
	e2e.Logf("parameters list is %s\n", cmd)
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(cmd...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process sriov resource %v", cmd))
	e2e.Logf("the file of resource is %s\n", configFile)

	_, err1 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", rs.namespace).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
}

//porcess sriov pod template and get a configuration file
func (pod *sriovPod) processPodTemplate(oc *exutil.CLI) string {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args("-f", pod.tempfile, "--ignore-unknown-parameters=true", "-p", "PODNAME="+pod.name, "SRIOVNETNAME="+pod.intfresource,
			"IPV4_ADDR="+pod.ipv4addr, "IPV6_ADDR="+pod.ipv6addr, "-o=jsonpath={.items[0]}").OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process pod resource %v", pod.name))
	e2e.Logf("the file of resource is %s\n", configFile)
	return configFile
}

//create pod
func (pod *sriovPod) createPod(oc *exutil.CLI) string {
	configFile := pod.processPodTemplate(oc)
	podsLog, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("--loglevel=10", "-f", configFile, "-n", pod.namespace).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	return podsLog
}

//delete pod
func (pod *sriovPod) deletePod(oc *exutil.CLI) {
	e2e.Logf("delete pod %s in namespace %s", pod.name, pod.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
}

// check pods of openshift-sriov-network-operator are running
func chkSriovOperatorStatus(oc *exutil.CLI, ns string) {
	e2e.Logf("check if openshift-sriov-network-operator pods are running properly")
	chkPodsStatus(oc, ns, "app=network-resources-injector")
	chkPodsStatus(oc, ns, "app=operator-webhook")
	chkPodsStatus(oc, ns, "app=sriov-network-config-daemon")
	chkPodsStatus(oc, ns, "name=sriov-network-operator")

}

// check specified pods are running
func chkPodsStatus(oc *exutil.CLI, ns, lable string) {
	podsStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", ns, "-l", lable, "-o=jsonpath={.items[*].status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podsStatus = strings.TrimSpace(podsStatus)
	statusList := strings.Split(podsStatus, " ")
	for _, podStat := range statusList {
		o.Expect(podStat).Should(o.MatchRegexp("Running"))
	}
	e2e.Logf("All pods with lable %s in namespace %s are Running", lable, ns)
}

//clear specified sriovnetworknodepolicy
func rmSriovNetworkPolicy(oc *exutil.CLI, policyname, ns string) {
	sriovPolicyList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("SriovNetworkNodePolicy", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(sriovPolicyList, policyname) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("SriovNetworkNodePolicy", policyname, "-n", ns).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForSriovPolicyReady(oc, ns)
	}
	e2e.Logf("SriovNetworkPolicy already be removed")
}

//clear specified sriovnetwork
func rmSriovNetwork(oc *exutil.CLI, netname, ns string) {
	sriovNetList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("SriovNetwork", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(sriovNetList, netname) {
		_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("SriovNetwork", netname, "-n", ns).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// Wait for Pod ready
func (pod *sriovPod) waitForPodReady(oc *exutil.CLI) {
	res := false
	err := wait.Poll(5*time.Second, 15*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", pod.name, "-n", pod.namespace, "-o=jsonpath={.status.phase}").Output()
		e2e.Logf("the status of pod is %v", status)
		if strings.Contains(status, "NotFound") {
			e2e.Logf("the pod was created fail.")
			res = false
			return true, nil
		}
		if err != nil {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "Running") {
			e2e.Logf("the pod is Ready.")
			res = true
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sriov pod %v is not ready", pod.name))
	o.Expect(res).To(o.Equal(true))
}

// Wait for sriov network policy ready
func waitForSriovPolicyReady(oc *exutil.CLI, ns string) bool {
	res := false
	err := wait.Poll(20*time.Second, 20*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sriovnetworknodestates", "-n", ns, "-o=jsonpath={.items[*].status.syncStatus}").Output()
		e2e.Logf("the status of sriov policy is %v", status)
		if err != nil {
			e2e.Logf("failed to get sriov policy status: %v, retrying...", err)
			return false, nil
		}
		nodesStatus := strings.TrimSpace(status)
		statusList := strings.Split(nodesStatus, " ")
		for _, nodeStat := range statusList {
			if nodeStat != "Succeeded" {
				return false, nil
			}
		}
		res = true
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "sriovnetworknodestates is not ready")
	return res
}

//check interface on pod
func (pod *sriovPod) getSriovIntfonPod(oc *exutil.CLI) string {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("exec").Args(pod.name, "-n", pod.namespace, "-i", "--", "ip", "address").Output()
	if err != nil {
		e2e.Logf("Execute ip address command failed with  err:%v .", err)
	}
	e2e.Logf("Get ip address info as:%v", msg)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).NotTo(o.BeEmpty())
	return msg
}

//create pod via HTTP request
func (pod *sriovPod) sendHTTPRequest(oc *exutil.CLI, user, cmd string) {
	//generate token for service acount
	testToken, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", user, "-n", pod.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(testToken).NotTo(o.BeEmpty())

	configFile := pod.processPodTemplate(oc)

	curlCmd := cmd + " -k " + " -H " + fmt.Sprintf("\"Authorization: Bearer %v\"", testToken) + " -d " + "@" + configFile

	e2e.Logf("Send curl request to create new pod: %s\n", curlCmd)

	res, err := exec.Command("bash", "-c", curlCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(res).NotTo(o.BeEmpty())

}
func (sriovPolicy *sriovNetworkNodePolicy) createPolicy(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", sriovPolicy.template, "-p", "NAMESPACE="+sriovPolicy.namespace, "DEVICEID="+sriovPolicy.deviceID, "SRIOVNETPOLICY="+sriovPolicy.policyName, "DEVICETYPE="+sriovPolicy.deviceType, "PFNAME="+sriovPolicy.pfName, "VENDOR="+sriovPolicy.vondor, "NUMVFS="+strconv.Itoa(sriovPolicy.numVfs), "RESOURCENAME="+sriovPolicy.resourceName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create sriovnetworknodePolicy %v", sriovPolicy.policyName))
}

func (sriovNetwork *sriovNetwork) createSriovNetwork(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", sriovNetwork.template, "-p", "NAMESPACE="+sriovNetwork.namespace, "SRIOVNETNAME="+sriovNetwork.name, "TARGETNS="+sriovNetwork.networkNamespace, "SRIOVNETPOLICY="+sriovNetwork.resourceName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create sriovnetwork %v", sriovNetwork.name))
}

func (sriovTestPod *sriovTestPod) createSriovTestPod(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", sriovTestPod.template, "-p", "PODNAME="+sriovTestPod.name, "SRIOVNETNAME="+sriovTestPod.networkName, "NAMESPACE="+sriovTestPod.namespace)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create test pod %v", sriovTestPod.name))
}

//get the pciAddress pod is used
func getPciAddress(namespace string, podName string) string {
	pciAddressEnv, err := e2e.RunHostCmd(namespace, podName, "env | grep PCIDEVICE_OPENSHIFT_IO")
	e2e.Logf("Get the pci address env is: %s", pciAddressEnv)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pciAddressEnv).NotTo(o.BeEmpty())
	pciAddress := strings.Split(pciAddressEnv, "=")
	e2e.Logf("Get the pciAddress is: %s", pciAddress[1])
	return strings.TrimSuffix(pciAddress[1], "\n")
}

//Get the sriov worker which the policy is used
func getSriovNode(oc *exutil.CLI, namespace string, label string) string {
	sriovNodeName := ""
	nodeNamesAll, err := oc.AsAdmin().Run("get").Args("-n", namespace, "node", "-l", label, "-ojsonpath={.items..metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNames := strings.Split(nodeNamesAll, " ")
	for _, nodeName := range nodeNames {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sriovnetworknodestates", nodeName, "-n", namespace, "-ojsonpath={.spec.interfaces}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if output != "" {
			sriovNodeName = nodeName
			break
		}
	}
	e2e.Logf("The sriov node is  %v ", sriovNodeName)
	o.Expect(sriovNodeName).NotTo(o.BeEmpty())
	return sriovNodeName
}

//checkDeviceIDExist will check the worker node contain the network card according to deviceID
func checkDeviceIDExist(oc *exutil.CLI, namespace string, deviceID string) bool {
	allDeviceID, err := oc.AsAdmin().Run("get").Args("sriovnetworknodestates", "-n", namespace, "-ojsonpath={.items[*].status.interfaces[*].deviceID}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("tested deviceID is %v and all supported deviceID on node are %v ", deviceID, allDeviceID)
	return strings.Contains(allDeviceID, deviceID)
}

// Wait for sriov network policy ready
func (rs *sriovNetResource) chkSriovPolicy(oc *exutil.CLI) bool {
	sriovPolicyList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("SriovNetworkNodePolicy", "-n", rs.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if !strings.Contains(sriovPolicyList, rs.name) {
		return false
	}
	return true
}
