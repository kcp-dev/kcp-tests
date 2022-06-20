package winc

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	v "github.com/openshift/openshift-tests-private/test/extended/mco"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	winVersion = "2019"
)

func createProject(oc *exutil.CLI, namespace string) {
	_, err := oc.WithoutNamespace().Run("new-project").Args(namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// this function delete a workspace, we intend to do it after each test case run
func deleteProject(oc *exutil.CLI, namespace string) {
	_, err := oc.WithoutNamespace().Run("delete").Args("project", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getConfigMapData(oc *exutil.CLI, dataKey string) string {
	dataValue, err := oc.WithoutNamespace().Run("get").Args("configmap", "winc-test-config", "-o=jsonpath='{.data."+dataKey+"}'", "-n", "winc-test").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return dataValue
}

func waitWindowsNodesReady(oc *exutil.CLI, nodesNumber int, interval time.Duration, timeout time.Duration) {
	pollErr := wait.Poll(interval, timeout, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "--no-headers").Output()
		nodesReady := strings.Count(msg, "Ready")
		if nodesReady != nodesNumber {
			e2e.Logf("Expected %v Windows nodes are not ready yet. Waiting %v seconds more ...", nodesNumber, interval)
			return false, nil
		}
		e2e.Logf("Expected %v Windows nodes are ready", nodesNumber)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Expected %v Windows nodes are not ready after waiting up to %v seconds ...", nodesNumber, timeout)
	}
}

// This function returns the windows build e.g windows-build: '10.0.19041'
func getWindowsBuildID(oc *exutil.CLI, nodeID string) (string, error) {
	build, err := oc.WithoutNamespace().Run("get").Args("node", nodeID, "-o=jsonpath={.metadata.labels.node\\.kubernetes\\.io\\/windows-build}").Output()
	return build, err
}

func checkPodsHaveSimilarHostIP(oc *exutil.CLI, pods []string, nodeIP string) bool {
	for _, pod := range pods {
		e2e.Logf("Pod host IP is %v, of node IP, %v", pod, nodeIP)
		if pod != nodeIP {
			return false
		}
	}
	return true
}

func waitVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string, interval time.Duration, timeout time.Duration) {
	pollErr := wait.Poll(interval, timeout, func() (bool, error) {
		retcode, err := checkVersionAnnotationReady(oc, windowsNodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !retcode {
			e2e.Logf("Version annotation is not applied to Windows node %s yet. Waiting %v more seconds", windowsNodeName, interval)
			return false, nil
		}
		e2e.Logf("Version annotation is applied to Windows node %s", windowsNodeName)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Version annotation is not applied to Windows node %s after waiting up to %v minutes ...", windowsNodeName, timeout)
	}
}

func checkVersionAnnotationReady(oc *exutil.CLI, windowsNodeName string) (bool, error) {
	msg, err := oc.WithoutNamespace().Run("get").Args("nodes", windowsNodeName, "-o=jsonpath='{.metadata.annotations.windowsmachineconfig\\.openshift\\.io\\/version}'").Output()
	if msg == "" {
		return false, err
	}
	return true, err
}

func getWindowsMachineSetName(oc *exutil.CLI) string {
	// fetch the Windows MachineSet from all machinesets list
	myJSON := "-o=jsonpath={.items[?(@.spec.template.metadata.labels.machine\\.openshift\\.io\\/os-id==\"Windows\")].metadata.name}"
	windowsMachineSetName, err := oc.WithoutNamespace().Run("get").Args(exutil.MapiMachineset, "-n", "openshift-machine-api", myJSON).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return windowsMachineSetName
}

func getWindowsHostNames(oc *exutil.CLI) []string {
	winHostNames, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "-o=jsonpath={.items[*].status.addresses[?(@.type==\"Hostname\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(winHostNames, " ")
}

func getWindowsInternalIPs(oc *exutil.CLI) []string {
	winInternalIPs, err := oc.WithoutNamespace().Run("get").Args("nodes", "-l", "kubernetes.io/os=windows", "-o=jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(winInternalIPs, " ")
}

func getSSHBastionHost(oc *exutil.CLI, iaasPlatform string) string {

	if iaasPlatform == "vsphere" {
		return "bastion.vmc.ci.openshift.org"
	}
	msg, err := oc.WithoutNamespace().Run("get").Args("service", "--all-namespaces", "-l=run=ssh-bastion", "-o=go-template='{{ with (index (index .items 0).status.loadBalancer.ingress 0) }}{{ or .hostname .ip }}{{end}}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).NotTo(o.BeEmpty())
	msg = removeOuterQuotes(msg)
	return (msg)
}

// A private function to translate the workload/pod/deployment name
func getWorkloadName(os string) string {
	name := ""
	if os == "windows" {
		name = "win-webserver"
	} else if os == "linux" {
		name = "linux-webserver"
	} else {
		name = "windows-machine-config-operator"
	}
	return name
}

// A private function to determine username by platform
func getAdministratorNameByPlatform(iaasPlatform string) (admin string) {
	if iaasPlatform == "azure" {
		return "capi"
	}
	return "Administrator"
}

func getBastionSSHUser(iaasPlatform string) (user string) {
	if iaasPlatform == "vsphere" {
		return "openshift-qe"
	}
	return "core"
}

func runPSCommand(bastionHost string, windowsHost string, command string, privateKey string, iaasPlatform string) (result string, err error) {
	windowsUser := getAdministratorNameByPlatform(iaasPlatform)
	command = "\"" + command + "\""
	cmd := "chmod 600 " + privateKey + "; ssh -i " + privateKey + " -t -o StrictHostKeyChecking=no -o ProxyCommand=\"ssh -i " + privateKey + " -A -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -W %h:%p " + getBastionSSHUser(iaasPlatform) + "@" + bastionHost + "\" " + windowsUser + "@" + windowsHost + " 'powershell " + command + "'"
	msg, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	return string(msg), err
}

// Returns a map with the uptime for each windows node,
// being the key the IP of the node and the value the
// uptime parsed as a time.Time value.
func getWindowsNodesUptime(oc *exutil.CLI, privateKey string, iaasPlatform string) map[string]time.Time {
	bastionHost := getSSHBastionHost(oc, iaasPlatform)
	layout := "1/2/2006 3:04:05 PM"
	var winUptime map[string]time.Time = make(map[string]time.Time)
	winInternalIP := getWindowsInternalIPs(oc)
	for _, winhost := range winInternalIP {
		uptime, err := runPSCommand(bastionHost, winhost, "Get-CimInstance -ClassName Win32_OperatingSystem | Select LastBootUpTime", privateKey, iaasPlatform)
		o.Expect(err).NotTo(o.HaveOccurred())
		winUptime[winhost], err = time.Parse(layout, strings.TrimSpace(strings.Split(uptime, "\r\n")[4]))
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return winUptime
}

func createLinuxWorkload(oc *exutil.CLI, namespace string) {
	linuxWebServer := filepath.Join(exutil.FixturePath("testdata", "winc"), "linux_web_server.yaml")
	// Wait up to 3 minutes for Linux workload ready
	oc.WithoutNamespace().Run("create").Args("-f", linuxWebServer, "-n", namespace).Output()
	poolErr := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		return checkWorkloadCreated(oc, "linux-webserver", namespace, 1), nil
	})
	if poolErr != nil {
		e2e.Failf("Linux workload is not ready after waiting up to 3 minutes ...")
	}
}

func checkWorkloadCreated(oc *exutil.CLI, deploymentName string, namespace string, replicas int) bool {
	msg, err := oc.WithoutNamespace().Run("get").Args("deployment", deploymentName, "-o=jsonpath={.status.readyReplicas}", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	workloads := strconv.Itoa(replicas)
	if msg != workloads {
		e2e.Logf("Deployment " + deploymentName + " did not scale to " + workloads)
		return false
	}
	e2e.Logf("Deployment " + deploymentName + " created successfuly.")
	return true
}

func createWindowsWorkload(oc *exutil.CLI, namespace string, workloadFile string, containerImage string) {
	windowsWebServer := getFileContent("winc", workloadFile)
	windowsWebServer = strings.ReplaceAll(windowsWebServer, "<windows_container_image>", containerImage)
	tempFileName := namespace + "-windows-workload"
	defer os.Remove(namespace + "-windows-workload")
	ioutil.WriteFile(tempFileName, []byte(windowsWebServer), 0644)
	oc.WithoutNamespace().Run("create").Args("-f", tempFileName, "-n", namespace).Output()
	// Wait up to 30 minutes for Windows workload ready in case of Windows image is not pre-pulled
	poolErr := wait.Poll(30*time.Second, 30*time.Minute, func() (bool, error) {
		return checkWorkloadCreated(oc, "win-webserver", namespace, 1), nil
	})
	if poolErr != nil {
		e2e.Failf("Windows workload is not ready after waiting up to 30 minutes ...")
	}
}

// Get an external IP of loadbalancer service
func getExternalIP(iaasPlatform string, oc *exutil.CLI, os string, namespace string) (extIP string, err error) {
	serviceName := getWorkloadName(os)
	if iaasPlatform == "azure" {
		extIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}", "-n", namespace).Output()
	} else {
		extIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.status.loadBalancer.ingress[0].hostname}", "-n", namespace).Output()
	}
	return extIP, err
}

// we retrieve the ClusterIP from a pod according to it's OS
func getServiceClusterIP(oc *exutil.CLI, os string, namespace string) (clusterIP string, err error) {
	serviceName := getWorkloadName(os)
	clusterIP, err = oc.WithoutNamespace().Run("get").Args("service", serviceName, "-o=jsonpath={.spec.clusterIP}", "-n", namespace).Output()
	return clusterIP, err
}

// Get file content in test/extended/testdata/<basedir>/<name>
func getFileContent(baseDir string, name string) (fileContent string) {
	filePath := filepath.Join(exutil.FixturePath("testdata", baseDir), name)
	fileOpen, err := os.Open(filePath)
	if err != nil {
		e2e.Failf("Failed to open file: %s", filePath)
	}
	fileRead, _ := ioutil.ReadAll(fileOpen)
	if err != nil {
		e2e.Failf("Failed to read file: %s", filePath)
	}
	return string(fileRead)
}

// this function scale the deployment workloads
func scaleDeployment(oc *exutil.CLI, os string, replicas int, namespace string) error {
	deploymentName := getWorkloadName(os)
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "deployment", deploymentName, "-n", namespace).Output()
	poolErr := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		return checkWorkloadCreated(oc, deploymentName, namespace, replicas), nil
	})
	if poolErr != nil {
		e2e.Failf("Workload did not scale after waiting up to 5 minutes ...")
	}
	return err
}

func scaleWindowsMachineSet(oc *exutil.CLI, windowsMachineSetName string, deadTime int, replicas int) {
	err := oc.WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), "machineset", windowsMachineSetName, "-n", "openshift-machine-api").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForMachinesetReady(oc, windowsMachineSetName, deadTime, replicas)
}

