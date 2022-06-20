package disasterrecovery

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"bufio"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func getNodeListByLabel(oc *exutil.CLI, labelKey string) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNameList := strings.Fields(output)
	return nodeNameList
}

func getPodListByLabel(oc *exutil.CLI, labelKey string) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-etcd", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podNameList := strings.Fields(output)
	return podNameList
}

func runDRBackup(oc *exutil.CLI, nodeNameList []string) (nodeName string, etcddb string) {
	var nodeN, etcdDb string
	succBackup := false
	for _, node := range nodeNameList {
		backupout, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", oc.Namespace(), "node/"+node, "--", "chroot", "/host", "/usr/local/bin/cluster-backup.sh", "/home/core/assets/backup").Output()
		if err != nil {
			e2e.Logf("Try for next master!")
			continue
		}
		if strings.Contains(backupout, "Snapshot saved at") && err == nil {
			e2e.Logf("backup on master %v ", node)
			regexp, _ := regexp.Compile("/home/core/assets/backup/snapshot.*db")
			etcdDb = regexp.FindString(backupout)
			nodeN = node
			succBackup = true
			break
		}
	}
	if !succBackup {
		e2e.Failf("Failed to run the backup!")
	}
	return nodeN, etcdDb
}

func getUserNameAndKeyonBationByPlatform(iaasPlatform string, privateKey string) (string, string) {
	user := ""
	keyOnBastion := ""
	switch iaasPlatform {
	case "aws":
		user = os.Getenv("SSH_CLOUD_PRIV_AWS_USER")
		if user == "" {
			user = "ec2-user"
		}
		keyOnBastion = "/home/ec2-user/" + filepath.Base(privateKey)
	case "gcp":
		user = os.Getenv("SSH_CLOUD_PRIV_GCP_USER")
		if user == "" {
			user = "cloud-user"
		}
		keyOnBastion = "/home/cloud-user/" + filepath.Base(privateKey)
	case "azure":
		user = os.Getenv("SSH_CLOUD_PRIV_AZURE_USER")
		if user == "" {
			user = "cloud-user"
		}
		keyOnBastion = "/home/cloud-user/" + filepath.Base(privateKey)
	}
	return user, keyOnBastion
}

func getNodeInternalIPListByLabel(oc *exutil.CLI, labelKey string) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", labelKey, "-o=jsonpath='{.items[*].status.addresses[?(.type==\"InternalIP\")].address}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeInternalIPList := strings.Fields(strings.ReplaceAll(output, "'", ""))
	return nodeInternalIPList
}

// Run the etcdrestroe shell script command on master or node
func runPSCommand(bastionHost string, nodeInternalIP string, command string, privateKeyForClusterNode string, privateKeyForBastion string, userForBastion string) (result string, err error) {
	var msg []byte
	msg, err = exec.Command("bash", "-c", "chmod 600 "+privateKeyForBastion+"; ssh -i "+privateKeyForBastion+" -t -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "+userForBastion+"@"+bastionHost+" sudo -i ssh  -o StrictHostKeyChecking=no  -o UserKnownHostsFile=/dev/null -i "+privateKeyForClusterNode+" core@"+nodeInternalIP+" "+command).CombinedOutput()
	return string(msg), err
}

