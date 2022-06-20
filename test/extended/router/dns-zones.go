package router

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("dns-zones", exutil.KubeConfigPath())

	// author: hongli@redhat.com
	g.It("Author:hongli-High-46183-DNS operator supports Random, RoundRobin and Sequential policy for servers.forwardPlugin [Disruptive]", func() {
		resourceName := "dns.operator.openshift.io/default"
		jsonPatch := "[{\"op\":\"add\", \"path\":\"/spec/servers\", \"value\":[{\"forwardPlugin\":{\"policy\":\"Random\",\"upstreams\":[\"8.8.8.8\"]},\"name\":\"test\",\"zones\":[\"mytest.ocp\"]}]}]"

		g.By("patch the dns.operator/default and add custom zones config")
		defer restoreDNSOperatorDefault(oc)
		patchGlobalResourceAsAdmin(oc, resourceName, jsonPatch)
		ensureDNSRollingUpdateDone(oc)

		g.By("check Corefile and ensure the policy is Random")
		oneDNSPod := getDNSPodName(oc)
		policy := readDNSCorefile(oc, oneDNSPod, "8.8.8.8", "-A2")
		o.Expect(policy).To(o.ContainSubstring(`policy random`))

		g.By("updateh the custom zones policy to RoundRobin ")
		patchGlobalResourceAsAdmin(oc, resourceName, "[{\"op\":\"replace\", \"path\":\"/spec/servers/0/forwardPlugin/policy\", \"value\":\"RoundRobin\"}]")
		ensureDNSRollingUpdateDone(oc)

		g.By("check Corefile and ensure the policy is round_robin")
		policy = readDNSCorefile(oc, oneDNSPod, "8.8.8.8", "-A2")
		o.Expect(policy).To(o.ContainSubstring(`policy round_robin`))

		g.By("updateh the custom zones policy to Sequential")
		patchGlobalResourceAsAdmin(oc, resourceName, "[{\"op\":\"replace\", \"path\":\"/spec/servers/0/forwardPlugin/policy\", \"value\":\"Sequential\"}]")
		ensureDNSRollingUpdateDone(oc)

		g.By("check Corefile and ensure the policy is sequential")
		policy = readDNSCorefile(oc, oneDNSPod, "8.8.8.8", "-A2")
		o.Expect(policy).To(o.ContainSubstring(`policy sequential`))
	})
})
