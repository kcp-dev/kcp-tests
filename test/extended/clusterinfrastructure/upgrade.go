package clusterinfrastructure

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLI("cluster-infrastructure-upgrade", exutil.KubeConfigPath())
		iaasPlatform string
	)

	g.BeforeEach(func() {
		iaasPlatform = exutil.CheckPlatform(oc)
	})

	// author: zhsun@redhat.com
	g.It("Longduration-NonPreRelease-PstChkUpgrade-Author:zhsun-High-43725-[Upgrade]Enable out-of-tree cloud providers with feature gate [Disruptive]", func() {
		g.By("Check if ccm on this platform is supported")
		if !(iaasPlatform == "aws" || iaasPlatform == "azure" || iaasPlatform == "openstack" || iaasPlatform == "gcp" || iaasPlatform == "vsphere") {
			g.Skip("Skip for ccm on this platform is not supported or don't need to enable!")
		}
		g.By("Check if ccm is deployed")
		ccm, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "-n", "openshift-cloud-controller-manager", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(ccm) != 0 {
			g.Skip("Skip for ccm is already be deployed!")
		}

		g.By("Enable out-of-tree cloud provider with feature gate")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate/cluster", "-p", `{"spec":{"featureSet": "TechPreviewNoUpgrade"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check cluster is still healthy")
		waitForClusterHealthy(oc)

		g.By("Check if appropriate `--cloud-provider=external` set on kubelet, KAPI and KCM")
		masterkubelet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-master-kubelet", "-o=jsonpath={.spec.config.systemd.units[0].contents}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masterkubelet).To(o.ContainSubstring("cloud-provider=external"))
		workerkubelet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-worker-kubelet", "-o=jsonpath={.spec.config.systemd.units[0].contents}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(workerkubelet).To(o.ContainSubstring("cloud-provider=external"))
		kapi, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm/config", "-n", "openshift-kube-apiserver", "-o=jsonpath={.data.config\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kapi).To(o.ContainSubstring("\"cloud-provider\":[\"external\"]"))
		kcm, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm/config", "-n", "openshift-kube-controller-manager", "-o=jsonpath={.data.config\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kcm).To(o.ContainSubstring("\"cloud-provider\":[\"external\"]"))
	})

	// author: zhsun@redhat.com
	g.It("Longduration-NonPreRelease-PreChkUpgrade-Author:zhsun-Medium-41804-[Upgrade]Spot/preemptible instances should not block upgrade - Azure [Disruptive]", func() {
		if iaasPlatform != "azure" {
			g.Skip("Skip this test scenario because it is not supported on the " + iaasPlatform + " platform")
		}
		randomMachinesetName := exutil.GetRandomMachineSetName(oc)
		region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachineset, randomMachinesetName, "-n", "openshift-machine-api", "-o=jsonpath={.spec.template.spec.providerSpec.value.location}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if region == "northcentralus" || region == "westus" || region == "usgovvirginia" {
			g.Skip("Skip this test scenario because it is not supported on the " + region + " region, because this region doesn't have zones")
		}

		g.By("Create a spot instance on azure")
		exutil.SkipConditionally(oc)
		ms := exutil.MachineSetDescription{"machineset-41804", 0}
		ms.CreateMachineSet(oc)
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(mapiMachineset, "machineset-41804", "-n", "openshift-machine-api", "-p", `{"spec":{"replicas":1,"template":{"spec":{"providerSpec":{"value":{"spotVMOptions":{}}}}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		exutil.WaitForMachinesRunning(oc, 1, "machineset-41804")

		g.By("Check machine and node were labelled `interruptible-instance`")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachine, "-n", machineAPINamespace, "-l", "machine.openshift.io/interruptible-instance=").Output()
		o.Expect(machine).NotTo(o.BeEmpty())
		node, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "machine.openshift.io/interruptible-instance=").Output()
		o.Expect(node).NotTo(o.BeEmpty())
	})

	// author: zhsun@redhat.com
	g.It("Longduration-NonPreRelease-PstChkUpgrade-Author:zhsun-Medium-41804-[Upgrade]Spot/preemptible instances should not block upgrade - Azure [Disruptive]", func() {
		if iaasPlatform != "azure" {
			g.Skip("Skip this test scenario because it is not supported on the " + iaasPlatform + " platform")
		}
		randomMachinesetName := exutil.GetRandomMachineSetName(oc)
		region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachineset, randomMachinesetName, "-n", "openshift-machine-api", "-o=jsonpath={.spec.template.spec.providerSpec.value.location}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if region == "northcentralus" || region == "westus" || region == "usgovvirginia" {
			g.Skip("Skip this test scenario because it is not supported on the " + region + " region, because this region doesn't have zones")
		}
		ms := exutil.MachineSetDescription{"machineset-41804", 0}
		defer ms.DeleteMachineSet(oc)

		g.By("Check machine and node were still be labelled `interruptible-instance`")
		machine, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMachine, "-n", machineAPINamespace, "-l", "machine.openshift.io/interruptible-instance=").Output()
		o.Expect(machine).NotTo(o.BeEmpty())
		node, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "machine.openshift.io/interruptible-instance=").Output()
		o.Expect(node).NotTo(o.BeEmpty())
	})
})
