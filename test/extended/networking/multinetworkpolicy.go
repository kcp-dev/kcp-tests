package networking

import (
	"fmt"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-multinetworkpolicy", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("routes", "console", "-n", "openshift-console").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(msg, "bm2-zzhao") {
			g.Skip("These cases only can be run on Beijing local baremetal server , skip for other envrionment!!!")
		}
	})

	// author: weliang@redhat.com
	g.It("Author:weliang-Medium-41168-MultiNetworkPolicy ingress allow same podSelector with same namespaceSelector. [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking/multinetworkpolicy")
		policyFile := filepath.Join(buildPruningBaseDir, "ingress-allow-same-podSelector-with-same-namespaceSelector.yaml")
		patchSResource := "networks.operator.openshift.io/cluster"

		ns1 := "project41168a"
		ns2 := "project41168b"
		patchInfo := fmt.Sprintf("{\"spec\":{\"useMultiNetworkPolicy\":true}}")
		defer oc.AsAdmin().Run("delete").Args("project", ns1, "--ignore-not-found").Execute()
		defer oc.AsAdmin().Run("delete").Args("project", ns2, "--ignore-not-found").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("patch").Args(patchSResource, "-p", `[{"op": "remove", "path": "/spec/useMultiNetworkPolicy"}]`, "--type=json").Execute()

		g.By("1. Prepare multus multinetwork including 2 ns,5 pods and 2 NADs")
		prepareMultinetworkTest(oc, ns1, ns2, patchInfo)

		g.By("2. Get IPs of the pod1ns1's secondary interface in first namespace.")
		pod1ns1IPv4, pod1ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-1")
		pod1ns1IPv4 = strings.TrimSpace(pod1ns1IPv4)
		pod1ns1IPv6 = strings.TrimSpace(pod1ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns1IPv6)

		g.By("3. Get IPs of the pod2ns1's secondary interface in first namespace.")
		pod2ns1IPv4, pod2ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-2")
		pod2ns1IPv4 = strings.TrimSpace(pod2ns1IPv4)
		pod2ns1IPv6 = strings.TrimSpace(pod2ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns1IPv6)

		g.By("4. Get IPs of the pod3ns1's secondary interface in first namespace.")
		pod3ns1IPv4, pod3ns1IPv6 := getPodMultiNetwork(oc, ns1, "red-pod-1")
		pod3ns1IPv4 = strings.TrimSpace(pod3ns1IPv4)
		pod3ns1IPv6 = strings.TrimSpace(pod3ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod3ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod3ns1IPv6)

		g.By("5. Get IPs of the pod1ns2's secondary interface in second namespace.")
		pod1ns2IPv4, pod1ns2IPv6 := getPodMultiNetwork(oc, ns2, "blue-pod-3")
		pod1ns2IPv4 = strings.TrimSpace(pod1ns2IPv4)
		pod1ns2IPv6 = strings.TrimSpace(pod1ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns2IPv6)

		g.By("6. Get IPs of the pod2ns2's secondary interface in second namespace.")
		pod2ns2IPv4, pod2ns2IPv6 := getPodMultiNetwork(oc, ns2, "red-pod-2")
		pod2ns2IPv4 = strings.TrimSpace(pod2ns2IPv4)
		pod2ns2IPv6 = strings.TrimSpace(pod2ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns2IPv6)

		g.By("7. All curl should pass before applying policy")
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("8. Create Ingress-allow-same-podSelector-with-same-namespaceSelector policy in ns1")
		oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", policyFile, "-n", ns1).Execute()
		output, err := oc.AsAdmin().Run("get").Args("multi-networkpolicy", "-n", ns1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("ingress-allow-same-podselector-with-same-namespaceselector"))

		g.By("9. Same curl testing, one curl pass and three curls will fail after applying policy")
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("10. Delete two namespaces and disable useMultiNetworkPolicy")

	})

	// author: weliang@redhat.com
	g.It("NonPreRelease-Author:weliang-Medium-41169-MultiNetworkPolicy ingress allow diff podSelector with same namespaceSelector. [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking/multinetworkpolicy")
		policyFile := filepath.Join(buildPruningBaseDir, "ingress-allow-diff-podSelector-with-same-namespaceSelector.yaml")
		patchSResource := "networks.operator.openshift.io/cluster"

		ns1 := "project41169a"
		ns2 := "project41169b"
		patchInfo := fmt.Sprintf("{\"spec\":{\"useMultiNetworkPolicy\":true}}")
		defer oc.AsAdmin().Run("delete").Args("project", ns1, "--ignore-not-found").Execute()
		defer oc.AsAdmin().Run("delete").Args("project", ns2, "--ignore-not-found").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("patch").Args(patchSResource, "-p", `[{"op": "remove", "path": "/spec/useMultiNetworkPolicy"}]`, "--type=json").Execute()

		g.By("1. Prepare multus multinetwork including 2 ns,5 pods and 2 NADs")
		prepareMultinetworkTest(oc, ns1, ns2, patchInfo)

		g.By("2. Get IPs of the pod1ns1's secondary interface in first namespace.")
		pod1ns1IPv4, pod1ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-1")
		pod1ns1IPv4 = strings.TrimSpace(pod1ns1IPv4)
		pod1ns1IPv6 = strings.TrimSpace(pod1ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns1IPv6)

		g.By("3. Get IPs of the pod2ns1's secondary interface in first namespace.")
		pod2ns1IPv4, pod2ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-2")
		pod2ns1IPv4 = strings.TrimSpace(pod2ns1IPv4)
		pod2ns1IPv6 = strings.TrimSpace(pod2ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns1IPv6)

		g.By("4. Get IPs of the pod3ns1's secondary interface in first namespace.")
		pod3ns1IPv4, pod3ns1IPv6 := getPodMultiNetwork(oc, ns1, "red-pod-1")
		pod3ns1IPv4 = strings.TrimSpace(pod3ns1IPv4)
		pod3ns1IPv6 = strings.TrimSpace(pod3ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod3ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod3ns1IPv6)

		g.By("5. Get IPs of the pod1ns2's secondary interface in second namespace.")
		pod1ns2IPv4, pod1ns2IPv6 := getPodMultiNetwork(oc, ns2, "blue-pod-3")
		pod1ns2IPv4 = strings.TrimSpace(pod1ns2IPv4)
		pod1ns2IPv6 = strings.TrimSpace(pod1ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns2IPv6)

		g.By("6. Get IPs of the pod2ns2's secondary interface in second namespace.")
		pod2ns2IPv4, pod2ns2IPv6 := getPodMultiNetwork(oc, ns2, "red-pod-2")
		pod2ns2IPv4 = strings.TrimSpace(pod2ns2IPv4)
		pod2ns2IPv6 = strings.TrimSpace(pod2ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns2IPv6)

		g.By("7. All curl should pass before applying policy")
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("8. Create Ingress-allow-same-podSelector-with-same-namespaceSelector policy in ns1")
		oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", policyFile, "-n", ns1).Execute()
		output, err := oc.AsAdmin().Run("get").Args("multi-networkpolicy", "-n", ns1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("ingress-allow-diff-podselector-with-same-namespaceselector"))

		g.By("9. Same curl testing, one curl fail and three curls will pass after applying policy")
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("10. Delete two namespaces and disable useMultiNetworkPolicy")
	})

	// author: weliang@redhat.com
	g.It("NonPreRelease-Author:weliang-Medium-41171-MultiNetworkPolicy egress allow same podSelector with same namespaceSelector. [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "networking/multinetworkpolicy")
		policyFile := filepath.Join(buildPruningBaseDir, "egress-allow-same-podSelector-with-same-namespaceSelector.yaml")
		patchSResource := "networks.operator.openshift.io/cluster"

		ns1 := "project41171a"
		ns2 := "project41171b"
		patchInfo := fmt.Sprintf("{\"spec\":{\"useMultiNetworkPolicy\":true}}")
		defer oc.AsAdmin().Run("delete").Args("project", ns1, "--ignore-not-found").Execute()
		defer oc.AsAdmin().Run("delete").Args("project", ns2, "--ignore-not-found").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("patch").Args(patchSResource, "-p", `[{"op": "remove", "path": "/spec/useMultiNetworkPolicy"}]`, "--type=json").Execute()

		g.By("1. Prepare multus multinetwork including 2 ns,5 pods and 2 NADs")
		prepareMultinetworkTest(oc, ns1, ns2, patchInfo)

		g.By("2. Get IPs of the pod1ns1's secondary interface in first namespace.")
		pod1ns1IPv4, pod1ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-1")
		pod1ns1IPv4 = strings.TrimSpace(pod1ns1IPv4)
		pod1ns1IPv6 = strings.TrimSpace(pod1ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns1IPv6)

		g.By("3. Get IPs of the pod2ns1's secondary interface in first namespace.")
		pod2ns1IPv4, pod2ns1IPv6 := getPodMultiNetwork(oc, ns1, "blue-pod-2")
		pod2ns1IPv4 = strings.TrimSpace(pod2ns1IPv4)
		pod2ns1IPv6 = strings.TrimSpace(pod2ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns1IPv6)

		g.By("4. Get IPs of the pod3ns1's secondary interface in first namespace.")
		pod3ns1IPv4, pod3ns1IPv6 := getPodMultiNetwork(oc, ns1, "red-pod-1")
		pod3ns1IPv4 = strings.TrimSpace(pod3ns1IPv4)
		pod3ns1IPv6 = strings.TrimSpace(pod3ns1IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod3ns1IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod3ns1IPv6)

		g.By("5. Get IPs of the pod1ns2's secondary interface in second namespace.")
		pod1ns2IPv4, pod1ns2IPv6 := getPodMultiNetwork(oc, ns2, "blue-pod-3")
		pod1ns2IPv4 = strings.TrimSpace(pod1ns2IPv4)
		pod1ns2IPv6 = strings.TrimSpace(pod1ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod1ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod1ns2IPv6)

		g.By("6. Get IPs of the pod2ns2's secondary interface in second namespace.")
		pod2ns2IPv4, pod2ns2IPv6 := getPodMultiNetwork(oc, ns2, "red-pod-2")
		pod2ns2IPv4 = strings.TrimSpace(pod2ns2IPv4)
		pod2ns2IPv6 = strings.TrimSpace(pod2ns2IPv6)
		e2e.Logf("The v4 address of pod1ns1is: %v", pod2ns2IPv4)
		e2e.Logf("The v6 address of pod1ns1is: %v", pod2ns2IPv6)

		g.By("7. All curl should pass before applying policy")
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("8. Create egress-allow-same-podSelector-with-same-namespaceSelector policy in ns1")
		oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", policyFile, "-n", ns1).Execute()
		output, err := oc.AsAdmin().Run("get").Args("multi-networkpolicy", "-n", ns1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("egress-allow-same-podselector-with-same-namespaceselector"))

		g.By("9. Same curl testing, one curl pass and three curls will fail after applying policy")
		curlPod2PodMultiNetworkPass(oc, ns1, "blue-pod-1", pod2ns1IPv4, pod2ns1IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod3ns1IPv4, pod3ns1IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod1ns2IPv4, pod1ns2IPv6)
		curlPod2PodMultiNetworkFail(oc, ns1, "blue-pod-1", pod2ns2IPv4, pod2ns2IPv6)

		g.By("10. Delete two namespaces and disable useMultiNetworkPolicy")
	})
})
