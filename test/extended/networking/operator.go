package networking

import (
	"fmt"
	"os"
	"os/exec"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-operator", exutil.KubeConfigPath())

	// author: jechen@redhat.com
	g.It("Author:jechen-Medium-44954-Newline is added between user CAs and system CAs [Disruptive]", func() {
		var (
			dirname  = "/tmp/OCP-44954"
			name     = dirname + "OCP-44954-custom"
			validity = 3650
			ca_subj  = dirname + "/OU=openshift/CN=admin-kubeconfig-signer-custom"
		)

		// Generation of a new self-signed CA
		g.By("1.  Generation of a new self-signed CA")
		err := os.MkdirAll(dirname, 0777)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(dirname)
		e2e.Logf("Generate the CA private key")
		openssl_cmd := fmt.Sprintf(`openssl genrsa -out %s-ca.key 4096`, name)
		err = exec.Command("bash", "-c", openssl_cmd).Run()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create the CA certificate")
		openssl_cmd = fmt.Sprintf(`openssl req -x509 -new -nodes -key %s-ca.key -sha256 -days %d -out %s-ca.crt -subj %s`, name, validity, name, ca_subj)
		err = exec.Command("bash", "-c", openssl_cmd).Run()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2. Create a configmap from the CA")
		configmapName := "custom-ca"
		customCA := "--from-file=ca-bundle.crt=" + name + "-ca.crt"
		e2e.Logf("\n customCA is  %v", customCA)
		_, error := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", configmapName, customCA, "-n", "openshift-config").Output()
		o.Expect(error).NotTo(o.HaveOccurred())
		defer func() {
			_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", configmapName, "-n", "openshift-config").Output()
			o.Expect(error).NotTo(o.HaveOccurred())
		}()

		g.By("3. Check if configmap is successfully configured in openshift-config namesapce")
		err = checkConfigMap(oc, "openshift-config", configmapName)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %v not found", configmapName))

		g.By("4. Patch the configmap created above to proxy/cluster")
		patchResourceAsAdmin(oc, "proxy/cluster", "{\"spec\":{\"trustedCA\":{\"name\":\"custom-ca\"}}}")
		defer patchResourceAsAdmin(oc, "proxy/cluster", "{\"spec\":{\"trustedCA\":{\"name\":\"\"}}}")

		g.By("5. Verify that a newline is added between custom user CAs and system CAs")
		ns := "openshift-config-managed"
		// get system CAs
		outputFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "-n", ns, "trusted-ca-bundle", "-o", "yaml").OutputToFile("trusted-ca")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(outputFile)

		// get the custom user CA in byte array
		certArray, err := exec.Command("bash", "-c", "cat "+name+"-ca.crt").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// get the ending portion the custom user CA in byte array
		certArrayPart := certArray[len(certArray)-35 : len(certArray)-30]

		// grep in the trusted-ca-bundle by the ending portion of the custom user CAs, get 4 lines after
		output, err := exec.Command("bash", "-c", "cat "+outputFile+" | grep "+string(certArrayPart)+" -A 4").Output()
		e2e.Logf("\nouput string is  --->%s<----", string(output))
		stringToMatch := string(certArrayPart) + ".+\n.*-----END CERTIFICATE-----\n\n.+\n.+-----BEGIN CERTIFICATE-----"
		o.Expect(output).To(o.MatchRegexp(stringToMatch))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

})
