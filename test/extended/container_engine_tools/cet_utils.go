package container_engine_tools

import (
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
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

type ctrcfgDescription struct {
	namespace  string
	name	   string
	pidlimit   int
	loglevel   string
	overlay    string
	logsizemax string
	command    string
	configFile string
	template   string
}

type ocp48876PodDescription struct {
	name             string
	namespace        string
	template         string
}
type newappDescription struct {
	appname string
}

type objectTableRefcscope struct {
	kind string
	name string
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

func (ocp48876Pod *ocp48876PodDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ocp48876Pod.template, "-p", "NAME="+ocp48876Pod.name, "NAMESPACE="+ocp48876Pod.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (ocp48876Pod *ocp48876PodDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ocp48876Pod.namespace, "pod", ocp48876Pod.name).Execute()
}

func (podModify *podModifyDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podModify.template, "-p", "NAME="+podModify.name, "NAMESPACE="+podModify.namespace, "MOUNTPATH="+podModify.mountpath, "COMMAND="+podModify.command, "ARGS="+podModify.args, "POLICY="+podModify.restartPolicy, "USER="+podModify.user, "ROLE="+podModify.role, "LEVEL="+podModify.level)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podModify.namespace, "pod", podModify.name).Execute()
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
		if strings.Contains(status, "Running") {
			e2e.Logf("Pod status is : %s", status)
			return true, nil
		}
		return false, nil
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
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ctrcfg.template, "-p", "NAME="+ctrcfg.name,"LOGLEVEL="+ctrcfg.loglevel, "OVERLAY="+ctrcfg.overlay, "LOGSIZEMAX="+ctrcfg.logsizemax)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func cleanupObjectsClusterScope(oc *exutil.CLI, objs ...objectTableRefcscope) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		for _, v := range objs {
			e2e.Logf("\n Start to remove: %v", v)
			status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(v.kind, v.name).Output()
			if strings.Contains(status, "Error") {
				e2e.Logf("Error getting resources... Seems resources objects are already deleted. \n")
				return true, nil
			} else {
				_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, v.name).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
		return true, nil
	})
}

func (ctrcfg *ctrcfgDescription) checkCtrcfgParameters(oc *exutil.CLI) error {
	return wait.Poll(3*time.Minute, 11*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)

			if nodeStatus == "Ready" {
				criostatus, err := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "crio", "config").OutputToFile("crio.conf")
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf(`\nCRI-O PARAMETER ON THE WORKER NODE :` + fmt.Sprintf("%s", v))
				e2e.Logf("\ncrio config file path is  %v", criostatus)

				wait.Poll(3*time.Minute, 10*time.Minute, func() (bool, error) {
					result, err1 := exec.Command("bash", "-c", "cat "+criostatus+" | egrep 'pids_limit|log_level'").Output()
					if err1 != nil {
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

func buildLog(oc *exutil.CLI, buildconfig string) error {
	return wait.Poll(30*time.Second, 5*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("buildconfig.build.openshift.io/"+buildconfig, "-n", oc.Namespace()).Output()
		e2e.Logf("Here is the build log %v\n", status)
		if err != nil {
			e2e.Logf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "error reading blob from source image") {
			e2e.Logf(" This is Error, File Bug. \n")
			return false, nil
		}
		return true, nil
	})
}

func checkPodmanInfo(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)

			if nodeStatus == "Ready" {
				podmaninfo, err := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "podman", "info").OutputToFile("crio.conf")
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf(`\nNODE NAME IS :` + fmt.Sprintf("%s", v))
				e2e.Logf("\npodman info is  %v", podmaninfo)

				wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
					result, err1 := exec.Command("bash", "-c", "cat "+podmaninfo+" | egrep ' arch:|os:'").Output()
					if err1 != nil {
						e2e.Logf("the result of ReadFile:%v", err1)
						return false, nil
					}
					e2e.Logf("\npodman info Parameters are %s", result)
					if strings.Contains(string(result), "arch") && strings.Contains(string(result), "os") {
						e2e.Logf("\nPodman info parameter arch and os configured successfully")
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

func (newapp *newappDescription) createNewApp(oc *exutil.CLI) error {
	return wait.Poll(30*time.Second, 1*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("new-app").Args(newapp.appname, "-n", oc.Namespace()).Output()
		e2e.Logf("Here is the newapp log %v\n", status)
		if err != nil {
			e2e.Logf("the result of ReadFile:%v", err)
			return false, nil
		}
		return true, nil
	})
}

func buildConfigStatus(oc *exutil.CLI) string {
	var buildConfigStatus string
	buildConfigStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("buildconfig", "-o=jsonpath={.items[0].metadata.name}", "-n", oc.Namespace()).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The buildconfig Name is: %v", buildConfigStatus)
	return buildConfigStatus
}

func checkNodeStatus(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)
		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)
			if nodeStatus == "Ready" {
				e2e.Logf("\n NODES ARE READY\n ")
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
			}
		}
		return true, nil
	})
}