// this function returns an array of workloads names by their OS type
func getWorkloadsNames(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	if workloadName == "windows-machine-config-operator" {
		workloadName = "name=" + workloadName
	} else {
		workloadName = "app=" + workloadName
	}
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector", workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].metadata.name}", "-n", namespace).Output()
	pods := strings.Split(workloads, " ")
	return pods, err
}

// this function returns an array of workloads IP's by their OS type
func getWorkloadsIP(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector", "app="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.podIP}", "-n", namespace).Output()
	ips := strings.Split(workloads, " ")
	return ips, err
}

// this function returns an array of workloads host IP's by their OS type
func getWorkloadsHostIP(oc *exutil.CLI, os string, namespace string) ([]string, error) {
	workloadName := getWorkloadName(os)
	workloads, err := oc.WithoutNamespace().Run("get").Args("pod", "--selector", "app="+workloadName, "--sort-by=.status.hostIP", "-o=jsonpath={.items[*].status.hostIP}", "-n", namespace).Output()
	ips := strings.Split(workloads, " ")
	return ips, err
}

func scaleDownWMCO(oc *exutil.CLI) error {
	_, err := oc.WithoutNamespace().Run("scale").Args("--replicas=0", "deployment", "windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
	return err
}

// The output from JSON contains quotes, here we remove them
func removeOuterQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// we truncate the go version to major Go version, e.g. 1.15.13 --> 1.15
func truncatedVersion(s string) string {
	s = removeOuterQuotes(s)
	str := strings.Split(s, ".")
	str = str[:2]
	return strings.Join(str[:], ".")
}
func getMachinesetFileName(oc *exutil.CLI, iaasPlatform, winVersion string, machineSetName string, fileName string) (machinesetFileName string, err error) {
	windowsMachineSet := ""
	infrastructureID := ""
	if iaasPlatform == "aws" {
		windowsMachineSet = getFileContent("winc", fileName)
		infrastructureID, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		// TODO fetch region/zone from configmap
		region, err := oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
		if err != nil {
			e2e.Logf("Using default AWS region: us-east-2")
			region = "us-east-2"
		}
		zone, err := oc.WithoutNamespace().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-o=jsonpath={.items[0].metadata.labels.machine\\.openshift\\.io\\/zone}").Output()
		if err != nil {
			e2e.Logf("Using default AWS zone: us-east-2a")
			zone = "us-east-2a"
		}
		windowsAMI := getConfigMapData(oc, "windows_container_ami")
		if winVersion == "20H2" {
			windowsAMI = getConfigMapData(oc, "windows_container_ami_20H2")
		}
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<name>", machineSetName)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<region>", region)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<zone>", zone)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<windows_image_with_container_runtime_installed>", windowsAMI)
	} else if iaasPlatform == "azure" {
		windowsMachineSet = getFileContent("winc", fileName)
		infrastructureID, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		location, err := oc.WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath=\"{.items[0].metadata.labels.topology\\.kubernetes\\.io\\/region}\"").Output()
		if err != nil {
			e2e.Logf("Using default Azure region: centralus")
			location = "centralus"
		}
		sku := "2019-Datacenter-with-Containers"
		if winVersion == "2004" {
			sku = "datacenter-core-2004-with-containers-smalldisk"
		}
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<location>", location)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<SKU>", sku)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<name>", machineSetName)
	} else if iaasPlatform == "vsphere" {
		windowsMachineSet = getFileContent("winc", "vsphere_byoh_machineset.yaml")
		template := "openshift-qe-winserver-ver-2004"
		if winVersion == "2022" {
			template = "openshift-qe-winserver-2022"
		}
		infrastructureID, err = oc.WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<infrastructureID>", infrastructureID)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<template>", template)
		windowsMachineSet = strings.ReplaceAll(windowsMachineSet, "<name>", machineSetName)
	} else {
		e2e.Failf("IAAS platform: %s is not automated yet", iaasPlatform)
	}
	machinesetFileName = "availWindowsMachineSet" + machineSetName
	ioutil.WriteFile(machinesetFileName, []byte(windowsMachineSet), 0644)
	return machinesetFileName, err
}

