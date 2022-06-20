package disasterrecovery

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-disasterrecovery] DR_Testing", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())
		iaasPlatform string
	)

	g.BeforeEach(func() {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		iaasPlatform = strings.ToLower(output)
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-NonPreRelease-Critical-42183-backup and restore should perform consistency checks on etcd snapshots [Disruptive]", func() {
		g.By("Test for case OCP-42183 backup and restore should perform consistency checks on etcd snapshots")

		g.By("select all the master node")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")

		g.By("Run the backup")
		masterN, etcdDb := runDRBackup(oc, masterNodeList)

		defer func() {
			_, err := oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "rm", "-rf", "/home/core/assets/backup").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Corrupt the etcd db file ")
		_, err := oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "truncate", "-s", "126k", etcdDb).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Run the restore")
		output, _ := oc.AsAdmin().Run("debug").Args("-n", oc.Namespace(), "node/"+masterN, "--", "chroot", "/host", "/usr/local/bin/cluster-restore.sh", "/home/core/assets/backup").Output()
		o.Expect(output).To(o.ContainSubstring("Backup appears corrupted. Aborting!"))
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Longduration-NonPreRelease-Critical-23803-Restoring back to a previous cluster state in ocp v4 [Disruptive][Slow]", func() {
		privateKeyForBastion := os.Getenv("SSH_CLOUD_PRIV_KEY")
		if privateKeyForBastion == "" {
			g.Skip("Failed to get the private key, skip the cases!!")
		}

		bastionHost := os.Getenv("QE_BASTION_PUBLIC_ADDRESS")
		if bastionHost == "" {
			g.Skip("Failed to get the qe bastion public ip, skip the cases!!")
		}

		g.By("check the platform is supported or not")
		supportedList := []string{"aws", "gcp", "azure"}
		support := in(iaasPlatform, supportedList)
		if support != true {
			g.Skip("The platform is not supported now, skip the cases!!")
		}

		g.By("make sure all the etcd pods are running")
		podAllRunning := checkEtcdPodStatus(oc)
		if podAllRunning != true {
			g.Skip("The ectd pods are not running")
		}
		defer o.Expect(checkEtcdPodStatus(oc)).To(o.BeTrue())

		g.By("select all the master node")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")
		masterNodeInternalIPList := getNodeInternalIPListByLabel(oc, "node-role.kubernetes.io/master=")

		userForBastion, privateKeyForClusterNode := getUserNameAndKeyonBationByPlatform(iaasPlatform, privateKeyForBastion)
		e2e.Logf("user on bastion is  : %v", userForBastion)
		e2e.Logf("key on bastion is  : %v", privateKeyForClusterNode)

		g.By("Run the backup on the first master")
		defer runPSCommand(bastionHost, masterNodeInternalIPList[0], "sudo rm -rf /home/core/assets/backup", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
		msg, err := runPSCommand(bastionHost, masterNodeInternalIPList[0], "sudo /usr/local/bin/cluster-backup.sh /home/core/assets/backup", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
		if err != nil {
			e2e.Logf("backup is failed , the msg is : %v", msg)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(msg).To(o.ContainSubstring("snapshot db and kube resources are successfully saved"))

		g.By("Stop the static pods on any other control plane nodes")
		//if assert err the cluster will be unavailable
		for i := 1; i < len(masterNodeInternalIPList); i++ {
			_, err := runPSCommand(bastionHost, masterNodeInternalIPList[i], "sudo mv /etc/kubernetes/manifests/etcd-pod.yaml /tmp", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForContainerDisappear(bastionHost, masterNodeInternalIPList[i], "sudo crictl ps | grep etcd | grep -v operator", privateKeyForClusterNode, privateKeyForBastion, userForBastion)

			_, err = runPSCommand(bastionHost, masterNodeInternalIPList[i], "sudo mv /etc/kubernetes/manifests/kube-apiserver-pod.yaml /tmp", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForContainerDisappear(bastionHost, masterNodeInternalIPList[i], "sudo crictl ps | grep kube-apiserver | grep -v operator", privateKeyForClusterNode, privateKeyForBastion, userForBastion)

			_, err = runPSCommand(bastionHost, masterNodeInternalIPList[i], "sudo cp -r /var/lib/etcd/ /tmp; sudo  rm -rf /var/lib/etcd", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Run the restore script on the recovery control plane host")
		msg, err = runPSCommand(bastionHost, masterNodeInternalIPList[0], "sudo -E /usr/local/bin/cluster-restore.sh /home/core/assets/backup", privateKeyForClusterNode, privateKeyForBastion, userForBastion)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("static-pod-resources"))

		g.By("Restart the kubelet service on all control plane hosts")
		for i := 0; i < len(masterNodeList); i++ {
			_, _ = runPSCommand(bastionHost, masterNodeInternalIPList[i], "sudo systemctl restart kubelet.service", privateKeyForClusterNode, privateKeyForBastion, userForBastion)

		}

		g.By("Wait for all the kubelet service on all control plane hosts are ready")
		for i := 0; i < len(masterNodeList); i++ {
			err := wait.Poll(5*time.Second, 240*time.Second, func() (bool, error) {
				out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", masterNodeList[i]).Output()
				if err != nil {
					e2e.Logf("Fail to get master, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString(" Ready", out); matched {
					e2e.Logf("kubelet service on %s is recover to normal:\n%s", masterNodeList[i], out)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "the kubelet is not recovered to normal")
		}

		g.By("Force etcd redeployment")
		t := time.Now()
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("etcd", "cluster", "--type=merge", "-p", fmt.Sprintf("{\"spec\": {\"forceRedeploymentReason\": \"recovery-%s\"}}", t.Format("2006-01-02 15:05:05"))).Execute()
			if err != nil {
				e2e.Logf("Fail to force the etcd redeployment, error: %s. Trying again", err)
				return false, nil
			}
			return true, nil

		})
		exutil.AssertWaitPollNoErr(err, "Failed to force etcd deployment")
		waitForOperatorRestart(oc, "etcd")

		g.By("Force the Kubernetes API server redeployment")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver", "cluster", "--type=merge", "-p", fmt.Sprintf("{\"spec\": {\"forceRedeploymentReason\": \"recovery-%s\"}}", t.Format("2006-01-02 15:05:05"))).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForOperatorRestart(oc, "kube-apiserver")

		g.By("Force the Kubernetes controller manager redeployment")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubecontrollermanager", "cluster", "--type=merge", "-p", fmt.Sprintf("{\"spec\": {\"forceRedeploymentReason\": \"recovery-%s\"}}", t.Format("2006-01-02 15:05:05"))).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForOperatorRestart(oc, "kube-controller-manager")

		g.By("Force the Kubernetes scheduler redeployment")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=merge", "-p", fmt.Sprintf("{\"spec\": {\"forceRedeploymentReason\": \"recovery-%s\"}}", t.Format("2006-01-02 15:05:05"))).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForOperatorRestart(oc, "kube-scheduler")

	})
	// author: geliu@redhat.com
	g.It("Author:geliu-NonPreRelease-Critical-50205-lost master can be replaced by new one with machine config recreation in ocp 4.x [Disruptive][Slow]", func() {
		g.By("Test for case lost master can be replaced by new one with machine config recreation in ocp 4.x")

		g.By("Get all the master node name & count")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")
		masterNodeCount := len(masterNodeList)

		g.By("make sure all the etcd pods are running")
		defer o.Expect(checkEtcdPodStatus(oc)).To(o.BeTrue())
		podAllRunning := checkEtcdPodStatus(oc)
		if podAllRunning != true {
			g.Skip("The ectd pods are not running")
		}

		g.By("Export the machine config file for 1st master node")
		output, err := oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=master", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterMachineNameList := strings.Fields(output)
		machineYmlFile := ""
		machineYmlFile, err = oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", masterMachineNameList[0], "-o", "yaml").OutputToFile("machine.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		newMachineConfigFile := strings.Replace(machineYmlFile, "machine.yaml", "machineUpd.yaml", -1)
		defer exec.Command("bash", "-c", "rm -f "+machineYmlFile).Output()
		defer exec.Command("bash", "-c", "rm -f "+newMachineConfigFile).Output()

		g.By("update machineYmlFile to newMachineYmlFile:")
		newMasterMachineNameSuffix := masterMachineNameList[0] + "00"
		o.Expect(updateMachineYmlFile(machineYmlFile, masterMachineNameList[0], newMasterMachineNameSuffix)).To(o.BeTrue())

		g.By("Create new machine")
		resultFile, _ := exec.Command("bash", "-c", "cat "+newMachineConfigFile).Output()
		e2e.Logf("####newMasterMachineNameSuffix is %s\n", string(resultFile))
		_, err = oc.AsAdmin().Run("create").Args("-n", "openshift-machine-api", "-f", newMachineConfigFile).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitMachineStatusRunning(oc, newMasterMachineNameSuffix)

		g.By("Delete machine of the unhealthy master node")
		_, err = oc.AsAdmin().Run("delete").Args("-n", "openshift-machine-api", "machine", masterMachineNameList[0]).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(getNodeListByLabel(oc, "node-role.kubernetes.io/master="))).To(o.Equal(masterNodeCount))
	})

	// author: skundu@redhat.com
	g.It("Longduration-Author:skundu-NonPreRelease-Critical-51109-Delete an existing machine at first and then add a new one. [Disruptive]", func() {
		g.By("Test for delete an existing machine at first and then add a new one")

		g.By("Get all the master node name & count")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")
		masterNodeCount := len(masterNodeList)

		g.By("Make sure all the etcd pods are running")
		defer o.Expect(checkEtcdPodStatus(oc)).To(o.BeTrue())
		podAllRunning := checkEtcdPodStatus(oc)
		if podAllRunning != true {
			g.Skip("The ectd pods are not running")
		}

		g.By("Export the machine config file for first master node")
		output, errMachineConfig := oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=master", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(errMachineConfig).NotTo(o.HaveOccurred())
		masterMachineNameList := strings.Fields(output)
		machineYmlFile := ""
		machineYmlFile, errMachineYaml := oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", masterMachineNameList[0], "-o", "yaml").OutputToFile("machine.yaml")
		o.Expect(errMachineYaml).NotTo(o.HaveOccurred())
		newMachineConfigFile := strings.Replace(machineYmlFile, "machine.yaml", "machineUpd.yaml", -1)
		defer func() { o.Expect(os.Remove(machineYmlFile)).NotTo(o.HaveOccurred()) }()
		defer func() { o.Expect(os.Remove(newMachineConfigFile)).NotTo(o.HaveOccurred()) }()

		g.By("Update machineYmlFile to newMachineYmlFile:")
		newMasterMachineNameSuffix := masterMachineNameList[0] + "-new"
		o.Expect(updateMachineYmlFile(machineYmlFile, masterMachineNameList[0], newMasterMachineNameSuffix)).To(o.BeTrue())

		g.By("At first delete machine of the master node without adding new one")
		errMachineDelete := oc.AsAdmin().Run("delete").Args("-n", "openshift-machine-api", "--wait=false", "machine", masterMachineNameList[0]).Execute()
		o.Expect(errMachineDelete).NotTo(o.HaveOccurred())
		g.By("Make sure the node count is not reduced. Machine deletion hooks will prevent the deletion of the node before addition of the new one")
		o.Expect(len(getNodeListByLabel(oc, "node-role.kubernetes.io/master="))).To(o.Equal(masterNodeCount))

		g.By("Verify that the machine is getting deleted...")
		machineStatusOutput, errVerifyDelete := oc.AsAdmin().Run("get").Args(exutil.MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=master", "-o", "jsonpath={.items[*].status.phase}").Output()
		o.Expect(errVerifyDelete).NotTo(o.HaveOccurred())
		masterMachineStatus := strings.Fields(machineStatusOutput)
		o.Expect(string(masterMachineStatus[0])).To(o.Equal("Deleting"))

		g.By("Creating new machine")
		resultFile, _ := exec.Command("bash", "-c", "cat "+newMachineConfigFile).Output()
		e2e.Logf("####newMasterMachineNameSuffix is %s\n", string(resultFile))
		errMachineCreation := oc.AsAdmin().Run("create").Args("-n", "openshift-machine-api", "-f", newMachineConfigFile).Execute()
		o.Expect(errMachineCreation).NotTo(o.HaveOccurred())
		waitMachineStatusRunning(oc, newMasterMachineNameSuffix)
		g.By("Verify that the machine is deleted. The machine count is same as before.")
		waitforDesiredMachineCount(oc, len(masterMachineNameList))

	})

})
