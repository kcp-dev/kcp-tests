package workspacetype

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-workspace]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-workspace")
	)

	g.It("Author:pewang-Medium-[Smoke] Multi levels workspaces lifecycle should works well", func() {
		g.By("# Create a test workspace under user home workspace should become ready to use")
		k.SetupWorkSpace()
		myWorkSpace := k.WorkSpace()

		g.By("# Create a child level workspace should become ready to use")
		k.SetupWorkSpaceWithSpecificPath(myWorkSpace.ServerURL)
		mySubWorkSpace := k.WorkSpace()

		g.By("# From the home workspace could get the test workspace but couldn't get its child level workspace")
		output, err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ParentServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(myWorkSpace.Name),
			o.ContainSubstring(myWorkSpace.ServerURL),
		))
		o.Expect(output).ShouldNot(o.ContainSubstring(mySubWorkSpace.Name))

		g.By("# From the test workspace could get its child workspace")
		output, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(mySubWorkSpace.Name),
			o.ContainSubstring(mySubWorkSpace.ServerURL),
		))

		g.By("# Delete the child workspace and the test workspace should be successful")
		// Delete the child workspace and check it deleted successfully
		output, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("delete").Args("--server="+myWorkSpace.ServerURL, "workspace", mySubWorkSpace.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("deleted"))
		o.Eventually(func() string {
			workSpaces, _ := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ServerURL, "workspace").Output()
			return workSpaces
		}, 60*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(mySubWorkSpace.Name))

		// Delete the test workspace under home workspace and check it deleted successfully
		output, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("delete").Args("--server="+myWorkSpace.ParentServerURL, "workspace", myWorkSpace.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("deleted"))
		o.Eventually(func() string {
			workSpaces, _ := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ParentServerURL, "workspace").Output()
			return workSpaces
		}, 60*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(myWorkSpace.Name))
	})
})
