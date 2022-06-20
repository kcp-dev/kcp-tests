package networking

import (
	"net"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())

	g.BeforeEach(func() {
		platform := checkPlatform(oc)
		if !strings.Contains(platform, "vsphere") {
			g.Skip("Skip for un-expected platform,not vsphere!")
		}
	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-33633-EgressIP works well with EgressFirewall. [Serial]", func() {
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip for not ovn cluster !!!")
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")
		egressFWTemplate := filepath.Join(buildPruningBaseDir, "egressfirewall1-template.yaml")

		g.By("create new namespace")
		oc.SetupProject()

		g.By("Label EgressIP node")
		var EgressNodeLabel = "k8s.ovn.org/egress-assignable"
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply EgressLabel Key for this test on one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel)

		g.By("Apply label to namespace")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name=test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name-").Output()

		g.By("Create an egressip object")
		sub1, _ := getDefaultSubnet(oc)
		ips := findUnUsedIPs(oc, sub1, 2)
		egressip1 := egressIPResource1{
			name:      "egressip-33633",
			template:  egressIPTemplate,
			egressIP1: ips[0],
			egressIP2: ips[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("Create an EgressFirewall object.")
		egressFW1 := egressFirewall1{
			name:      "default",
			namespace: oc.Namespace(),
			template:  egressFWTemplate,
		}
		egressFW1.createEgressFWObject1(oc)
		defer egressFW1.deleteEgressFWObject1(oc)

		g.By("Create a pod ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: oc.Namespace(),
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)
		defer pod1.deletePingPod(oc)

		g.By("Check source IP is EgressIP")
		sourceIp, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoServer()+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIp).Should(o.BeElementOf(ips))

		g.By("Check www.test.com is blocked")
		_, err = e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s www.test.com --connect-timeout 5")
		o.Expect(err).To(o.HaveOccurred())

		g.By("EgressIP works well with EgressFirewall!!! ")

	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-49161-[Bug 2014202] Service IP should be accessed when egressIP set to the namespace. [Disruptive]", func() {
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip for not ovn cluster !!!")
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		testPodFile := filepath.Join(buildPruningBaseDir, "testpod.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("1. Pick a node as egressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		var EgressNodeLabel = "k8s.ovn.org/egress-assignable"
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode, EgressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode, EgressNodeLabel)

		g.By("2. Create an egressip object.")
		sub1, err := getDefaultSubnet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		freeIps := findUnUsedIPs(oc, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-49161",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("3. Create first namespace then create svc/pods in it")
		oc.SetupProject()
		ns := oc.Namespace()
		createResourceFromFile(oc, ns, testPodFile)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")

		g.By("4. Apply label to namespace matched EgressIP object.")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns, "name=test").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns, "name-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5. Get svc in the namespace")
		svc_url := net.JoinHostPort(getSvcIPv4(oc, ns, "test-service"), "27017")

		g.By("6. Service IP should be accessed from one node.")
		masterNode, err := exutil.GetFirstMasterNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		stdout, err := exutil.DebugNode(oc, masterNode, "curl", svc_url, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(stdout)
		o.Expect(stdout).Should(o.ContainSubstring("Hello OpenShift"))
	})

})
