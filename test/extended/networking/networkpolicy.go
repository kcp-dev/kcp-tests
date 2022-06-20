package networking

import (
	"net"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-networkpolicy", exutil.KubeConfigPath())

	// author: zzhao@redhat.com
	g.It("Author:zzhao-Critical-49076-service domain can be resolved when egress type is enabled", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking")
			testPodFile         = filepath.Join(buildPruningBaseDir, "testpod.yaml")
			helloSdnFile        = filepath.Join(buildPruningBaseDir, "hellosdn.yaml")
			egressTypeFile      = filepath.Join(buildPruningBaseDir, "networkpolicy/egress-allow-all.yaml")
			ingressTypeFile     = filepath.Join(buildPruningBaseDir, "networkpolicy/ingress-allow-all.yaml")
		)
		g.By("create new namespace")
		oc.SetupProject()

		g.By("create test pods")
		createResourceFromFile(oc, oc.Namespace(), testPodFile)
		createResourceFromFile(oc, oc.Namespace(), helloSdnFile)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=hellosdn")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=hellosdn not ready")

		g.By("create egress and ingress type networkpolicy")
		createResourceFromFile(oc, oc.Namespace(), egressTypeFile)
		output, err := oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("allow-all-egress"))
		createResourceFromFile(oc, oc.Namespace(), ingressTypeFile)
		output, err = oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("allow-all-ingress"))

		g.By("check hellosdn pods can reolsve the dns after apply the networkplicy")
		helloSdnName := getPodName(oc, oc.Namespace(), "name=hellosdn")
		digOutput, err := e2e.RunHostCmd(oc.Namespace(), helloSdnName[0], "dig kubernetes.default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(digOutput).Should(o.ContainSubstring("Got answer"))
		o.Expect(digOutput).ShouldNot(o.ContainSubstring("connection timed out"))

		g.By("check test-pods can reolsve the dns after apply the networkplicy")
		testPodName := getPodName(oc, oc.Namespace(), "name=test-pods")
		digOutput, err = e2e.RunHostCmd(oc.Namespace(), testPodName[0], "dig kubernetes.default")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(digOutput).Should(o.ContainSubstring("Got answer"))
		o.Expect(digOutput).ShouldNot(o.ContainSubstring("connection timed out"))

	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-Critical-49186-[Bug 2035336] Networkpolicy egress rule should work for statefulset pods.", func() {
		var (
			buildPruningBaseDir  = exutil.FixturePath("testdata", "networking")
			testPodFile          = filepath.Join(buildPruningBaseDir, "testpod.yaml")
			helloStatefulsetFile = filepath.Join(buildPruningBaseDir, "statefulset-hello.yaml")
			egressTypeFile       = filepath.Join(buildPruningBaseDir, "networkpolicy/allow-egress-red.yaml")
		)
		g.By("1. Create first namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("2. Create a statefulset pod in first namespace.")
		createResourceFromFile(oc, ns1, helloStatefulsetFile)
		err := waitForPodWithLabelReady(oc, ns1, "app=hello")
		exutil.AssertWaitPollNoErr(err, "this pod with label app=hello not ready")
		helloPodName := getPodName(oc, ns1, "app=hello")

		g.By("3. Create networkpolicy with egress rule in first namespace.")
		createResourceFromFile(oc, ns1, egressTypeFile)
		output, err := oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("allow-egress-to-red"))

		g.By("4. Create second namespace.")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("5. Create test pods in second namespace.")
		createResourceFromFile(oc, ns2, testPodFile)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")

		g.By("6. Add label to first test pod in second namespace.")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "team=qe").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		testPodName := getPodName(oc, ns2, "name=test-pods")
		err = exutil.LabelPod(oc, ns2, testPodName[0], "type=red")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("6. Get IP of the test pods in second namespace.")
		testPodIP1 := getPodIPv4(oc, ns2, testPodName[0])
		testPodIP2 := getPodIPv4(oc, ns2, testPodName[1])

		g.By("7. Check networkpolicy works.")
		output, err = e2e.RunHostCmd(ns1, helloPodName[0], "curl --connect-timeout 5 -s "+net.JoinHostPort(testPodIP1, "8080"))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("Hello OpenShift"))
		_, err = e2e.RunHostCmd(ns1, helloPodName[0], "curl --connect-timeout 5  -s "+net.JoinHostPort(testPodIP2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.ContainSubstring("exit status 28"))

		g.By("8. Delete statefulset pod for a couple of times.")
		for i := 0; i < 5; i++ {
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", helloPodName[0], "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err := waitForPodWithLabelReady(oc, ns1, "app=hello")
			exutil.AssertWaitPollNoErr(err, "this pod with label app=hello not ready")
		}

		g.By("9. Again checking networkpolicy works.")
		output, err = e2e.RunHostCmd(ns1, helloPodName[0], "curl --connect-timeout 5 -s "+net.JoinHostPort(testPodIP1, "8080"))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("Hello OpenShift"))
		_, err = e2e.RunHostCmd(ns1, helloPodName[0], "curl --connect-timeout 5 -s "+net.JoinHostPort(testPodIP2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.ContainSubstring("exit status 28"))

	})

	// author: anusaxen@redhat.com
	g.It("Author:anusaxen-High-49437-[BZ 2037647] Ingress network policy shouldn't be overruled by egress network policy on another pod", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking")
			egressTypeFile      = filepath.Join(buildPruningBaseDir, "networkpolicy/default-allow-egress.yaml")
			ingressTypeFile     = filepath.Join(buildPruningBaseDir, "networkpolicy/default-deny-ingress.yaml")
			pingPodNodeTemplate = filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")
		)
		g.By("Create first namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < 2 {
			g.Skip("This case requires 2 nodes, but the cluster has less than two nodes")
		}
		g.By("create a hello pod in first namespace")
		podns1 := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns1,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		podns1.createPingPodNode(oc)
		waitPodReady(oc, podns1.namespace, podns1.name)

		g.By("create default allow egress type networkpolicy in first namespace")
		createResourceFromFile(oc, ns1, egressTypeFile)
		output, err := oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("default-allow-egress"))

		g.By("Create Second namespace")
		oc.SetupProject()
		ns2 := oc.Namespace()
		g.By("create a hello-pod on 2nd namesapce on same node as first namespace")
		pod1Ns2 := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns2,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod1Ns2.createPingPodNode(oc)
		waitPodReady(oc, pod1Ns2.namespace, pod1Ns2.name)

		g.By("create another hello-pod on 2nd namesapce but on different node")
		pod2Ns2 := pingPodResourceNode{
			name:      "hello-pod-other-node",
			namespace: ns2,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod2Ns2.createPingPodNode(oc)
		waitPodReady(oc, pod2Ns2.namespace, pod2Ns2.name)

		helloPodNameNs2 := getPodName(oc, ns2, "name=hello-pod")

		g.By("create default deny ingress type networkpolicy in 2nd namespace")
		createResourceFromFile(oc, ns2, ingressTypeFile)
		output, err = oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("default-deny-ingress"))

		g.By("3. Get IP of the test pods in second namespace.")
		hellopodIP1Ns2 := getPodIPv4(oc, ns2, helloPodNameNs2[0])
		hellopodIP2Ns2 := getPodIPv4(oc, ns2, helloPodNameNs2[1])

		g.By("4. Curl both ns2 pods from ns1.")
		_, err = e2e.RunHostCmd(ns1, podns1.name, "curl --connect-timeout 5  -s "+net.JoinHostPort(hellopodIP1Ns2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.ContainSubstring("exit status 28"))
		_, err = e2e.RunHostCmd(ns1, podns1.name, "curl --connect-timeout 5  -s "+net.JoinHostPort(hellopodIP2Ns2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.ContainSubstring("exit status 28"))
	})

	// author: anusaxen@redhat.com
	g.It("Author:anusaxen-Medium-49686-network policy with ingress rule with ipBlock", func() {
		var (
			buildPruningBaseDir          = exutil.FixturePath("testdata", "networking")
			ipBlockIngressTemplateDual   = filepath.Join(buildPruningBaseDir, "networkpolicy/ipblock/ipBlock-ingress-dual-CIDRs-template.yaml")
			ipBlockIngressTemplateSingle = filepath.Join(buildPruningBaseDir, "networkpolicy/ipblock/ipBlock-ingress-single-CIDR-template.yaml")
			pingPodNodeTemplate          = filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")
		)

		ipStackType := checkIPStackType(oc)
		if ipStackType == "ipv4single" {
			g.Skip("This case requires dualstack or Single Stack Ipv6 cluster")
		}

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < 2 {
			g.Skip("This case requires 2 nodes, but the cluster has less than two nodes")
		}
		g.By("Create first namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("create 1st hello pod in ns1")
		pod1ns1 := pingPodResourceNode{
			name:      "hello-pod1",
			namespace: ns1,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod1ns1.createPingPodNode(oc)
		waitPodReady(oc, pod1ns1.namespace, pod1ns1.name)

		g.By("create 2nd hello pod in ns1")
		pod2ns1 := pingPodResourceNode{
			name:      "hello-pod2",
			namespace: ns1,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod2ns1.createPingPodNode(oc)
		waitPodReady(oc, pod2ns1.namespace, pod2ns1.name)

		g.By("create 3rd hello pod in ns1")
		pod3ns1 := pingPodResourceNode{
			name:      "hello-pod3",
			namespace: ns1,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod3ns1.createPingPodNode(oc)
		waitPodReady(oc, pod3ns1.namespace, pod3ns1.name)

		helloPod1ns1IPv6, helloPod1ns1IPv4 := getPodIP(oc, ns1, pod1ns1.name)
		helloPod1ns1IPv4WithCidr := helloPod1ns1IPv4 + "/32"
		helloPod1ns1IPv6WithCidr := helloPod1ns1IPv6 + "/128"

		if ipStackType == "dualstack" {
			g.By("create ipBlock Ingress Dual CIDRs Policy in ns1")
			npIPBlockNS1 := ipBlockIngressDual{
				name:      "ipblock-dual-cidrs-ingress",
				template:  ipBlockIngressTemplateDual,
				cidrIpv4:  helloPod1ns1IPv4WithCidr,
				cidrIpv6:  helloPod1ns1IPv6WithCidr,
				namespace: ns1,
			}
			npIPBlockNS1.createipBlockIngressObjectDual(oc)

			output, err := oc.Run("get").Args("networkpolicy").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("ipblock-dual-cidrs-ingress"))
		} else {
			npIPBlockNS1 := ipBlockIngressSingle{
				name:      "ipblock-single-cidr-ingress",
				template:  ipBlockIngressTemplateSingle,
				cidr:      helloPod1ns1IPv6WithCidr,
				namespace: ns1,
			}
			npIPBlockNS1.createipBlockIngressObjectSingle(oc)

			output, err := oc.Run("get").Args("networkpolicy").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("ipblock-single-cidr-ingress"))
		}
		g.By("Checking connectivity from pod1 to pod3")
		CurlPod2PodPass(oc, ns1, "hello-pod1", ns1, "hello-pod3")

		g.By("Checking connectivity from pod2 to pod3")
		CurlPod2PodFail(oc, ns1, "hello-pod2", ns1, "hello-pod3")

		g.By("Create 2nd namespace")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("create 1st hello pod in ns2")
		pod1ns2 := pingPodResourceNode{
			name:      "hello-pod1",
			namespace: ns2,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod1ns2.createPingPodNode(oc)
		waitPodReady(oc, pod1ns2.namespace, pod1ns2.name)

		g.By("create 2nd hello pod in ns2")
		pod2ns2 := pingPodResourceNode{
			name:      "hello-pod2",
			namespace: ns2,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod2ns2.createPingPodNode(oc)
		waitPodReady(oc, pod2ns2.namespace, pod2ns2.name)

		g.By("Checking connectivity from pod1ns2 to pod3ns1")
		CurlPod2PodFail(oc, ns2, "hello-pod1", ns1, "hello-pod3")

		g.By("Checking connectivity from pod2ns2 to pod1ns1")
		CurlPod2PodFail(oc, ns2, "hello-pod2", ns1, "hello-pod1")

		if ipStackType == "dualstack" {
			g.By("Delete networkpolicy from ns1")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("networkpolicy", "ipblock-dual-cidrs-ingress", "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			g.By("Delete networkpolicy from ns1")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("networkpolicy", "ipblock-single-cidr-ingress", "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		helloPod2ns2IPv6, helloPod2ns2IPv4 := getPodIP(oc, ns2, pod2ns2.name)
		helloPod2ns2IPv4WithCidr := helloPod2ns2IPv4 + "/32"
		helloPod2ns2IPv6WithCidr := helloPod2ns2IPv6 + "/128"

		if ipStackType == "dualstack" {
			g.By("create ipBlock Ingress Dual CIDRs Policy in ns1 again but with ipblock for pod2 ns2")
			npIPBlockNS1New := ipBlockIngressDual{
				name:      "ipblock-dual-cidrs-ingress",
				template:  ipBlockIngressTemplateDual,
				cidrIpv4:  helloPod2ns2IPv4WithCidr,
				cidrIpv6:  helloPod2ns2IPv6WithCidr,
				namespace: ns1,
			}
			npIPBlockNS1New.createipBlockIngressObjectDual(oc)
		} else {
			npIPBlockNS1New := ipBlockIngressSingle{
				name:      "ipblock-single-cidr-ingress",
				template:  ipBlockIngressTemplateSingle,
				cidr:      helloPod2ns2IPv6WithCidr,
				namespace: ns1,
			}
			npIPBlockNS1New.createipBlockIngressObjectSingle(oc)
		}
		g.By("Checking connectivity from pod2 ns2 to pod3 ns1")
		CurlPod2PodPass(oc, ns2, "hello-pod2", ns1, "hello-pod3")

		g.By("Checking connectivity from pod1 ns2 to pod3 ns1")
		CurlPod2PodFail(oc, ns2, "hello-pod1", ns1, "hello-pod3")

		if ipStackType == "dualstack" {
			g.By("Delete networkpolicy from ns1 again so no networkpolicy in namespace")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("networkpolicy", "ipblock-dual-cidrs-ingress", "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			g.By("Delete networkpolicy from ns1 again so no networkpolicy in namespace")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("networkpolicy", "ipblock-single-cidr-ingress", "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Check connectivity works fine across all failed ones above to make sure all policy flows are cleared properly")

		g.By("Checking connectivity from pod2ns1 to pod3ns1")
		CurlPod2PodPass(oc, ns1, "hello-pod2", ns1, "hello-pod3")

		g.By("Checking connectivity from pod1ns2 to pod3ns1")
		CurlPod2PodPass(oc, ns2, "hello-pod1", ns1, "hello-pod3")

		g.By("Checking connectivity from pod2ns2 to pod1ns1 on IPv4 interface")
		CurlPod2PodPass(oc, ns2, "hello-pod2", ns1, "hello-pod1")

	})

	// author: zzhao@redhat.com
	g.It("Author:zzhao-Critical-49696-mixed ingress and egress policies can work well", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking")
			testPodFile         = filepath.Join(buildPruningBaseDir, "testpod.yaml")
			helloSdnFile        = filepath.Join(buildPruningBaseDir, "hellosdn.yaml")
			egressTypeFile      = filepath.Join(buildPruningBaseDir, "networkpolicy/egress_49696.yaml")
			ingressTypeFile     = filepath.Join(buildPruningBaseDir, "networkpolicy/ingress_49696.yaml")
		)
		g.By("create one namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("create test pods")
		createResourceFromFile(oc, ns1, testPodFile)
		createResourceFromFile(oc, ns1, helloSdnFile)
		err := waitForPodWithLabelReady(oc, ns1, "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")
		err = waitForPodWithLabelReady(oc, ns1, "name=hellosdn")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=hellosdn not ready")
		hellosdnPodNameNs1 := getPodName(oc, ns1, "name=hellosdn")

		g.By("create egress type networkpolicy in ns1")
		createResourceFromFile(oc, ns1, egressTypeFile)

		g.By("create ingress type networkpolicy in ns1")
		createResourceFromFile(oc, ns1, ingressTypeFile)

		g.By("create second namespace")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("create test pods in second namespace")
		createResourceFromFile(oc, ns2, helloSdnFile)
		err = waitForPodWithLabelReady(oc, ns2, "name=hellosdn")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=hellosdn not ready")

		g.By("Get IP of the test pods in second namespace.")
		hellosdnPodNameNs2 := getPodName(oc, ns2, "name=hellosdn")
		hellosdnPodIP1Ns2 := getPodIPv4(oc, ns2, hellosdnPodNameNs2[0])

		g.By("curl from ns1 hellosdn pod to ns2 pod")
		_, err = e2e.RunHostCmd(ns1, hellosdnPodNameNs1[0], "curl --connect-timeout 5  -s "+net.JoinHostPort(hellosdnPodIP1Ns2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).Should(o.ContainSubstring("exit status 28"))

	})

	// author: anusaxen@redhat.com
	g.It("Author:anusaxen-High-46246-Network Policies should work with OVNKubernetes when traffic hairpins back to the same source through a service", func() {
		var (
			buildPruningBaseDir    = exutil.FixturePath("testdata", "networking")
			pingPodNodeTemplate    = filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")
			allowfromsameNS        = filepath.Join(buildPruningBaseDir, "networkpolicy/allow-from-same-namespace.yaml")
			genericServiceTemplate = filepath.Join(buildPruningBaseDir, "service-generic-template.yaml")
		)

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < 2 {
			g.Skip("This case requires 2 nodes, but the cluster has less than two nodes")
		}
		g.By("Create a namespace")
		oc.SetupProject()
		ns := oc.Namespace()

		g.By("create 1st hello pod in ns1")

		pod1 := pingPodResourceNode{
			name:      "hello-pod1",
			namespace: ns,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod1.createPingPodNode(oc)
		waitPodReady(oc, ns, pod1.name)

		g.By("create 2nd hello pod in same namespace but on different node")

		pod2 := pingPodResourceNode{
			name:      "hello-pod2",
			namespace: ns,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod2.createPingPodNode(oc)
		waitPodReady(oc, ns, pod2.name)

		g.By("Create a test service backing up both the above pods")
		svc := genericServiceResource{
			servicename:           "test-service",
			namespace:             ns,
			protocol:              "TCP",
			selector:              "hello-pod",
			serviceType:           "ClusterIP",
			ipFamilyPolicy:        "",
			internalTrafficPolicy: "Cluster",
			externalTrafficPolicy: "", //This no value parameter will be ignored
			template:              genericServiceTemplate,
		}
		svc.ipFamilyPolicy = "SingleStack"
		svc.createServiceFromParams(oc)

		g.By("create allow-from-same-namespace ingress networkpolicy in ns")
		createResourceFromFile(oc, ns, allowfromsameNS)

		g.By("curl from hello-pod1 to hello-pod2")
		CurlPod2PodPass(oc, ns, "hello-pod1", ns, "hello-pod2")

		g.By("curl from hello-pod2 to hello-pod1")
		CurlPod2PodPass(oc, ns, "hello-pod2", ns, "hello-pod1")

		for i := 0; i < 5; i++ {

			g.By("curl from hello-pod1 to service:port")
			CurlPod2SvcPass(oc, ns, ns, "hello-pod1", "test-service")

			g.By("curl from hello-pod2 to service:port")
			CurlPod2SvcPass(oc, ns, ns, "hello-pod2", "test-service")
		}

		g.By("Make sure pods are curl'able from respective nodes")
		CurlNode2PodPass(oc, pod1.nodename, ns, "hello-pod1")
		CurlNode2PodPass(oc, pod2.nodename, ns, "hello-pod2")

		ipStackType := checkIPStackType(oc)

		if ipStackType == "dualstack" {
			g.By("Delete testservice from ns")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("svc", "test-service", "-n", ns).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Checking pod to svc:port behavior now on with PreferDualStack Service")
			svc.ipFamilyPolicy = "PreferDualStack"
			svc.createServiceFromParams(oc)
			for i := 0; i < 5; i++ {
				g.By("curl from hello-pod1 to service:port")
				CurlPod2SvcPass(oc, ns, ns, "hello-pod1", "test-service")

				g.By("curl from hello-pod2 to service:port")
				CurlPod2SvcPass(oc, ns, ns, "hello-pod2", "test-service")
			}
		}
	})
})
