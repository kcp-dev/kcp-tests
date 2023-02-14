package apibinding

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/apiexports]", func() {
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

	g.It("Author:zxiao-Critical-[KCP] Verify if APIBinding binds with exported custom resource", func() {
		g.By("# Create a test workspace")
		k.SetupWorkSpace()

		g.By("# Create cowboy custom resource APIResourceSchema")
		rsTemplate := exutil.FixturePath("testdata", "apibinding", "api_rs.yaml")
		_, err := exutil.CreateResourceFromTemplate(k, rsTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create APIExport for custom resource")
		exportTemplate := exutil.FixturePath("testdata", "apibinding", "api_export.yaml")
		_, err = exutil.CreateResourceFromTemplate(k, exportTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		// BUG: https://github.com/kcp-dev/kcp/issues/1939
		g.By("# BUG: apply role binding hack to allow api-binding for non-admin user")
		roleHackTemplate := exutil.FixturePath("testdata", "apibinding", "role_hack.yaml")
		_, err = exutil.CreateResourceFromTemplate(k, roleHackTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create APIBinding for custom resource cowboy")
		bindingTemplate := exutil.FixturePath("testdata", "apibinding", "api_binding.yaml")
		_, err = exutil.CreateResourceFromTemplate(k, bindingTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Check if custom resource is available")
		output, err := k.Run("api-resources").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("cowboy"))

		g.By("# Create cowboy custom resource object")
		cowboyTemplate := exutil.FixturePath("testdata", "apibinding", "cowboy.yaml")
		_, err = exutil.CreateResourceFromTemplate(k, cowboyTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Author:zxiao-Critical-[API] Verify that user can perform wildcard search via APIExport virtual workspace", func() {
		g.By("# Create a test workspace")
		k.SetupWorkSpace()

		g.By("# Create custom resource cowboy using APIResourceSchema")
		rsTemplate := exutil.FixturePath("testdata", "apibinding", "api_rs.yaml")
		_, err := exutil.CreateResourceFromTemplate(k, rsTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create APIExport for custom resource ")
		exportTemplate := exutil.FixturePath("testdata", "apibinding", "api_export.yaml")
		_, err = exutil.CreateResourceFromTemplate(k, exportTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Get APIExport virtual workspace URL")
		workspaceURL, err := k.Run("get").Args("apiexport.apis.kcp.dev/today-cowboys", "-o", "jsonpath={.status.virtualWorkspaces[*].url}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// append wildcard cluster search URL
		workspaceURL = workspaceURL + "/clusters/*/"

		g.By("# Execute virtual workspace URL wildcard search")
		output, err := k.WithoutWorkSpaceServer().Run("--server=" + workspaceURL).Args("api-resources").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("wildwest.dev/v1alpha1"))
	})
})
