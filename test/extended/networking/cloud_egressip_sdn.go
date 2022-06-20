package networking

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var (
		ipEchoURL string
		a         *exutil.AwsClient

		oc = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		platform := exutil.CheckPlatform(oc)
		networkType := checkNetworkType(oc)
		e2e.Logf("\n\nThe platform is %v,  networkType is %v\n", platform, networkType)
		acceptedPlatform := strings.Contains(platform, "aws") || strings.Contains(platform, "gcp")
		if !acceptedPlatform || !strings.Contains(networkType, "sdn") {
			g.Skip("Test cases should be run on AWS or GCP cluster with Openshift-SDN network plugin, skip for other platforms or other network plugin!!")
		}

		switch platform {
		case "aws":
			e2e.Logf("\n AWS is detected, running the case on AWS\n")
			if ipEchoURL == "" {
				getAwsCredentialFromCluster(oc)
				a = exutil.InitAwsSession()
				_, err := getAwsIntSvcInstanceID(a, oc)
				if err != nil {
					e2e.Logf("There is no int svc instance in this cluster, %v", err)
					g.Skip("There is no int svc instance in this cluster, skip the cases!!")
				}

				ipEchoURL, err = installIPEchoServiceOnAWS(a, oc)
				if err != nil {
					e2e.Logf("No ip-echo service installed on the bastion host, %v", err)
					g.Skip("No ip-echo service installed on the bastion host, skip the cases!!")
				}
			}
		case "gcp":
			e2e.Logf("\n GCP is detected, running the case on GCP\n")
			if ipEchoURL == "" {
				// If an int-svc instance with external IP found, IpEcho service will be installed on the int-svc instance
				// otherwise, just give error message and skip the test
				infraID, err := exutil.GetInfraId(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				host, err := getIntSvcExternalIPFromGcp(oc, infraID)
				if err != nil {
					e2e.Logf("There is no int svc instance in this cluster, %v", err)
					g.Skip("There is no int svc instance in this cluster, skip the cases!!")
				}
				ipEchoURL, err = installIPEchoServiceOnGCP(oc, infraID, host)
				if err != nil {
					e2e.Logf("No ip-echo service installed on the bastion host, %v", err)
					g.Skip("No ip-echo service installed on the bastion host, skip the cases!!")
				}
			}
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46701-The same egressIP will not be assigned to different netnamespace. [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")

		g.By("1. Pick a node as egressIP node, add egressCIDRs to it")
		// get CIDR on the node
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		sub1 := getIfaddrFromNode(egressNode, oc)

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressCIDRs\":[\""+sub1+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressCIDRs\":[]}")

		g.By("2. Create first namespace then create a test pod in it")
		oc.SetupProject()
		ns1 := oc.Namespace()

		pod1 := pingPodResource{
			name:      "hello-pod1",
			namespace: ns1,
			template:  pingPodTemplate,
		}

		pod1.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod1.name, "-n", pod1.namespace).Execute()
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("3. Find an unused IP on the node, use it as egressIP address, add it to netnamespace of the first project")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode, sub1, 1)
		o.Expect(len(freeIPs) == 1).Should(o.BeTrue())

		patchResourceAsAdmin(oc, "netnamespace/"+pod1.namespace, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+pod1.namespace, "{\"egressIPs\":[]}")

		g.By("4. Verify egressCIDRs and egressIPs on the node")
		output := getEgressCIDRs(oc, egressNode)
		o.Expect(output).To(o.ContainSubstring(sub1))
		ip, err := getEgressIPonSDNHost(oc, nodeList.Items[0].Name, 1)
		e2e.Logf("\n\n\n got egressIP as -->%v<--\n\n\n", ip)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(freeIPs[0]).Should(o.BeElementOf(ip))

		g.By("4.From first namespace, check source IP is EgressIP")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIPs[0]))

		g.By("5. Create second namespace, create a test pod in it")
		oc.SetupProject()
		ns2 := oc.Namespace()

		pod2 := pingPodResource{
			name:      "hello-pod2",
			namespace: ns2,
			template:  pingPodTemplate,
		}

		pod2.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod2.name, "-n", pod2.namespace).Execute()
		waitPodReady(oc, pod2.namespace, pod2.name)

		g.By("6. Add same egressIP address to netnamespace of the second project")
		patchResourceAsAdmin(oc, "netnamespace/"+pod2.namespace, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+pod2.namespace, "{\"egressIPs\":[]}")

		g.By("7. check egressIP again after second netnamespace is patched with same egressIP")
		ip, err = getEgressIPonSDNHost(oc, nodeList.Items[0].Name, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(freeIPs[0]).Should(o.BeElementOf(ip))

		g.By("8.Check source IP again from first project, curl command should not succeed, error is expected")
		_, err = e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).To(o.HaveOccurred())

		g.By("9.Check source IP again from second project, curl command should not succeed, error is expected")
		_, err = e2e.RunHostCmd(pod2.namespace, pod2.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).To(o.HaveOccurred())
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46709-Master balance egressIPs across nodes when there are multiple nodes handling egressIP. [Disruptive]", func() {
		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet, add egressCIDRs to them")
		var egressNode1, egressNode2 string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		sub := getIfaddrFromNode(egressNode1, oc)

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[]}")

		g.By("2. Find 6 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 6)
		o.Expect(len(freeIPs) == 6).Should(o.BeTrue())

		g.By("3. Create 6 namespaces, apply one egressIP from the freeIPs to each namespace")
		var ns = [6]string{}
		for i := 0; i < 6; i++ {
			oc.SetupProject()
			ns[i] = oc.Namespace()
			patchResourceAsAdmin(oc, "netnamespace/"+ns[i], "{\"egressIPs\":[\""+freeIPs[i]+"\"]}")
			defer patchResourceAsAdmin(oc, "netnamespace/"+ns[i], "{\"egressIPs\":[]}")
		}

		g.By("4. check egressIP for each node, each node should have 3 egressIP addresses assigned from the pool")
		ip1, err := getEgressIPonSDNHost(oc, egressNode1, 3)
		o.Expect(err).NotTo(o.HaveOccurred())
		ip2, err := getEgressIPonSDNHost(oc, egressNode2, 3)
		o.Expect(err).NotTo(o.HaveOccurred())
		for i := 0; i < 3; i++ {
			o.Expect(ip1[i]).Should(o.BeElementOf(freeIPs))
			o.Expect(ip2[i]).Should(o.BeElementOf(freeIPs))
		}
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46554-[Automatic EgressIP] no more than one egress IP per node for each namespace. [Disruptive]", func() {
		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet, add egressCIDRs to them")
		var egressNode1, egressNode2 string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		sub := getIfaddrFromNode(egressNode1, oc)

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[]}")

		g.By("2. Find 3 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 3)
		o.Expect(len(freeIPs) == 3).Should(o.BeTrue())

		g.By("3. Create one namespace, apply 3 egressIP to the namespace")
		oc.SetupProject()
		ns := oc.Namespace()
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\",\""+freeIPs[1]+"\",\""+freeIPs[2]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		g.By("4. Check egressIP for each node, each node should have 1 egressIP addresses assigned from the pool")
		ip1, err := getEgressIPonSDNHost(oc, egressNode1, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		ip2, err := getEgressIPonSDNHost(oc, egressNode2, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip1[0]).Should(o.BeElementOf(freeIPs))
		o.Expect(ip2[0]).Should(o.BeElementOf(freeIPs))
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46556-[Automatic EgressIP] A pod that is on a node hosting egressIP, it will always use the egressIP of the node . [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet, add egressCIDRs to them")
		var egressNode1, egressNode2 string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		sub := getIfaddrFromNode(egressNode1, oc)

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[]}")

		g.By("2. Find 2 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 2)
		o.Expect(len(freeIPs) == 2).Should(o.BeTrue())

		g.By("3. Create a namespaces, apply the both egressIPs to the namespace, and create a test pod on the egress node")

		oc.SetupProject()
		ns := oc.Namespace()
		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  egressNode1,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\", \""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		// check the source IP that the test pod uses
		g.By("4. get egressIP on the node where test pod resides")
		ip, err := getEgressIPonSDNHost(oc, egressNode1, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip[0]).Should(o.BeElementOf(freeIPs))

		g.By("5.Check source IP from the test pod for 5 times, it should always use the egressIP of the egressNode that it resides on")
		for i := 0; i < 5; i++ {
			sourceIP, err := e2e.RunHostCmd(ns, podns.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
			e2e.Logf("sourceIP is %v", sourceIP)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sourceIP).Should(o.BeElementOf(ip))
		}
	})

	// author: jechen@redhat.com
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jechen-High-46705-The egressIP should still work fine after the node or network service restarted. [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")

		g.By("1. Pick a node as egressIP node")
		// get CIDR on the node
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		sub1 := getIfaddrFromNode(egressNode, oc)

		g.By("2. Create first namespace then create a test pod in it")
		oc.SetupProject()
		ns1 := oc.Namespace()

		pod1 := pingPodResource{
			name:      "hello-pod1",
			namespace: ns1,
			template:  pingPodTemplate,
		}

		pod1.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod1.name, "-n", pod1.namespace).Execute()
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("3. Find an unused IP on the node, use it as egressIP address, add it to the egress node and netnamespace of the project")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode, sub1, 1)
		o.Expect(len(freeIPs) == 1).Should(o.BeTrue())

		patchResourceAsAdmin(oc, "netnamespace/"+pod1.namespace, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+pod1.namespace, "{\"egressIPs\":[]}")

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressIPs\":[]}")

		g.By("4. Verify egressIPs on the node")
		ip, err := getEgressIPonSDNHost(oc, nodeList.Items[0].Name, 1)
		e2e.Logf("\n\n\n got egressIP as -->%v<--\n\n\n", ip)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(freeIPs[0]).Should(o.BeElementOf(ip))

		g.By("5.From the namespace, check source IP is EgressIP")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIPErr := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
			if !contains(freeIPs, sourceIP) || err != nil {
				e2e.Logf("\n got sourceIP as %v while egressIP is %v, or got the error: %v\n", sourceIP, ip, err)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(sourceIPErr, fmt.Sprintf("Failed to get sourceIP:%s", sourceIPErr))

		g.By("6.Reboot egress node.\n")
		defer checkNodeStatus(oc, egressNode, "Ready")
		rebootNode(oc, egressNode)
		checkNodeStatus(oc, egressNode, "NotReady")
		checkNodeStatus(oc, egressNode, "Ready")

		g.By("7.check source IP is EgressIP again")
		sourceIPErr = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
			if !contains(freeIPs, sourceIP) || err != nil {
				e2e.Logf("\n got sourceIP as %v while egressIP is %v, or got the error: %v\n", sourceIP, ip, err)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(sourceIPErr, fmt.Sprintf("Failed to get sourceIP:%s", sourceIPErr))
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46555-Medium-46962-[Automatic EgressIP] Random egressIP is used on a pod that is not on a node hosting an egressIP, and random outages with egressIP . [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Identify two worker nodes with same subnet as egressIP nodes, pick a third node as non-egressIP node")
		var egressNode1, egressNode2, nonEgressNode string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 || len(nodeList.Items) < 3 {
			g.Skip("Need at least 3 worker nodes, the prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]
		for i := 0; i < len(nodeList.Items); i++ {
			if nodeList.Items[i].Name != egressNode1 && nodeList.Items[i].Name != egressNode2 {
				nonEgressNode = nodeList.Items[i].Name
				break
			}
		}
		if nonEgressNode == "" {
			g.Skip("Did not get a node that is not egressIP node, skip the case!!")
		}

		e2e.Logf("\nEgressNode1: %v\n", egressNode1)
		e2e.Logf("\nEgressNode2: %v\n", egressNode2)
		e2e.Logf("\nnonEgressNode: %v\n", nonEgressNode)

		g.By("2. Get subnet from egressIP node, add egressCIDRs to both egressIP nodes")
		sub := getIfaddrFromNode(egressNode1, oc)
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[]}")

		g.By("3. Find 2 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 2)
		o.Expect(len(freeIPs) == 2).Should(o.BeTrue())

		g.By("4. Create a namespaces, apply the both egressIPs to the namespace, but create a test pod on the third non-egressIP node")
		oc.SetupProject()
		ns := oc.Namespace()
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\", \""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  nonEgressNode,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		g.By("5.Check SDN pod log of the non-egress node, there should be no 'may be offline' error log ")
		sdnPodName, err := exutil.GetPodName(oc, "openshift-sdn", "app=sdn", nonEgressNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sdnPodName).NotTo(o.BeEmpty())
		podlogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(sdnPodName, "-n", "openshift-sdn", "-c", "sdn").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podlogs).NotTo(o.ContainSubstring("may be offline"))

		g.By("6.Check source IP from the test pod for 10 times, it should use either egressIP address as its sourceIP")
		sourceIP, err := execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs[1]))
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46557-[Manual EgressIP] Random egressIP is used on a pod that is not on a node hosting an egressIP . [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Identify two worker nodes with same subnet as egressIP nodes, pick a third node as non-egressIP node")
		var egressNode1, egressNode2, nonEgressNode string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 || len(nodeList.Items) < 3 {
			g.Skip("Need at least 3 worker nodes, the prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]
		for i := 0; i < len(nodeList.Items); i++ {
			if nodeList.Items[i].Name != egressNode1 && nodeList.Items[i].Name != egressNode2 {
				nonEgressNode = nodeList.Items[i].Name
				break
			}
		}
		if nonEgressNode == "" {
			g.Skip("Did not get a node that is not egressIP node, skip the case!!")
		}

		e2e.Logf("\nEgressNode1: %v\n", egressNode1)
		e2e.Logf("\nEgressNode2: %v\n", egressNode2)
		e2e.Logf("\nnonEgressNode: %v\n", nonEgressNode)

		g.By("2. Find 2 unused IPs from one egress node")
		sub := getIfaddrFromNode(egressNode1, oc)
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 2)
		o.Expect(len(freeIPs) == 2).Should(o.BeTrue())

		g.By("3. Patch egressIP address to egressIP nodes")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressIPs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressIPs\":[\""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressIPs\":[]}")

		g.By("4. Create a namespaces, apply the both egressIPs to the namespace, but create a test pod on the third non-egressIP node")
		oc.SetupProject()
		ns := oc.Namespace()
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\", \""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  nonEgressNode,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		g.By("5.Check source IP from the test pod for 10 times, it should use either egressIP address as its sourceIP")
		sourceIP, err := execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs[1]))
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-High-46558-[Manual EgressIP] A pod that is on a node hosting egressIP, it will always use the egressIP of the node . [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet, add egressCIDRs to them")
		var egressNode1, egressNode2 string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		sub := getIfaddrFromNode(egressNode1, oc)

		g.By("2. Find 2 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 2)
		o.Expect(len(freeIPs) == 2).Should(o.BeTrue())

		g.By("3. Patch egressIP address to egressIP nodes")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressIPs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressIPs\":[\""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressIPs\":[]}")

		g.By("3. Create a namespaces, apply the both egressIPs to the namespace, and create a test pod on the egress node")

		oc.SetupProject()
		ns := oc.Namespace()
		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  egressNode1,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\", \""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		// check the source IP that the test pod uses
		g.By("4. get egressIP on the node where test pod resides")
		ip, err := getEgressIPonSDNHost(oc, egressNode1, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip[0]).Should(o.BeElementOf(freeIPs))

		g.By("5.Check source IP from the test pod for 10 times, it should always use the egressIP of the egressNode that it resides on")
		sourceIP, err := execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs[0]))
		o.Expect(sourceIP).ShouldNot(o.ContainSubstring(freeIPs[1]))
	})

	// author: jechen@redhat.com
	g.It("ConnectedOnly-Author:jechen-Medium-46963-Should remove the egressIP from the array if it was not being used. [Disruptive]", func() {
		g.By("1. Pick a node as egressIP node, add egressCIDRs to it")
		// get CIDR on the node
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		sub := getIfaddrFromNode(egressNode, oc)

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressCIDRs\":[]}")

		g.By("2. Find 5 unused IPs from the egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode, sub, 5)
		o.Expect(len(freeIPs) == 5).Should(o.BeTrue())

		g.By("3. Create a namespace, patch one egressIP from the freeIPs to the netnamespace, repeat 5 times, replaces the egressIP each time")
		oc.SetupProject()
		ns := oc.Namespace()

		for i := 0; i < 5; i++ {
			patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[i]+"\"]}")
			defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")
		}

		g.By("4. check egressIP for the node, it should have only the last egressIP from the freeIPs as its egressIP address")
		ip, err := getEgressIPonSDNHost(oc, egressNode, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip[0]).Should(o.BeElementOf(freeIPs[4]))
		for i := 0; i < 4; i++ {
			o.Expect(ip[0]).ShouldNot(o.BeElementOf(freeIPs[i]))
		}
	})

	// author: jechen@redhat.com
	g.It("NonPreRelease-ConnectedOnly-Author:jechen-High-47054-The egressIP can be HA if netnamespace has single egressIP . [Disruptive][Slow]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet, add egressCIDRs to them")
		var egressNode1, egressNode2, nonEgressNode string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		// find a node that is not egress node
		for i := 0; i < len(nodeList.Items); i++ {
			if nodeList.Items[i].Name != egressNode1 && nodeList.Items[i].Name != egressNode2 {
				nonEgressNode = nodeList.Items[i].Name
				break
			}
		}
		if nonEgressNode == "" {
			g.Skip("Did not get a node that is not egressIP node, skip the case!!")
		}
		e2e.Logf("\n non-egress node: %v\n", nonEgressNode)

		g.By("2. Get subnet from egressIP node, add egressCIDRs to both egressIP nodes")
		sub := getIfaddrFromNode(egressNode1, oc)
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode1, "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode2, "{\"egressCIDRs\":[]}")

		g.By("3. Find 1 unused IPs from one egress node")
		freeIPs := findUnUsedIPsOnNode(oc, egressNode1, sub, 1)
		o.Expect(len(freeIPs) == 1).Should(o.BeTrue())

		g.By("4. Create a namespaces, apply the egressIP to the namespace, and create a test pod on a non-egress node")
		oc.SetupProject()
		ns := oc.Namespace()

		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  nonEgressNode,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", podns.name, "-n", podns.namespace).Execute()
		waitPodReady(oc, podns.namespace, podns.name)

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		g.By("5. Find the host that is assigned the egressIP address")
		foundHost := SDNHostwEgressIP(oc, egressNodes, freeIPs[0])
		e2e.Logf("\n\n\n foundHost that has the egressIP: %v\n\n\n", foundHost)
		o.Expect(foundHost).NotTo(o.BeEmpty())

		g.By("6. reboot the host that has egressIP address")
		defer checkNodeStatus(oc, foundHost, "Ready")
		rebootNode(oc, foundHost)

		g.By("7. Get the egressNode that does not have egressIP address assigned previously")
		// remove the host with egressIP address from the egress node list, get the egressNode that does not have egressIP address assigned
		var hostLeft []string
		for i, v := range egressNodes {
			if v == foundHost {
				hostLeft = append(egressNodes[:i], egressNodes[i+1:]...)
				break
			}
		}
		e2e.Logf("\n Get the egressNode that did not have egressIP address previously: %v\n\n\n", hostLeft)

		g.By("8. check if egressIP address is moved to the other egressIP node after original host is rebooted")
		ip, err := getEgressIPonSDNHost(oc, hostLeft[0], 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip[0]).Should(o.BeElementOf(freeIPs))

		g.By("9.From the namespace, check source IP is egressIP address")
		sourceIP, err := e2e.RunHostCmd(podns.namespace, podns.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		if !contains(freeIPs, sourceIP) || err != nil {
			err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
				sourceIP, err := e2e.RunHostCmd(podns.namespace, podns.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
				if !contains(freeIPs, sourceIP) || err != nil {
					e2e.Logf("\n got sourceIP as %v while egressIP is %v, or got the error: %v\n, try again", sourceIP, ip, err)
					return false, nil
				}
				return true, nil
			})
		}
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed to get sourceIP:%s", err))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIPs[0]))
	})

	// author: jechen@redhat.com
	g.It("NonPreRelease-ConnectedOnly-Author:jechen-High-46559-[Automatic EgressIP] If some egress node is unavailable, pods continue use other available egressIPs after a short delay. [Disruptive][Slow]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Get list of nodes, get subnet as egressCIDR and an unused ip address from each node that have same subnet, add egressCIDR to each egress node")
		var egressNodes []string
		nodeNum := 4
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < nodeNum {
			g.Skip("Not enough worker nodes for this test, skip the case!!")
		}

		// choose first 3 nodes as egress nodes
		for i := 0; i < (nodeNum - 1); i++ {
			egressNodes = append(egressNodes, nodeList.Items[i].Name)
		}
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		sub2 := getIfaddrFromNode(nodeList.Items[1].Name, oc)
		sub3 := getIfaddrFromNode(nodeList.Items[2].Name, oc)
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[0], "{\"egressCIDRs\":[\""+sub1+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[0], "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[1], "{\"egressCIDRs\":[\""+sub2+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[1], "{\"egressCIDRs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[2], "{\"egressCIDRs\":[\""+sub3+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[2], "{\"egressCIDRs\":[]}")

		// Find 1 unused IPs from each egress node
		freeIPs1 := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub1, 1)
		o.Expect(len(freeIPs1) == 1).Should(o.BeTrue())
		freeIPs2 := findUnUsedIPsOnNode(oc, nodeList.Items[1].Name, sub2, 1)
		o.Expect(len(freeIPs2) == 1).Should(o.BeTrue())
		freeIPs3 := findUnUsedIPsOnNode(oc, nodeList.Items[2].Name, sub3, 1)
		o.Expect(len(freeIPs3) == 1).Should(o.BeTrue())

		g.By("2. Create a namespaces, patch all egressIPs to the namespace, and create a test pod on a non-egress node")
		oc.SetupProject()
		ns := oc.Namespace()
		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  nodeList.Items[nodeNum-1].Name,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs1[0]+"\", \""+freeIPs2[0]+"\", \""+freeIPs3[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		g.By("3.Check source IP from the test pod for 10 times, it should use any egressIP address as its sourceIP")
		sourceIP, err := execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n Get sourceIP as: %v\n", sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs3[0]))

		g.By("4. Find the host that is assigned the first egressIP address")
		foundHost := SDNHostwEgressIP(oc, egressNodes, freeIPs1[0])
		e2e.Logf("\n\n\n foundHost that has the egressIP: %v\n\n\n", foundHost)
		o.Expect(foundHost).NotTo(o.BeEmpty())

		g.By("5. Get the zone info for the host, shutdown the host that has the first egressIP address")
		var instance []string
		var zone string
		switch exutil.CheckPlatform(oc) {
		case "aws":
			e2e.Logf("\n AWS is detected \n")
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			defer startInstanceOnAWS(a, nodeList.Items[1].Name)
			stopInstanceOnAWS(a, nodeList.Items[1].Name)
			checkNodeStatus(oc, nodeList.Items[1].Name, "NotReady")
		case "gcp":
			// for gcp, remove the postfix "c.openshift-qe.internal" to get its instance name
			instance = strings.Split(foundHost, ".")
			e2e.Logf("\n\n\n the worker node to be shutdown is: %v\n\n\n", instance[0])
			infraID, err := exutil.GetInfraId(oc)
			zone, err = getZoneOfInstanceFromGcp(oc, infraID, instance[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			defer checkNodeStatus(oc, foundHost, "Ready")
			defer startInstanceOnGcp(oc, instance[0], zone)
			err = stopInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, foundHost, "NotReady")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

		g.By("6.From the namespace, check source IP is egressIP address")
		sourceIP, err = execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n Get sourceIP as: %v\n", sourceIP)
		o.Expect(sourceIP).ShouldNot(o.ContainSubstring(freeIPs1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs3[0]))

		g.By("7. Bring the host back up")
		switch exutil.CheckPlatform(oc) {
		case "aws":
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			startInstanceOnAWS(a, nodeList.Items[1].Name)
			checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
		case "gcp":
			defer checkNodeStatus(oc, foundHost, "Ready")
			err = startInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, foundHost, "Ready")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

	})

	// author: jechen@redhat.com
	g.It("NonPreRelease-ConnectedOnly-Author:jechen-High-46561-[Manual EgressIP] If some egress node is unavailable, pods continue use other available egressIPs after a short delay. [Disruptive][Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")

		g.By("1. Get list of nodes, get subnet and an unused ip address from each node, add egressIP to each egress node")
		var egressNodes []string
		nodeNum := 4
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < nodeNum {
			g.Skip("Not enough worker nodes for this test, skip the case!!")
		}

		// choose first 3 nodes as egress nodes
		for i := 0; i < (nodeNum - 1); i++ {
			egressNodes = append(egressNodes, nodeList.Items[i].Name)
		}
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		sub2 := getIfaddrFromNode(nodeList.Items[1].Name, oc)
		sub3 := getIfaddrFromNode(nodeList.Items[2].Name, oc)

		// Find 1 unused IPs from each egress node
		freeIPs1 := findUnUsedIPsOnNode(oc, egressNodes[0], sub1, 1)
		o.Expect(len(freeIPs1) == 1).Should(o.BeTrue())
		freeIPs2 := findUnUsedIPsOnNode(oc, egressNodes[1], sub2, 1)
		o.Expect(len(freeIPs2) == 1).Should(o.BeTrue())
		freeIPs3 := findUnUsedIPsOnNode(oc, egressNodes[2], sub3, 1)
		o.Expect(len(freeIPs3) == 1).Should(o.BeTrue())

		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[0], "{\"egressIPs\":[\""+freeIPs1[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[0], "{\"egressIPs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[1], "{\"egressIPs\":[\""+freeIPs2[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[1], "{\"egressIPs\":[]}")
		patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[2], "{\"egressIPs\":[\""+freeIPs3[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNodes[2], "{\"egressIPs\":[]}")

		g.By("2. Create a namespaces, patch all egressIPs to the namespace, and create a test pod on a non-egress node")
		oc.SetupProject()
		ns := oc.Namespace()
		podns := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			nodename:  nodeList.Items[nodeNum-1].Name,
			template:  pingPodNodeTemplate,
		}
		podns.createPingPodNode(oc)
		waitPodReady(oc, podns.namespace, podns.name)

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs1[0]+"\", \""+freeIPs2[0]+"\", \""+freeIPs3[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		g.By("3.Check source IP from the test pod for 10 times, it should use any egressIP address as its sourceIP")
		sourceIP, err := execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n Get sourceIP as: %v\n", sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs3[0]))

		g.By("4. Find the host that is assigned the first egressIP address")
		foundHost := SDNHostwEgressIP(oc, egressNodes, freeIPs1[0])
		e2e.Logf("\n\n\n foundHost that has the egressIP: %v\n\n\n", foundHost)
		o.Expect(foundHost).NotTo(o.BeEmpty())

		g.By("5. Get the zone info for the host, shutdown the host that has the first egressIP address")
		var instance []string
		var zone string
		switch exutil.CheckPlatform(oc) {
		case "aws":
			e2e.Logf("\n AWS is detected \n")
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			defer startInstanceOnAWS(a, nodeList.Items[1].Name)
			stopInstanceOnAWS(a, nodeList.Items[1].Name)
			checkNodeStatus(oc, nodeList.Items[1].Name, "NotReady")
		case "gcp":
			// for gcp, remove the postfix "c.openshift-qe.internal" to get its instance name
			instance = strings.Split(foundHost, ".")
			e2e.Logf("\n\n\n the worker node to be shutdown is: %v\n\n\n", instance[0])
			infraID, err := exutil.GetInfraId(oc)
			zone, err = getZoneOfInstanceFromGcp(oc, infraID, instance[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			defer checkNodeStatus(oc, foundHost, "Ready")
			defer startInstanceOnGcp(oc, instance[0], zone)
			err = stopInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, foundHost, "NotReady")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

		g.By("6.From the namespace, check source IP is egressIP address")
		sourceIP, err = execCommandInSpecificPod(oc, podns.namespace, podns.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n Get sourceIP as: %v\n", sourceIP)
		o.Expect(sourceIP).ShouldNot(o.ContainSubstring(freeIPs1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIPs3[0]))

		g.By("7. Bring the host back up")
		switch exutil.CheckPlatform(oc) {
		case "aws":
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			startInstanceOnAWS(a, nodeList.Items[1].Name)
			checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
		case "gcp":
			defer checkNodeStatus(oc, foundHost, "Ready")
			err = startInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, foundHost, "Ready")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}
	})

	// author: jechen@redhat.com
	g.It("NonPreRelease-ConnectedOnly-Author:jechen-High-47455-The egressIP could be assigned to project automatically once it is defined in hostsubnet egressCIDR. [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")

		g.By("1. Get list of nodes, use the first node as egressNode, get subnet and an unused ip address from the node, apply egressCIDRs to the nod")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		sub := getIfaddrFromNode(nodeList.Items[0].Name, oc)

		// Find 3 unused IPs from the egress node
		freeIPs := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub, 3)
		o.Expect(len(freeIPs) == 3).Should(o.BeTrue())

		patchResourceAsAdmin(oc, "hostsubnet/"+nodeList.Items[0].Name, "{\"egressCIDRs\":[\""+sub+"\"]}")
		defer patchResourceAsAdmin(oc, "hostsubnet/"+nodeList.Items[0].Name, "{\"egressCIDRs\":[]}")

		g.By("2. Create a namespaces, patch the first egressIP to the namespace, create a test pod in the namespace")
		oc.SetupProject()
		ns := oc.Namespace()

		pod := pingPodResource{
			name:      "hello-pod1",
			namespace: ns,
			template:  pingPodTemplate,
		}

		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[0]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		g.By("3.Check the first egressIP is added to node's primary NIC, verify source IP from this namespace is the first EgressIP")
		checkPrimaryNIC(oc, nodeList.Items[0].Name, freeIPs[0], true)

		pod.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod.name, "-n", pod.namespace).Execute()
		waitPodReady(oc, pod.namespace, pod.name)

		var expectedEgressIP = []string{freeIPs[0]}
		checkEgressIPonSDNHost(oc, nodeList.Items[0].Name, expectedEgressIP)

		sourceIP, err := e2e.RunHostCmd(pod.namespace, pod.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		e2e.Logf("\n Get sourceIP as %v \n", sourceIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIPs[0]))

		g.By("4.Unpatch egressIP to the namespace, Check egressIP is removed from node's primary NIC, verify source IP from this namespace is node's IP address")
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		checkPrimaryNIC(oc, nodeList.Items[0].Name, freeIPs[0], false)

		PodNodeName, err := exutil.GetPodNodeName(oc, pod.namespace, pod.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeIP := getNodeIPv4(oc, ns, PodNodeName)
		sourceIP, err = e2e.RunHostCmd(pod.namespace, pod.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		e2e.Logf("\n Get sourceIP as %v \n", sourceIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(nodeIP))

		g.By("5.Patch the second egressIP to the namespace, verify it is added to node's primary NIC, verify source IP from this namespace is the second EgressIP now")
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[1]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		expectedEgressIP = []string{freeIPs[1]}
		checkEgressIPonSDNHost(oc, nodeList.Items[0].Name, expectedEgressIP)

		checkPrimaryNIC(oc, nodeList.Items[0].Name, freeIPs[1], true)

		sourceIP, err = e2e.RunHostCmd(pod.namespace, pod.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		e2e.Logf("\n Get sourceIP as %v \n", sourceIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIPs[1]))

		g.By("6.Patch the third egressIP to the namespace, verify it is added to node's primary NIC, verify source IP from this namespace is the third EgressIP now")
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+freeIPs[2]+"\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		expectedEgressIP = []string{freeIPs[2]}
		checkEgressIPonSDNHost(oc, nodeList.Items[0].Name, expectedEgressIP)

		checkPrimaryNIC(oc, nodeList.Items[0].Name, freeIPs[2], true)

		sourceIP, err = e2e.RunHostCmd(pod.namespace, pod.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		e2e.Logf("\n Get sourceIP as %v \n", sourceIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIPs[2]))

		g.By("7.Patch the namespace with a private IP that is definitely not within the CIDR, verify it is not added to node's primary NIC, curl command should not succeed, error is expected ")
		patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\"192.168.1.100\"]}")
		defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

		checkPrimaryNIC(oc, nodeList.Items[0].Name, "192.168.1.100", false)

		_, err = e2e.RunHostCmd(pod.namespace, pod.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).To(o.HaveOccurred())
	})
})
