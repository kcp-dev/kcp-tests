package router

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("coredns-upstream-resolvers-log", exutil.KubeConfigPath())
	// author: shudili@redhat.com
	g.It("Author:shudili-NonPreRelease-Critical-46868-Configure forward policy for CoreDNS flag [Disruptive]", func() {
		var (
			resourceName           = "dns.operator.openshift.io/default"
			cfg_mul_ipv4_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" + 
			                         "{\"address\":\"10.100.1.11\",\"port\":53,\"type\":\"Network\"}, " + 
			                         "{\"address\":\"10.100.1.12\",\"port\":53,\"type\":\"Network\"}, " + 
			                         "{\"address\":\"10.100.1.13\",\"port\":5353,\"type\":\"Network\"}]}]"
			cfg_default_upstreams  = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"port\":53,\"type\":\"SystemResolvConf\"}]}]"
			cfg_policy_random      = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"Random\"}]"
			cfg_policy_rr          = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"RoundRobin\"}]"
			cfg_policy_seq         = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"Sequential\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default values of forward policy for CoreDNS")
		podList       := getAllDNSPodsNames(oc)
		dnspodname    := getRandomDNSPodName(podList)
		policy_output := readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with multiple ipv4 upstreams")
		dnspodname  =  getRandomDNSPodName(podList)
		attrList   :=  getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_mul_ipv4_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check multiple ipv4 forward upstreams in CoreDNS")
		upstreams  :=  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . 10.100.1.11:53 10.100.1.12:53 10.100.1.13:5353"))
		g.By("Check default forward policy in the CM after multiple ipv4 forward upstreams are configured")
		output_pcfg,err_pcfg := oc.AsAdmin().Run("get").Args("cm/dns-default", "-n", "openshift-dns", "-o=jsonpath={.data.Corefile}").Output()
		o.Expect(err_pcfg).NotTo(o.HaveOccurred())
		o.Expect(output_pcfg).To(o.ContainSubstring("policy sequential"))
		g.By("Check default forward policy in CoreDNS after multiple ipv4 forward upstreams are configured")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with policy random for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_random)
		waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy random in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy random"))

		g.By("Patch dns operator with policy roundrobin for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_rr)
		waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy roundrobin in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy round_robin"))

		g.By("Patch dns operator with policy sequential for upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_policy_seq)
		waitCorefileUpdated(oc, attrList)
		g.By("Check forward policy sequential in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))

		g.By("Patch dns operator with default upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_default_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check upstreams is restored to default in CoreDNS")
		upstreams  =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . /etc/resolv.conf"))
		g.By("Check forward policy sequential in Corefile of coredns")
		policy_output = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(policy_output).To(o.ContainSubstring("policy sequential"))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-Critical-46872-Configure logLevel for CoreDNS under DNS operator flag [Disruptive]", func() {
		var (
			resourceName        = "dns.operator.openshift.io/default"
			cfg_loglevel_debug  = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Debug\"}]"
			cfg_loglevel_trace  = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Trace\"}]"
			cfg_loglevel_normal = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"Normal\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default log level of CoreDNS")
		podList    := getAllDNSPodsNames(oc)
		dnspodname := getRandomDNSPodName(podList)
		log_output := readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class error"))

		g.By("Patch dns operator with logLevel Debug for CoreDNS")
		dnspodname =  getRandomDNSPodName(podList)
		attrList  :=  getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_debug)
		waitCorefileUpdated(oc, attrList)
		output_lcfg,err_lcfg := oc.AsAdmin().Run("get").Args("cm/dns-default", "-n", "openshift-dns", "-o=jsonpath={.data.Corefile}").Output()
		o.Expect(err_lcfg).NotTo(o.HaveOccurred())
		o.Expect(output_lcfg).To(o.ContainSubstring("class denial error"))
		g.By("Check log class for logLevel Debug in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class denial error"))

		g.By("Patch dns operator with logLevel Trace for CoreDNS")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_trace)
		waitCorefileUpdated(oc, attrList)
		g.By("Check log class for logLevel Trace in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class all"))

		g.By("Patch dns operator with logLevel Normal for CoreDNS")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_loglevel_normal)
		waitCorefileUpdated(oc, attrList)
		g.By("Check log class for logLevel Trace in Corefile of coredns")
		log_output = readDNSCorefile(oc, dnspodname, "log", "-A2")
		o.Expect(log_output).To(o.ContainSubstring("class error"))
	})

	g.It("Author:shudili-NonPreRelease-Critical-46867-Configure upstream resolvers for CoreDNS flag [Disruptive]", func() {
		var (
			resourceName           = "dns.operator.openshift.io/default"
			cfg_default_upstreams  = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"port\":53,\"type\":\"SystemResolvConf\"}]}]"
			cfg_mul_ipv4_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"address\":\"10.100.1.11\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"10.100.1.12\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"10.100.1.13\",\"port\":5353,\"type\":\"Network\"}]}]"
			exp_mul_ipv4_upstreams = "forward . 10.100.1.11:53 10.100.1.12:53 10.100.1.13:5353"
			cfg_one_ipv4_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"address\":\"20.100.1.11\",\"port\":53,\"type\":\"Network\"}]}]"
			exp_one_ipv4_upstreams = "forward . 20.100.1.11:53"
			cfg_max_15_upstreams   =  "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"address\":\"30.100.1.11\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.12\",\"port\":53,\"type\":\"Network\"}, " + 
			                         "{\"address\":\"30.100.1.13\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.14\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.15\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.16\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.17\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.18\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.19\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.20\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.21\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.22\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.23\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.24\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.25\",\"port\":53,\"type\":\"Network\"}]}]"
			exp_max_15_upstreams   = "forward . 30.100.1.11:53 30.100.1.12:53 30.100.1.13:53 30.100.1.14:53 30.100.1.15:53 " +
			                         "30.100.1.16:53 30.100.1.17:53 30.100.1.18:53 30.100.1.19:53 30.100.1.20:53 " +
			                         "30.100.1.21:53 30.100.1.22:53 30.100.1.23:53 30.100.1.24:53 30.100.1.25:53"
			cfg_mul_ipv6_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"address\":\"1001::aaaa\",\"port\":5353,\"type\":\"Network\"}, " +
			                         "{\"address\":\"1001::BBBB\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"1001::cccc\",\"port\":53,\"type\":\"Network\"}]}]"
			exp_mul_ipv6_upstreams = "forward . [1001::AAAA]:5353 [1001::BBBB]:53 [1001::CCCC]:53"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Check default values of forward upstream resolvers for CoreDNS")
		podList       := getAllDNSPodsNames(oc)
		dnspodname    := getRandomDNSPodName(podList)
		upstreams     := readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . /etc/resolv.conf"))

		g.By("Patch dns operator with multiple ipv4 upstreams")
		dnspodname = getRandomDNSPodName(podList)
		attrList  := getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_mul_ipv4_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check multiple ipv4 forward upstream resolvers in config map")
		output_cfg,err_cfg := oc.AsAdmin().Run("get").Args("cm/dns-default", "-n", "openshift-dns", "-o=jsonpath={.data.Corefile}").Output()
		o.Expect(err_cfg).NotTo(o.HaveOccurred())
		o.Expect(output_cfg).To(o.ContainSubstring(exp_mul_ipv4_upstreams))
		g.By("Check multiple ipv4 forward upstream resolvers in CoreDNS")
		upstreams = readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring(exp_mul_ipv4_upstreams))

		g.By("Patch dns operator with a single ipv4 upstream")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_one_ipv4_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check a single ipv4 forward upstream resolver for CoreDNS")
		upstreams =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring(exp_one_ipv4_upstreams))

		g.By("Patch dns operator with max 15 ipv4 upstreams")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_max_15_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check max 15 ipv4 forward upstream resolvers for CoreDNS")
		upstreams =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring(exp_max_15_upstreams))

		g.By("Patch dns operator with multiple ipv6 upstreams")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_mul_ipv6_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check multiple ipv6 forward upstream resolvers for CoreDNS")
		upstreams =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring(exp_mul_ipv6_upstreams))

		g.By("Patch dns operator with default upstream resolvers")
		dnspodname = getRandomDNSPodName(podList)
		attrList   = getOneCorefileStat(oc, dnspodname)
		patchGlobalResourceAsAdmin(oc, resourceName, cfg_default_upstreams)
		waitCorefileUpdated(oc, attrList)
		g.By("Check upstreams is restored to default in CoreDNS")
		upstreams  =  readDNSCorefile(oc, dnspodname, "forward", "-A2")
		o.Expect(upstreams).To(o.ContainSubstring("forward . /etc/resolv.conf"))
	})

	g.It("Author:shudili-Medium-46869-Negative test of configuring upstream resolvers and policy flag [Disruptive]", func() {
		var (
			resourceName           = "dns.operator.openshift.io/default"
			cfg_addone_upstreams   = "[{\"op\":\"add\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                         "{\"address\":\"30.100.1.11\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.12\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.13\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.14\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.15\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.16\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.17\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.18\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.19\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.20\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.21\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.22\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.23\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.24\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.25\",\"port\":53,\"type\":\"Network\"}, " +
			                         "{\"address\":\"30.100.1.26\",\"port\":53,\"type\":\"Network\"}]}]"
			invalidCfg_string_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                              "{\"address\":\"str_test\",\"port\":53,\"type\":\"Network\"}]}]"
			invalidCfg_number_upstreams = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/upstreams\", \"value\":[" +
			                              "{\"address\":\"100\",\"port\":53,\"type\":\"Network\"}]}]"
			invalidCfg_sring_policy     = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"string_test\"}]"
			invalidCfg_number_policy    = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"2\"}]"
			invalidCfg_random_policy    = "[{\"op\":\"replace\", \"path\":\"/spec/upstreamResolvers/policy\", \"value\":\"random\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Try to add one more upstream resolver, totally 16 upstream resolvers by patching dns operator")
		output, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+cfg_addone_upstreams, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("spec.upstreamResolvers.upstreams in body should have at most 15 items"))

		g.By("Try to add a upstream resolver with a string as an address")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_string_upstreams, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Invalid value: \"str_test\""))

		g.By("Try to add a upstream resolver with a number as an address")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_number_upstreams, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Invalid value: \"100\""))

		g.By("Try to configure the polciy with a string")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_sring_policy, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"string_test\""))

		g.By("Try to configure the polciy with a number")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_number_policy, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"2\""))

		g.By("Try to configure the polciy with a similar string like random")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_random_policy, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"random\""))
	})

	g.It("Author:shudili-Medium-46874-negative test for configuring logLevel and operatorLogLevel flag [Disruptive]", func() {
		var (
			resourceName = "dns.operator.openshift.io/default"
			invalidCfg_string_loglevel = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"string_test\"}]"
			invalidCfg_number_loglevel = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"2\"}]"
			invalidCfg_trace_loglevel  = "[{\"op\":\"replace\", \"path\":\"/spec/logLevel\", \"value\":\"trace\"}]"
			invalidCfg_string_OPloglevel = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"string_test\"}]"
			invalidCfg_number_OPloglevel = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"2\"}]"
			invalidCfg_trace_OPloglevel  = "[{\"op\":\"replace\", \"path\":\"/spec/operatorLogLevel\", \"value\":\"trace\"}]"
		)
		defer restoreDNSOperatorDefault(oc)

		g.By("Try to configure log level with a string")
		output, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_string_loglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"string_test\""))

		g.By("Try to configure log level with a number")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_number_loglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"2\""))

		g.By("Try to configure log level with a similar string like trace")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_trace_loglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"trace\""))

		g.By("Try to configure dns operator log level with a string")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_string_OPloglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"string_test\""))

		g.By("Try to configure dns operator log level with a number")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_number_OPloglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"2\""))

		g.By("Try to configure dns operator log level with a similar string like trace")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceName, "--patch="+invalidCfg_trace_OPloglevel, "--type=json").Output()
		o.Expect(output).To(o.ContainSubstring("Unsupported value: \"trace\""))
	})
})
