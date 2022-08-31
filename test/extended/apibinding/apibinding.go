package apibinding

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area-apiexports]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-apibinding")
	)

	g.It("Author:pewang-Critical-[Smoke] Verify APIBinding working with personal workspace", func() {
		// Shared compute could be only accessed from dev-provided test environments
		// Skip for non-supported test environments
		exutil.PreCheckEnvSupport(k, "kcp-stable.apps.kcp-internal", "kcp-unstable.apps.kcp-internal")
		myWs := k.WorkSpace()

		g.By("# Apibinding can't be used in an organization workspace")
		myAPIBinding := NewAPIBinding(SetAPIBindingReferencePath("root:redhat-acm-compute"), SetAPIBindingReferenceExportName("kubernetes"))
		myAPIBinding.CreateAsExpectedResult(k.WithOrgWorkSpaceServer(), false, `cannot get resource "apibindings" in API group "apis.kcp.dev" at the cluster scope`)

		g.By("# Validate apibinding creation is successful in user home workspace")
		myAPIBinding.Create(k.WithSpecificWorkSpaceServer(myWs))

		g.By("# Create workload using the shared compute provided by ACM should work well")
		myDeployment := exutil.NewDeployment()
		myDeployment.Create(k)
		myDeployment.WaitUntilReady(k)

		g.By("# Check the deployment's finalizers and state should be correct")
		labels, err := myDeployment.GetFieldByJSONPath(k, "{.metadata.labels}")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(labels).Should(o.And(
			o.ContainSubstring("state.workload.kcp.dev"),
			o.ContainSubstring("Sync"),
		))
		finalizers, err := myDeployment.GetFieldByJSONPath(k, "{.metadata.finalizers}")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(finalizers).Should(o.ContainSubstring("workload.kcp.dev/syncer"))
	})
})
