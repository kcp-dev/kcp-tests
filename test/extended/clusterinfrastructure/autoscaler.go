package clusterinfrastructure

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc                        = exutil.NewCLI("cluster-autoscaler-operator", exutil.KubeConfigPath())
		autoscalerBaseDir         string
		clusterAutoscalerTemplate string
		machineAutoscalerTemplate string
		workLoadTemplate          string
		clusterAutoscaler         clusterAutoscalerDescription
		machineAutoscaler         machineAutoscalerDescription
		workLoad                  workLoadDescription
	)

	g.BeforeEach(func() {
		autoscalerBaseDir = exutil.FixturePath("testdata", "clusterinfrastructure", "autoscaler")
		clusterAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "clusterautoscaler.yaml")
		machineAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "machineautoscaler.yaml")
		workLoadTemplate = filepath.Join(autoscalerBaseDir, "workload.yaml")
		clusterAutoscaler = clusterAutoscalerDescription{
			maxNode:   100,
			minCore:   0,
			maxCore:   320000,
			minMemory: 0,
			maxMemory: 6400000,
			template:  clusterAutoscalerTemplate,
		}
		workLoad = workLoadDescription{
			name:      "workload",
			namespace: "openshift-machine-api",
			template:  workLoadTemplate,
		}
	})
	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-43174-ClusterAutoscaler CR could be deleted with foreground deletion", func() {
		g.By("Create clusterautoscaler")
		clusterAutoscaler.createClusterAutoscaler(oc)
		defer clusterAutoscaler.deleteClusterAutoscaler(oc)
		g.By("Delete clusterautoscaler with foreground deletion")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterautoscaler", "default", "--cascade=foreground").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterautoscaler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("default"))
	})

	//author: miyadav@redhat.com
	g.It("Longduration-NonPreRelease-Author:miyadav-Low-45430-MachineSet scaling from 0 should be evaluated correctly for the new or changed instance types [Serial][Slow][Disruptive]", func() {
		machineAutoscaler = machineAutoscalerDescription{
			name:           "machineautoscaler-45430",
			namespace:      "openshift-machine-api",
			maxReplicas:    1,
			minReplicas:    0,
			template:       machineAutoscalerTemplate,
			machineSetName: "machineset-45430",
		}

		g.By("Create machineset with instance type other than default in cluster")
		exutil.SkipConditionally(oc)
		platform := exutil.CheckPlatform(oc)
		if platform == "aws" {
			ms := exutil.MachineSetDescription{"machineset-45430", 0}
			defer ms.DeleteMachineSet(oc)
			ms.CreateMachineSet(oc)
			g.By("Update machineset with instanceType")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(mapiMachineset, "machineset-45430", "-n", "openshift-machine-api", "-p", `{"spec":{"template":{"spec":{"providerSpec":{"value":{"instanceType": "m5.4xlarge"}}}}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create MachineAutoscaler")
			defer machineAutoscaler.deleteMachineAutoscaler(oc)
			machineAutoscaler.createMachineAutoscaler(oc)

			g.By("Create clusterautoscaler")
			defer clusterAutoscaler.deleteClusterAutoscaler(oc)
			clusterAutoscaler.createClusterAutoscaler(oc)

			g.By("Create workload")
			defer workLoad.deleteWorkLoad(oc)
			workLoad.createWorkLoad(oc)

			g.By("Check machine could be created successful")
			// Creat a new machine taking roughly 5 minutes , set timeout as 7 minutes
			exutil.WaitForMachinesRunning(oc, 1, "machineset-45430")
		}
	})

	//author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-44816-Cluster version operator could remove unrecognized volume mounts [Disruptive]", func() {
		//As cluster-autoscaler-operator deployment will be synced by cvo, so we don't need defer to resotre autoscaler deployment
		g.By("Update cluster-autoscaler-operator deployment's volumeMounts")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("deploy/cluster-autoscaler-operator", "-n", machineAPINamespace, "-p", `[{"op": "add", "path": "/spec/template/spec/containers/0/volumeMounts/0","value":{"mountPath":"/etc/cluster-autoscaler-operator-invalid/service-ca","name":"cert","readOnly":true}}]`, "--type=json").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check cluster-autoscaler-operator deployment was synced by cvo soon")
		err = wait.Poll(15*time.Second, 3*time.Minute, func() (bool, error) {
			caoDeploy, _ := oc.AsAdmin().WithoutNamespace().Run("describe").Args("deploy/cluster-autoscaler-operator", "-n", machineAPINamespace).Output()
			if strings.Contains(caoDeploy, "service-ca") {
				e2e.Logf("cluster-autoscaler-operator deployment was not synced by cvo")
				return false, nil
			}
			e2e.Logf("cluster-autoscaler-operator deployment was synced by cvo")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "cluster-autoscaler-operator deployment was not synced by cvo in 3m")

		g.By("Check cluster-autoscaler-operator pod is running")
		podsStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "k8s-app=cluster-autoscaler-operator", "-o=jsonpath={.items[0].status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podsStatus).To(o.ContainSubstring("Running"))
	})

	//author: huliu@redhat.com
	g.It("Longduration-NonPreRelease-Author:huliu-Medium-47656-[CAO] Cluster autoscaler could scale down based on scale down utilization threshold [Slow][Disruptive]", func() {
		exutil.SkipConditionally(oc)
		machinesetName := "machineset-47656"
		utilThreshold := "0.08"
		utilThresholdNum := 8
		clusterAutoscalerTemplate = filepath.Join(autoscalerBaseDir, "clusterautoscalerutil.yaml")
		clusterAutoscaler = clusterAutoscalerDescription{
			maxNode:              100,
			minCore:              0,
			maxCore:              320000,
			minMemory:            0,
			maxMemory:            6400000,
			utilizationThreshold: utilThreshold,
			template:             clusterAutoscalerTemplate,
		}
		machineAutoscaler = machineAutoscalerDescription{
			name:           "machineautoscaler-47656",
			namespace:      "openshift-machine-api",
			maxReplicas:    3,
			minReplicas:    1,
			template:       machineAutoscalerTemplate,
			machineSetName: machinesetName,
		}

		g.By("Create a new machineset")
		ms := exutil.MachineSetDescription{machinesetName, 1}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)

		g.By("Create clusterautoscaler")
		defer clusterAutoscaler.deleteClusterAutoscaler(oc)
		clusterAutoscaler.createClusterAutoscaler(oc)

		g.By("Create MachineAutoscaler")
		defer machineAutoscaler.deleteMachineAutoscaler(oc)
		machineAutoscaler.createMachineAutoscaler(oc)

		g.By("Create workload")
		defer workLoad.deleteWorkLoad(oc)
		workLoad.createWorkLoad(oc)

		g.By("Check machine could be created successful")
		exutil.WaitForMachinesRunning(oc, 3, "machineset-47656")
		workLoad.deleteWorkLoad(oc)
		/*
			Refer to autoscaler use case OCP-28108.
			Wait five minutes after deleting workload, the machineset will scale down,
			so wait five minutes here, then check whether the machineset is scaled down based on utilizationThreshold.
		*/
		time.Sleep(300 * time.Second)
		g.By("Check machineset could scale down based on utilizationThreshold")
		out, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachineset, machinesetName, "-o=jsonpath={.status.readyReplicas}", "-n", machineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(out)

		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachine, "-o=jsonpath={.items[0].status.nodeRef.name}", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeInfoFile, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node", nodeName, "-n", machineAPINamespace).OutputToFile("OCP-47656-nodeinfo.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		getUtilCmd := fmt.Sprintf(`grep -A 10 "Allocated resources:" %s |egrep "cpu|memory"|awk -F"[(%%]" 'BEGIN{util=0} $2>util{util=$2} END{print util}'`, nodeInfoFile)
		util, err := exec.Command("bash", "-c", getUtilCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		utilNum, err := strconv.Atoi(strings.TrimSpace(string(util)))
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("utilNum:%s utilThresholdNum:%s", utilNum, utilThresholdNum)
		if utilNum < utilThresholdNum {
			o.Expect(machinesRunning).Should(o.Equal(1))
		} else {
			o.Expect(machinesRunning).Should(o.Equal(3))
		}
	})
})
