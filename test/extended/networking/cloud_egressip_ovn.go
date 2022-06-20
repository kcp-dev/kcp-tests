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
		ipEchoURL       string
		a               *exutil.AwsClient
		egressNodeLabel = "k8s.ovn.org/egress-assignable"

		oc = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		platform := exutil.CheckPlatform(oc)
		networkType := checkNetworkType(oc)
		e2e.Logf("\n\nThe platform is %v,  networkType is %v\n", platform, networkType)
		acceptedPlatform := strings.Contains(platform, "aws") || strings.Contains(platform, "gcp")
		if !acceptedPlatform || !strings.Contains(networkType, "ovn") {
			g.Skip("Test cases should be run on AWS or GCP cluster with ovn network plugin, skip for other platforms or other network plugin!!")
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

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-High-47163-High-47026-Deleting EgressIP object and recreating it works. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("create new namespace")
		oc.SetupProject()

		g.By("Label EgressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Apply EgressLabel Key for this test on one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)

		g.By("Apply label to namespace")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name=test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name-").Output()

		g.By("Create an egressip object")
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		freeIps := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47163",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("Create a pod ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: oc.Namespace(),
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		defer pod1.deletePingPod(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("Check source IP is EgressIP")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))

		g.By("Deleting egressip object")
		egressip1.deleteEgressIPObject1(oc)
		egressipErr := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
			sourceIP, err = e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
			if contains(freeIps, sourceIP) {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("Failed to clear egressip:%s", egressipErr))

		g.By("Recreating egressip object")
		egressip1.createEgressIPObject1(oc)

		g.By("Check source IP is EgressIP")
		wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			sourceIP, err = e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
			if !contains(freeIps, sourceIP) {
				eip, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("egressip", "-o=jsonpath={.}").Output()
				e2e.Logf(eip)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))

		g.By("Deleting EgressIP object and recreating it works!!! ")
	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Medium-47272-Pods will not be affected by the egressIP set on other netnamespace. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIP2Template := filepath.Join(buildPruningBaseDir, "egressip-config2-template.yaml")

		g.By("1.1 Label EgressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		g.By("1.2 Apply EgressLabel Key to one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)

		g.By("2.1 Create first egressip object")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:          "egressip-47272-1",
			template:      egressIP2Template,
			egressIP1:     freeIps[0],
			nsLabelKey:    "org",
			nsLabelValue:  "qe",
			podLabelKey:   "color",
			podLabelValue: "pink",
		}
		egressip1.createEgressIPObject2(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("2.2 Create second egressip object")
		egressip2 := egressIPResource1{
			name:          "egressip-47272-2",
			template:      egressIP2Template,
			egressIP1:     freeIps[1],
			nsLabelKey:    "org",
			nsLabelValue:  "qe",
			podLabelKey:   "color",
			podLabelValue: "blue",
		}
		egressip2.createEgressIPObject2(oc)
		defer egressip2.deleteEgressIPObject1(oc)

		g.By("3.1 create first namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("3.2 Apply a label to first namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org=qe").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3.3 Create a pod in first namespace. ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: ns1,
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod1.name, "-n", pod1.namespace).Execute()
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("3.4 Apply label to pod in first namespace")
		err = exutil.LabelPod(oc, ns1, pod1.name, "color=pink")
		defer exutil.LabelPod(oc, ns1, pod1.name, "color-")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4.1 create second namespace")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("4.2 Apply a label to second namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "org=qe").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "org-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4.3 Create a pod in second namespace ")
		pod2 := pingPodResource{
			name:      "hello-pod",
			namespace: ns2,
			template:  pingPodTemplate,
		}
		pod2.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod2.name, "-n", pod2.namespace).Execute()
		waitPodReady(oc, pod2.namespace, pod2.name)

		g.By("4.4 Apply label to pod in second namespace")
		err = exutil.LabelPod(oc, ns2, pod2.name, "color=blue")
		defer exutil.LabelPod(oc, ns2, pod2.name, "color-")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5.1 Check source IP in first namespace using first egressip object")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIps[0]))

		g.By("5.2 Check source IP in second namespace using second egressip object")
		sourceIP, err = e2e.RunHostCmd(pod2.namespace, pod2.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIps[1]))

		g.By("Pods will not be affected by the egressIP set on other netnamespace.!!! ")
	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Medium-47164-Be able to update egressip object. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIP2Template := filepath.Join(buildPruningBaseDir, "egressip-config2-template.yaml")

		g.By("1.1 Label EgressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		g.By("1.2 Apply EgressLabel Key to one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)

		g.By("2.1 Create first egressip object")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:          "egressip-47164",
			template:      egressIP2Template,
			egressIP1:     freeIps[0],
			nsLabelKey:    "org",
			nsLabelValue:  "qe",
			podLabelKey:   "color",
			podLabelValue: "pink",
		}
		egressip1.createEgressIPObject2(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("3.1 create first namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("3.2 Apply a label to first namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org=qe").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3.3 Create a pod in first namespace. ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: ns1,
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", pod1.name, "-n", pod1.namespace).Execute()
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("3.4 Apply label to pod in first namespace")
		err = exutil.LabelPod(oc, ns1, pod1.name, "color=pink")
		defer exutil.LabelPod(oc, ns1, pod1.name, "color-")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4. Update the egressip in egressip object")
		updateEgressIPObject(oc, egressip1.name, freeIps[1])

		g.By("5. Check source IP is updated IP")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIps[1]))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Medium-47030-An EgressIP object can not have multiple egress IP assignments on the same node. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("1. Label EgressIP node")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("2. Apply EgressLabel Key for this test on one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)

		g.By("3. Create an egressip object")
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		freeIps := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47030",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("4. Check only one EgressIP assigned in the object.")
		egressIPMaps := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps) == 1).Should(o.BeTrue())

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Medium-47028-After remove EgressIP node tag, EgressIP will failover to other availabel egress nodes. [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("1. Get list of nodes, get subnet from two worker nodes that have same subnet \n")
		var egressNode1, egressNode2 string
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}
		egressNode1 = egressNodes[0]
		egressNode2 = egressNodes[1]

		g.By("2. Apply EgressLabel Key for this test on one node.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode1, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode1, egressNodeLabel)

		g.By("3.1 Create new namespace\n")
		oc.SetupProject()
		ns1 := oc.Namespace()
		g.By("3.2 Apply label to namespace\n")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name=test").Output()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4. Create a pod in first namespace. \n")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: ns1,
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)

		g.By("5. Create an egressip object\n")
		sub1 := getIfaddrFromNode(egressNode1, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode1, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47028",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("4. Check EgressIP assigned in the object.\n")
		egressIPMaps := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps) == 1).Should(o.BeTrue())

		g.By("5. Update Egress node to egressNode2.\n")
		e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode1, egressNodeLabel)
		//This timeout is a workaround for bug https://bugzilla.redhat.com/show_bug.cgi?id=2070392
		time.Sleep(5 * time.Second)
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode2, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode2, egressNodeLabel)

		g.By("6. Check the egress node was updated in the egressip object.\n")
		egressipErr := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
			egressIPMaps = getAssignedEIPInEIPObject(oc, egressip1.name)
			if len(egressIPMaps) != 1 || egressIPMaps[0]["node"] == egressNode1 {
				e2e.Logf("Wait for new egress node applied,try next round.")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("Failed to update egress node:%s", egressipErr))
		o.Expect(egressIPMaps[0]["node"]).Should(o.ContainSubstring(egressNode2))

		g.By("7. Check the source ip.\n")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(egressIPMaps[0]["egressIP"]))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Longduration-NonPreRelease-High-47031-After reboot egress node EgressIP still work.  [Disruptive]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		testPodFile := filepath.Join(buildPruningBaseDir, "testpod.yaml")
		egressIP2Template := filepath.Join(buildPruningBaseDir, "egressip-config2-template.yaml")

		g.By("1.1 Label EgressIP node\n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name
		g.By("1.2 Apply EgressLabel Key to one node.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel)

		g.By("2.1 Create first egressip object\n")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 1)
		o.Expect(len(freeIps) == 1).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:          "egressip-47031",
			template:      egressIP2Template,
			egressIP1:     freeIps[0],
			nsLabelKey:    "org",
			nsLabelValue:  "qe",
			podLabelKey:   "color",
			podLabelValue: "pink",
		}
		egressip1.createEgressIPObject2(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("3.1 create first namespace\n")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("3.2 Apply a label to test namespace.\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org=qe").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "org-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3.3 Create pods in test namespace. \n")
		createResourceFromFile(oc, ns1, testPodFile)
		err = waitForPodWithLabelReady(oc, ns1, "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")

		g.By("3.4 Apply label to one pod in test namespace\n")
		testPodName := getPodName(oc, ns1, "name=test-pods")
		err = exutil.LabelPod(oc, ns1, testPodName[0], "color=pink")
		defer exutil.LabelPod(oc, ns1, testPodName[0], "color-")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4. Check only one EgressIP assigned in the object.\n")
		egressIPMaps := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps) == 1).Should(o.BeTrue())

		g.By("5.Reboot egress node.\n")
		defer checkNodeStatus(oc, egressNode, "Ready")
		rebootNode(oc, egressNode)
		checkNodeStatus(oc, egressNode, "NotReady")
		checkNodeStatus(oc, egressNode, "Ready")

		g.By("7. Check EgressIP assigned in the object.\n")
		egressIPMaps2 := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps2) == 1).Should(o.BeTrue())

		g.By("8. Check source IP is egressIP \n")
		e2e.Logf(" ipEchoURL is %v", ipEchoURL)
		sourceIP, err := e2e.RunHostCmd(ns1, testPodName[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.Equal(freeIps[0]))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Critical-47032-Traffic is load balanced between egress nodes. [Serial]", func() {

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("create new namespace\n")
		oc.SetupProject()

		g.By("Label EgressIP node\n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		ok, egressNodes := getTwoNodesSameSubnet(oc, nodeList)
		if !ok || egressNodes == nil || len(egressNodes) < 2 {
			g.Skip("The prerequirement was not fullfilled, skip the case!!")
		}

		g.By("Apply EgressLabel Key for this test on one node.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNodes[0], egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNodes[0], egressNodeLabel)
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNodes[1], egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNodes[1], egressNodeLabel)

		g.By("Apply label to namespace\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name-").Execute()

		g.By("Create an egressip object\n")
		sub1 := getIfaddrFromNode(egressNodes[0], oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNodes[0], sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47032",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("Create a pod ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: oc.Namespace(),
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		defer pod1.deletePingPod(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("Check source IP is randomly one of egress ips.\n")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err := execCommandInSpecificPod(oc, pod1.namespace, pod1.name, "for i in {1..10}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIps[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIps[1]))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-High-47019-High-47023-EgressIP works well with networkpolicy and egressFirewall. [Serial]", func() {
		//EgressFirewall case cannot run in proxy cluster, skip if proxy cluster.
		if checkProxy(oc) {
			g.Skip("This is proxy cluster, skip the test.")
		}

		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		networkPolicyFile := filepath.Join(buildPruningBaseDir, "networkpolicy/default-deny-ingress.yaml")
		testPodFile := filepath.Join(buildPruningBaseDir, "testpod.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")
		egressFWTemplate := filepath.Join(buildPruningBaseDir, "egressfirewall2-template.yaml")

		g.By("1. Label EgressIP node\n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		egressNode := nodeList.Items[0].Name
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2. Apply EgressLabel Key for this test on one node.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel)

		g.By("3. create new namespace\n")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("4. Apply label to namespace\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name-").Execute()

		g.By("5. Create an egressip object\n")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47019",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("6. Create test pods \n")
		createResourceFromFile(oc, ns1, testPodFile)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")

		g.By("7. Create default deny ingress type networkpolicy in test namespace\n")
		createResourceFromFile(oc, ns1, networkPolicyFile)
		output, err := oc.Run("get").Args("networkpolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("default-deny-ingress"))

		g.By("8. Create an EgressFirewall object with rule deny.")
		ipEchoIP := strings.Split(ipEchoURL, ":")[0]
		e2e.Logf(ipEchoIP)
		egressFW2 := egressFirewall2{
			name:      "default",
			namespace: ns1,
			ruletype:  "Deny",
			cidr:      ipEchoIP + "/32",
			template:  egressFWTemplate,
		}
		egressFW2.createEgressFW2Object(oc)
		defer egressFW2.deleteEgressFW2Object(oc)

		g.By("9. Get test pods IP and test pod name in test namespace\n")
		testPodName := getPodName(oc, oc.Namespace(), "name=test-pods")

		g.By("10. Check network policy works. \n")
		CurlPod2PodFail(oc, ns1, testPodName[0], ns1, testPodName[1])

		g.By("11. Check EgressFirewall policy works. \n")
		_, err = e2e.RunHostCmd(ns1, testPodName[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).To(o.HaveOccurred())

		g.By("12.Update EgressFirewall to allow")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("egressfirewall.k8s.ovn.org/default", "-n", ns1, "-p", "{\"spec\":{\"egress\":[{\"type\":\"Allow\",\"to\":{\"cidrSelector\":\""+ipEchoIP+"/32\"}}]}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("13. Check EgressFirewall Allow rule works and EgressIP works.\n")
		egressipErr := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			sourceIP, err := e2e.RunHostCmd(ns1, testPodName[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
			if err != nil {
				e2e.Logf("Wait for EgressFirewall taking effect. %v", err)
				return false, nil
			}
			if !contains(freeIps, sourceIP) {
				eip, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("egressip", "-o=jsonpath={.}").Output()
				e2e.Logf(eip)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("The source Ip is not same as the egressIP expected!"))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Medium-47018-Medium-47017-Multiple projects use same EgressIP. [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		testPodFile := filepath.Join(buildPruningBaseDir, "testpod.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("1. Label EgressIP node\n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		egressNode := nodeList.Items[0].Name
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2. Apply EgressLabel Key for this test on one node.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel)

		g.By("3. create first namespace\n")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("4. Create test pods in first namespace. \n")
		createResourceFromFile(oc, ns1, testPodFile)
		err = waitForPodWithLabelReady(oc, ns1, "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")
		testPodNs1Name := getPodName(oc, ns1, "name=test-pods")

		g.By("5. Apply label to ns1 namespace\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name-").Execute()

		g.By("6. Create an egressip object\n")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 2)
		o.Expect(len(freeIps) == 2).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:      "egressip-47018",
			template:  egressIPTemplate,
			egressIP1: freeIps[0],
			egressIP2: freeIps[1],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("7. create new namespace\n")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("8. Apply label to namespace\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "name=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "name-").Execute()

		g.By("9. Create test pods in second namespace  \n")
		createResourceFromFile(oc, ns2, testPodFile)
		err = waitForPodWithLabelReady(oc, ns2, "name=test-pods")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=test-pods not ready")
		testPodNs2Name := getPodName(oc, ns2, "name=test-pods")

		g.By("10. Check source IP from both namespace, should be egressip.  \n")
		sourceIP, err := e2e.RunHostCmd(ns1, testPodNs1Name[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))
		sourceIP, err = e2e.RunHostCmd(ns1, testPodNs1Name[1], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))
		sourceIP, err = e2e.RunHostCmd(ns2, testPodNs2Name[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))
		sourceIP, err = e2e.RunHostCmd(ns2, testPodNs2Name[1], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).Should(o.BeElementOf(freeIps))

		g.By("11. Remove matched labels from namespace ns1  \n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("12.  Check source IP from namespace ns1, should not be egressip. \n")
		sourceIP, err = e2e.RunHostCmd(ns1, testPodNs1Name[0], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).ShouldNot(o.BeElementOf(freeIps))
		sourceIP, err = e2e.RunHostCmd(ns1, testPodNs1Name[1], "curl -s "+ipEchoURL+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceIP).ShouldNot(o.BeElementOf(freeIps))

	})

	// author: huirwang@redhat.com
	g.It("ConnectedOnly-Author:huirwang-Longduration-NonPreRelease-Medium-47033-If an egress node is NotReady traffic is still load balanced between available egress nodes. [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodNodeTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("1. create new namespace\n")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("2. Label EgressIP node\n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < 3 {
			g.Skip("Not enough worker nodes for this test, skip the case!!")
		}

		g.By("3. Apply EgressLabel Key for this test on 3 nodes.\n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, egressNodeLabel)
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, egressNodeLabel)
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, egressNodeLabel)

		g.By("4. Apply label to namespace\n")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name=test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "name-").Execute()

		g.By("5. Create an egressip object\n")
		sub1 := getIfaddrFromNode(nodeList.Items[0].Name, oc)
		freeIP1 := findUnUsedIPsOnNode(oc, nodeList.Items[0].Name, sub1, 1)
		o.Expect(len(freeIP1) == 1).Should(o.BeTrue())
		sub2 := getIfaddrFromNode(nodeList.Items[1].Name, oc)
		freeIP2 := findUnUsedIPsOnNode(oc, nodeList.Items[1].Name, sub2, 1)
		o.Expect(len(freeIP2) == 1).Should(o.BeTrue())
		sub3 := getIfaddrFromNode(nodeList.Items[2].Name, oc)
		freeIP3 := findUnUsedIPsOnNode(oc, nodeList.Items[2].Name, sub3, 1)
		o.Expect(len(freeIP3) == 1).Should(o.BeTrue())

		egressip1 := egressIPResource1{
			name:      "egressip-47033",
			template:  egressIPTemplate,
			egressIP1: freeIP1[0],
			egressIP2: freeIP2[0],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("6. Update an egressip object with three egressips.\n")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("egressip/egressip-47033", "-p", "{\"spec\":{\"egressIPs\":[\""+freeIP1[0]+"\",\""+freeIP2[0]+"\",\""+freeIP3[0]+"\"]}}", "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7. Create a pod \n")
		pod1 := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns1,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod1.createPingPodNode(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)

		g.By("8. Check source IP is randomly one of egress ips.\n")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		egressIPMaps2 := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps2) == 3).Should(o.BeTrue())
		sourceIP, err := execCommandInSpecificPod(oc, pod1.namespace, pod1.name, "for i in {1..15}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP3[0]))

		g.By("9. Stop one egress node.\n")
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
			instance = strings.Split(nodeList.Items[1].Name, ".")
			e2e.Logf("\n\n\n the worker node to be shutdown is: %v\n\n\n", instance[0])
			infraID, err := exutil.GetInfraId(oc)
			zone, err = getZoneOfInstanceFromGcp(oc, infraID, instance[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			defer checkNodeStatus(oc, instance[0], "Ready")
			defer startInstanceOnGcp(oc, instance[0], zone)
			err = stopInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, nodeList.Items[1].Name, "NotReady")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

		g.By("10. Check EgressIP updated in EIP object, sourceIP contains 2 IPs. \n")
		egressipErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			egressIPMaps2 = getAssignedEIPInEIPObject(oc, egressip1.name)
			if len(egressIPMaps2) != 2 {
				return false, nil
			}
			sourceIP, err := execCommandInSpecificPod(oc, pod1.namespace, pod1.name, "for i in {1..15}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
			e2e.Logf(sourceIP)
			if err != nil {
				return false, nil
			}
			if strings.Contains(sourceIP, egressIPMaps2[0]["egressIP"]) && strings.Contains(sourceIP, egressIPMaps2[1]["egressIP"]) {
				sourceIPSlice := findIP(sourceIP)
				if len(unique(sourceIPSlice)) == 2 {
					return true, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("The source Ip is not same as the egressIP expected!"))

		g.By("11. Start the stopped egress node \n")
		switch exutil.CheckPlatform(oc) {
		case "aws":
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			startInstanceOnAWS(a, nodeList.Items[1].Name)
			checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
		case "gcp":
			defer checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
			err = startInstanceOnGcp(oc, instance[0], zone)
			o.Expect(err).NotTo(o.HaveOccurred())
			checkNodeStatus(oc, nodeList.Items[1].Name, "Ready")
		default:
			e2e.Logf("Not support cloud provider for auto egressip cases for now.")
			g.Skip("Not support cloud provider for auto egressip cases for now.")
		}

		g.By("12. Check source IP is randomly one of 3 egress IPs.\n")
		e2e.Logf("\n ipEchoURL is %v\n", ipEchoURL)
		sourceIP, err = execCommandInSpecificPod(oc, pod1.namespace, pod1.name, "for i in {1..15}; do curl -s "+ipEchoURL+" --connect-timeout 5 ; sleep 2;echo ;done")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf(sourceIP)
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP1[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP2[0]))
		o.Expect(sourceIP).Should(o.ContainSubstring(freeIP3[0]))
	})

})

var _ = g.Describe("[sig-networking] SDN OVN EgressIP Basic", func() {
	//Cases in this function, do not need curl ip-echo
	defer g.GinkgoRecover()

	var (
		egressNodeLabel = "k8s.ovn.org/egress-assignable"
		oc              = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		platform := exutil.CheckPlatform(oc)
		networkType := checkNetworkType(oc)
		e2e.Logf("\n\nThe platform is %v,  networkType is %v\n", platform, networkType)
		acceptedPlatform := strings.Contains(platform, "aws") || strings.Contains(platform, "gcp")
		if !acceptedPlatform || !strings.Contains(networkType, "ovn") {
			g.Skip("Test cases should be run on AWS or GCP cluster with ovn network plugin, skip for other platforms or other network plugin!!")
		}
	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-NonPreRelease-Medium-47020-Low-47024-EgressIP node liveness probe. [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		egressIP2Template := filepath.Join(buildPruningBaseDir, "egressip-config2-template.yaml")

		g.By("1 Get list of nodes \n")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		egressNode := nodeList.Items[0].Name

		g.By("2 Create first egressip object \n")
		sub1 := getIfaddrFromNode(egressNode, oc)
		freeIps := findUnUsedIPsOnNode(oc, egressNode, sub1, 1)
		o.Expect(len(freeIps) == 1).Should(o.BeTrue())
		egressip1 := egressIPResource1{
			name:          "egressip-47020",
			template:      egressIP2Template,
			egressIP1:     freeIps[0],
			nsLabelKey:    "org",
			nsLabelValue:  "qe",
			podLabelKey:   "color",
			podLabelValue: "pink",
		}
		egressip1.createEgressIPObject2(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("3. Check warning event. \n")
		warnErr := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
			warningEvent, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("event", "-n", "default").Output()
			if err != nil {
				e2e.Logf("Wait for waring event generated.%v", err)
				return false, nil
			}
			if !strings.Contains(warningEvent, "NoMatchingNodeFound") {
				e2e.Logf("Wait for waring event generated. ")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(warnErr, fmt.Sprintf("Warning event doesn't conclude: NoMatchingNodeFound."))

		g.By("4 Apply EgressLabel Key to one node. \n")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, egressNode, egressNodeLabel)

		g.By("5. Check EgressIP assigned in the object.\n")
		egressIPMaps1 := getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps1) == 1).Should(o.BeTrue())

		g.By("6. Add iptables on ovn-k8s-mp0 port to block probe detect.\n")
		_, err1 := exutil.DebugNodeWithChroot(oc, egressNode, "iptables", "-I", "INPUT", "1", "-i", "ovn-k8s-mp0", "-p", "tcp", "--destination-port", "9", "-j", "DROP")
		defer exutil.DebugNodeWithChroot(oc, egressNode, "iptables", "-D", "INPUT", "-i", "ovn-k8s-mp0", "-p", "tcp", "--destination-port", "9", "-j", "DROP")
		o.Expect(err1).NotTo(o.HaveOccurred())

		g.By("7. Wait for the egressip object updated, should no egress node assigned.\n")
		egressIPMaps1 = getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps1) == 0).Should(o.BeTrue())

		g.By("8. Remove iptables ovn-k8s-mp0 port to unblock probe detect.\n")
		_, err1 = exutil.DebugNodeWithChroot(oc, egressNode, "iptables", "-D", "INPUT", "-i", "ovn-k8s-mp0", "-p", "tcp", "--destination-port", "9", "-j", "DROP")
		o.Expect(err1).NotTo(o.HaveOccurred())

		g.By("9. Check EgressIP assigned in the object.\n")
		egressIPMaps1 = getAssignedEIPInEIPObject(oc, egressip1.name)
		o.Expect(len(egressIPMaps1) == 1).Should(o.BeTrue())

	})

})
