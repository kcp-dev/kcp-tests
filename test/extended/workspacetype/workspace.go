package workspacetype

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-workspace]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithoutNamespace("kcp-workspace")
	)

	g.It("Author:pewang-Medium-[Smoke] Multi levels workspaces lifecycle should works well", func() {
		g.By("# Create a test workspace should become ready to use")
		k.SetupWorkSpace()
		myWorkSpace := k.WorkSpace()

		g.By("# Create a child level workspace should become ready to use")
		k.SetupWorkSpaceWithSpecificPath(myWorkSpace.ServerURL)
		mySubWorkSpace := k.WorkSpace()

		g.By("# From the org workspace could get the test workspace but couldn't get its child level workspace")
		output, err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ParentServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(myWorkSpace.Name),
			o.ContainSubstring(strings.TrimPrefix(myWorkSpace.ServerURL, k.OrgServerURL())),
		))
		o.Expect(output).ShouldNot(o.ContainSubstring(mySubWorkSpace.Name))

		g.By("# From the test workspace could get its child workspace")
		output, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(mySubWorkSpace.Name),
			o.ContainSubstring(strings.TrimPrefix(mySubWorkSpace.ServerURL, k.OrgServerURL())),
		))
	})
})
