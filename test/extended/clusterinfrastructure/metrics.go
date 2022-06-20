package clusterinfrastructure

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("metrics", exutil.KubeConfigPath())
	)

	// author: zhsun@redhat.com
	g.It("Author:zhsun-Medium-45499-mapi_current_pending_csr should reflect real pending CSR count [Flaky]", func() {
		g.By("Check the pending csr count")
		csrStatuses, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csr", "-o=jsonpath={.items[*].status.conditions[0].type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		csrStatusList := strings.Split(csrStatuses, " ")
		pending := 0
		for _, status := range csrStatusList {
			if status == "Pending" {
				pending++
			}
		}
		g.By("Get machine-approver-controller pod name")
		machineApproverPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", machineApproverNamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the value of mapi_current_pending_csr")
		token := getPrometheusSAToken(oc)
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(machineApproverPodName, "-c", "machine-approver-controller", "-n", machineApproverNamespace, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://localhost:9192/metrics").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		o.Expect(metrics).To(o.ContainSubstring("mapi_current_pending_csr " + strconv.Itoa(pending)))
	})

	// author: zhsun@redhat.com
	g.It("NonPreRelease-Author:zhsun-Medium-43764-MachineHealthCheckUnterminatedShortCircuit alert should be fired when a MHC has been in a short circuit state [Serial][Slow][Disruptive]", func() {
		g.By("Create a new machineset")
		exutil.SkipConditionally(oc)
		ms := exutil.MachineSetDescription{"machineset-43764", 1}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)

		g.By("Create a MachineHealthCheck")
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mhcBaseDir := exutil.FixturePath("testdata", "clusterinfrastructure", "mhc")
		mhcTemplate := filepath.Join(mhcBaseDir, "mhc.yaml")
		mhc := mhcDescription{
			clusterid:      clusterID,
			maxunhealthy:   "0%",
			machinesetName: "machineset-43764",
			name:           "mhc-43764",
			template:       mhcTemplate,
		}
		defer mhc.deleteMhc(oc)
		mhc.createMhc(oc)

		g.By("Delete the node attached to the machine")
		machineName := exutil.GetMachineNamesFromMachineSet(oc, "machineset-43764")[0]
		nodeName := exutil.GetNodeNameFromMachine(oc, machineName)
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("node", nodeName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get machine-api-controller pod name")
		machineAPIControllerPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-l", "api=clusterapi", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check metrics mapi_machinehealthcheck_short_circuit")
		token := getPrometheusSAToken(oc)
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(machineAPIControllerPodName, "-c", "machine-healthcheck-controller", "-n", machineAPINamespace, "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://localhost:8444/metrics").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		o.Expect(metrics).To(o.ContainSubstring("mapi_machinehealthcheck_short_circuit{name=\"" + mhc.name + "\",namespace=\"openshift-machine-api\"} " + strconv.Itoa(1)))

		g.By("Check alert MachineHealthCheckUnterminatedShortCircuit is raised")
		checkAlertRaised(oc, "MachineHealthCheckUnterminatedShortCircuit")
	})

	// author: huliu@redhat.com
	g.It("NonPreRelease-Author:huliu-High-36989-mapi_instance_create_failed metrics should work [Disruptive]", func() {
		exutil.SkipConditionally(oc)
		var patchstr string
		platform := exutil.CheckPlatform(oc)
		switch platform {
		case "aws", "alibabacloud":
			patchstr = `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"instanceType":"invalid"}}}}}}`
		case "gcp":
			patchstr = `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"machineType":"invalid"}}}}}}`
		case "azure":
			patchstr = `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"vmSize":"invalid"}}}}}}`
		/*
			there is a bug(https://bugzilla.redhat.com/show_bug.cgi?id=1900538) for openstack
			case "openstack":
				patchstr = `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"flavor":"invalid"}}}}}}`
		*/
		case "vsphere":
			patchstr = `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"workspace":{"folder":"/SDDC-Datacenter/vm/invalid"}}}}}}}`
		default:
			e2e.Logf("Not support cloud provider for the case for now.")
			g.Skip("Not support cloud provider for the case for now.")
		}

		g.By("Create a new machineset")
		machinesetName := "machineset-36989"
		ms := exutil.MachineSetDescription{machinesetName, 0}
		defer ms.DeleteMachineSet(oc)
		ms.CreateMachineSet(oc)

		g.By("Update machineset with invalid instanceType(or other similar field)")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(mapiMachineset, machinesetName, "-n", "openshift-machine-api", "-p", patchstr, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.WaitForMachineFailed(oc, machinesetName)

		machineName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachine, "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machinesetName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check metrics mapi_instance_create_failed is shown")
		checkMetricsShown(oc, "mapi_instance_create_failed", machineName)
	})
})