func machineconfigStatus(oc *exutil.CLI) error {
	return wait.Poll(30*time.Second, 5*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig", "-o=jsonpath={.items[*].metadata.name}").Output()
		e2e.Logf("Here is the machineconfig %v\n", status)
		if err != nil {
			e2e.Logf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "containerruntime") {
			e2e.Logf(" This is Error, File Bug. \n")
			return false, nil
		}
		return true, nil
	})
}

func checkPodmanCrictlVersion(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)

			if nodeStatus == "Ready" {
				podmanver, err := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "podman", "--version").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				crictlver, err1 := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "crictl", "version").Output()
				o.Expect(err1).NotTo(o.HaveOccurred())
				e2e.Logf(`NODE NAME IS :` + fmt.Sprintf("%s", v))

				if strings.Contains(string(podmanver), "podman version 3.") && strings.Contains(string(crictlver), "RuntimeVersion:  1.2") {
					e2e.Logf("\n Podman and crictl is on latest version %s %s", podmanver, crictlver)
				} else {
					e2e.Logf("\nPodman and crictl version are NOT Updated")
					return false, nil
				}
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
			}
		}
		return true, nil
	})
}

func (ctrcfg *ctrcfgDescription) checkCtrcfgStatus(oc *exutil.CLI) error {
	return wait.Poll(3*time.Second, 1*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ContainerRuntimeConfig", ctrcfg.name , "-o=jsonpath={.status.conditions[0].message}").Output()
		e2e.Logf("The ContainerRuntimeConfig message is %v", status)
                o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(status, "Success") {
			e2e.Logf("ContainerRuntimeConfig whose finalizer > 63 characters is applied Sucessfully \n")
			return true, nil
		}
		return false, nil
	})
}

func getPodName(oc *exutil.CLI, ns string) string {
    podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nPod Name is is %v", podName)
	return podName
}

func getPodIPv4(oc *exutil.CLI, podName string, ns string) string {
    IPv4add, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-o=jsonpath={.status.podIP}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nPod IP address is %v", IPv4add)
	return IPv4add
}

func pingIpaddr(oc *exutil.CLI, ns string, podName string, cmd string) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, podName , "--", "/bin/bash", "-c", cmd ).OutputToFile("pingipaddr.txt")
		o.Expect(err).NotTo(o.HaveOccurred())
		result, err1 := exec.Command("bash", "-c", "cat "+status+" | egrep '64 bytes from 8.8.8.8: icmp_seq'").Output()
			if err1 != nil {
					e2e.Failf("the result of ReadFile:%v", err1)
					return false, nil
			}
			e2e.Logf("\nPing output is %s\n", result)
			if strings.Contains(string(result), "64 bytes from 8.8.8.8: icmp_seq") {
					e2e.Logf("\nPing Successful \n")
					return true, nil
			}
		return false, nil
	})
}
