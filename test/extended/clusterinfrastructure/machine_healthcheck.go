package clusterinfrastructure

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("machine-healthcheck", exutil.KubeConfigPath())
	)

	// author: huliu@redhat.com
	g.It("Author:huliu-Low-45343-[MHC] - nodeStartupTimeout in MachineHealthCheck should revert back to default [Flaky]", func() {
		g.By("Get the default nodeStartupTimeout")
		nodeStartupTimeoutBeforeUpdate, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMHC, "machine-api-termination-handler", "-o=jsonpath={.spec.nodeStartupTimeout}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("before update --- nodeStartupTimeout: " + nodeStartupTimeoutBeforeUpdate)

		g.By("Update nodeStartupTimeout to 30m")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(mapiMHC, "machine-api-termination-handler", "-n", machineAPINamespace, "-p", `{"spec":{"nodeStartupTimeout":"30m"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait nodeStartupTimeout revert back to default itself")
		err = wait.Poll(30*time.Second, 360*time.Second, func() (bool, error) {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMHC, "machine-api-termination-handler", "-o=jsonpath={.spec.nodeStartupTimeout}", "-n", machineAPINamespace).Output()
			if output == "30m" {
				e2e.Logf("nodeStartupTimeout is not changed back and waiting up to 30 seconds ...")
				return false, nil
			}
			e2e.Logf("nodeStartupTimeout is changed back")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check mhc failed"))

		g.By("Get the nodeStartupTimeout should revert back to default")
		nodeStartupTimeoutAfterUpdate, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(mapiMHC, "machine-api-termination-handler", "-o=jsonpath={.spec.nodeStartupTimeout}", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("after update --- nodeStartupTimeout: " + nodeStartupTimeoutAfterUpdate)

		o.Expect(nodeStartupTimeoutAfterUpdate == nodeStartupTimeoutBeforeUpdate).To(o.BeTrue())
	})
})
