package cloudcredential

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cco] Cluster_Operator CCO should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("default-cco", exutil.KubeConfigPath())
		modeInMetric string
	)

	// author: lwan@redhat.com
	// It is destructive case, will remove root credentials, so adding [Disruptive]. The case duration is greater than 5 minutes
	// so adding [Slow]
	g.It("Author:lwan-High-31768-Report the mode of cloud-credential operation as a metric [Slow][Disruptive]", func() {
		g.By("Check if the current platform is a supported platform")
		rootSecretName, err := GetRootSecretName(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if rootSecretName == "" {
			e2e.Logf("unsupported platform, there is no root credential in kube-system namespace,  will pass the test")
		} else {
			g.By("Check if cco mode in metric is the same as cco mode in cluster resources")
			g.By("Get cco mode from Cluster Resource")
			modeInCR, err := GetCloudCredentialMode(oc)
			e2e.Logf("cco mode in cluster CR is %v", modeInCR)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Check if cco mode in Metric is correct")
			err = CheckModeInMetric(oc, modeInCR)
			if err != nil {
				e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
			}
			if modeInCR == "mint" {
				g.By("if cco is in mint mode currently, then run the below test")
				g.By("Check cco mode when cco is in Passathrough mode")
				e2e.Logf("Force cco mode to Passthrough")
				originCCOMode, err := oc.AsAdmin().Run("get").Args("cloudcredential/cluster", "-o=jsonpath={.spec.credentialsMode}").Output()
				if originCCOMode == "" {
					originCCOMode = "\"\""
				}
				patchYaml := `
spec:
  credentialsMode: ` + originCCOMode
				err = oc.AsAdmin().Run("patch").Args("cloudcredential/cluster", "-p", `{"spec":{"credentialsMode":"Passthrough"}}`, "--type=merge").Execute()
				defer func() {
					err := oc.AsAdmin().Run("patch").Args("cloudcredential/cluster", "-p", patchYaml, "--type=merge").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					err = CheckModeInMetric(oc, modeInCR)
					if err != nil {
						e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
					}
				}()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err := GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Check if cco mode in Metric is correct")
				err = CheckModeInMetric(oc, modeInCR)
				if err != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}
				g.By("Check cco mode when root credential is removed when cco is not in manual mode")
				e2e.Logf("remove root creds")
				rootSecretName, err := GetRootSecretName(oc)
				rootSecretYaml, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", rootSecretName, "-n=kube-system", "-o=yaml").OutputToFile("root-secret.yaml")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", rootSecretName, "-n=kube-system").Execute()
				defer func() {
					err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", rootSecretYaml).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from cluster CR")
				modeInCR, err = GetCloudCredentialMode(oc)
				e2e.Logf("cco mode in cluster CR is %v", modeInCR)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Get cco mode from Metric")
				err = CheckModeInMetric(oc, modeInCR)
				if err != nil {
					e2e.Failf("Failed to check cco mode metric after waiting up to 3 minutes, cco mode should be %v, but is %v in metric", modeInCR, modeInMetric)
				}
			}
		}
	})
	//For bug https://bugzilla.redhat.com/show_bug.cgi?id=1940142
	//For bug https://bugzilla.redhat.com/show_bug.cgi?id=1952891
	g.It("Author:lwan-High-45415-[Bug 1940142] Reset CACert to correct path [Disruptive]", func() {
		g.By("Check if it's an osp cluster")
		platformType, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.ToLower(platformType) != "openstack" {
			g.Skip("Skip for non-osp cluster!")
		}
		g.By("Get openstack root credential clouds.yaml field")
		goodCreds, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "openstack-credentials", "-n=kube-system", "-o=jsonpath={.data.clouds\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		goodCredsYaml := `
data:
  clouds.yaml: ` + goodCreds

		g.By("Check cacert path is correct")
		CredsTXT, err := base64.StdEncoding.DecodeString(goodCreds)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check if it's a kuryr cluster")
		if !strings.Contains(string(CredsTXT), "cacert") {
			g.Skip("Skip for non-kuryr cluster!")
		}
		o.Expect(CredsTXT).To(o.ContainSubstring("cacert: /etc/kubernetes/static-pod-resources/configmaps/cloud-config/ca-bundle.pem"))

		g.By("Patch cacert path to an wrong path")
		var filename = "creds_45415.txt"
		err = ioutil.WriteFile(filename, []byte(CredsTXT), 0644)
		defer os.Remove(filename)
		o.Expect(err).NotTo(o.HaveOccurred())
		wrongPath, err := exec.Command("bash", "-c", fmt.Sprintf("sed -i -e \"s/cacert: .*/cacert: path-no-exist/g\" %s && cat %s", filename, filename)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(wrongPath).To(o.ContainSubstring("cacert: path-no-exist"))
		o.Expect(wrongPath).NotTo(o.ContainSubstring("cacert: /etc/kubernetes/static-pod-resources/configmaps/cloud-config/ca-bundle.pem"))
		badCreds := base64.StdEncoding.EncodeToString(wrongPath)
		wrongCredsYaml := `
data:
  clouds.yaml: ` + badCreds
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "openstack-credentials", "-n=kube-system", "--type=merge", "-p", wrongCredsYaml).Execute()
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "openstack-credentials", "-n=kube-system", "--type=merge", "-p", goodCredsYaml).Execute()
			g.By("Wait for the storage operator to recover")
			err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args("co", "storage").Output()
				if err != nil {
					e2e.Logf("Fail to get clusteroperator storage, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
					e2e.Logf("clusteroperator storage is recover to normal:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "clusteroperator storage is not recovered to normal")
		}()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check cco change wrong path to correct one")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "openstack-credentials", "-n=kube-system", "-o=jsonpath={.data.clouds\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		credsTXT, err := base64.StdEncoding.DecodeString(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(credsTXT).To(o.ContainSubstring("cacert: /etc/kubernetes/static-pod-resources/configmaps/cloud-config/ca-bundle.pem"))
		o.Expect(credsTXT).NotTo(o.ContainSubstring("cacert: path-no-exist"))

		g.By("Patch cacert path to an empty path")
		wrongPath, err = exec.Command("bash", "-c", fmt.Sprintf("sed -i -e \"s/cacert: .*/cacert:/g\" %s && cat %s", filename, filename)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(wrongPath).To(o.ContainSubstring("cacert:"))
		o.Expect(wrongPath).NotTo(o.ContainSubstring("cacert: path-no-exist"))
		badCreds = base64.StdEncoding.EncodeToString(wrongPath)
		wrongCredsYaml = `
data:
  clouds.yaml: ` + badCreds
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "openstack-credentials", "-n=kube-system", "--type=merge", "-p", wrongCredsYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check cco remove cacert field when it's value is empty")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "openstack-credentials", "-n=kube-system", "-o=jsonpath={.data.clouds\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		credsTXT, err = base64.StdEncoding.DecodeString(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(credsTXT).NotTo(o.ContainSubstring("cacert:"))

		g.By("recover root credential")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "openstack-credentials", "-n=kube-system", "--type=merge", "-p", goodCredsYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "openstack-credentials", "-n=kube-system", "-o=jsonpath={.data.clouds\\.yaml}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		credsTXT, err = base64.StdEncoding.DecodeString(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(credsTXT).To(o.ContainSubstring("cacert: /etc/kubernetes/static-pod-resources/configmaps/cloud-config/ca-bundle.pem"))
	})
})
