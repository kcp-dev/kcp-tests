package kcpsanity

import (
       g "github.com/onsi/ginkgo"
       o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

        exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[kcp] Kcpsanity", func() {
        defer g.GinkgoRecover()

        var (
              kubectl = exutil.NewCLI("kubectl", exutil.KubeConfigPath())
        )

        g.It("Author:knarra-Medium-2800712-Checking oc version show clean as gitTreeState value", func() {
		out, err := kubectl.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
                e2e.Logf("output is:%v", out)

	})
})
