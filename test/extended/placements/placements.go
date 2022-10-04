package placements

import (
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	apb "github.com/kcp-dev/kcp-tests/test/extended/apibinding"
	nsc "github.com/kcp-dev/kcp-tests/test/extended/syncer"
	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/transparent-multi-cluster]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-placements")
	)

	g.It("Author:knarra-Critical-[Regression] Verify default placements for shared compute", func() {
		// Shared compute could be only accessed from dev-provided test environments
		// Skip for non-supported test environments
		exutil.PreCheckEnvSupport(k, "kcp-stable.apps.kcp-internal", "kcp-unstable.apps.kcp-internal")
		myWs := k.WorkSpace()

		g.By("# Validate apibinding creation is successful in user home workspace")
		myAPIBinding := apb.NewAPIBinding(apb.SetAPIBindingReferencePath("root:redhat-acm-compute"), apb.SetAPIBindingReferenceExportName("kubernetes"))
		myAPIBinding.Create(k.WithSpecificWorkSpaceServer(myWs))

		g.By("# Verify that default placement has been created for shared compute")
		defaultPlacementSc, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("placement", "default", "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(defaultPlacementSc).Should(o.And(
			o.ContainSubstring("locationWorkspace"),
			o.ContainSubstring("root:redhat-acm-compute"),
		))

		g.By("# Verify that APIBinding will be annotated with workload.kcp.dev/skip-default-object-creation")
		apiBindingAnnotation, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("apibinding", myAPIBinding.Metadata.Name, "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiBindingAnnotation).Should(o.ContainSubstring("workload.kcp.dev/skip-default-object-creation"))

		g.By("# Verify creating a namespace in the current workspace")
		k.WithoutKubeconf().Run("create").Args("namespace", "test52823").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Verify that it randomly select a location and bind to all namespaces in this workspace")
		namespaces := []string{"default", "test52823"}
		for _, namespace := range namespaces {
			out, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("namespace", namespace, "-o", "json").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.And(
				o.ContainSubstring("scheduling.kcp.dev/placement"),
				o.ContainSubstring("state.workload.kcp.dev/"),
				o.ContainSubstring("Sync"),
			))
		}
	})

	g.It("Author:knarra-Critical-[Regression] Verify default placements for BYO", func() {
		g.By("# verify default placement has been created for BYO clusters")
		pclusterKubeconfig := os.Getenv("PCLUSTER_KUBECONFIG")
		if pclusterKubeconfig == "" {
			g.Skip("No pcluster kubeconfig set for the test scenario")
		}

		k.SetupWorkSpaceWithNamespace()
		myWs := k.WorkSpace()
		k.SetPClusterKubeconf(pclusterKubeconfig)

		g.By("# Create workload sync and generate syncer resources manifests in current workspace")
		mySyncer := nsc.NewSyncTarget()
		mySyncer.OutputFilePath = "/tmp/" + myWs.Name + "." + mySyncer.Name + ".yaml"
		mySyncer.Create(k)
		defer mySyncer.Clean(k)

		g.By("# Apply syncer resources on pcluster and wait for synctarget become ready")
		defer k.AsPClusterKubeconf().WithoutNamespace().Run("delete").Args("-f", mySyncer.OutputFilePath).Execute()
		err := k.AsPClusterKubeconf().WithoutNamespace().Run("apply").Args("-f", mySyncer.OutputFilePath).Execute()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		mySyncer.WaitUntilReady(k)

		g.By("# Verify that default placement has been created for BYO cluster")
		default_placement_byo, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("placement", "default", "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(default_placement_byo).Should(o.And(
			o.ContainSubstring("locationWorkspace"),
			o.ContainSubstring(myWs.Name),
			o.ContainSubstring("internal.workload.kcp.dev/synctarget"),
		))

		g.By("# Verify that APIBinding will be annotated with workload.kcp.dev/skip-default-object-creation")
		byoApiBindingAnnotation, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("apibinding", "kubernetes", "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(byoApiBindingAnnotation).Should(o.ContainSubstring("workload.kcp.dev/skip-default-object-creation"))

		g.By("# Verify creating a namespace in the current workspace")
		k.WithoutKubeconf().Run("create").Args("namespace", "test528231").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Verify that it randomly select a location and bind to all namespaces in this workspace")
		namespacesByo := []string{"default", "test528231"}
		for _, namespaceByo := range namespacesByo {
			out, err := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("namespace", namespaceByo, "-o", "json").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.And(
				o.ContainSubstring("scheduling.kcp.dev/placement"),
				o.ContainSubstring("state.workload.kcp.dev/"),
				o.ContainSubstring("Sync"),
			))
		}

	})
})
