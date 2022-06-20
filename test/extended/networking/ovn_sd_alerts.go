package networking

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"strings"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-alerts", exutil.KubeConfigPath())

	g.It("Author:weliang-Medium-51438-Upgrade NoRunningOvnMaster to critical severity and inclue runbook.", func() {
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}

		alertName, NameErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[*].alert}").Output()
		o.Expect(NameErr).NotTo(o.HaveOccurred())
		e2e.Logf("The alertName is %v", alertName)
		o.Expect(alertName).To(o.ContainSubstring("NoRunningOvnMaster"))

		alertSeverity, severityErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[?(@.alert==\"NoRunningOvnMaster\")].labels.severity}").Output()
		o.Expect(severityErr).NotTo(o.HaveOccurred())
		e2e.Logf("alertSeverity is %v", alertSeverity)
		o.Expect(alertSeverity).To(o.ContainSubstring("critical"))

		alertRunbook, runbookErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[?(@.alert==\"NoRunningOvnMaster\")].annotations.runbook_url}").Output()
		o.Expect(runbookErr).NotTo(o.HaveOccurred())
		e2e.Logf("The alertRunbook is %v", alertRunbook)
		o.Expect(alertRunbook).To(o.ContainSubstring("https://github.com/openshift/runbooks/blob/master/alerts/cluster-network-operator/NoRunningOvnMaster.md"))
	})

	g.It("Author:weliang-Medium-51439-Upgrade NoOvnMasterLeader to critical severity and inclue runbook.", func() {
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}

		alertName, NameErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[*].alert}").Output()
		o.Expect(NameErr).NotTo(o.HaveOccurred())
		e2e.Logf("The alertName is %v", alertName)
		o.Expect(alertName).To(o.ContainSubstring("NoOvnMasterLeader"))

		alertSeverity, severityErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[?(@.alert==\"NoOvnMasterLeader\")].labels.severity}").Output()
		o.Expect(severityErr).NotTo(o.HaveOccurred())
		e2e.Logf("alertSeverity is %v", alertSeverity)
		o.Expect(alertSeverity).To(o.ContainSubstring("critical"))

		alertRunbook, runbookErr := oc.AsAdmin().Run("get").Args("prometheusrule", "-n", "openshift-ovn-kubernetes", "master-rules", "-o=jsonpath={.spec.groups[*].rules[?(@.alert==\"NoOvnMasterLeader\")].annotations.runbook_url}").Output()
		o.Expect(runbookErr).NotTo(o.HaveOccurred())
		e2e.Logf("The alertRunbook is %v", alertRunbook)
		o.Expect(alertRunbook).To(o.ContainSubstring("https://github.com/openshift/runbooks/blob/master/alerts/cluster-network-operator/NoOvnMasterLeader.md"))
	})
})
