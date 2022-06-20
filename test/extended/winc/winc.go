package winc

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-windows] Windows_Containers NonUnifyCI", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLIWithoutNamespace("default")
		iaasPlatform string
		privateKey   = "../internal/config/keys/openshift-qe.pem"
		publicKey    = "../internal/config/keys/openshift-qe.pub"
		svcs         = map[int]string{
			0: "windows_exporter",
			1: "kubelet",
			2: "hybrid-overlay-node",
			3: "kube-proxy",
		}
		folders = map[int]string{
			1: "c:\\k",
			2: "c:\\temp",
			3: "c:\\var\\log",
		}
	)

	g.BeforeEach(func() {
		output, _ := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		iaasPlatform = strings.ToLower(output)
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-Critical-33612-Windows node basic check", func() {
		g.By("Check Windows worker nodes run the same kubelet version as other Linux worker nodes")
		linuxKubeletVersion, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=linux", "-o=jsonpath={.items[0].status.nodeInfo.kubeletVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windowsKubeletVersion, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].status.nodeInfo.kubeletVersion}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if windowsKubeletVersion[0:5] != linuxKubeletVersion[0:5] {
			e2e.Failf("Failed to check Windows %s and Linux %s kubelet version should be the same", windowsKubeletVersion, linuxKubeletVersion)
		}

		g.By("Check worker label is applied to Windows nodes")
		msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "--no-headers", "-l=kubernetes.io/os=windows").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, node := range strings.Split(msg, "\n") {
			if !strings.Contains(node, "worker") {
				e2e.Failf("Failed to check worker label is applied to Windows node %s", node)
			}
		}

		g.By("Check version annotation is applied to Windows nodes")
		// Note: Case 33536 also is covered
		windowsHostName := getWindowsHostNames(oc)
		for _, host := range windowsHostName {
			retcode, err := checkVersionAnnotationReady(oc, host)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !retcode {
				e2e.Failf("Failed to check version annotation is applied to Windows node %s", host)
			}
		}

		g.By("Check dockerfile prepare required binaries in operator image")
		checkFolders := []struct {
			folder   string
			expected string
		}{
			{
				folder:   "/payload",
				expected: "azure-cloud-node-manager.exe cni containerd hybrid-overlay-node.exe kube-node powershell windows_exporter.exe wmcb.exe",
			},
			{
				folder:   "/payload/containerd",
				expected: "containerd-shim-runhcs-v1.exe containerd.exe",
			},
			{
				folder:   "/payload/cni",
				expected: "cni-conf-template.json host-local.exe win-bridge.exe win-overlay.exe",
			},
			{
				folder:   "/payload/kube-node",
				expected: "kube-proxy.exe kubelet.exe",
			},
			{
				folder:   "/payload/powershell",
				expected: "hns.psm1 wget-ignore-cert.ps1",
			},
		}
		for _, checkFolder := range checkFolders {
			g.By("Check required files in" + checkFolder.folder)
			command := []string{"exec", "-n", "openshift-windows-machine-config-operator", "deployment.apps/windows-machine-config-operator", "--", "ls", checkFolder.folder}
			msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			actual := strings.ReplaceAll(msg, "\n", " ")
			if actual != checkFolder.expected {
				e2e.Failf("Failed to check required files in %s, expected: %s actual: %s", checkFolder.folder, checkFolder.expected, actual)
			}
		}

		bastionHost := getSSHBastionHost(oc, iaasPlatform)
		winInternalIP := getWindowsInternalIPs(oc)
		for _, winhost := range winInternalIP {
			for _, svc := range svcs {
				g.By(fmt.Sprintf("Check %v service is running in worker %v", svc, winhost))
				msg, _ = runPSCommand(bastionHost, winhost, fmt.Sprintf("Get-Service %v", svc), privateKey, iaasPlatform)
				if !strings.Contains(msg, "Running") {
					e2e.Failf("Failed to check %v service is running in %v: %s", svc, winhost, msg)
				}
			}
		}
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-Critical-32615-Generate userData secret [Serial]", func() {
		g.By("Check secret windows-user-data generated and contain correct public key")
		output, err := exec.Command("bash", "-c", "cat "+publicKey+"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		publicKeyContent := strings.Split(string(output), " ")[1]
		msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-n", "openshift-machine-api", "-o=jsonpath={.data.userData}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
		if !strings.Contains(string(decodedUserData), publicKeyContent) {
			e2e.Failf("Failed to check public key in windows-user-data secret %s", string(decodedUserData))
		}
		g.By("Check delete secret windows-user-data")
		// May fail other cases in parallel, so run it in serial
		_, err = oc.WithoutNamespace().Run("delete").Args("secret", "windows-user-data", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pollErr := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			msg, _ := oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-machine-api").Output()
			if !strings.Contains(msg, "windows-user-data") {
				e2e.Logf("Secret windows-user-data does not exist yet and wait up to 1 minute ...")
				return false, nil
			}
			e2e.Logf("Secret windows-user-data exist now")
			msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-o=jsonpath={.data.userData}", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
			if !strings.Contains(string(decodedUserData), publicKeyContent) {
				e2e.Failf("Failed to check public key in windows-user-data secret %s", string(decodedUserData))
			}
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Secret windows-user-data does not exist after waiting up to 1 minutes ...")
		}
		g.By("Check update secret windows-user-data")
		// May fail other cases in parallel, so run it in serial
		// Update userData to "aW52YWxpZAo=" which is base64 encoded "invalid"
		msg, err = oc.WithoutNamespace().Run("patch").Args("secret", "windows-user-data", "-p", `{"data":{"userData":"aW52YWxpZAo="}}`, "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pollErr = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			msg, err := oc.WithoutNamespace().Run("get").Args("secret", "windows-user-data", "-o=jsonpath={.data.userData}", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			decodedUserData, _ := base64.StdEncoding.DecodeString(msg)
			if !strings.Contains(string(decodedUserData), publicKeyContent) {
				e2e.Logf("Secret windows-user-data is not updated yet and wait up to 1 minute ...")
				return false, nil
			}
			e2e.Logf("Secret windows-user-data is updated")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Secret windows-user-data is not updated after waiting up to 1 minutes ...")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Low-32554-wmco run in a pod with HostNetwork", func() {
		winInternalIP := getWindowsInternalIPs(oc)[0]
		curlDest := winInternalIP + ":22"
		command := []string{"exec", "-n", "openshift-windows-machine-config-operator", "deployment.apps/windows-machine-config-operator", "--", "curl", curlDest}
		msg, _ := oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "SSH-2.0-OpenSSH") {
			e2e.Failf("Failed to check WMCO run in a pod with HostNetwork: %s", msg)
		}
	})

	// author: sgao@redhat.com refactored:v1
	g.It("Smokerun-Author:sgao-Critical-28632-Windows and Linux east west network during a long time", func() {
		// Note: Flexy alredy created workload in winc-test, here we check it still works after a long time
		namespace := "winc-test"
		g.By("Check communication: Windows pod <--> Linux pod")
		winPodNames, err := getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		windPodIPs, err := getWorkloadsIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodNames, err := getWorkloadsNames(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodIPs, err := getWorkloadsIP(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		command := []string{"exec", "-n", namespace, winPodNames[0], "--", "curl", linuxPodIPs[0] + ":8080"}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}
		command = []string{"exec", "-n", namespace, linuxPodNames[0], "--", "curl", windPodIPs[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}
	})

	// author: sgao@redhat.com refactored:v1
	g.It("Smokerun-Author:sgao-Critical-32273-Configure kube proxy and external networking check", func() {
		if iaasPlatform == "vsphere" {
			g.Skip("vSphere does not support Load balancer, skipping")
		}
		namespace := "winc-32273"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_web_server.yaml", getConfigMapData(oc, "windows_container_image"))
		externalIP, err := getExternalIP(iaasPlatform, oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		// Load balancer takes about 3 minutes to work, set timeout as 5 minutes
		pollErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
			if !strings.Contains(string(msg), "Windows Container Web Server") {
				e2e.Logf("Load balancer is not ready yet and waiting up to 5 minutes ...")
				return false, nil
			}
			e2e.Logf("Load balancer is ready")
			return true, nil
		})
		if pollErr != nil {
			e2e.Failf("Load balancer is not ready after waiting up to 5 minutes ...")
		}
	})

	// author rrasouli@redhat.com
	g.It("Smokerun-Longduration-Author:rrasouli-NonPreRelease-High-37096-Schedule Windows workloads with cluster running multiple Windows OS variants [Slow][Disruptive]", func() {
		// TODO remove skip line when more OS variants are supported
		g.Skip("Test is not in use, no multiple OS variants supported")
		// we assume 2 Windows Nodes created with the default server 2019 image, here we create new server
		namespace := "winc-37096"
		winVersion := "20H2"
		machinesetName := "win20h2"
		machinesetMultiOSFileName := "aws_windows_machineset.yaml"
		if iaasPlatform == "azure" {
			machinesetMultiOSFileName = "azure_windows_machineset.yaml"
		}
		machinesetFileName, err := getMachinesetFileName(oc, iaasPlatform, winVersion, machinesetName, machinesetMultiOSFileName)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.WithoutNamespace().Run("delete").Args(exutil.MapiMachineset, machinesetName, "-n", "openshift-machine-api").Output()
		defer os.Remove(machinesetFileName)
		createMachineset(oc, machinesetFileName)
		waitForMachinesetReady(oc, machinesetName, 10, 1)
		// Here we fetch machine IP from machineset
		machineIP := fetchAddress(oc, "IP", machinesetName)
		nodeName, err := getNodeNameFromIP(oc, machineIP[0], iaasPlatform)
		o.Expect(err).NotTo(o.HaveOccurred())
		// here we update the runtime class file with the Kernel ID of multiple OS
		defer oc.WithoutNamespace().Run("delete").Args("runtimeclass", "multiple-windows-os")
		createRuntimeClass(oc, "runtime-class.yaml", nodeName)
		// here we provision 1 webservers with a runtime class ID, up to 20 minutes due to pull image on AWS
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_webserver_runtime_class.yaml", getConfigMapData(oc, "windows_container_ami_20H2"))
		e2e.Logf("-------- Windows workload scaled on node IP %v -------------", machineIP[0])
		e2e.Logf("-------- Scaling up workloads to 5 -------------")
		scaleDeployment(oc, "windows", 5, namespace)
		// we get a list of all hostIPs all should be on the same host
		pods, err := getWorkloadsHostIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		// we check that all workloads hostIP are similar for all pods
		if !checkPodsHaveSimilarHostIP(oc, pods, machineIP[0]) {
			e2e.Failf("Windows workloads did not bootstrap on the same host...")
		} else {
			e2e.Logf("Windows workloads succesfully bootstrap on the same host %v", nodeName)
		}
	})

	// author rrasouli@redhat.com
	g.It("Author:rrasouli-NonPreRelease-Longduration-Critical-42496-byoh-Configure Windows instance with DNS [Slow] [Disruptive]", func() {
		bastionHost := getSSHBastionHost(oc, iaasPlatform)
		machinesetName := "byoh"
		if iaasPlatform == "aws" {
			infrastructureID, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
			zone, err := oc.WithoutNamespace().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-o=jsonpath={.items[0].metadata.labels.machine\\.openshift\\.io\\/zone}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			machinesetName = infrastructureID + "-" + machinesetName + "-worker-" + zone
		}
		address := setBYOH(oc, iaasPlatform, "InternalDNS", machinesetName)
		defer oc.WithoutNamespace().Run("delete").Args(exutil.MapiMachineset, machinesetName, "-n", "openshift-machine-api").Output()
		defer oc.WithoutNamespace().Run("delete").Args("configmap", "windows-instances", "-n", "openshift-windows-machine-config-operator").Output()
		// removing the config map
		g.By("Delete the BYOH congigmap for node deconfiguration")
		oc.WithoutNamespace().Run("delete").Args("configmap", "windows-instances", "-n", "openshift-windows-machine-config-operator").Output()
		// adding 12 minutes sleep here
		e2e.Logf("Temporarly using sleep of 12 minutes")
		// TODO find a better way not to use sleep but waitUntilStatusChanged function, currently the function isn't working properly and always return values
		// which are not equivelant with the WMCO log
		// waitUntilStatusChanged(oc, "instance has been deconfigured")
		time.Sleep(12 * time.Minute)
		// check services are not running
		g.By("Check services are not running after deleting the Windows Node")
		runningServices, err := getWinSVCs(bastionHost, address, privateKey, iaasPlatform)
		o.Expect(err).NotTo(o.HaveOccurred())
		svcBool, svc := checkRunningServicesOnWindowsNode(*&svcs, runningServices)
		if svcBool {
			e2e.Failf("Service %v still running on Windows node after deconfiguration", svc)
		}
		g.By("Check folder do not exist after deleting the Windows Node")
		for _, folder := range *&folders {
			if checkFoldersDoNotExist(bastionHost, address, fmt.Sprintf("%v", folder), privateKey, iaasPlatform) {
				e2e.Failf("Folders still exists on a deleted node %v", fmt.Sprintf("%v", folder))
			}
		}
		// TODO check network removal test

	})

	// author rrasouli@redhat.com
	g.It("Author:rrasouli-NonPreRelease-Longduration-Critical-42516-byoh-Configure Windows instance with IP [Slow][Disruptive]", func() {
		namespace := "winc-42516"
		machinesetName := "byoh"
		if iaasPlatform == "aws" {
			infrastructureID, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
			zone, err := oc.WithoutNamespace().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-o=jsonpath={.items[0].metadata.labels.machine\\.openshift\\.io\\/zone}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			machinesetName = infrastructureID + "-" + machinesetName + "-worker-" + zone
		}
		defer oc.WithoutNamespace().Run("delete").Args(exutil.MapiMachineset, machinesetName, "-n", "openshift-machine-api").Output()
		defer oc.WithoutNamespace().Run("delete").Args("configmap", "windows-instances", "-n", "openshift-windows-machine-config-operator").Output()
		setBYOH(oc, iaasPlatform, "InternalIP", machinesetName)
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_web_server_byoh.yaml", getConfigMapData(oc, "windows_container_image"))
		scaleDeployment(oc, "windows", 5, namespace)
		msg, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(msg)
	})

	// author rrasouli@redhat.com
	g.It("Smokerun-Author:rrasouli-NonPreRelease-High-39451-Access Windows workload through clusterIP [Slow][Disruptive]", func() {
		namespace := "winc-39451"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_web_server.yaml", getConfigMapData(oc, "windows_container_image"))
		createLinuxWorkload(oc, namespace)
		g.By("Check access through clusterIP from Linux and Windows pods")
		windowsClusterIP, err := getServiceClusterIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxClusterIP, err := getServiceClusterIP(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		winPodArray, err := getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodArray, err := getWorkloadsNames(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("windows cluster IP: " + windowsClusterIP)
		e2e.Logf("Linux cluster IP: " + linuxClusterIP)

		//we query the Linux ClusterIP by a windows pod
		command := []string{"exec", "-n", namespace, winPodArray[0], "--", "curl", linuxClusterIP + ":8080"}

		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
		e2e.Logf("***** Now testing Windows node from a linux host : ****")
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a linux pod")
		}

		g.By("Scale up the Deployment Windows pod continue to be available to curl Linux web server")
		e2e.Logf("Scalling up the Deployment to 2")
		scaleDeployment(oc, "windows", 2, namespace)

		o.Expect(err).NotTo(o.HaveOccurred())
		winPodArray, err = getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a Linux pod")
		}

		command = []string{"exec", "-n", namespace, winPodArray[1], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}

		g.By("Scale up the MachineSet")
		e2e.Logf("Scalling up the Windows node to 3")
		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 2)
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
		// Testing the Windows server is reachable via Linux pod
		command = []string{"exec", "-n", namespace, linuxPodArray[0], "--", "curl", windowsClusterIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows ClusterIP from a Linux pod")
		}
		// Testing the Linux server is reachable with the second windows pod created
		command = []string{"exec", "-n", namespace, winPodArray[1], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
		// Testing the Linux server is reachable with the first Windows pod created.
		command = []string{"exec", "-n", namespace, winPodArray[0], "--", "curl", linuxClusterIP + ":8080"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux ClusterIP from a windows pod")
		}
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-Critical-31276-Configure CNI and internal networking check", func() {
		namespace := "winc-31276"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_web_server.yaml", getConfigMapData(oc, "windows_container_image"))
		createLinuxWorkload(oc, namespace)
		// we scale the deployment to 5 windows pods
		scaleDeployment(oc, "windows", 5, namespace)
		winPodNameArray, err := getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodNameArray, err := getWorkloadsNames(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		winPodIPArray, err := getWorkloadsIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodIPArray, err := getWorkloadsIP(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		hostIPArray, err := getWorkloadsHostIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check communication: Windows pod <--> Linux pod")
		winPodNameArray, err = getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodNameArray, err = getWorkloadsNames(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		winPodIPArray, err = getWorkloadsIP(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		linuxPodIPArray, err = getWorkloadsIP(oc, "linux", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		command := []string{"exec", "-n", namespace, linuxPodNameArray[0], "--", "curl", winPodIPArray[0]}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Linux pod")
		}
		linuxSVC := linuxPodIPArray[0] + ":8080"
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", linuxSVC}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Linux Container Web Server") {
			e2e.Failf("Failed to curl Linux web server from Windows pod")
		}

		g.By("Check communication: Windows pod <--> Windows pod in the same node")
		if hostIPArray[0] != hostIPArray[1] {
			e2e.Failf("Failed to get Windows pod in the same node")
		}
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", winPodIPArray[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", winPodIPArray[1]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod in the same node")
		}

		g.By("Check communication: Windows pod <--> Windows pod across different Windows nodes")
		lastHostIP := hostIPArray[len(hostIPArray)-1]
		if hostIPArray[0] == lastHostIP {
			e2e.Failf("Failed to get Windows pod across different Windows nodes")
		}
		lastIP := winPodIPArray[len(winPodIPArray)-1]
		command = []string{"exec", "-n", namespace, winPodNameArray[0], "--", "curl", lastIP}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod across different Windows nodes")
		}
		lastPodName := winPodNameArray[len(winPodNameArray)-1]
		command = []string{"exec", "-n", namespace, lastPodName, "--", "curl", winPodIPArray[0]}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "Windows Container Web Server") {
			e2e.Failf("Failed to curl Windows web server from Windows pod across different Windows nodes")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-Medium-33768-NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes", func() {
		g.By("Check NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes")
		// Retrieve the Prometheus' pod id
		prometheusPod, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", "openshift-monitoring", "-l=app.kubernetes.io/name=prometheus", "-o", "jsonpath='{.items[0].metadata.name}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Run locally, in the Prometheus container the API query to /api/v1/rules
		// saving us from having to perform port-forwarding, which does not work
		// with intermediate bastion hosts.
		getAlertCMD, err := oc.WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", strings.Trim(prometheusPod, `'`), "--", "curl", "localhost:9090/api/v1/rules").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Search for required string in the rules output
		if !strings.Contains(string(getAlertCMD), "kube_node_labels{label_kubernetes_io_os=\\\"windows\\\"}") {
			e2e.Failf("Failed to check NodeWithoutOVNKubeNodePodRunning alert ignore Windows nodes")
		}
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-Critical-33779-Retrieving Windows node logs", func() {
		g.By("Check a cluster-admin can retrieve kubelet logs")
		msg, err := oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=kubelet/kubelet.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		windowsHostNames := getWindowsHostNames(oc)
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve kubelet log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName+" Log file created at:") {
				e2e.Failf("Failed to retrieve kubelet log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve kube-proxy logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=kube-proxy/kube-proxy.exe.WARNING").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve kube-proxy log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName+" Log file created at:") {
				e2e.Failf("Failed to retrieve kube-proxy log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve hybrid-overlay logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=hybrid-overlay/hybrid-overlay.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, winHostName := range windowsHostNames {
			e2e.Logf("Retrieve hybrid-overlay log on: " + winHostName)
			if !strings.Contains(string(msg), winHostName) {
				e2e.Failf("Failed to retrieve hybrid-overlay log on: " + winHostName)
			}
		}

		g.By("Check a cluster-admin can retrieve container runtime logs")
		msg, err = oc.WithoutNamespace().Run("adm").Args("node-logs", "-l=kubernetes.io/os=windows", "--path=journal", "-u=docker").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Retrieve container runtime logs")
		if !strings.Contains(string(msg), "Starting up") {
			e2e.Failf("Failed to retrieve container runtime logs")
		}
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-NonPreRelease-Critical-33783-Enable must gather on Windows node [Slow][Disruptive]", func() {
		g.By("Check must-gather on Windows node")
		// Note: Marked as [Disruptive] in case of /tmp folder full
		msg, err := oc.WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir=/tmp/must-gather-33783").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer exec.Command("bash", "-c", "rm -rf /tmp/must-gather-33783").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mustGather := string(msg)
		checkMessage := []string{
			"host_service_logs/windows/",
			"host_service_logs/windows/log_files/",
			"host_service_logs/windows/log_files/hybrid-overlay/",
			"host_service_logs/windows/log_files/hybrid-overlay/hybrid-overlay.log",
			"host_service_logs/windows/log_files/kube-proxy/",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.ERROR",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.INFO",
			"host_service_logs/windows/log_files/kube-proxy/kube-proxy.exe.WARNING",
			"host_service_logs/windows/log_files/kubelet/",
			"host_service_logs/windows/log_files/kubelet/kubelet.log",
			"host_service_logs/windows/log_winevent/",
			"host_service_logs/windows/log_winevent/docker_winevent.log",
		}
		for _, v := range checkMessage {
			if !strings.Contains(mustGather, v) {
				e2e.Failf("Failed to check must-gather on Windows node: " + v)
			}
		}
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-NonPreRelease-High-33794-Watch cloud private key secret [Slow][Disruptive]", func() {
		g.By("Check watch cloud-private-key secret")
		oc.WithoutNamespace().Run("delete").Args("secret", "cloud-private-key", "-n", "openshift-windows-machine-config-operator").Output()
		defer oc.WithoutNamespace().Run("create").Args("secret", "generic", "cloud-private-key", "--from-file=private-key.pem="+privateKey, "-n", "openshift-windows-machine-config-operator").Output()
		oc.WithoutNamespace().Run("delete").Args("secret", "windows-user-data", "-n", "openshift-machine-api").Output()

		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 2)

		g.By("Check Windows machine should be in Provisioning phase and not reconciled")
		pollErr := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
			msg, _ := oc.WithoutNamespace().Run("get").Args("events", "-n", "openshift-machine-api").Output()
			if strings.Contains(msg, "Secret \"windows-user-data\" not found") {
				return true, nil
			}
			return false, nil
		})
		if pollErr != nil {
			e2e.Failf("Failed to check Windows machine should be in Provisioning phase and not reconciled after waiting up to 5 minutes ...")
		}

		oc.WithoutNamespace().Run("create").Args("secret", "generic", "cloud-private-key", "--from-file=private-key.pem="+privateKey, "-n", "openshift-windows-machine-config-operator").Output()
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
	})

	// author: sgao@redhat.com
	g.It("Smokerun-Author:sgao-NonPreRelease-Medium-37472-Idempotent check of service running in Windows node [Slow][Disruptive]", func() {
		namespace := "winc-37472"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		createWindowsWorkload(oc, namespace, "windows_web_server.yaml", getConfigMapData(oc, "windows_container_image"))
		windowsHostName := getWindowsHostNames(oc)[0]
		oc.WithoutNamespace().Run("annotate").Args("node", windowsHostName, "windowsmachineconfig.openshift.io/version-").Output()

		g.By("Check after reconciled Windows node should be Ready")
		waitVersionAnnotationReady(oc, windowsHostName, 60*time.Second, 1200*time.Second)
		g.By("Check LB service works well")
		if iaasPlatform != "vsphere" {
			externalIP, err := getExternalIP(iaasPlatform, oc, "windows", namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Load balancer takes about 3 minutes to work, set timeout as 5 minutes
			pollErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
				msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
				if !strings.Contains(string(msg), "Windows Container Web Server") {
					e2e.Logf("Load balancer is not ready yet and waiting up to 5 minutes ...")
					return false, nil
				}
				e2e.Logf("Load balancer is ready")
				return true, nil
			})
			if pollErr != nil {
				e2e.Failf("Load balancer is not ready after waiting up to 5 minutes ...")
			}
		} else {
			e2e.Logf("Skipped step Check LB service works, not supported on vSphere")
		}
	})

	// author: sgao@redhat.com
	g.It("Author:sgao-NonPreRelease-Medium-39030-Re queue on Windows machine's edge cases [Slow][Disruptive]", func() {
		g.By("Scale down WMCO")
		namespace := "openshift-windows-machine-config-operator"
		scaleDownWMCO(oc)
		defer scaleDeployment(oc, "wmco", 1, namespace)

		g.By("Scale up the MachineSet")
		windowsMachineSetName := getWindowsMachineSetName(oc)
		scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 3)
		defer scaleWindowsMachineSet(oc, windowsMachineSetName, 10, 2)

		g.By("Scale up WMCO")
		scaleDeployment(oc, "wmco", 1, namespace)

		g.By("Check Windows machines created before WMCO starts are successfully reconciling and Windows nodes added")
		waitWindowsNodesReady(oc, 3, 60*time.Second, 1200*time.Second)
	})

	// author: rrasouli@redhat.com
	g.It("Author:rrasouli-Medium-37362-[wmco] wmco using correct golang version", func() {
		g.By("Fetch the correct golang version")
		// get the golang version
		getCMD := "oc version -ojson | jq '.serverVersion.goVersion'"
		goVersion, _ := exec.Command("bash", "-c", getCMD).Output()
		s := string(goVersion)
		tVersion := truncatedVersion(s)
		g.By("Compare fetched version with WMCO log version")
		msg, err := oc.WithoutNamespace().Run("logs").Args("deployment.apps/windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, tVersion) {
			e2e.Failf("Unmatching golang version")
		}

	})
	// author: rrasouli@redhat.com
	g.It("Smokerun-Author:rrasouli-High-38186-[wmco] Windows LB service [Slow]", func() {
		if iaasPlatform == "vsphere" {
			g.Skip("vSphere does not support Load balancer, skipping")
		}
		namespace := "winc-test"
		// we determine range of 20
		attempts := 20
		// fetching here the external IP
		externalIP, err := getExternalIP(iaasPlatform, oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("External IP is: " + externalIP)
		g.By("Test LB works well several times and should not get Connection timed out error")
		if checkLBConnectivity(attempts, externalIP) {
			e2e.Logf("Successfully tested loadbalancer on the same node")
		} else {
			e2e.Failf("Failed testing loadbalancer on same node scheduling")
		}

		g.By("2 Windows node + N Windows pods, N >= 2 and Windows pods should be landed on different nodes, we scale to 5 Windows workloads")
		// scaleDeployment(oc, "windows", 6, namespace)
		if checkLBConnectivity(attempts, externalIP) {
			e2e.Logf("Successfully tested loadbalancer on differnt node scheduling")
		} else {
			e2e.Failf("Failed testing loadbalancer on differnt node scheduling")
		}
	})

	// author: sgao@redhat.com refactored:v1
	g.It("Smokerun-Author:sgao-Critical-25593-Prevent scheduling non Windows workloads on Windows nodes", func() {
		namespace := "winc-25593"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		g.By("Check Windows node have a taint 'os=Windows:NoSchedule'")
		msg, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l=kubernetes.io/os=windows", "-o=jsonpath={.items[0].spec.taints[0].key}={.items[0].spec.taints[0].value}:{.items[0].spec.taints[0].effect}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if msg != "os=Windows:NoSchedule" {
			e2e.Failf("Failed to check Windows node have taint os=Windows:NoSchedule")
		}
		g.By("Check deployment without tolerations would not land on Windows nodes")
		windowsWebServerNoTaint := filepath.Join(exutil.FixturePath("testdata", "winc"), "windows_web_server_no_taint.yaml")
		oc.WithoutNamespace().Run("create").Args("-f", windowsWebServerNoTaint, "-n", namespace).Output()
		poolErr := wait.Poll(20*time.Second, 60*time.Second, func() (bool, error) {
			msg, err = oc.WithoutNamespace().Run("get").Args("pod", "--selector=app=win-webserver-no-taint", "-o=jsonpath={.items[].status.conditions[].message}", "-n", namespace).Output()
			if strings.Contains(msg, "didn't tolerate") {
				return true, nil
			}
			return false, nil
		})
		if poolErr != nil {
			e2e.Failf("Failed to check deployment without tolerations would not land on Windows nodes")
		}
		g.By("Check deployment with tolerations already covered in function createWindowsWorkload()")
		g.By("Check none of core/optional operators/operands would land on Windows nodes")
		for _, winHostName := range getWindowsHostNames(oc) {
			e2e.Logf("Check pods running on Windows node: " + winHostName)
			msg, err = oc.WithoutNamespace().Run("get").Args("pods", "--all-namespaces", "-o=jsonpath={.items[*].metadata.namespace}", "--field-selector", "spec.nodeName="+winHostName, "--no-headers").Output()
			for _, namespace := range strings.Split(msg, " ") {
				e2e.Logf("Found pods running in namespace: " + namespace)
				if namespace != "" && !strings.Contains(namespace, "winc") {
					e2e.Failf("Failed to check none of core/optional operators/operands would land on Windows nodes")
				}
			}
		}
	})

	// author: rrasouli@redhat.com refactored:v1
	g.It("Smokerun-Author:rrasouli-Medium-42204-Create Windows pod with a Projected Volume", func() {
		namespace := "winc-42204"
		defer deleteProject(oc, namespace)
		createProject(oc, namespace)
		username := "admin"
		password := getRandomString(8)

		// we write files with the content of username/password
		ioutil.WriteFile("username-42204.txt", []byte(username), 0644)
		defer os.Remove("username-42204.txt")
		ioutil.WriteFile("password-42204.txt", []byte(password), 0644)
		defer os.Remove("password-42204.txt")

		g.By("Create username and password secrets")
		_, err := oc.WithoutNamespace().Run("create").Args("secret", "generic", "user", "--from-file=username=username-42204.txt", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.WithoutNamespace().Run("create").Args("secret", "generic", "pass", "--from-file=password=password-42204.txt", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create Windows Pod with Projected Volume")
		createWindowsWorkload(oc, namespace, "windows_webserver_projected_volume.yaml", getConfigMapData(oc, "windows_container_image"))
		winpod, err := getWorkloadsNames(oc, "windows", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check in Windows pod, the projected-volume directory contains your projected sources")
		command := []string{"exec", winpod[0], "-n", namespace, "--", "powershell", " ls .\\projected-volume", ";", "ls C:\\var\\run\\secrets\\kubernetes.io\\serviceaccount"}
		msg, err := oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Projected volume output is:\n %v", msg)

		g.By("Check username and password exist on projected volume pod")
		command = []string{"exec", winpod[0], "-n", namespace, "--", "powershell", "cat C:\\projected-volume\\username"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Username output is:\n %v", msg)
		command = []string{"exec", winpod[0], "-n", namespace, "--", "powershell", "cat C:\\projected-volume\\password"}
		msg, err = oc.WithoutNamespace().Run(command...).Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Password output is:\n %v", msg)
	})

	// author: rrasouli@redhat.com refactored:v1
	g.It("Smokerun-Author:rrasouli-Critical-48873-Add description OpenShift managed to Openshift services", func() {
		bastionHost := getSSHBastionHost(oc, iaasPlatform)
		// use config map to fetch the actual Windows version
		machineset := getWindowsMachineSetName(oc)
		address := fetchAddress(oc, "IP", machineset)
		for _, machineIP := range address {
			svcDescription, err := getSVCsDescription(bastionHost, machineIP, privateKey, iaasPlatform)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, svc := range svcs {
				svcDesc := svcDescription[svc]
				e2e.Logf("Service is %v", svcDesc)

				if !strings.Contains(svcDesc, "OpenShift managed") {
					e2e.Failf("Description is missing on service %v", svc)
				}
			}
		}
	})

	g.It("Longduration-Smokerun-Author:rrasouli-NonPreRelease-Critical-39858-Windows servicemonitor and endpoints check [Slow][Serial][Disruptive]", func() {

		g.By("Get Endpoints and service monitor values")
		namespace := "openshift-windows-machine-config-operator"
		// need to fetch service monitor age
		serviceMonitorAge1, err := oc.WithoutNamespace().Run("get").Args("endpoints", "-n", namespace, "-o=jsonpath={.items[].metadata.creationTimestamp}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// here we fetch a list of endpoints
		endpointsIPsBefore := getEndpointsIPs(oc, namespace)
		// restarting the WMCO deployment
		g.By("Restart WMCO pod by deleting")
		wmcoID, err := getWorkloadsNames(oc, "wmco", namespace)
		wmcoStartTime, err := oc.WithoutNamespace().Run("get").Args("endpoints", "-n", namespace, "-o=jsonpath={.status.StartTime}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("WMCO start time before restart", wmcoStartTime)
		oc.WithoutNamespace().Run("delete").Args("pod", wmcoID[0], "-n", namespace).Output()
		// checking that the WMCO has no errors and restarted properly
		poolErr := wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
			return checkWorkloadCreated(oc, "windows-machine-config-operator", namespace, 1), nil
		})
		if poolErr != nil {
			e2e.Failf("Error restarting WMCO up to 3 minutes ...")
		}
		g.By("Test endpoints IPs survives a WMCO restart")
		waitForEndpointsReady(oc, namespace, 5, len(strings.Split(endpointsIPsBefore, " ")))

		endpointsIPsAfter := getEndpointsIPs(oc, namespace)
		endpointsIPsBeforeArray := strings.Split(endpointsIPsBefore, " ")
		sort.Strings(endpointsIPsBeforeArray)
		endpointsIPsAfterArray := strings.Split(endpointsIPsAfter, " ")
		sort.Strings(endpointsIPsAfterArray)
		if !reflect.DeepEqual(endpointsIPsBeforeArray, endpointsIPsAfterArray) {
			e2e.Failf("Endpoints list mismatch after WMCO restart %v, %v", endpointsIPsBeforeArray, endpointsIPsAfterArray)
		}
		g.By("Test service-monitor restarted")
		serviceMonitorAge2, err := oc.WithoutNamespace().Run("get").Args("endpoints", "-n", namespace, "-o=jsonpath={.items[].metadata.creationTimestamp}").Output()
		timeOriginal, err := time.Parse(time.RFC3339, serviceMonitorAge1)
		timeLast, err := time.Parse(time.RFC3339, serviceMonitorAge2)
		if timeOriginal.Unix() >= timeLast.Unix() {
			e2e.Failf("Service monitor %v did not restart, bigger than %v new service monitor age", serviceMonitorAge1, serviceMonitorAge2)
		}
		g.By("Scale down nodes")
		defer scaleWindowsMachineSet(oc, getWindowsMachineSetName(oc), 20, 2)
		scaleWindowsMachineSet(oc, getWindowsMachineSetName(oc), 5, 0)
		g.By("Test endpoints IP are deleted after scalling down")
		waitForEndpointsReady(oc, namespace, 5, 0)
		endpointsIPsLast := getEndpointsIPs(oc, namespace)
		if endpointsIPsLast != "" {
			e2e.Failf("Endpoints %v are still exists after scalling down Windows nodes", endpointsIPsLast)
		}
	})

	g.It("Smokerun-Author:jfrancoa-Critical-50924-Windows instances react to kubelet CA rotation [Disruptive]", func() {

		// Retrieve previous certificate which will get rotated.
		certToExpire, err := oc.WithoutNamespace().Run("get").Args("configmap", "kube-apiserver-to-kubelet-client-ca", "-n", "openshift-kube-apiserver-operator", "-o=jsonpath='{.data.ca\\-bundle\\.crt}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Force the kubelet CA rotation")

		initialCertNotBefore, err := oc.WithoutNamespace().Run("get").Args("secrets", "kube-apiserver-to-kubelet-signer", "-n", "openshift-kube-apiserver-operator", "-o=jsonpath='{.metadata.annotations.auth\\.openshift\\.io\\/certificate-not-before}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		initialCertNotBeforeParsed, err := time.Parse(time.RFC3339, strings.Trim(initialCertNotBefore, `'`))
		o.Expect(err).NotTo(o.HaveOccurred())

		// CA rotation
		err = oc.WithoutNamespace().Run("patch").Args("secret", "-p", `{"metadata": {"annotations": {"auth.openshift.io/certificate-not-after": null}}}`, "kube-apiserver-to-kubelet-signer", "-n", "openshift-kube-apiserver-operator").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Confirm that the rotation has taken place by
		// comparing initial certificate-not-before with the certificate-not-before annotation
		// after forcing the rotation
		rotatedCertNotBefore, err := oc.WithoutNamespace().Run("get").Args("secrets", "kube-apiserver-to-kubelet-signer", "-n", "openshift-kube-apiserver-operator", "-o=jsonpath='{.metadata.annotations.auth\\.openshift\\.io\\/certificate-not-before}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		rotatedCertNotBeforeParsed, err := time.Parse(time.RFC3339, strings.Trim(rotatedCertNotBefore, `'`))
		o.Expect(err).NotTo(o.HaveOccurred())
		if initialCertNotBeforeParsed.Equal(rotatedCertNotBeforeParsed) {
			e2e.Failf("Kubelet CA rotation did not happen. certificate-not-before: %v", rotatedCertNotBefore)
		}

		// Force the expired certificate deletion from kubelet's client CA
		// First we get the current content on kubelet's client CA
		cmOutput, err := oc.WithoutNamespace().Run("get").Args("configmap", "kube-apiserver-to-kubelet-client-ca", "-n", "openshift-kube-apiserver-operator", "-o=jsonpath='{.data.ca\\-bundle\\.crt}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Delete the expired certificate (stored at the beggining of test) by using ReplaceAll
		formattedCertToExpire := strings.Trim(strings.TrimSpace(certToExpire), "'")
		cmWithoutExpired := strings.ReplaceAll(cmOutput, formattedCertToExpire, "")
		formattedcmWithoutExpired := strings.ReplaceAll(strings.Trim(strings.TrimSpace(cmWithoutExpired), "'"), "\n", "\\n")
		// Patch the data.ca-bundle.crt field with the new config map content
		// without the expired certificate
		_, err = oc.WithoutNamespace().Run("patch").Args("configmap", "kube-apiserver-to-kubelet-client-ca", "-n", "openshift-kube-apiserver-operator", "-p", `{"data":{"ca-bundle.crt": "`+formattedcmWithoutExpired+`"}}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verify kubelet client CA is updated in Windows workers")
		bastionHost := getSSHBastionHost(oc, iaasPlatform)
		caBundlePath := folders[1] + "\\kubelet-ca.crt"
		winInternalIP := getWindowsInternalIPs(oc)
		for _, winhost := range winInternalIP {

			g.By(fmt.Sprintf("Verify kubelet client CA content is include in Windows worker %v ", winhost))

			poolErr := wait.Poll(15*time.Second, 10*time.Minute, func() (bool, error) {
				// Get kubelet client CA content from confimap
				kubeletCA, err := oc.WithoutNamespace().Run("get").Args("configmap", "kube-apiserver-to-kubelet-client-ca", "-n", "openshift-kube-apiserver-operator", "-o=jsonpath='{.data.ca\\-bundle\\.crt}'").Output()
				if err != nil {
					e2e.Logf("error getting kubelet client CA from ConfigMap: %v", err)
					return false, nil
				}

				// Fetch CA bundle from Windows worker node
				bundleContent, err := runPSCommand(bastionHost, winhost, fmt.Sprintf("Get-Content -Raw -Path %s", caBundlePath), privateKey, iaasPlatform)
				if err != nil {
					e2e.Logf("failed fetching CA bundle from Windows node %v: %v", winhost, err)
					return false, nil
				}

				kubeletCAFormatted := strings.Trim(strings.TrimSpace(kubeletCA), "'")
				// Check that the kubelet client CA is included in bundleContent variable
				// We need to split bundleContent by \r\n as it contains both Stdout and Stderr
				// and we are just interested on the Stdout
				if strings.Contains(strings.Split(bundleContent, "\r\n")[1], kubeletCAFormatted) {
					return true, nil
				}
				e2e.Logf("kubelet CA not found in Windows worker node bundle %v. Retrying...", winhost)
				return false, nil
			})
			if poolErr != nil {
				e2e.Failf("failed to verify that the kubelet client CA is included in Windows worker %v bundle", winhost)
			}

		}

		g.By("Ensure that none of the Windows Workers got restarted and that we have communication to the Windows workers")
		for ip, up := range getWindowsNodesUptime(oc, privateKey, iaasPlatform) {
			// If the timestamp from the moment when the certificate got rotated
			// is older than the worker's uptime timestamp it means that
			// a restart took place
			if rotatedCertNotBeforeParsed.Before(up) {
				e2e.Failf("windows worker %v got restarted after CA rotation", ip)
			}
		}

	})
})