func waitForOperatorRestart(oc *exutil.CLI, operatorName string) {
	g.By("Check the operator should be in Progressing")
	err := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", operatorName).Output()
		if err != nil {
			e2e.Logf("clusteroperator not start new progress, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
			e2e.Logf("clusteroperator is Progressing:\n%s", output)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "clusteroperator is not Progressing")

	g.By("Wait for the operator to rollout")
	err = wait.Poll(60*time.Second, 900*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", operatorName).Output()
		if err != nil {
			e2e.Logf("Fail to get clusteroperator %s, error: %s. Trying again", operatorName, err)
			return false, nil
		}
		if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
			e2e.Logf("clusteroperator %s is recover to normal:\n%s", operatorName, output)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "clusteroperator is not recovered to normal")
}

func waitForContainerDisappear(bastionHost string, nodeInternalIP string, command string, privateKeyForClusterNode string, privateKeyForBastion string, userForBastion string) {
	g.By("Wait for the container to disappear")
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		msg, err := runPSCommand(bastionHost, nodeInternalIP, command, privateKeyForClusterNode, privateKeyForBastion, userForBastion)
		if err != nil {
			e2e.Logf("Fail to get container, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString("", msg); matched {
			e2e.Logf("The container has disappeared")
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The pod is not disappeared as expected")
}

//Check if the iaasPlatform in the supported list
func in(target string, strArray []string) bool {
	for _, element := range strArray {
		if target == element {
			return true
		}
	}
	return false
}

//make sure all the ectd pods are running
func checkEtcdPodStatus(oc *exutil.CLI) bool {
	output, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=etcd", "-n", "openshift-etcd", "-o=jsonpath='{.items[*].status.phase}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	statusList := strings.Fields(output)
	for _, podStatus := range statusList {
		if match, _ := regexp.MatchString("Running", podStatus); !match {
			e2e.Logf("Find etcd pod is not running")
			return false
		}
	}
	return true
}

//make sure all the machine are running
func waitMachineStatusRunning(oc *exutil.CLI, newMasterMachineName string) {
	err := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		machineStatus, err := oc.AsAdmin().Run("get").Args("-n", "openshift-machine-api", exutil.MapiMachine, newMasterMachineName, "-o=jsonpath='{.status.phase}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if match, _ := regexp.MatchString("Running", machineStatus); match {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The machine is not Running as expected")
}

//make sure correct number of machines are present
func waitforDesiredMachineCount(oc *exutil.CLI, machineCount int) {
	err := wait.Poll(60*time.Second, 480*time.Second, func() (bool, error) {
		output, errGetMachine := oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=master", "-o=jsonpath='{.items[*].metadata.name}'").Output()
		o.Expect(errGetMachine).NotTo(o.HaveOccurred())
		machineNameList := strings.Fields(output)
		if len(machineNameList) == machineCount {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "The machine count didn't match")
}

//update new machine file
func updateMachineYmlFile(machineYmlFile string, oldMachineName string, newMasterMachineName string) bool {
	fileName := machineYmlFile
	in, err := os.OpenFile(fileName, os.O_RDONLY, 0666)
	if err != nil {
		e2e.Logf("open machineYaml file fail:", err)
		return false
	}
	defer in.Close()

	out, err := os.OpenFile(strings.Replace(fileName, "machine.yaml", "machineUpd.yaml", -1), os.O_RDWR|os.O_CREATE, 0766)
	if err != nil {
		e2e.Logf("Open write file fail:", err)
		return false
	}
	defer out.Close()

	br := bufio.NewReader(in)
	index := 1
	matchTag := false
	newLine := ""

	for {
		line, _, err := br.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			e2e.Logf("read err:", err)
			return false
		}
		if strings.Contains(string(line), "providerID: ") {
			matchTag = true
		} else if strings.Contains(string(line), "status:") {
			break
		} else if strings.Contains(string(line), "generation: ") {
			matchTag = true
		} else if strings.Contains(string(line), "machine.openshift.io/instance-state: ") {
			matchTag = true
		} else if strings.Contains(string(line), "resourceVersion: ") {
			matchTag = true
		} else if strings.Contains(string(line), "uid: ") {
			matchTag = true
		} else if strings.Contains(string(line), oldMachineName) {
			newLine = strings.Replace(string(line), oldMachineName, newMasterMachineName, -1)
		} else {
			newLine = string(line)
		}
		if !matchTag {
			_, err = out.WriteString(newLine + "\n")
			if err != nil {
				e2e.Logf("Write to file fail:", err)
				return false
			}
		} else {
			matchTag = false
		}
		index++
	}
	e2e.Logf("Update Machine FINISH!")
	return true
}
