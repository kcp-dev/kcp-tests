package common

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-common-version]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-version")
	)

	g.It("Author:knarra-Medium-[Smoke] Checking kcp server version should display correctly", func() {
		g.By("# Check the kcp server version display correctly")
		out, err := k.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		kcpServerVersion := gjson.Get(out, `serverVersion.gitVersion`)
		o.Expect(kcpServerVersion).Should(o.ContainSubstring("kcp-v"))
		if strings.Contains(k.WorkSpace().ServerURL, "kcp-stable.apps") {
			o.Expect(kcpServerVersion).ShouldNot(o.ContainSubstring("v0.0.0"))
		}
	})
})
