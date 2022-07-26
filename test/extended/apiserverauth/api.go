package apiserverauth

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-API_Server]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithoutNamespace("kcp-api")
	)

	g.It("Author:knarra-Medium-1001-[Smoke] Checking apiserver version should display correctly", func() {
		g.By("# Check the apiserver version display correctly")
		out, err := k.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		kcpServerVersion := gjson.Get(out, `serverVersion.gitVersion`)
		o.Expect(kcpServerVersion).Should(o.ContainSubstring("kcp-v"))
		if strings.Contains(k.WorkSpace().ServerURL, "kcp-stable.apps") {
			o.Expect(kcpServerVersion).ShouldNot(o.ContainSubstring("v0.0.0"))
		}
	})

	g.It("Author:knarra-Medium-1002-[Smoke] Multi levels workspaces lifecycle should works well", func() {
		g.By("# Create a test workspace should become ready to use")
		k.SetupWorkSpace()
		myWorkSpace := k.WorkSpace()

		g.By("# Create a child level workspace should become ready to use")
		k.CreateWorkSpace()
		mySubWorkSpace := k.WorkSpace()

		g.By("# Create the home workspace could get the test workspace but couldn't get its child level workspace")
		output, err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ParentServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(myWorkSpace.Name),
			o.ContainSubstring(myWorkSpace.ServerURL),
		))
		o.Expect(output).ShouldNot(o.ContainSubstring(mySubWorkSpace.Name))

		g.By("# Create the child level workspace could get from parent workspace")
		output, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+myWorkSpace.ServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring(mySubWorkSpace.Name),
			o.ContainSubstring(mySubWorkSpace.ServerURL),
		))
	})
})
