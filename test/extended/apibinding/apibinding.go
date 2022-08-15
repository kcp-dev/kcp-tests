package apibinding

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-apibinding]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-apibinding")
	)

	g.It("Author:pewang-Critical-[Smoke] Create an apibinding attach a workspace to the exist shared compute should work well", func() {
		// Share compute could only access from specific test environments
		// Skip for none supported test environments
		exutil.PreCheckEnvSupport(k, "kcp-stable.apps.kcp-internal", "kcp-unstable.apps.kcp-internal")

		g.By("# Create an apibinding attach a workspace to the exist shared compute")
		myAPIBing := NewAPIBinding(SetAPIBindingReferencePath("root:redhat-acm-compute"), SetAPIBindingReferenceExportName("kubernetes"))
		myAPIBing.Create(k)
		defer myAPIBing.Delete(k)

		g.By("# Create an deployment should become ready and schedule to the shared compute")
		myDeployment := exutil.NewDeployment()
		myDeployment.Create(k)
		defer myDeployment.Delete(k)
		myDeployment.WaitReady(k)

		g.By("# Check the deployment finalizers and state")
		labels, err := myDeployment.GetSpecificField(k, "{.metadata.labels}")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(labels).Should(o.And(
			o.ContainSubstring("state.workload.kcp.dev"),
			o.ContainSubstring("Sync"),
		))
		finalizers, err := myDeployment.GetSpecificField(k, "{.metadata.finalizers}")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(finalizers).Should(o.ContainSubstring("workload.kcp.dev/syncer"))
	})
})
