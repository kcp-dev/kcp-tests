package syncer

import (
	"os"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/transparent-multi-cluster]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-syncer")
	)

	g.It("Author:pewang-Critical-[Smoke][BYO] Validate creating, modifying and deleting a deployment from KCP get synced to the pcluster", func() {
		pclusterKubeconfig := os.Getenv("PCLUSTER_KUBECONFIG")
		if pclusterKubeconfig == "" {
			g.Skip("No pcluster kubeconfig set for the test scenario")
		}
		k.SetupWorkSpaceWithNamespace()
		myWs := k.WorkSpace()
		k.SetPClusterKubeconf(pclusterKubeconfig)

		g.By("# Create workload sync and generate syncer resources manifests in current workspace")
		mySyncer := NewSyncTarget()
		mySyncer.OutputFilePath = "/tmp/" + myWs.Name + "." + mySyncer.Name + ".yaml"
		mySyncer.Create(k)
		defer mySyncer.Clean(k)

		g.By("# Apply syncer resources on pcluster and wait for synctarget become ready")
		defer k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("delete").Args("-f", mySyncer.OutputFilePath).Execute()
		err := k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("apply").Args("-f", mySyncer.OutputFilePath).Execute()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		mySyncer.WaitUntilReady(k)
		mySyncer.CheckDisplayColumns(k)

		g.By("# Creating workload using the BYO compute should work well")
		myDeployment := exutil.NewDeployment(exutil.SetDeploymentNameSpace(myWs.CurrentNameSpace))
		defer myDeployment.Clean(k)
		myDeployment.Create(k)
		myDeployment.WaitUntilReady(k)
		myDeployment.CheckDisplayColumns(k)

		g.By("# Check the deployment's status on pcluster")
		avaiablableReplicasOnPcluster, getError := k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("get").Args("deployment", "-A", `-o=jsonpath={.items[?(@.metadata.name=="`+myDeployment.Name+`")].status.availableReplicas}`).Output()
		o.Expect(getError).ShouldNot(o.HaveOccurred())
		o.Expect(avaiablableReplicasOnPcluster).Should(o.Equal("1"))

		g.By("# Scale up the deployment replicas and wait for scaling up successfully")
		myDeployment.ScaleReplicas(k, "10")
		myDeployment.WaitUntilReady(k)

		g.By("# Check the deployment replicas number on pcluster is as expected")
		avaiablableReplicasOnPcluster, getError = k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("get").Args("deployment", "-A", `-o=jsonpath={.items[?(@.metadata.name=="`+myDeployment.Name+`")].status.availableReplicas}`).Output()
		o.Expect(getError).ShouldNot(o.HaveOccurred())
		o.Expect(avaiablableReplicasOnPcluster).Should(o.Equal("10"))

		g.By("# Delete the workload and verify the deployment is no longer present in the pcluster")
		myDeployment.Delete(k)
		// Wait for the deployment deleted under the workspace's namespace
		o.Eventually(func() string {
			depInfo, _ := k.WithoutNamespace().WithoutKubeconf().Run("get").Args("deployment", myDeployment.Name, "-n", myDeployment.Namespace).Output()
			return depInfo
		}, 180*time.Second, 5*time.Second).Should(o.ContainSubstring("not found"))
		// Verify the deployment is no longer present in the pcluster
		o.Eventually(func() string {
			allDeploys, _ := k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("get").Args("deployment", "-A", `-o=jsonpath={.items.*.metadata.name}`).Output()
			return allDeploys
		}, 180*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(myDeployment.Name))
	})
})
