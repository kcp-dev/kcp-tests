package apiserverauth

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-API_Server]", func() {
	defer g.GinkgoRecover()

	var (
		kubectl = exutil.NewCLIWithoutNamespace("default")
	)

	g.It("Author:knarra-Medium-1001-[Smoke] Checking apiserver version should display", func() {
		g.By("# Check the apiserver version gitVersion should exist")
		out, err := kubectl.Run("version").Args("-o", "json").Output()
		o.Expect(gjson.Get(out, `serverVersion.gitVersion`)).Should(o.ContainSubstring("kcp-v"))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Author:knarra-Medium-1002-[Smoke] Create workspace", func() {
		// To do: General admin.kubeconfig no need to switch context
		// g.By("# Switch to stable ENV to test")
		// _, err := kubectl.Run("config").Args("get-contexts").Output()
		// o.Expect(err).NotTo(o.HaveOccurred())
		// err = kubectl.Run("config").Args("use-context", "kcp-stable").Execute()
		// o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create a test workspace should become ready to use")
		wsName := "api-e2e-" + exutil.GetRandomString()
		createWs, err := kubectl.WithoutNamespace().WithoutKubeconf().Run("ws").Args("create", wsName, "--enter").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer kubectl.WithoutNamespace().WithoutKubeconf().Run("delete").Args("workspace", wsName)
		o.Expect(createWs).To(o.ContainSubstring("is ready to use"))
	})
})
