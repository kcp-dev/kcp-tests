package workloads

import (
	"os/exec"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cli] Workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("ocmustgather", exutil.KubeConfigPath())
	)

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-45694-Support to collect olm data in must-gather [Slow]", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("Check if logging operator installed or not")
		err := oc.AsAdmin().Run("get").Args("project", "openshift-logging").Execute()
		if err != nil {
			g.Skip("Skip for no logging operator installed")
		}

		g.By("run the must-gather")
		defer exec.Command("bash", "-c", "rm -rf /tmp/must-gather-45694").Output()
		msg, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("-n", oc.Namespace(), "must-gather", "--dest-dir=/tmp/must-gather-45694").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mustGather := string(msg)
		checkMessage := []string{
			"namespaces/openshift-logging/operators.coreos.com/installplans",
			"namespaces/openshift-logging/operators.coreos.com/operatorconditions",
			"namespaces/openshift-logging/operators.coreos.com/operatorgroups",
			"namespaces/openshift-logging/operators.coreos.com/subscriptions",
		}
		for _, v := range checkMessage {
                        if !strings.Contains(mustGather, v) {
                                e2e.Failf("Failed to check the olm data: " + v)
                        }
                }
	})

})