func createMachineset(oc *exutil.CLI, file string) error {
	_, err := oc.WithoutNamespace().Run("create").Args("-f", file).Output()
	return err
}

func waitForMachinesetReady(oc *exutil.CLI, machinesetName string, deadTime int, expectedReplicas int) {
	pollErr := wait.Poll(30*time.Second, time.Duration(deadTime)*time.Minute, func() (bool, error) {
		msg, err := oc.WithoutNamespace().Run("get").Args(exutil.MapiMachineset, machinesetName, "-o=jsonpath={.status.readyReplicas}", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		numberOfMachines := 0
		if msg != "" {
			numberOfMachines, err = strconv.Atoi(msg)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		if numberOfMachines == expectedReplicas {
			e2e.Logf("numberOfMachines value is: %v", numberOfMachines)
			return true, nil
		}
		e2e.Logf("Windows machine is not provisioned yet. Waiting 30 seconds more ...")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(pollErr, fmt.Sprintf("Windows machine is not provisioned after waiting up to %v minutes ...", deadTime))

}

func getNodeNameFromIP(oc *exutil.CLI, nodeIP string, iaasPlatform string) (string, error) {
	// Azure and AWS indexes for IP addresses are different
	index := "0"
	if iaasPlatform == "azure" {
		index = "1"
	}
	nodeName, err := oc.WithoutNamespace().Run("get").Args("node", "-o=jsonpath={.items[?(@.status.addresses["+index+"].address==\""+nodeIP+"\")].metadata.name}").Output()
	return nodeName, err
}

func createRuntimeClass(oc *exutil.CLI, runtimeClassFile, node string) error {
	runtimeClass := ""
	runtimeClass = getFileContent("winc", runtimeClassFile)
	buildID, err := getWindowsBuildID(oc, node)
	e2e.Logf("-------- Windows build ID is " + buildID + "-----------")
	runtimeClass = strings.ReplaceAll(runtimeClass, "<kernelID>", buildID)
	defer os.Remove(runtimeClassFile)
	ioutil.WriteFile(runtimeClassFile, []byte(runtimeClass), 0644)
	_, err = oc.WithoutNamespace().Run("create").Args("-f", runtimeClassFile).Output()
	return err
}

func checkLBConnectivity(attempts int, externalIP string) bool {
	retcode := true
	for v := 1; v < attempts; v++ {
		e2e.Logf("Check the Load balancer cluster IP responding: " + externalIP)
		msg, _ := exec.Command("bash", "-c", "curl "+externalIP).Output()
		if !strings.Contains(string(msg), "Windows Container Web Server") {
			e2e.Logf("Windows Load balancer isn't working properly on the %v attempt", v)
			retcode = false
			break
		}
	}
	return retcode
}

func fetchAddress(oc *exutil.CLI, addressType string, machinesetName string) []string {
	machineAddresses := ""
	pollErr := wait.Poll(5*time.Second, 200*time.Second, func() (bool, error) {
		var err error
		machineAddresses, err = oc.WithoutNamespace().Run("get").Args(exutil.MapiMachine, "-ojsonpath={.items[?(@.metadata.labels.machine\\.openshift\\.io\\/cluster-api-machineset==\""+machinesetName+"\")].status.addresses[?(@.type==\""+addressType+"\")].address}", "-n", "openshift-machine-api").Output()
		if err != nil || machineAddresses == "" {
			e2e.Logf("Did not get address, trying next round")
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(pollErr, "Windows machine is not provisioned after waiting up to 200 seconds ...")

	// Filter out any IPv6 address which could have been configured in the machine
	machinesAddressesArray := []string{}
	for _, machineAddress := range strings.Split(string(machineAddresses), " ") {
		if addressType == "InternalDNS" || ip4or6(machineAddress) == "version 4" {
			machinesAddressesArray = append(machinesAddressesArray, machineAddress)
		}
	}
	e2e.Logf("Machine Address is %v", machinesAddressesArray)
	return machinesAddressesArray
}

func ip4or6(s string) string {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return "version 4"
		case ':':
			return "version 6"
		}
	}
	return "unknown"
}

func setConfigmap(oc *exutil.CLI, address string, administrator string, configMapFile string) error {
	configmap := ""
	configmap = getFileContent("winc", configMapFile)
	configmap = strings.ReplaceAll(configmap, "<address>", address)
	configmap = strings.ReplaceAll(configmap, "<username>", administrator)
	defer os.Remove("configMapFile")
	ioutil.WriteFile("configMapFile", []byte(configmap), 0644)
	_, err := oc.WithoutNamespace().Run("create").Args("-f", "configMapFile").Output()
	return err
}

func getWinSVCs(bastionHost string, addr string, privateKey string, iaasPlatform string) (map[string]string, error) {
	cmd := "Get-Service | Select-Object -Property Name,Status | ConvertTo-Csv -NoTypeInformation"
	msg, err := runPSCommand(bastionHost, addr, cmd, privateKey, iaasPlatform)
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		e2e.Failf("error running SSH job")
	}
	svcSplit := strings.SplitAfterN(msg, "\"Name\",\"Status\"\r\n", 2)
	if len(svcSplit) != 2 {
		e2e.Logf("unexpected command output: " + msg)
	}
	svcTrimmed := strings.TrimSpace(svcSplit[1])
	services := make(map[string]string)
	lines := strings.Split(svcTrimmed, "\r\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) != 2 {
			e2e.Logf("expected comma separated values, found: " + line)
		}
		services[strings.Trim(fields[0], "\"")] = strings.Trim(fields[1], "\"")
	}
	return services, nil
}

func getSVCsDescription(bastionHost string, addr string, privateKey string, iaasPlatform string) (map[string]string, error) {
	cmd := "Get-CimInstance -ClassName Win32_Service | Select-Object -Property Name,Description | ConvertTo-Csv -NoTypeInformation"
	msg, err := runPSCommand(bastionHost, addr, cmd, privateKey, iaasPlatform)
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		e2e.Failf("error running SSH job")
	}
	svcSplit := strings.SplitAfterN(msg, "\"Name\",\"Description\"\r\n", 2)
	svcTrimmed := strings.TrimSpace(svcSplit[1])
	services := make(map[string]string)
	lines := strings.Split(svcTrimmed, "\r\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) != 2 {
			e2e.Logf("expected comma separated values, found: " + line)
		}
		services[strings.Trim(fields[0], "\"")] = strings.Trim(fields[1], "\"")
	}
	return services, nil
}

func checkRunningServicesOnWindowsNode(svcs map[int]string, winServices map[string]string) (expectedService bool, svc string) {
	for _, svc = range svcs {
		_, expectedService := winServices[svc]
		if !expectedService {
			e2e.Logf("Service %v does not exist", svc)
		} else {
			e2e.Logf("Service %v exists", svc)
		}
	}
	return expectedService, svc
}

func checkFoldersDoNotExist(bastionHost string, winInternalIP string, folder string, privateKey string, iaasPlatform string) bool {
	msg, _ := runPSCommand(bastionHost, winInternalIP, fmt.Sprintf("Get-Item %v", folder), privateKey, iaasPlatform)
	if !strings.Contains(msg, "ItemNotFoundException") {
		return true
	}
	return false
}

// currently not available need to be fix logic
func waitUntilWMCOStatusChanged(oc *exutil.CLI, message string) {
	waitLogErr := wait.Poll(10*time.Second, 15*time.Minute, func() (bool, error) {
		msg, err := oc.WithoutNamespace().Run("logs").Args("deployment.apps/windows-machine-config-operator", "-n", "openshift-windows-machine-config-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(msg, message) {
			e2e.Logf("Failed to check %v, try next round", message)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(waitLogErr, fmt.Sprintf("%v still watch label", message))
}

func waitForEndpointsReady(oc *exutil.CLI, namespace string, waitTime int, numberOfEndpoints int) {
	waitLogErr := wait.Poll(10*time.Second, time.Duration(waitTime)*time.Minute, func() (bool, error) {
		msg, err := oc.WithoutNamespace().Run("get").Args("endpoints", "-n", namespace, "-ojsonpath={.items[*].subsets[*].addresses[*].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if (msg == "" && numberOfEndpoints == 0) || len(strings.Split(msg, " ")) == numberOfEndpoints {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(waitLogErr, fmt.Sprintf("Required number of endpoints %d not reached", numberOfEndpoints))
}

func getRandomString(len int) string {
	buff := make([]byte, len)
	rand.Read(buff)
	str := base64.StdEncoding.EncodeToString(buff)
	// Base 64 can be longer than len
	return str[:len]
}

func getEndpointsIPs(oc *exutil.CLI, namespace string) string {
	endpoints, err := oc.WithoutNamespace().Run("get").Args("endpoints", "-n", namespace, "-o=jsonpath={.items[].subsets[].addresses[*].ip}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return endpoints
}

func setBYOH(oc *exutil.CLI, iaasPlatform string, addressType string, machinesetName string) string {
	user := getAdministratorNameByPlatform(iaasPlatform)
	clusterVersions, _, err := exutil.GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	if v.CompareVersions(clusterVersions, ">", "4.9") {
		winVersion = "2022"
	} else if v.CompareVersions(clusterVersions, "<=", "4.9") && iaasPlatform == "vsphere" {
		winVersion = "2004"
	}
	machinesetFileName := "aws_byoh_machineset.yaml"
	if iaasPlatform == "azure" {
		machinesetFileName = "azure_byoh_machineset.yaml"
	} else if iaasPlatform == "vsphere" {
		machinesetFileName = "vsphere_byoh_machineset.yaml"
	}
	// here we need to use a hardcoded machineset 'byoh' since AWS machineset name is too long.
	MSFileName, err := getMachinesetFileName(oc, iaasPlatform, winVersion, "byoh", machinesetFileName)
	defer os.Remove(MSFileName)
	createMachineset(oc, MSFileName)
	o.Expect(err).NotTo(o.HaveOccurred())
	addressesArray := fetchAddress(oc, addressType, machinesetName)
	setConfigmap(oc, addressesArray[0], user, "config-map.yaml")
	waitForMachinesetReady(oc, machinesetName, 15, 1)
	return addressesArray[0]
}
