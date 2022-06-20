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

	var oc = exutil.NewCLI("networking-"+getRandomString(), exutil.KubeConfigPath())

	g.BeforeEach(func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("routes", "console", "-n", "openshift-console").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "bm2-zzhao") {
			g.Skip("These cases only can be run on Beijing local baremetal server , skip for other envrionment!!!")
		}
	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-Medium-43466-EgressIP works well with ipv6 address. [Serial]", func() {
		ipStackType := checkIPStackType(oc)
		if ipStackType == "ipv4single" {
			g.Skip("Current env is ipv4 single stack cluster, skip this test!!!")
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

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
		sub1, _ := getDefaultIPv6Subnet(oc)
		ips, _ := findUnUsedIPv6(oc, sub1, 2)
		egressip1 := egressIPResource1{
			name:      "egressip-43466",
			template:  egressIPTemplate,
			egressIP1: ips[0],
			egressIP2: ips[1],
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
		waitPodReady(oc, pod1.namespace, pod1.name)
		defer pod1.deletePingPod(oc)

		g.By("Check source IP is EgressIP")
		msg, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -s -6 "+ipv6EchoServer(true)+" --connect-timeout 5")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, ips[0]) || strings.Contains(msg, ips[1])).To(o.BeTrue())

		g.By("EgressIP works well with IPv6. ")

	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-43465-Both ipv4 and ipv6 addresses can be configured on dualstack as egressip. [Serial]", func() {
		ipStackType := checkIPStackType(oc)
		if ipStackType != "dualstack" {
			g.Skip("Current env is not dualstack cluster, skip this case!!!")
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		egressIPTemplate := filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")

		g.By("create new namespace")
		oc.SetupProject()

		g.By("Label two EgressIP nodes")
		var EgressNodeLabel = "k8s.ovn.org/egress-assignable"
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply EgressLabel Key for this test on one node.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel, "true")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, EgressNodeLabel, "true")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, EgressNodeLabel)

		g.By("Apply label to namespace")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name=test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "name-").Output()

		g.By("Create an egressip object")

		g.By("Find unsued ipv4 address")
		subIpv4, _ := getDefaultSubnet(oc)
		ipv4s := findUnUsedIPs(oc, subIpv4, 1)

		g.By("Find unsued ipv6 address")
		subIpv6, _ := getDefaultIPv6Subnet(oc)
		ipv6s, _ := findUnUsedIPv6(oc, subIpv6, 1)

		egressip1 := egressIPResource1{
			name:      "egressip-43465",
			template:  egressIPTemplate,
			egressIP1: ipv6s[0],
			egressIP2: ipv4s[0],
		}
		egressip1.createEgressIPObject1(oc)
		defer egressip1.deleteEgressIPObject1(oc)

		g.By("Wait for egressip deployed.")
		err = wait.Poll(10*time.Second, 10*time.Second, func() (bool, error) {
			msg1, err1 := oc.WithoutNamespace().AsAdmin().Run("get").Args("egressip", egressip1.name).Output()
			if err1 != nil {
				e2e.Logf("the err:%v, wait for egressip %v to be deployed.", err1, egressip1.name)
				return false, nil
			}
			e2e.Logf("The egressip is :\n %v", msg1)
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("egressip %v is not ready", egressip1.name))

		g.By("Check EgressIP object status includes both IPv4 and IPv6")
		msg, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("egressip", egressip1.name, "-o=jsonpath={.status.items[*]}").Output()
		o.Expect(strings.Contains(msg, ipv6s[0]) && strings.Contains(msg, ipv4s[0])).To(o.BeTrue())

		g.By("Both ipv4 and ipv6 addresses can be configured on dualstack as egressip. !!!!")

	})

	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-43464-EgressFirewall works with IPv6 address.", func() {
		ipStackType := checkIPStackType(oc)
		if ipStackType == "ipv4single" {
			g.Skip("Current env is ipv4 single cluster, skip the test!!!")
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking")
		pingPodTemplate := filepath.Join(buildPruningBaseDir, "ping-for-pod-template.yaml")
		egressFWTemplate := filepath.Join(buildPruningBaseDir, "egressfirewall2-template.yaml")

		g.By("create new namespace")
		oc.SetupProject()

		g.By("Create an EgressFirewall object with rule deny.")
		egressFW2 := egressFirewall2{
			name:      "default",
			namespace: oc.Namespace(),
			ruletype:  "Deny",
			cidr:      "::/0",
			template:  egressFWTemplate,
		}
		egressFW2.createEgressFW2Object(oc)
		defer egressFW2.deleteEgressFW2Object(oc)

		g.By("Create a pod ")
		pod1 := pingPodResource{
			name:      "hello-pod",
			namespace: oc.Namespace(),
			template:  pingPodTemplate,
		}
		pod1.createPingPod(oc)
		waitPodReady(oc, pod1.namespace, pod1.name)
		defer pod1.deletePingPod(oc)

		g.By("Check the cidr address is blocked")
		_, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -6 [2620:52:0:4974:def4:1ff:fee7:8144]:8000 --connect-timeout 5")
		e2e.Logf("%v", err)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(err.Error(), "Connection timed out")).To(o.BeTrue())

		g.By("Remove egressfirewall object")
		egressFW2.deleteEgressFW2Object(oc)

		g.By("Create an EgressFirewall object with rule allow.")
		egressFW2 = egressFirewall2{
			name:      "default",
			namespace: oc.Namespace(),
			ruletype:  "Allow",
			cidr:      "::/0",
			template:  egressFWTemplate,
		}
		egressFW2.createEgressFW2Object(oc)

		g.By("Check the cidr address is accessed")
		msg, err := e2e.RunHostCmd(pod1.namespace, pod1.name, "curl -6 [2620:52:0:4974:def4:1ff:fee7:8144]:8000 --connect-timeout 5 -I")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(msg, "HTTP/1.1")).To(o.BeTrue())

		g.By("Egressfirewall works well with ipv6 address!!! ")

	})

})
