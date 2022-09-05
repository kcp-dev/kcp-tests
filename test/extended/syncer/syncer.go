package syncer

import (
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	apb "github.com/kcp-dev/kcp-tests/test/extended/apibinding"
	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/transparent-multi-cluster]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-syncer")
	)

	g.It("Author:pewang-Critical-[Smoke][BYO] Validate create, modifying, deleting a deployment from KCP gets synced to the pcluster", func() {
		pclusterKubeconfig := os.Getenv("PCLUSTER_KUBECONFIG")
		if pclusterKubeconfig == "" {
			g.Skip("None pcluster kubeconfig set for the test scenario")
		}
		myWs := k.WorkSpace()
		k.SetGuestKubeconf(pclusterKubeconfig)

		g.By("# Create workload sync and generate syncer resources manifests in current workspace")
		mySyncer := NewSyncTarget()
		mySyncer.OutputFilePath = "/tmp/" + myWs.Name + "." + mySyncer.Name + ".yaml"
		mySyncer.Create(k)
		defer mySyncer.Clean(k)

		g.By("# Apply syncer resources on pcluster and wait for synctarget become ready")
		defer k.AsGuestKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("delete").Args("-f", mySyncer.OutputFilePath).Execute()
		err := k.AsGuestKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("apply").Args("-f", mySyncer.OutputFilePath).Execute()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		mySyncer.WaitUntilReady(k)
		mySyncer.CheckDisplayAttributes(k)

		g.By("# Create a new workspace and create an APIBinding to attach the compute to the new workspace")
		k.SetupWorkSpace()
		tempSlice := strings.Split(myWs.ServerURL, "/")
		myAPIBinding := apb.NewAPIBinding(apb.SetAPIBindingReferencePath(tempSlice[len(tempSlice)-1]), apb.SetAPIBindingReferenceExportName("kubernetes"))
		myAPIBinding.Create(k)
		defer myAPIBinding.Clean(k)

		g.By("# Create workload using the shared compute should work well")
		myDeployment := exutil.NewDeployment()
		myDeployment.Create(k)
		defer myDeployment.Clean(k)
		myDeployment.WaitUntilReady(k)
		myDeployment.CheckDisplayAttributes(k)

		g.By("# Scale up the deployment replicas and wait for scaling up successfully")
		myDeployment.ScaleReplicas(k, "10")
		myDeployment.WaitUntilReady(k)

		g.By("# Check the deployment replicas number on pcluster is as expected")
		actualReplicas, getError := k.AsGuestKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("get").Args("deployment", "-A", `-o=jsonpath={.items[?(@.metadata.name=="`+myDeployment.Name+`")].spec.replicas}`).Output()
		o.Expect(getError).ShouldNot(o.HaveOccurred())
		o.Expect(actualReplicas).Should(o.Equal("10"))

		g.By("# Delete the workload and verify the deployment is no longer present in the pcluster")
		myDeployment.Delete(k)
		// Wait for the deployment deleted under the workspace's namespace
		o.Eventually(func() string {
			depInfo, _ := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("deployment", myDeployment.Name, "-n", myDeployment.Namespace).Output()
			return depInfo
		}, 180*time.Second, 5*time.Second).Should(o.ContainSubstring("not found"))
		// Verify the deployment is no longer present in the pcluster
		o.Eventually(func() string {
			allDeploys, _ := k.AsGuestKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("get").Args("deployment", "-A", `-o=jsonpath={.items.*.metadata.name}`).Output()
			return allDeploys
		}, 180*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(myDeployment.Name))
	})
})
