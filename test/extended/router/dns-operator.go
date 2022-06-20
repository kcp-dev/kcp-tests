package router

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("dns-operator", exutil.KubeConfigPath())
	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Critical-41049-DNS controlls pod placement by node selector [Disruptive]", func() {
		var (
			dns_worker_nodeselector = "[{\"op\":\"add\", \"path\":\"/spec/nodePlacement/nodeSelector\", \"value\":{\"node-role.kubernetes.io/worker\":\"\"}}]"
			dns_master_nodeselector = "[{\"op\":\"replace\", \"path\":\"/spec/nodePlacement/nodeSelector\", \"value\":{\"node-role.kubernetes.io/master\":\"\"}}]"
		)

		g.By("check the default dns pod placement is present")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/os=linux"))

		g.By("Patch dns operator with worker as node selector in dns.operator default")
		dnspodname := getDNSPodName(oc)
		defer restoreDNSOperatorDefault(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_worker_nodeselector)
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/worker"))
		output_lcfg, err_lcfg := oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.nodeSelector}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`"node-role.kubernetes.io/worker":""`))

		g.By("Patch dns operator with master as node selector in dns.operator default")
		dnspodname1 := getDNSPodName(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_master_nodeselector)
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname1)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname1))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/master"))
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.nodeSelector}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`"node-role.kubernetes.io/master":""`))
	})
	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Critical-41050-DNS controll pod placement by tolerations [Disruptive]", func() {
		var (
			dns_master_toleration = "[{\"op\":\"replace\", \"path\":\"/spec/nodePlacement\", \"value\":{\"tolerations\":[" +
				"{\"effect\":\"NoExecute\",\"key\":\"my-dns-test\", \"operators\":\"Equal\", \"value\":\"abc\"}]}}]"
		)
		g.By("check the dns pod placement to confirm it is running on default mode")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-dns", "ds/dns-default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubernetes.io/os=linux"))

		g.By("check dns pod placement to confirm it is running on default tolerations")
		output_lcfg, err_lcfg := oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.tolerations}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`{"key":"node-role.kubernetes.io/master","operator":"Exists"}`))

		g.By("Patch dns operator config with custom tolerations of dns pod, not to tolerate master node taints)")
		dnspodname := getDNSPodName(oc)
		defer restoreDNSOperatorDefault(oc)
		patchGlobalResourceAsAdmin(oc, "dns.operator.openshift.io/default", dns_master_toleration)
		err = waitForResourceToDisappear(oc, "openshift-dns", "pod/"+dnspodname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+dnspodname))
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("ds/dns-default", "-n", "openshift-dns", "-o=jsonpath={.spec.template.spec.tolerations}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring(`{"effect":"NoExecute","key":"my-dns-test","value":"abc"}`))

		g.By("check dns.operator status to see any error messages")
		output_lcfg, err_lcfg = oc.AsAdmin().Run("get").Args("dns.operator/default", "-o=jsonpath={.status}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).NotTo(o.ContainSubstring("error"))
	})
	// author: shudili@redhat.com
	g.It("Author:shudili-NonPreRelease-Medium-46873-Configure operatorLogLevel under the default dns operator and check the logs flag [Disruptive]", func() {
		var (
			resourceName          = "dns.operator.openshift.io/default"
			cfg_oploglevel_debug  = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"Debug\"}]"
			cfg_oploglevel_trace  = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"Trace\"}]"
			cfg_oploglevel_normal = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"Normal\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default log level of dns operator")
		output_opcfg,err_opcfg := oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.operator", "default", "-o=jsonpath={.spec.operatorLogLevel}").Output()
		o.Expect(err_opcfg).NotTo(o.HaveOccurred())
		o.Expect(output_opcfg).To(o.ContainSubstring("Normal"))

		//Remove the dns operator pod and wait for the new pod is created, which is useful to check the dns operator log
		g.By("Remove dns operator pod")
		dnsOperatorPodName := getPodName(oc, "openshift-dns-operator", "name=dns-operator")[0]
		_, err_delpod := oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", dnsOperatorPodName, "-n", "openshift-dns-operator").Output()
		o.Expect(err_delpod).NotTo(o.HaveOccurred())
		err_podDis := waitForResourceToDisappear(oc, "openshift-dns-operator", "pod/"+dnsOperatorPodName)
		exutil.AssertWaitPollNoErr(err_podDis, fmt.Sprintf("the dns-operator pod isn't terminated"))
		err_podRdy := waitForPodWithLabelReady(oc, "openshift-dns-operator", "name=dns-operator")
		exutil.AssertWaitPollNoErr(err_podRdy, fmt.Sprintf("dns-operator pod isn't ready"))

		g.By("Patch dns operator with operator logLevel Debug")
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_oploglevel_debug)
		g.By("Check logLevel debug in dns operator")
		output_opcfg,err_opcfg = oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.operator", "default", "-o=jsonpath={.spec.operatorLogLevel}").Output()
		o.Expect(err_opcfg).NotTo(o.HaveOccurred())
		o.Expect(output_opcfg).To(o.ContainSubstring("Debug"))

		g.By("Patch dns operator with operator logLevel trace")
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_oploglevel_trace)
		g.By("Check logLevel trace in dns operator")
		output_opcfg,err_opcfg = oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.operator", "default", "-o=jsonpath={.spec.operatorLogLevel}").Output()
		o.Expect(err_opcfg).NotTo(o.HaveOccurred())
		o.Expect(output_opcfg).To(o.ContainSubstring("Trace"))

		g.By("Patch dns operator with operator logLevel normal")
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_oploglevel_normal)
		g.By("Check logLevel normal in dns operator")
		output_opcfg,err_opcfg = oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.operator", "default", "-o=jsonpath={.spec.operatorLogLevel}").Output()
		o.Expect(err_opcfg).NotTo(o.HaveOccurred())
		o.Expect(output_opcfg).To(o.ContainSubstring("Normal"))

		g.By("Check logs of dns operator")
		output_logs, err_log := oc.AsAdmin().Run("logs").Args("deployment/dns-operator", "-n", "openshift-dns-operator", "-c", "dns-operator").Output()
		o.Expect(err_log).NotTo(o.HaveOccurred())
		o.Expect(output_logs).To(o.ContainSubstring("level=info"))
	})
})
