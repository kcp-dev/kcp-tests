package quota

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/quota]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-quota")
	)

	g.It("Author:zxiao-Critical-[API] Verify that quota works for cluster-scoped resources across all namespaces in the workspace", func() {
		g.By("# Create test workspace")
		k.SetupWorkSpace()

		g.By("# List secrets, config-map resources")
		err := k.Run("get").Args("cm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = k.Run("get").Args("secrets").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create quota under the current workspace")
		quotaTemplate := exutil.FixturePath("testdata", "quota", "quota.yaml")
		exutil.CreateResourceFromTemplateWithVariables(k, quotaTemplate, map[string]string{
			"NUM_CONFIGMAP": "20",
			"NUM_SECRET":    "20",
		})

		g.By("Create a few secrets and configmaps in default namespace.")
		for i := 1; i <= 21; i++ {
			err = k.WithoutNamespace().Run("create").Args("secret", "generic", fmt.Sprintf("e2e-quota-secret-%d", i), "--from-literal", "abcdefg='12345^&*()'").Execute()
			if i >= 20 {
				o.Expect(err).To(o.HaveOccurred())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			err = k.WithoutNamespace().Run("create").Args("configmap", fmt.Sprintf("e2e-quota-configmap-%d", i), "--from-literal", fmt.Sprintf("test.key-%d=value-%d", i, i)).Execute()
			if i >= 20 {
				o.Expect(err).To(o.HaveOccurred())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		g.By("# Increase the upper quota, create a few secrets and configmaps to exceed the upper limit")
		exutil.ApplyResourceFromTemplateWithVariables(k, quotaTemplate, map[string]string{
			"NUM_CONFIGMAP": "25",
			"NUM_SECRET":    "25",
		})

		for i := 20; i <= 26; i++ {
			err = k.WithoutNamespace().Run("create").Args("secret", "generic", fmt.Sprintf("e2e-quota-secret-%d", i), "--from-literal", "abcdefg='12345^&*()'").Execute()
			if i >= 25 {
				o.Expect(err).To(o.HaveOccurred())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			err = k.WithoutNamespace().Run("create").Args("configmap", fmt.Sprintf("e2e-quota-configmap-%d", i), "--from-literal", fmt.Sprintf("test.key-%d=value-%d", i, i)).Execute()
			if i >= 25 {
				o.Expect(err).To(o.HaveOccurred())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		g.By("# Delete quota limit")
		err = k.Run("delete").Args("-f", quotaTemplate).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
