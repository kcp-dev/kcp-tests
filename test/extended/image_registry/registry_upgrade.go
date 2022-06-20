package imageregistry

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("default-registry-upgrade", exutil.KubeConfigPath())
	)
	// author: wewang@redhat.com
	g.It("NonPreRelease-PreChkUpgrade-Author:wewang-High-26401-Upgrade cluster with insecureRegistries and blockedRegistries defined prepare [Disruptive]", func() {
		g.By("Add insecureRegistries and blockedRegistries to image.config")
		output, err := oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec":{"registrySources":{"blockedRegistries": ["untrusted.com"],"insecureRegistries": ["insecure.com"]}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("registries.conf gets updated")
		workNode, _ := exutil.GetFirstWorkerNode(oc)
		err = wait.Poll(30*time.Second, 8*time.Minute, func() (bool, error) {
			registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf |grep -E '\"untrusted.com\"|\"insecure.com\"'")
			if strings.Contains(registriesstatus, "location = \"untrusted.com\"") && strings.Contains(registriesstatus, "location = \"insecure.com\"") {
				e2e.Logf("registries.conf updated")
				return true, nil
			}
			e2e.Logf("registries.conf not update")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "registries.conf not contains registrysources")
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:wewang-High-26401-Upgrade cluster with insecureRegistries and blockedRegistries defined after upgrade [Disruptive]", func() {
		g.By("registries.conf gets updated")
		defer oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"registrySources": null}}`, "--type=merge").Execute()
		workNode, _ := exutil.GetFirstWorkerNode(oc)
		registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf | grep -E '\"untrusted.com\"|\"insecure.com\"'")
		if strings.Contains(registriesstatus, "location = \"untrusted.com\"") && strings.Contains(registriesstatus, "location = \"insecure.com\"") {
			e2e.Logf("registries.conf updated")
		} else {
			e2e.Failf("registries.conf not update")
		}
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PreChkUpgrade-Author:wewang-High-41400-Users providing custom AWS tags are set with bucket creation prepare", func() {
		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check the cluster is with resourceTags")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.aws}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "resourceTags") {
			g.Skip("Skip for no resourceTags")
		}
		g.By("Get bucket name")
		bucket, _ := oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()
		o.Expect(bucket).NotTo(o.BeEmpty())

		g.By("Check the tags")
		aws := getAWSClient(oc)
		tag, err := awsGetBucketTagging(aws, bucket)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))
	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:wewang-High-41400- Users providing custom AWS tags are set with bucket creation after upgrade", func() {
		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check the cluster is with resourceTags")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.aws}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "resourceTags") {
			g.Skip("Skip for no resourceTags")
		}
		g.By("Get bucket name")
		bucket, _ := oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()
		o.Expect(bucket).NotTo(o.BeEmpty())

		g.By("Check the tags")
		aws := getAWSClient(oc)
		tag, err := awsGetBucketTagging(aws, bucket)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))
	})

	// author: xiuwang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:xiuwang-Medium-45346-Payload imagestream new tags should properly updated during cluster upgrade prepare", func() {
		g.By("Check payload imagestream")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("is", "-n", "openshift", "-l", "samples.operator.openshift.io/managed!=true", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		plimage := strings.Split(output, " ")
		for _, isname := range plimage {
			output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("is", isname, "-n", "openshift", "-o=jsonpath={.spec.tags[*].name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			tagname := strings.Split(output, " ")
			for _, tname := range tagname {
				e2e.Logf("tag is %s", tname)
				if tname == "" {
					e2e.Failf("The imagestream %s is broken after upgrade", isname)
				}
			}
		}
	})

	// author: xiuwang@redhat.com
	g.It("NonPreRelease-PreChkUpgrade-Author:xiuwang-Critial-24345-Set proxy in Image-registry-operator before upgrade", func() {
		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, noProxy := saveGlobalProxy(oc)
		if !strings.Contains(httpProxy, "http") {
			g.Skip("Skip for non-proxy platform")
		}

		g.By("Check if registry operator degraded")
		registryDegrade := checkRegistryDegraded(oc)
		if registryDegrade {
			e2e.Failf("Image registry is degraded")
		}

		g.By("Set image-registry proxy setting")
		err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"proxy":{"http": "`+httpProxy+`","https":"`+httpsProxy+`","noProxy":"`+noProxy+`"}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
			registryDegrade := checkRegistryDegraded(oc)
			if registryDegrade {
				e2e.Logf("wait for next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is degraded")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "prepare-24345", oc.Namespace())
	})

	// author: xiuwang@redhat.com
	g.It("NonPreRelease-PstChkUpgrade-Author:xiuwang-Critial-24345-Set proxy in Image-registry-operator after upgrade", func() {
		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, noProxy := saveGlobalProxy(oc)
		if !strings.Contains(httpProxy, "http") {
			g.Skip("Skip for non-proxy platform")
		}

		g.By("Check if registry operator degraded")
		registryDegrade := checkRegistryDegraded(oc)
		if registryDegrade {
			e2e.Failf("Image registry is degraded")
		}

		g.By("Check image-registry proxy setting")
		Output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.proxy}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(Output, httpProxy) || !strings.Contains(Output, httpsProxy) || !strings.Contains(Output, noProxy) {
			e2e.Failf("http proxy is not same")
		}

		oc.SetupProject()
		checkRegistryFunctionFine(oc, "check-24345", oc.Namespace())
	})

})
