package kcpsanity

import (
	"math/rand"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

var _ = g.Describe("[kcp] Kcpsanity", func() {
	defer g.GinkgoRecover()

	var (
		kubectl = exutil.NewCLIWithoutNamespace("default")
	)

	g.It("Author:knarra-Medium-2800712-Checking oc version show clean as gitTreeState value", func() {
		out, err := kubectl.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("output is:%v", out)

	})

        g.It("Author:knarra-Medium-2800713-Create workspace", func() {
               out, err := kubectl.Run("config").Args("get-contexts").Output()
               o.Expect(err).NotTo(o.HaveOccurred())
               e2e.Logf("Context is:%v", out)
               err = kubectl.Run("config").Args("use-context", "kcp-stable").Execute()
               o.Expect(err).NotTo(o.HaveOccurred())
               createWs, err := kubectl.WithoutNamespace().WithoutKubeconf().Run("kcp").Args("workspace", "create", "qetest2", "--enter").Output()
               o.Expect(err).NotTo(o.HaveOccurred())
               o.Expect(createWs).To(o.ContainSubstring("root:rh-sso-15878744:qetest2"))


        })
})
