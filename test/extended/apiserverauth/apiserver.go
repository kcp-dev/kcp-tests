package apiserverauth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: kewang@redhat.com
	g.It("Author:kewang-Medium-32383-bug 1793694 init container setup should have the proper securityContext", func() {
		checkItems := []struct {
			namespace string
			container string
		}{
			{
				namespace: "openshift-kube-apiserver",
				container: "kube-apiserver",
			},
			{
				namespace: "openshift-apiserver",
				container: "openshift-apiserver",
			},
		}

		for _, checkItem := range checkItems {
			g.By("Get one pod name of " + checkItem.namespace)
			e2e.Logf("namespace is :%s", checkItem.namespace)
			podName, err := oc.AsAdmin().Run("get").Args("-n", checkItem.namespace, "pods", "-l apiserver", "-o=jsonpath={.items[0].metadata.name}").Output()
			if err != nil {
				e2e.Failf("Failed to get kube-apiserver pod name and returned error: %v", err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Get the kube-apiserver pod name: %s", podName)

			g.By("Get privileged value of container " + checkItem.container + " of pod " + podName)
			jsonpath := "-o=jsonpath={range .spec.containers[?(@.name==\"" + checkItem.container + "\")]}{.securityContext.privileged}"
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)

			g.By("Get privileged value of initcontainer of pod " + podName)
			jsonpath = "-o=jsonpath={.spec.initContainers[].securityContext.privileged}"
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, jsonpath, "-n", checkItem.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("true"))
			e2e.Logf("#### privileged value: %s ####", msg)
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make kube-apiserver roll out, so adding [Disruptive]. One rollout costs about 25mins, so adding [Slow]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-Author:xxia-Medium-25806-Force encryption key rotation for etcd datastore [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		output, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == output {
			g.By("Get encryption prefix")
			var err error
			var oasEncValPrefix1, kasEncValPrefix1 string

			oasEncValPrefix1, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key routes ")
			e2e.Logf("openshift-apiserver resource encrypted value prefix before test is %s", oasEncValPrefix1)

			kasEncValPrefix1, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key secrets ")
			e2e.Logf("kube-apiserver resource encrypted value prefix before test is %s", kasEncValPrefix1)

			var oasEncNumber, kasEncNumber int
			oasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			kasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)

			t := time.Now().Format(time.RFC3339)
			patchYamlToRestore := `[{"op":"replace","path":"/spec/unsupportedConfigOverrides","value":null}]`
			// Below cannot use the patch format "op":"replace" due to it is uncertain
			// whether it is `unsupportedConfigOverrides: null`
			// or the unsupportedConfigOverrides is not existent
			patchYaml := `
spec:
  unsupportedConfigOverrides:
    encryption:
      reason: force OAS rotation ` + t
			for _, kind := range []string{"openshiftapiserver", "kubeapiserver"} {
				defer func() {
					e2e.Logf("Restoring %s/cluster's spec", kind)
					err := oc.WithoutNamespace().Run("patch").Args(kind, "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}()
				g.By("Forcing " + kind + " encryption")
				err := oc.WithoutNamespace().Run("patch").Args(kind, "cluster", "--type=merge", "-p", patchYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			newOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber+1)
			newKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber+1)

			g.By("Check the new encryption key secrets appear")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("secrets", newOASEncSecretName, newKASEncSecretName, "-n", "openshift-config-managed").Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption key secrets, error: %s. Trying again", err)
					return false, nil
				}
				e2e.Logf("Got new encryption key secrets:\n%s", output)
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("new encryption key secrets %s, %s not found", newOASEncSecretName, newKASEncSecretName))

			g.By("Waiting for the force encryption completion")
			// Only need to check kubeapiserver because kubeapiserver takes more time.
			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newKASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))

			var oasEncValPrefix2, kasEncValPrefix2 string
			g.By("Get encryption prefix after force encryption completed")
			oasEncValPrefix2, err = GetEncryptionPrefix(oc, "/openshift.io/routes")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key routes ")
			e2e.Logf("openshift-apiserver resource encrypted value prefix after test is %s", oasEncValPrefix2)

			kasEncValPrefix2, err = GetEncryptionPrefix(oc, "/kubernetes.io/secrets")
			exutil.AssertWaitPollNoErr(err, "fail to get encryption prefix for key secrets ")
			e2e.Logf("kube-apiserver resource encrypted value prefix after test is %s", kasEncValPrefix2)

			o.Expect(oasEncValPrefix2).Should(o.ContainSubstring("k8s:enc:aescbc:v1"))
			o.Expect(kasEncValPrefix2).Should(o.ContainSubstring("k8s:enc:aescbc:v1"))
			o.Expect(oasEncValPrefix2).NotTo(o.Equal(oasEncValPrefix1))
			o.Expect(kasEncValPrefix2).NotTo(o.Equal(kasEncValPrefix1))
		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make kube-apiserver roll out, so adding [Disruptive]. One rollout costs about 25mins, so adding [Slow]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-NonPreRelease-Author:xxia-Medium-25811-Etcd encrypted cluster could self-recover when related encryption configuration is deleted [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		output, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == output {
			uidsOld, err := oc.WithoutNamespace().Run("get").Args("secret", "encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver", "-n", "openshift-config-managed", `-o=jsonpath={.items[*].metadata.uid}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Delete secrets encryption-config-* in openshift-config-managed")
			for _, item := range []string{"encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver"} {
				e2e.Logf("Remove finalizers from secret %s in openshift-config-managed", item)
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-config-managed", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("Delete secret %s in openshift-config-managed", item)
				err = oc.WithoutNamespace().Run("delete").Args("secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			uidsOldSlice := strings.Split(uidsOld, " ")
			e2e.Logf("uidsOldSlice = %s", uidsOldSlice)
			err = wait.Poll(2*time.Second, 60*time.Second, func() (bool, error) {
				uidsNew, err := oc.WithoutNamespace().Run("get").Args("secret", "encryption-config-openshift-apiserver", "encryption-config-openshift-kube-apiserver", "-n", "openshift-config-managed", `-o=jsonpath={.items[*].metadata.uid}`).Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption-config-* secrets, error: %s. Trying again", err)
					return false, nil
				}
				uidsNewSlice := strings.Split(uidsNew, " ")
				e2e.Logf("uidsNewSlice = %s", uidsNewSlice)
				if uidsNewSlice[0] != uidsOldSlice[0] && uidsNewSlice[1] != uidsOldSlice[1] {
					e2e.Logf("Saw recreated secrets encryption-config-* in openshift-config-managed")
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "do not see recreated secrets encryption-config in openshift-config-managed")

			var oasEncNumber, kasEncNumber int
			oasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred())
			kasEncNumber, err = GetEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred())

			oldOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber)
			oldKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber)
			g.By("Delete secrets encryption-key-* in openshift-config-managed")
			for _, item := range []string{oldOASEncSecretName, oldKASEncSecretName} {
				e2e.Logf("Remove finalizers from key %s in openshift-config-managed", item)
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-config-managed", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("Delete secret %s in openshift-config-managed", item)
				err = oc.WithoutNamespace().Run("delete").Args("secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			newOASEncSecretName := "encryption-key-openshift-apiserver-" + strconv.Itoa(oasEncNumber+1)
			newKASEncSecretName := "encryption-key-openshift-kube-apiserver-" + strconv.Itoa(kasEncNumber+1)
			g.By("Check the new encryption key secrets appear")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("secrets", newOASEncSecretName, newKASEncSecretName, "-n", "openshift-config-managed").Output()
				if err != nil {
					e2e.Logf("Fail to get new encryption-key-* secrets, error: %s. Trying again", err)
					return false, nil
				}
				e2e.Logf("Got new encryption-key-* secrets:\n%s", output)
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("new encryption key secrets %s, %s not found", newOASEncSecretName, newKASEncSecretName))

			var completed bool
			completed, err = WaitEncryptionKeyMigration(oc, newOASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newOASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))
			completed, err = WaitEncryptionKeyMigration(oc, newKASEncSecretName)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("saw all migrated-resources for %s", newKASEncSecretName))
			o.Expect(completed).Should(o.Equal(true))

		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: xxia@redhat.com
	// It is destructive case, will make openshift-kube-apiserver and openshift-apiserver namespaces deleted, so adding [Disruptive].
	// In test the recovery costs about 22mins in max, so adding [Slow]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-NonPreRelease-Author:xxia-Medium-36801-Etcd encrypted cluster could self-recover when related encryption namespaces are deleted [Slow][Disruptive]", func() {
		// only run this case in Etcd Encryption On cluster
		g.By("Check if cluster is Etcd Encryption On")
		encryptionType, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if "aescbc" == encryptionType {
			jsonPath := `{.items[?(@.metadata.finalizers[0]=="encryption.apiserver.operator.openshift.io/deletion-protection")].metadata.name}`

			secretNames, err := oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-apiserver", "-o=jsonpath="+jsonPath).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			// These secrets have deletion-protection finalizers by design. Remove finalizers, otherwise deleting the namespaces will be stuck
			e2e.Logf("Remove finalizers from secret %s in openshift-apiserver", secretNames)
			for _, item := range strings.Split(secretNames, " ") {
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			e2e.Logf("Remove finalizers from secret %s in openshift-kube-apiserver", secretNames)
			secretNames, err = oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-kube-apiserver", "-o=jsonpath="+jsonPath).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, item := range strings.Split(secretNames, " ") {
				err := oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-kube-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			var uidsOld string
			uidsOld, err = oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", "openshift-apiserver", `-o=jsonpath={.items[*].metadata.uid}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			uidOldKasNs, uidOldOasNs := strings.Split(uidsOld, " ")[0], strings.Split(uidsOld, " ")[1]

			e2e.Logf("Check openshift-kube-apiserver pods' revisions before deleting namespace")
			oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()
			g.By("Delete namespaces: openshift-kube-apiserver, openshift-apiserver in the background")
			oc.WithoutNamespace().Run("delete").Args("ns", "openshift-kube-apiserver", "openshift-apiserver").Background()
			// Deleting openshift-kube-apiserver may usually need to hang 1+ minutes and then exit.
			// But sometimes (not always, though) if race happens, it will hang forever. We need to handle this as below code
			isKasNsNew, isOasNsNew := false, false
			// In test, observed the max wait time can be 4m, so the parameter is larger
			err = wait.Poll(5*time.Second, 6*time.Minute, func() (bool, error) {
				if !isKasNsNew {
					uidNewKasNs, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", `-o=jsonpath={.metadata.uid}`).Output()
					if err == nil {
						if uidNewKasNs != uidOldKasNs {
							isKasNsNew = true
							oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver").Execute()
							e2e.Logf("New ns/openshift-kube-apiserver is seen")

						} else {
							stuckTerminating, _ := oc.WithoutNamespace().Run("get").Args("ns", "openshift-kube-apiserver", `-o=jsonpath={.status.conditions[?(@.type=="NamespaceFinalizersRemaining")].status}`).Output()
							if stuckTerminating == "True" {
								// We need to handle the race (not always happening) by removing new secrets' finazliers to make namepace not stuck in Terminating
								e2e.Logf("Hit race: when ns/openshift-kube-apiserver is Terminating, new encryption-config secrets are seen")
								secretNames, _, _ := oc.WithoutNamespace().Run("get").Args("secret", "-n", "openshift-kube-apiserver", "-o=jsonpath="+jsonPath).Outputs()
								for _, item := range strings.Split(secretNames, " ") {
									oc.WithoutNamespace().Run("patch").Args("secret", item, "-n", "openshift-kube-apiserver", `-p={"metadata":{"finalizers":null}}`).Execute()
								}
							}
						}
					}
				}
				if !isOasNsNew {
					uidNewOasNs, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-apiserver", `-o=jsonpath={.metadata.uid}`).Output()
					if err == nil {
						if uidNewOasNs != uidOldOasNs {
							isOasNsNew = true
							oc.WithoutNamespace().Run("get").Args("ns", "openshift-apiserver").Execute()
							e2e.Logf("New ns/openshift-apiserver is seen")
						}
					}
				}
				if isKasNsNew && isOasNsNew {
					e2e.Logf("Now new openshift-apiserver and openshift-kube-apiserver namespaces are both seen")
					return true, nil
				}

				return false, nil
			})

			exutil.AssertWaitPollNoErr(err, "new openshift-apiserver and openshift-kube-apiserver namespaces are not both seen")

			// After new namespaces are seen, it goes to self recovery
			err = wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if err == nil {
					matched, _ := regexp.MatchString("True.*True.*(True|False)", output)
					if matched {
						e2e.Logf("Detected self recovery is in progress\n%s", output)
						return true, nil
					}
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "Detected self recovery is not in progress")
			e2e.Logf("Check openshift-kube-apiserver pods' revisions when self recovery is in progress")
			oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()

			// In test the recovery costs about 22mins in max, so the parameter is larger
			err = wait.Poll(10*time.Second, 25*time.Minute, func() (bool, error) {
				output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if err == nil {
					matched, _ := regexp.MatchString("True.*False.*False", output)
					if matched {
						time.Sleep(100 * time.Second)
						output, err := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
						if err == nil {
							if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
								e2e.Logf("co/kubeapiserver True False False already lasts 100s. Means status is stable enough. Recovery completed\n%s", output)
								e2e.Logf("Check openshift-kube-apiserver pods' revisions when recovery completed")
								oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-L=revision").Execute()
								return true, nil
							}
						}
					}
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "openshift-kube-apiserver pods revisions recovery not completed")

			var output string
			output, err = oc.WithoutNamespace().Run("get").Args("co/openshift-apiserver").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			matched, _ := regexp.MatchString("True.*False.*False", output)
			o.Expect(matched).Should(o.Equal(true))

		} else {
			g.By("cluster is Etcd Encryption Off, this case intentionally runs nothing")
		}
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Longduration-Author:rgangwar-Low-25926-Wire cipher config from apiservers/cluster into apiserver and authentication operators [Disruptive] [Slow]", func() {
		// Check authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io
		var (
			cipherToRecover = `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":}]`
			cipherOps       = []string{"openshift-authentication", "openshiftapiservers.operator", "kubeapiservers.operator"}
			cipherToMatch   = `["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"] VersionTLS12`
		)

		cipherItems := []struct {
			cipherType    string
			cipherToCheck string
			patch         string
		}{
			{
				cipherType:    "custom",
				cipherToCheck: `["TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"] VersionTLS11`,
				patch:         `[{"op": "add", "path": "/spec/tlsSecurityProfile", "value":{"custom":{"ciphers":["ECDHE-ECDSA-CHACHA20-POLY1305","ECDHE-RSA-CHACHA20-POLY1305","ECDHE-RSA-AES128-GCM-SHA256","ECDHE-ECDSA-AES128-GCM-SHA256"],"minTLSVersion":"VersionTLS11"},"type":"Custom"}}]`,
			},
			{
				cipherType:    "Intermediate",
				cipherToCheck: cipherToMatch, // cipherSuites of "Intermediate" seems to equal to the default values when .spec.tlsSecurityProfile not set.
				patch:         `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"intermediate":{},"type":"Intermediate"}}]`,
			},
			{
				cipherType:    "Old",
				cipherToCheck: `["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_AES_128_GCM_SHA256","TLS_RSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_128_CBC_SHA256","TLS_RSA_WITH_AES_128_CBC_SHA","TLS_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_3DES_EDE_CBC_SHA"] VersionTLS10`,
				patch:         `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"old":{},"type":"Old"}}]`,
			},
		}

		// Check ciphers for authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io:
		for _, s := range cipherOps {
			err := verifyCiphers(oc, cipherToMatch, s)
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not matched : %s", s))
		}

		//Recovering apiserver/cluster's ciphers:
		defer func() {
			g.By("Restoring apiserver/cluster's ciphers")
			output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipherToRecover).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(output, "patched (no change)") {
				e2e.Logf("Apiserver/cluster's ciphers are not changed from the default values")
			} else {
				for _, s := range cipherOps {
					err := verifyCiphers(oc, cipherToMatch, s)
					exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not restored : %s", s))
				}
				g.By("Checking KAS, OAS, Auththentication operators should be in Progressing and Available after rollout and recovery")
				e2e.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
				expectedStatus := map[string]string{"Progressing": "True"}
				err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")
				e2e.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
				expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				err = waitCoBecomes(oc, "kube-apiserver", 1500, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")

				// Using 60s because KAS takes long time, when KAS finished rotation, OAS and Auth should have already finished.
				e2e.Logf("Checking openshift-apiserver operator should be Available in 60 seconds")
				err = waitCoBecomes(oc, "openshift-apiserver", 60, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "openshift-apiserver operator is not becomes available in 60 seconds")

				e2e.Logf("Checking authentication operator should be Available in 60 seconds")
				err = waitCoBecomes(oc, "authentication", 60, expectedStatus)
				exutil.AssertWaitPollNoErr(err, "authentication operator is not becomes available in 60 seconds")
				e2e.Logf("KAS, OAS and Auth operator are available after rollout and cipher's recovery")
			}
		}()

		// Check custom, intermediate, old ciphers for authentication operator cliconfig, openshiftapiservers.operator.openshift.io and kubeapiservers.operator.openshift.io:
		for _, cipherItem := range cipherItems {
			g.By("Patching the apiserver cluster with ciphers : " + cipherItem.cipherType)
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipherItem.patch).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Calling verify_cipher function to check ciphers and minTLSVersion
			for _, s := range cipherOps {
				err := verifyCiphers(oc, cipherItem.cipherToCheck, s)
				exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Ciphers are not matched: %s : %s", s, cipherItem.cipherType))
			}
			g.By("Checking KAS, OAS, Auththentication operators should be in Progressing and Available after rollout")
			// Calling waitCoBecomes function to wait for define waitTime so that KAS, OAS, Authentication operator becomes progressing and available.
			// WaitTime 100s for KAS becomes Progressing=True and 1500s to become Available=True and Progressing=False and Degraded=False.
			e2e.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
			expectedStatus := map[string]string{"Progressing": "True"}
			err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus) // Wait it to become Progressing=True
			exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")
			e2e.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
			expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			err = waitCoBecomes(oc, "kube-apiserver", 1500, expectedStatus) // Wait it to become Available=True and Progressing=False and Degraded=False
			exutil.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")

			// Using 60s because KAS takes long time, when KAS finished rotation, OAS and Auth should have already finished.
			e2e.Logf("Checking openshift-apiserver operator should be Available in 60 seconds")
			err = waitCoBecomes(oc, "openshift-apiserver", 60, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "openshift-apiserver operator is not becomes available in 60 seconds")

			e2e.Logf("Checking authentication operator should be Available in 60 seconds")
			err = waitCoBecomes(oc, "authentication", 60, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "authentication operator is not becomes available in 60 seconds")
		}
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Author:rgangwar-High-41899-Replacing the admin kubeconfig generated at install time [Disruptive] [Slow]", func() {
		var (
			dirname        = "/tmp/-OCP-41899-ca/"
			name           = dirname + "custom"
			validity       = 3650
			caSubj         = dirname + "/OU=openshift/CN=admin-kubeconfig-signer-custom"
			user           = "system:admin"
			userCert       = dirname + "system-admin"
			group          = "system:masters"
			userSubj       = dirname + "/O=" + group + "/CN=" + user
			newKubeconfig  = dirname + "kubeconfig." + user
			patch          = `[{"op": "add", "path": "/spec/clientCA", "value":{"name":"client-ca-custom"}}]`
			patchToRecover = `[{"op": "replace", "path": "/spec/clientCA", "value":}]`
			configmapBkp   = dirname + "OCP-41899-bkp.yaml"
		)

		defer os.RemoveAll(dirname)
		defer func() {
			g.By("Restoring cluster")
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", newKubeconfig, "-f", configmapBkp).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
					output, _ := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
					if output == "system:admin" {
						e2e.Logf("Old kubeconfig is restored : %s", output)
						// Adding wait time to ensure old kubeconfig restored properly
						time.Sleep(60 * time.Second)
						return true, nil
					} else if output == "error: You must be logged in to the server (Unauthorized)" {
						return false, nil
					}
					return false, nil
				})
				exutil.AssertWaitPollNoErr(err, "Old kubeconfig is not restored")
				restoreClusterOcp41899(oc)
				e2e.Logf("Cluster recovered")
			} else if err == nil {
				output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patchToRecover).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(output, "patched (no change)") {
					e2e.Logf("Apiserver/cluster is not changed from the default values")
					restoreClusterOcp41899(oc)
				} else {
					output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patchToRecover).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					restoreClusterOcp41899(oc)
				}
			}
		}()

		//Taking backup of configmap "admin-kubeconfig-client-ca" to restore old kubeconfig
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Get the default CA backup")
		configmapBkp, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", "-o", "yaml").OutputToFile("OCP-41899-ca/OCP-41899-bkp.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		sedCmd := fmt.Sprintf(`sed -i '/creationTimestamp:\|resourceVersion:\|uid:/d' %s`, configmapBkp)
		_, err = exec.Command("bash", "-c", sedCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new self-signed CA, in case a corporate or another CA is already existing can be used.
		g.By("Generation of a new self-signed CA")
		e2e.Logf("Generate the CA private key")
		opensslCmd := fmt.Sprintf(`openssl genrsa -out %s-ca.key 4096`, name)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create the CA certificate")
		opensslCmd = fmt.Sprintf(`openssl req -x509 -new -nodes -key %s-ca.key -sha256 -days %d -out %s-ca.crt -subj %s`, name, validity, name, caSubj)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new system:admin certificate. The client certificate must have the user into the x.509 subject CN field and the group into the O field.
		g.By("Generation of a new system:admin certificate")
		e2e.Logf("Create the user CSR")
		opensslCmd = fmt.Sprintf(`openssl req -nodes -newkey rsa:2048 -keyout %s.key -subj %s -out %s.csr`, userCert, userSubj, userCert)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// sign the user CSR and generate the certificate, the certificate must have the `clientAuth` extension
		e2e.Logf("Sign the user CSR and generate the certificate")
		opensslCmd = fmt.Sprintf(`openssl x509 -extfile <(printf "extendedKeyUsage = clientAuth") -req -in %s.csr -CA %s-ca.crt -CAkey %s-ca.key -CAcreateserial -out %s.crt -days %d -sha256`, userCert, name, name, userCert, validity)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// In order to have a safe replacement, before removing the default CA the new certificate is added as an additional clientCA.
		g.By("Create the client-ca ConfigMap")
		caFile := fmt.Sprintf(`--from-file=ca-bundle.crt=%s-ca.crt`, name)
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", "client-ca-custom", "-n", "openshift-config", caFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Patching apiserver")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Checking openshift-controller-manager operator should be in Progressing in 100 seconds")
		expectedStatus := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "openshift-controller-manager", 100, expectedStatus) // Wait it to become Progressing=True
		exutil.AssertWaitPollNoErr(err, "openshift-controller-manager operator is not start progressing in 100 seconds")
		e2e.Logf("Checking openshift-controller-manager operator should be Available in 300 seconds")
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "openshift-controller-manager", 300, expectedStatus) // Wait it to become Available=True and Progressing=False and Degraded=False
		exutil.AssertWaitPollNoErr(err, "openshift-controller-manager operator is not becomes available in 300 seconds")

		g.By("Create the new kubeconfig")
		e2e.Logf("Add system:admin credentials, context to the kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-credentials", user, "--client-certificate="+userCert+".crt", "--client-key="+userCert+".key", "--embed-certs", "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create context for the user")
		clusterName, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].name}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-context", user, "--cluster="+clusterName, "--namespace=default", "--user="+user, "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Extract certificate authority")
		podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication", "-o", "name").Output()
		podname := strings.Fields(podnames)
		ingressCrt, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-authentication", podname[0], "cat", "/run/secrets/kubernetes.io/serviceaccount/ca.crt").OutputToFile("OCP-41899-ca/OCP-41899-ingress-ca.crt")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set certificate authority data")
		serverName, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].cluster.server}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-cluster", clusterName, "--server="+serverName, "--certificate-authority="+ingressCrt, "--kubeconfig="+newKubeconfig, "--embed-certs").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set current context")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", user, "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test the new kubeconfig, be aware that the following command may requires some seconds for let the operator reconcile the newly added CA.
		g.By("Testing the new kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("login").Args("--kubeconfig", newKubeconfig, "-u", user).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("--kubeconfig", newKubeconfig, "node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// If the previous commands are successful is possible to replace the default CA.
		e2e.Logf("Replace the default CA")
		configmapYaml, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("--kubeconfig", newKubeconfig, "configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", caFile, "--dry-run=client", "-o", "yaml").OutputToFile("OCP-41899-ca/OCP-41899.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", newKubeconfig, "-f", configmapYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Is now possible to remove the additional CA which we set earlier.
		e2e.Logf("Removing the additional CA")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("--kubeconfig", newKubeconfig, "apiserver/cluster", "--type=json", "-p", patchToRecover).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Now the old kubeconfig should be invalid, the following command is expected to fail (make sure to set the proper kubeconfig path).
		e2e.Logf("Testing old kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", "admin").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				e2e.Logf("Test pass: Old kubeconfig not working!")
				// Adding wait time to ensure new kubeconfig work properly
				time.Sleep(60 * time.Second)
				return true, nil
			} else if err == nil {
				e2e.Logf("Still Old kubeconfig is working!")
				return false, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Test failed: Old kubeconfig is working!")
	})
	// author: rgangwar@redhat.com
	g.It("Author:rgangwar-Medium-43889-Examine non critical kube-apiserver errors", func() {
		var (
			keywords     = "(error|fail|tcp dial timeout|connect: connection refused|Unable to connect to the server: dial tcp|remote error: tls: bad certificate)"
			exceptions   = "panic|fatal|SHOULD NOT HAPPEN"
			format       = "[0-9TZ.:]{2,30}"
			words        = `(\w+?[^0-9a-zA-Z]+?){,5}`
			afterwords   = `(\w+?[^0-9a-zA-Z]+?){,12}`
			co           = "openshift-kube-apiserver-operator"
			dirname      = "/tmp/-OCP-43889/"
			regexToGrep1 = "(" + words + keywords + words + ")" + "+"
			regexToGrep2 = "(" + words + keywords + afterwords + ")" + "+"
		)

		defer os.RemoveAll(dirname)
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check the log files of KAS operator")
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", co, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podlog, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", co, podname).OutputToFile("OCP-43889/kas-o-grep.log")
		o.Expect(errlog).NotTo(o.HaveOccurred())
		cmd := fmt.Sprintf(`cat %v |grep -ohiE '%s' |grep -iEv '%s' | sed -E 's/%s/../g' | sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, podlog, regexToGrep1, exceptions, format)
		kasOLog, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", kasOLog)

		g.By("Check the log files of KAS")
		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		cmd = fmt.Sprintf(`grep -rohiE '%s' |grep -iEv '%s' /var/log/pods/openshift-kube-apiserver_kube-apiserver*/*/* | sed -E 's/%s/../g'`, regexToGrep2, exceptions, format)
		for i := 0; i < len(masterName); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "default", "node/"+masterName[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_pod.log." + masterName[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_pod.log.*")
		kasPodlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", kasPodlogs)

		g.By("Check the audit log files of KAS")
		cmd = fmt.Sprintf(`grep -rohiE '%s' /var/log/kube-apiserver/audit*.log |grep -iEv '%s' | sed -E 's/%s/../g'`, regexToGrep2, exceptions, format)
		for i := 0; i < len(masterName); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "default", "node/"+masterName[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_audit.log." + masterName[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_audit.log.*")
		kasAuditlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%s", kasAuditlogs)

		g.By("Checking pod and audit logs")
		if len(kasOLog) > 0 || len(kasPodlogs) > 0 || len(kasAuditlogs) > 0 {
			e2e.Failf("Found some non-critical-errors....Check non critical errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("Test pass: No errors found from KAS operator, KAS logs/audit logs")
		}
	})

	// author: rgangwar@redhat.com
	g.It("PreChkUpgrade-NonPreRelease-Author:rgangwar-Critical-40667-Prepare Upgrade cluster under stress with API Priority and Fairness feature [Slow]", func() {
		var (
			dirname    = "/tmp/-OCP-40667/"
			exceptions = "panicked: false, err: context canceled, panic-reason:|panicked: false, err: <nil>, panic-reason: <nil>"
			keywords   = "body: net/http: request canceled (Client.Timeout|panic"
			// Creating below variable for clusterbuster commands "N" argument parameter.
			namespaceCount = 0
		)
		defer os.RemoveAll(dirname)
		err := os.MkdirAll(dirname, 0755)
		g.By("Check the configuration of priority level")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "workload-low", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`100`))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "global-default", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`20`))

		g.By("Checking cluster worker load before running clusterbuster")
		cpuAvgVal, memAvgVal := checkClusterLoad(oc, "worker", "OCP-40667/nodes.log")
		node, err := exutil.GetAllNodes(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Number of nodes are %d", len(node))
		noOfNodes := len(node)
		if noOfNodes > 1 && cpuAvgVal < 50 && memAvgVal < 50 {
			e2e.Logf("Cluster has load normal..CPU %d %% and Memory %d %%...So using value of N=8", cpuAvgVal, memAvgVal)
			namespaceCount = 8
		} else if noOfNodes == 1 && cpuAvgVal < 60 && memAvgVal < 60 {
			e2e.Logf("Cluster is SNO...CPU %d %% and Memory %d %%....So using value of N=3", cpuAvgVal, memAvgVal)
			namespaceCount = 3
		} else {
			e2e.Logf("Cluster has slighty high load...CPU %d %% and Memory %d %%....So using value of N=6", cpuAvgVal, memAvgVal)
			namespaceCount = 6
		}

		g.By("Stress the cluster")
		cmd := fmt.Sprintf(`clusterbuster -P server -b 5 -p 10 -D .01 -M 1 -N %d -r 4 -d 2 -c 10 -m 1000 -v -s 20 -x > %v`, namespaceCount, dirname+"clusterbuster.log")
		_, err = exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -iE '%s' || true`, dirname+"clusterbuster.log", keywords)
		busterLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(busterLogs) > 0 {
			e2e.Logf("%s", busterLogs)
			e2e.Logf("Found some panic or timeout errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No errors found in clusterbuster logs")
		}

		g.By("Check the abnormal pods")
		var podLogs []byte
		errPod := wait.Poll(15*time.Second, 900*time.Second, func() (bool, error) {
			_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-A").OutputToFile("OCP-40667/pod.log")
			o.Expect(err).NotTo(o.HaveOccurred())
			cmd = fmt.Sprintf(`cat %v | grep -i 'clusterbuster' | grep -ivE 'Running|Completed|namespace' || true`, dirname+"pod.log")
			podLogs, err = exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podLogs) > 0 {
				e2e.Logf("clusterbuster pods are not still running and completed")
				return false, nil
			}
			e2e.Logf("No abnormality found in pods...")
			return true, nil
		})
		if errPod != nil {
			e2e.Logf("%s", podLogs)
		}
		exutil.AssertWaitPollNoErr(errPod, "Abnormality found in clusterbuster pods.")

		g.By("Check the abnormal nodes")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--no-headers").OutputToFile("OCP-40667/node.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -Ei 'NotReady|SchedulingDisabled' || true`, dirname+"node.log")
		nodeLogs, err := exec.Command("bash", "-c", cmd).Output()
		e2e.Logf("%s", nodeLogs)
		if len(nodeLogs) > 0 {
			e2e.Logf("Some nodes are NotReady or SchedulingDisabled...Please check")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			e2e.Logf("Nodes are normal...")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Check the abnormal operators")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "--no-headers").OutputToFile("OCP-40667/co.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -v '.True.*False.*False' || true`, dirname+"co.log")
		coLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(coLogs) > 0 {
			e2e.Logf("%s", coLogs)
			e2e.Logf("Found abnormal cluster operators, if errors are  potential bug then file a bug.")
		} else {
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("No abnormality found in cluster operators...")
		}

		g.By("Checking KAS logs")
		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		for i := 0; i < len(masterName); i++ {
			_, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-kube-apiserver", "kube-apiserver-"+masterName[i]).OutputToFile("OCP-40667/kas.log." + masterName[i])
			o.Expect(errlog).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v | grep -iE 'apf_controller.go|apf_filter.go' | grep 'no route' || true`, dirname+"kas.log.*")
		noRouteLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -i 'panic' | grep -Ev "%s" || true`, dirname+"kas.log.*", exceptions)
		panicLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(noRouteLogs) > 0 || len(panicLogs) > 0 {
			e2e.Logf("%s", panicLogs)
			e2e.Logf("%s", noRouteLogs)
			e2e.Logf("Found some panic or no route errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No errors found in KAS logs")
		}

		g.By("Check the all worker nodes workload are normal")
		cpuAvgVal, memAvgVal = checkClusterLoad(oc, "worker", "OCP-40667/nodes_new.log")
		if cpuAvgVal > 75 || memAvgVal > 75 {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Nodes CPU avg %d %% and Memory avg %d %% consumption is high, please investigate the consumption...", cpuAvgVal, memAvgVal)
		} else {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Node CPU %d %% and Memory %d %% consumption is normal....", cpuAvgVal, memAvgVal)
		}

		g.By("Summary of resources used")
		resourceDetails := checkResources(oc, "OCP-40667/resources.log")
		for key, value := range resourceDetails {
			e2e.Logf("Number of %s is %v\n", key, value)
		}

		if cpuAvgVal > 75 || memAvgVal > 75 || len(noRouteLogs) > 0 || len(panicLogs) > 0 || len(coLogs) > 0 || len(nodeLogs) > 0 || len(busterLogs) > 0 {
			e2e.Failf("Prechk Test case: Failed.....Check above errors in case run logs.")
		} else {
			e2e.Logf("Prechk Test case: Passed.....There is no error abnormaliy found..")
		}
	})

	// author: rgangwar@redhat.com
	g.It("PstChkUpgrade-NonPreRelease-Author:rgangwar-Critical-40667-Post Upgrade cluster under stress with API Priority and Fairness feature [Slow]", func() {
		var (
			dirname    = "/tmp/-OCP-40667/"
			exceptions = "panicked: false, err: context canceled, panic-reason:|panicked: false, err: <nil>, panic-reason: <nil>"
		)
		defer os.RemoveAll(dirname)
		defer func() {
			cmd := fmt.Sprintf(`clusterbuster --cleanup`)
			_, err := exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		err := os.MkdirAll(dirname, 0755)
		g.By("Check the configuration of priority level")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "workload-low", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`100`))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "global-default", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`20`))

		g.By("Check the abnormal nodes")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--no-headers").OutputToFile("OCP-40667/node.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd := fmt.Sprintf(`cat %v | grep -Ei 'NotReady|SchedulingDisabled' || true`, dirname+"node.log")
		nodeLogs, err := exec.Command("bash", "-c", cmd).Output()
		e2e.Logf("%s", nodeLogs)
		if len(nodeLogs) > 0 {
			e2e.Logf("Some nodes are NotReady or SchedulingDisabled...Please check")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			e2e.Logf("Nodes are normal...")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Check the abnormal operators")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "--no-headers").OutputToFile("OCP-40667/co.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -v '.True.*False.*False' || true`, dirname+"co.log")
		coLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(coLogs) > 0 {
			e2e.Logf("%s", coLogs)
			e2e.Logf("Found abnormal cluster operators, if errors are  potential bug then file a bug.")
		} else {
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("No abnormality found in cluster operators...")
		}

		g.By("Check the abnormal pods")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-A").OutputToFile("OCP-40667/pod.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -i 'clusterbuster' |grep -ivE 'Running|Completed|namespace' || true`, dirname+"pod.log")
		podLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(podLogs) > 0 {
			e2e.Logf("%s", podLogs)
			e2e.Logf("Found abnormal pods, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No abnormality found in pods...")
		}

		g.By("Checking KAS logs")
		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		for i := 0; i < len(masterName); i++ {
			_, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-kube-apiserver", "kube-apiserver-"+masterName[i]).OutputToFile("OCP-40667/kas.log." + masterName[i])
			o.Expect(errlog).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v | grep -iE 'apf_controller.go|apf_filter.go' | grep 'no route' || true`, dirname+"kas.log.*")
		noRouteLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -i 'panic' | grep -Ev "%s" || true`, dirname+"kas.log.*", exceptions)
		panicLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(noRouteLogs) > 0 || len(panicLogs) > 0 {
			e2e.Logf("%s", panicLogs)
			e2e.Logf("%s", noRouteLogs)
			e2e.Logf("Found some panic or no route errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No errors found in KAS logs")
		}

		g.By("Check the all worker nodes workload are normal")
		cpuAvgVal, memAvgVal := checkClusterLoad(oc, "worker", "OCP-40667/nodes_new.log")
		if cpuAvgVal > 75 || memAvgVal > 75 {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Nodes CPU avg %d %% and Memory avg %d %% consumption is high, please investigate the consumption...", cpuAvgVal, memAvgVal)
		} else {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Node CPU %d %% and Memory %d %% consumption is normal....", cpuAvgVal, memAvgVal)
		}

		g.By("Summary of resources used")
		resourceDetails := checkResources(oc, "OCP-40667/resources.log")
		for key, value := range resourceDetails {
			e2e.Logf("Number of %s is %v\n", key, value)
		}

		if cpuAvgVal > 75 || memAvgVal > 75 || len(noRouteLogs) > 0 || len(panicLogs) > 0 || len(coLogs) > 0 || len(nodeLogs) > 0 {
			e2e.Failf("Postchk Test case: Failed.....Check above errors in case run logs.")
		} else {
			e2e.Logf("Postchk Test case: Passed.....There is no error abnormaliy found..")
		}
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Author:rgangwar-Critical-40861-[Apiserver] [bug 1912564] cluster works fine wihtout panic under stress with API Priority and Fairness feature [Slow]", func() {
		var (
			dirname    = "/tmp/-OCP-40861/"
			exceptions = "panicked: false, err: context canceled, panic-reason:|panicked: false, err: <nil>, panic-reason: <nil>"
			keywords   = "body: net/http: request canceled (Client.Timeout|panic"
			// Creating below variable for clusterbuster commands "N" argument parameter.
			namespaceCount = 0
		)
		defer os.RemoveAll(dirname)
		defer func() {
			cmd := fmt.Sprintf(`clusterbuster --cleanup`)
			_, err := exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		err := os.MkdirAll(dirname, 0755)
		g.By("Check the configuration of priority level")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "workload-low", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`100`))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", "global-default", "-o", `jsonpath={.spec.limited.assuredConcurrencyShares}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.Equal(`20`))

		g.By("Checking cluster worker load before running clusterbuster")
		cpuAvgVal, memAvgVal := checkClusterLoad(oc, "worker", "OCP-40861/nodes.log")
		node, err := exutil.GetAllNodes(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Number of nodes are %d", len(node))
		noOfNodes := len(node)
		if noOfNodes > 1 && cpuAvgVal < 50 && memAvgVal < 50 {
			e2e.Logf("Cluster has load normal..CPU %d %% and Memory %d %%...So using value of N=10", cpuAvgVal, memAvgVal)
			namespaceCount = 10
		} else if noOfNodes == 1 && cpuAvgVal < 60 && memAvgVal < 60 {
			e2e.Logf("Cluster is SNO...CPU %d %% and Memory %d %%....So using value of N=3", cpuAvgVal, memAvgVal)
			namespaceCount = 3
		} else {
			e2e.Logf("Cluster has slighty high load...CPU %d %% and Memory %d %%....So using value of N=6", cpuAvgVal, memAvgVal)
			namespaceCount = 6
		}

		g.By("Stress the cluster")
		cmd := fmt.Sprintf(`clusterbuster -P server -b 5 -p 10 -D .01 -M 1 -N %d -r 4 -d 2 -c 10 -m 1000 -v -s 20 -t 1200 -x > %v`, namespaceCount, dirname+"clusterbuster.log")
		_, err = exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -iE '%s' || true`, dirname+"clusterbuster.log", keywords)
		busterLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(busterLogs) > 0 {
			e2e.Logf("%s", busterLogs)
			e2e.Logf("Found some panic or timeout errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No errors found in clusterbuster logs")
		}

		g.By("Check the abnormal pods")
		var podLogs []byte
		errPod := wait.Poll(15*time.Second, 600*time.Second, func() (bool, error) {
			_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-A").OutputToFile("OCP-40861/pod.log")
			o.Expect(err).NotTo(o.HaveOccurred())
			cmd = fmt.Sprintf(`cat %v | grep -i 'clusterbuster' | grep -ivE 'Running|Completed|namespace' || true`, dirname+"pod.log")
			podLogs, err = exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podLogs) > 0 {
				e2e.Logf("clusterbuster pods are not still running and completed")
				return false, nil
			}
			e2e.Logf("No abnormality found in pods...")
			return true, nil
		})
		if errPod != nil {
			e2e.Logf("%s", podLogs)
		}
		exutil.AssertWaitPollNoErr(errPod, "Abnormality found in clusterbuster pods.")

		g.By("Check the abnormal nodes")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--no-headers").OutputToFile("OCP-40861/node.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -Ei 'NotReady|SchedulingDisabled' || true`, dirname+"node.log")
		nodeLogs, err := exec.Command("bash", "-c", cmd).Output()
		e2e.Logf("%s", nodeLogs)
		if len(nodeLogs) > 0 {
			e2e.Logf("Some nodes are NotReady or SchedulingDisabled...Please check")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			e2e.Logf("Nodes are normal...")
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Check the abnormal operators")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "--no-headers").OutputToFile("OCP-40861/co.log")
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -v '.True.*False.*False' || true`, dirname+"co.log")
		coLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(coLogs) > 0 {
			e2e.Logf("%s", coLogs)
			e2e.Logf("Found abnormal cluster operators, if errors are  potential bug then file a bug.")
		} else {
			err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("No abnormality found in cluster operators...")
		}

		g.By("Checking KAS logs")
		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		for i := 0; i < len(masterName); i++ {
			_, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-kube-apiserver", "kube-apiserver-"+masterName[i]).OutputToFile("OCP-40861/kas.log." + masterName[i])
			o.Expect(errlog).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v | grep -iE 'apf_controller.go|apf_filter.go' | grep 'no route' || true`, dirname+"kas.log.*")
		noRouteLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd = fmt.Sprintf(`cat %v | grep -i 'panic' | grep -Ev "%s" || true`, dirname+"kas.log.*", exceptions)
		panicLogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(noRouteLogs) > 0 || len(panicLogs) > 0 {
			e2e.Logf("%s", panicLogs)
			e2e.Logf("%s", noRouteLogs)
			e2e.Logf("Found some panic or no route errors, if errors are  potential bug then file a bug.")
		} else {
			e2e.Logf("No errors found in KAS logs")
		}

		g.By("Check the all worker nodes workload are normal")
		cpuAvgVal, memAvgVal = checkClusterLoad(oc, "worker", "OCP-40861/nodes_new.log")
		if cpuAvgVal > 75 || memAvgVal > 75 {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Nodes CPU avg %d %% and Memory avg %d %% consumption is high, please investigate the consumption...", cpuAvgVal, memAvgVal)
		} else {
			errlog := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").Execute()
			o.Expect(errlog).NotTo(o.HaveOccurred())
			e2e.Logf("Node CPU %d %% and Memory %d %% consumption is normal....", cpuAvgVal, memAvgVal)
		}

		g.By("Summary of resources used")
		resourceDetails := checkResources(oc, "OCP-40861/resources.log")
		for key, value := range resourceDetails {
			e2e.Logf("Number of %s is %v\n", key, value)
		}

		if cpuAvgVal > 75 || memAvgVal > 75 || len(noRouteLogs) > 0 || len(panicLogs) > 0 || len(coLogs) > 0 || len(nodeLogs) > 0 || len(busterLogs) > 0 {
			e2e.Failf("Test case: Failed.....Check above errors in case run logs.")
		} else {
			e2e.Logf("Test case: Passed.....There is no error abnormaliy found..")
		}
	})

	// author: kewang@redhat.com
	g.It("Longduration-NonPreRelease-Author:kewang-Medium-12308-Customizing template for project creation [Serial][Slow]", func() {
		var (
			caseID           = "ocp-12308"
			dirname          = "/tmp/-ocp-12308"
			templateYaml     = "template.yaml"
			templateYamlFile = filepath.Join(dirname, templateYaml)
			patchYamlFile    = filepath.Join(dirname, "patch.yaml")
			project1         = caseID + "-test1"
			project2         = caseID + "-test2"
			patchJSON        = `[{"op": "replace", "path": "/spec/projectRequestTemplate", "value":{"name":"project-request"}}]`
			restorePatchJSON = `[{"op": "replace", "path": "/spec", "value" :{}}]`
			initRegExpr      = []string{`limits.cpu[\s]+0[\s]+6`, `limits.memory[\s]+0[\s]+16Gi`, `pods[\s]+0[\s]+10`, `requests.cpu[\s]+0[\s]+4`, `requests.memory[\s]+0[\s]+8Gi`}
			regexpr          = []string{`limits.cpu[\s]+[1-9]+[\s]+6`, `limits.memory[\s]+[A-Za-z0-9]+[\s]+16Gi`, `pods[\s]+[1-9]+[\s]+10`, `requests.cpu[\s]+[A-Za-z0-9]+[\s]+4`, `requests.memory[\s]+[A-Za-z0-9]+[\s]+8Gi`}
		)

		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(dirname)

		g.By("1) Create a bootstrap project template and output it to a file template.yaml")
		_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("create-bootstrap-project-template", "-o", "yaml").OutputToFile(filepath.Join(caseID, templateYaml))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2) To customize template.yaml and add ResourceQuota and LimitRange objects.")
		patchYaml := `- apiVersion: v1
  kind: "LimitRange"
  metadata:
    name: ${PROJECT_NAME}-limits
  spec:
    limits:
      - type: "Container"
        default:
          cpu: "1"
          memory: "1Gi"
        defaultRequest:
          cpu: "500m"
          memory: "500Mi"
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    name: ${PROJECT_NAME}-quota
  spec:
    hard:
      pods: "10"
      requests.cpu: "4"
      requests.memory: 8Gi
      limits.cpu: "6"
      limits.memory: 16Gi
      requests.storage: "20G"
`
		f, _ := os.Create(patchYamlFile)
		defer f.Close()
		w := bufio.NewWriter(f)
		_, err = fmt.Fprintf(w, "%s", patchYaml)
		w.Flush()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Insert the patch Ymal before the keyword 'parameters:' in template yaml file
		sedCmd := fmt.Sprintf(`sed -i '/^parameters:/e cat %s' %s`, patchYamlFile, templateYamlFile)
		e2e.Logf("Check sed cmd %s description:", sedCmd)
		_, err = exec.Command("bash", "-c", sedCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("3) Create a project request template from the customized template.yaml file in the openshift-config namespace.")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", templateYamlFile, "-n", "openshift-config").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("templates", "project-request", "-n", "openshift-config").Execute()

		g.By("4) Create new project before applying the customized template of projects.")
		err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(project1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", project1).Execute()

		g.By("5) Associate the template with projectRequestTemplate in the project resource of the config.openshift.io/v1.")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("project.config.openshift.io/cluster", "--type=json", "-p", patchJSON).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("patch").Args("project.config.openshift.io/cluster", "--type=json", "-p", restorePatchJSON).Execute()
			expectedStatus := map[string]string{"Progressing": "True"}
			err = waitCoBecomes(oc, "openshift-apiserver", 240, expectedStatus)
			exutil.AssertWaitPollNoErr(err, `openshift-apiserver status has not yet changed to {"Progressing": "True"} in 240 seconds`)
			expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			err = waitCoBecomes(oc, "openshift-apiserver", 360, expectedStatus)
			exutil.AssertWaitPollNoErr(err, `openshift-apiserver operator status has not yet changed to {"Available": "True", "Progressing": "False", "Degraded": "False"} in 360 seconds`)
			e2e.Logf("openshift-apiserver pods are all running.")
		}()

		g.By("5.1) Wait until the openshift-apiserver clusteroperator complete degradation and in the normal status ...")
		// It needs a bit more time to wait for all openshift-apiservers getting back to normal.
		expectedStatus := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "openshift-apiserver", 240, expectedStatus)
		exutil.AssertWaitPollNoErr(err, `openshift-apiserver status has not yet changed to {"Progressing": "True"} in 240 seconds`)
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "openshift-apiserver", 360, expectedStatus)
		exutil.AssertWaitPollNoErr(err, `openshift-apiserver operator status has not yet changed to {"Available": "True", "Progressing": "False", "Degraded": "False"} in 360 seconds`)
		e2e.Logf("openshift-apiserver operator is normal and pods are all running.")

		g.By("6) The resource quotas will be used for a new project after the customized template of projects is applied.")
		err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(project2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", project2).Execute()

		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("project", project2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Check quotas setting of project %s description:", project2)
		o.Expect(string(output)).To(o.ContainSubstring(project2 + "-quota"))
		for _, regx := range initRegExpr {
			o.Expect(string(output)).Should(o.MatchRegexp(regx))
		}

		g.By("7) To add applications to created project, check if Quota usage of the project is changed.")
		err = oc.AsAdmin().WithoutNamespace().Run("new-app").Args("openshift/hello-openshift").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Waiting for all pods of hello-openshift application to be ready ...")
		err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "--no-headers").Output()
			if err != nil {
				e2e.Logf("Failed to get pods' status of project %s, error: %s. Trying again", project2, err)
				return false, nil
			}
			if matched, _ := regexp.MatchString(`(ContainerCreating|Init|Pending)`, output); matched {
				e2e.Logf("Some of pods still not get ready:\n%s", output)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Some of pods still not get ready!")

		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("project", project2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Check quotas changes of project %s after new app is created:", project2)
		for _, regx := range regexpr {
			o.Expect(string(output)).Should(o.MatchRegexp(regx))
		}

		g.By("8) Check the previously created project, no qutoas setting is applied.")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("project", project1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Check quotas changes of project %s after new app is created:", project1)
		o.Expect(string(output)).NotTo(o.ContainSubstring(project1 + "-quota"))
		o.Expect(string(output)).NotTo(o.ContainSubstring(project1 + "-limits"))

		g.By("9) After deleted all resource objects for created application, the quota usage of the project is released.")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("all", "--selector", "app=hello-openshift").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Wait for deletion of application to complete
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, _ := oc.WithoutNamespace().Run("get").Args("all").Output()
			if matched, _ := regexp.MatchString("No resources found.*", output); matched {
				e2e.Logf("All resource objects for created application have been completely deleted\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "All resource objects for created application haven't been completely deleted!")

		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("project", project2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Check quotas setting of project %s description:", project2)
		for _, regx := range initRegExpr {
			o.Expect(string(output)).Should(o.MatchRegexp(regx))
		}
		g.By(fmt.Sprintf("Last) %s SUCCESS", caseID))
	})

	// author: zxiao@redhat.com
	g.It("Author:zxiao-High-24698-Check the http accessible /readyz for kube-apiserver [Serial]", func() {
		g.By("1) Check if port 6080 is available")
		err := wait.Poll(10*time.Second, 40*time.Second, func() (bool, error) {
			checkOutput, _ := exec.Command("bash", "-c", "lsof -i:6080").Output()
			// no need to check error since some system output stderr for valid result
			if len(checkOutput) == 0 {
				return true, nil
			}
			e2e.Logf("Port 6080 is occupied, trying again")
			return false, nil
		})

		exutil.AssertWaitPollNoErr(err, "Port 6080 is available")

		g.By("2) Check if openshift-kube-apiserver pods are ready")
		output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", "openshift-kube-apiserver", "-l", "apiserver", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if matched, _ := regexp.MatchString("kube-apiserver.*Running", output); !matched {
			e2e.Logf("Some of openshift-kube-apiserver are abnormal:\n%s", output)
		}
		e2e.Logf("All pods of openshift-kube-apiserver are ready")

		g.By("3) Get kube-apiserver pods")
		err = oc.AsAdmin().Run("project").Args("openshift-kube-apiserver").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("project").Args("defult").Execute() // switch to default project

		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver").List(metav1.ListOptions{LabelSelector: "apiserver"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podList.Size()).NotTo(o.Equal(0))
		e2e.Logf("Fetched all pods from openshift-kube-apiserver")

		g.By("4) Perform port-forward on the first pod available")
		pod := podList.Items[0]
		_, _, _, err = oc.AsAdmin().Run("port-forward").Args(pod.Name, "6080").Background()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer exec.Command("kill", "-9", "$(lsof -t -i:6080)").Output()
		e2e.Logf("Port forward running in background, sleep for 30 seconds")

		// sleep 30 seconds to make sure that port forwarding is correctly configured
		time.Sleep(30 * time.Second)

		g.By("5) check if port forward succeed")
		checkOutput, err := exec.Command("bash", "-c", "curl http://127.0.0.1:6080/readyz --noproxy \"127.0.0.1\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(checkOutput)).To(o.Equal("ok"))
		e2e.Logf("Port forwarding works fine")
	})

	// author: dpunia@redhat.com
	g.It("Author:dpunia-High-41664-Check deprecated APIs to be removed in next release and next EUS release", func() {
		var (
			ignoreCase  = "system:kube-controller-manager|system:serviceaccount|system:admin"
			eusReleases = map[float64][]float64{4.8: {1.21, 1.22, 1.23}, 4.10: {1.24, 1.25}}
		)

		//Anonymous function to check elements available in slice, it return true if elements exists otherwise return false.
		elemsCheckers := func(elems []float64, value float64) bool {
			for _, element := range elems {
				if value == element {
					return true
				}
			}
			return false
		}

		g.By("1) Get current cluster version")
		clusterVersions, _, err := exutil.GetClusterVersion(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		clusterVersion, err := strconv.ParseFloat(clusterVersions, 64)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("%v", clusterVersion)

		g.By("2) Get current k8s release & next release")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/kube-apiserver", "-o", `jsonpath='{.status.versions[?(@.name=="kube-apiserver")].version}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmd := fmt.Sprintf(`echo '%v' | awk -F"." '{print $1"."$2}'`, out)
		k8sVer, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		currRelese, _ := strconv.ParseFloat(strings.Trim(string(k8sVer), "\n"), 64)
		e2e.Logf("Current Release : %v", currRelese)
		nxtReleases := currRelese + 0.01
		e2e.Logf("APIRemovedInNextReleaseInUse : %v", nxtReleases)

		g.By("3) Get the removedInRelease of api groups list")
		out, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("apirequestcount", "-o", `jsonpath='{range .items[?(@.status.removedInRelease != "")]}{.metadata.name}{"\t"}{.status.removedInRelease}{"\n"}{end}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		listOutput := strings.Trim(string(out), "'")
		if len(listOutput) == 0 {
			e2e.Logf("There is no api for next APIRemovedInNextReleaseInUse & APIRemovedInNextEUSReleaseInUse\n")
		} else {
			e2e.Logf("List of api Removed in next EUS & Non-EUS releases\n %v", listOutput)
			apisRmRelList := bufio.NewScanner(strings.NewReader(listOutput))
			for apisRmRelList.Scan() {
				removeReleaseAPI := strings.Fields(apisRmRelList.Text())[0]
				removeRelease, _ := strconv.ParseFloat(strings.Fields(apisRmRelList.Text())[1], 64)
				// Checking the alert & logs for next APIRemovedInNextReleaseInUse & APIRemovedInNextEUSReleaseInUse
				if removeRelease == nxtReleases {
					g.By("4) Checking Alert For APIRemovedInNextReleaseInUse")
					e2e.Logf("Api %v and release %v", removeReleaseAPI, removeRelease)
					// Checking alerts, Wait for max 5 min to generate all the alert.
					err = wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
						// Generating Alert for removed apis
						_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(removeReleaseAPI).Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						alertOutput, err := oc.Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-s", "-k", "http://localhost:9090/api/v1/alerts").Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						cmd := fmt.Sprintf(`echo '%v' | egrep 'APIRemovedInNextReleaseInUse' | grep -oh '%s'`, alertOutput, removeReleaseAPI)
						_, outerr := exec.Command("bash", "-c", cmd).Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						if outerr == nil {
							e2e.Logf("Got the Alert for APIRemovedInNextReleaseInUse, %v and release %v", removeReleaseAPI, removeRelease)
							e2e.Logf("Step 4, Tests passed")
							return true, nil
						}
						e2e.Logf("Not Get the alert for APIRemovedInNextReleaseInUse, Api %v : release %v. Trying again", removeReleaseAPI, removeRelease)
						return false, nil
					})
					exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Test Fail:  Not Get Alert for APIRemovedInNextReleaseInUse, %v : release %v", removeReleaseAPI, removeRelease))

					g.By("5) Checking Client compenents accessing the APIRemovedInNextReleaseInUse")
					out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("apirequestcount", removeReleaseAPI, "-o", `jsonpath='{range .status.currentHour..byUser[*]}{..byVerb[*].verb}{","}{.username}{","}{.userAgent}{"\n"}{end}'`).Output()
					stdOutput := strings.TrimRight(strings.Trim(out, "'"), "\n")
					o.Expect(err).NotTo(o.HaveOccurred())
					cmd := fmt.Sprintf(`echo "%s" | egrep -v '%s' || true`, stdOutput, ignoreCase)
					clientAccessLog, err := exec.Command("bash", "-c", cmd).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					if len(clientAccessLog) > 0 {
						e2e.Logf("%v", string(clientAccessLog))
						e2e.Failf("Test Failed: Client components access Apis logs found, file a bug.")
					} else {
						e2e.Logf("Test Passed: No client components access Apis logs found\n")
					}
				}
				// Checking the alert & logs for next APIRemovedInNextEUSReleaseInUse
				if elemsCheckers(eusReleases[clusterVersion], removeRelease) {
					g.By("6) Checking the alert for APIRemovedInNextEUSReleaseInUse")
					e2e.Logf("Api %v and release %v", removeReleaseAPI, removeRelease)
					// Checking alerts, Wait for max 5 min to generate all the alert.
					err = wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
						// Generating Alert for removed apis
						_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(removeReleaseAPI).Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						alertOutput, err := oc.Run("exec").Args("-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-s", "-k", "http://localhost:9090/api/v1/alerts").Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						cmd := fmt.Sprintf(`echo '%v' | egrep 'APIRemovedInNextEUSReleaseInUse' | grep -oh '%s'`, alertOutput, removeReleaseAPI)
						_, outerr := exec.Command("bash", "-c", cmd).Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						if outerr == nil {
							e2e.Logf("Got the Alert for APIRemovedInNextEUSReleaseInUse, %v and release %v", removeReleaseAPI, removeRelease)
							e2e.Logf("Step 6, Tests passed")
							return true, nil
						}
						e2e.Logf("Not Get the alert for APIRemovedInNextEUSReleaseInUse, %v : release %v. Trying again", removeReleaseAPI, removeRelease)
						return false, nil
					})
					exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Test Fail:  Not Get Alert for APIRemovedInNextEUSReleaseInUse, Api %v : release %v", removeReleaseAPI, removeRelease))

					// Checking logs for APIRemovedInNextEUSReleaseInUse apis client components logs.
					g.By("7) Checking client components access logs for APIRemovedInNextEUSReleaseInUse")
					out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("apirequestcount", removeReleaseAPI, "-o", `jsonpath='{range .status.currentHour..byUser[*]}{..byVerb[*].verb}{","}{.username}{","}{.userAgent}{"\n"}{end}'`).Output()
					stdOutput := strings.TrimRight(strings.Trim(out, "'"), "\n")
					o.Expect(err).NotTo(o.HaveOccurred())
					cmd := fmt.Sprintf(`echo "%s" | egrep -v '%s' || true`, stdOutput, ignoreCase)
					clientCompAccess, err := exec.Command("bash", "-c", cmd).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					if len(clientCompAccess) > 0 {
						e2e.Logf(string(clientCompAccess))
						e2e.Failf("Test Failed: Client components access Apis logs found, file a bug.")
					} else {
						e2e.Logf("Test Passed: No client components access Apis logs found")
					}
				}
			}
		}
	})

	// author: zxiao@redhat.com
	g.It("Author:zxiao-Low-27665-Check if the kube-storage-version-migrator operator related manifests has been loaded", func() {
		resource := "customresourcedefinition"
		resourceNames := []string{"storagestates.migration.k8s.io", "storageversionmigrations.migration.k8s.io", "kubestorageversionmigrators.operator.openshift.io"}
		g.By("1) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames)

		resource = "clusteroperators"
		resourceNames = []string{"kube-storage-version-migrator"}
		g.By("2) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames)

		resource = "configmap"
		resourceNames = []string{"config", "openshift-kube-storage-version-migrator-operator-lock"}
		namespace := "openshift-kube-storage-version-migrator-operator"
		g.By("3) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "] under namespace [" + namespace + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)

		resource = "service"
		resourceNames = []string{"metrics"}
		g.By("4) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)

		resource = "serviceaccount"
		resourceNames = []string{"kube-storage-version-migrator-operator"}
		g.By("5) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "] under namespace [" + namespace + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)

		resource = "deployment"
		resourceNames = []string{"kube-storage-version-migrator-operator"}
		g.By("6) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "] under namespace [" + namespace + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)

		resource = "serviceaccount"
		resourceNames = []string{"kube-storage-version-migrator-sa"}
		namespace = "openshift-kube-storage-version-migrator"
		g.By("7) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "] under namespace [" + namespace + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)

		resource = "deployment"
		resourceNames = []string{"migrator"}
		g.By("8) Check if [" + strings.Join(resourceNames, ", ") + "] is available in [" + resource + "] under namespace [" + namespace + "]")
		CheckIfResourceAvailable(oc, resource, resourceNames, namespace)
	})

	// author: jmekkatt@redhat.com
	g.It("Author:jmekkatt-High-50188-An informational error on kube-apiserver in case an admission webhook is installed for a virtual resource [Serial]", func() {
		var (
			validatingWebhookName = "test-validating-cfg"
			mutatingWebhookName   = "test-mutating-cfg"
			validatingWebhook     = getTestDataFilePath("ValidatingWebhookConfiguration-with-virtualresources.yaml")
			mutatingWebhook       = getTestDataFilePath("MutatingWebhookConfiguration-with-virtualresources.yaml")
			kubeApiserverCoStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		)

		g.By("1) Create a ValidatingWebhookConfiguration with virtual resource reference.")
		defer func() {
			oc.Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookName, "--ignore-not-found").Execute()
		}()
		err := oc.Run("create").Args("-f", validatingWebhook).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.Run("get").Args("ValidatingWebhookConfiguration", validatingWebhookName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(validatingWebhookName), "Validating webhook not present in cluster.")
		e2e.Logf(output)
		e2e.Logf("Test step-1 has passed : Creation of ValidatingWebhookConfiguration with virtual resource reference succeeded.")

		g.By("2) Check for kube-apiserver operator status after virtual resource reference for a validating webhook added.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Test step-2 has passed : Kube-apiserver operators are in normal after virtual resource reference for a validating webhook added.")

		g.By("3) Check for information message on kube-apiserver cluster w.r.t virtual resource reference for a validating webhook")

		output, err = oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="VirtualResourceAdmissionError")]}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("kube-apiserver reports the virtual resource error in Validating admission webhook as \n %s ", string(output))
		o.Expect(output).Should(o.And(
			o.MatchRegexp(`"message":"Validating webhook.*virtual resource.*"`),
			o.MatchRegexp(`"reason":"AdmissionWebhookMatchesVirtualResource"`),
			o.MatchRegexp(`"status":"True"`),
			o.MatchRegexp(`"type":"VirtualResourceAdmissionError"`)), "Mismatch in admission errors reported")
		err = oc.Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Test step-3 has passed : Kube-apiserver reports expected informational errors after virtual resource reference for a validating webhook added.")

		g.By("4) Create a MutatingWebhookConfiguration with a virtual resource reference.")
		defer func() {
			oc.Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookName, "--ignore-not-found").Execute()
		}()
		err = oc.Run("create").Args("-f", mutatingWebhook).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.Run("get").Args("MutatingWebhookConfiguration", mutatingWebhookName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(mutatingWebhookName), "Mutating webhook not present in cluster.")
		e2e.Logf(output)
		e2e.Logf("Test step-4 has passed : Creation of MutatingWebhookConfiguration with virtual resource reference succeeded.")

		g.By("5) Check for kube-apiserver operator status after virtual resource reference for a Mutating webhook added.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Test step-5 has passed : Kube-apiserver operators are in normal after virtual resource reference for a mutating webhook added.")

		g.By("6) Check for information message on kube-apiserver cluster w.r.t virtual resource reference for mutating webhook")
		output, err = oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="VirtualResourceAdmissionError")]}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("kube-apiserver reports the virtual resource error in Mutating admission webhook as \n %s ", string(output))
		o.Expect(output).Should(o.And(
			o.MatchRegexp(`"message":"Mutating webhook.*virtual resource.*"`),
			o.MatchRegexp(`"reason":"AdmissionWebhookMatchesVirtualResource"`),
			o.MatchRegexp(`"status":"True"`),
			o.MatchRegexp(`"type":"VirtualResourceAdmissionError"`)), "Mismatch in admission errors reported")
		err = oc.Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Test step-6 has passed : Kube-apiserver reports expected informational errors after virtual resource reference for a mutating webhook added.")

		g.By("7) Check for webhook admission error free kube-apiserver cluster after deleting webhooks.")
		output, err = oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="VirtualResourceAdmissionError")]}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.MatchRegexp(`"type":"VirtualResourceAdmissionError"`),
			o.MatchRegexp(`"status":"False"`)), "VirtualResourceAdmissionError is wrongly set for kube-apiserver.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Test step-7 has passed : No webhook admission error seen after purging webhooks.")
		e2e.Logf("All test case steps are passed.!")
	})

	// author: zxiao@redhat.com
	g.It("Author:zxiao-Low-21246-Check the exposed prometheus metrics of operators", func() {
		g.By("1) get serviceaccount token")
		token, err := exutil.GetSAToken(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		resources := []string{"openshift-apiserver-operator", "kube-apiserver-operator", "kube-storage-version-migrator-operator", "kube-controller-manager-operator"}
		patterns := []string{"workqueue_adds", "workqueue_depth", "workqueue_queue_duration", "workqueue_retries", "workqueue_work_duration"}
		step := 2
		for _, resource := range resources {
			g.By(fmt.Sprintf("%v) For resource %s, check the exposed prometheus metrics", step, resource))

			namespace := resource
			if strings.Contains(resource, "kube-") {
				// need to add openshift prefix for kube resource
				namespace = "openshift-" + resource
			}

			label := "app=" + resource
			g.By(fmt.Sprintf("%v.1) wait for a pod with label %s to be ready within 15 mins", step, label))
			pods, err := exutil.GetAllPodsWithLabel(oc, namespace, label)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(pods).ShouldNot(o.BeEmpty())
			pod := pods[0]
			exutil.AssertPodToBeReady(oc, pod, namespace)

			g.By(fmt.Sprintf("%v.2) request exposed prometheus metrics on pod %s", step, pod))
			command := []string{pod, "-n", namespace, "--", "curl", "--connect-timeout", "30", "--retry", "3", "-N", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), "https://localhost:8443/metrics"}
			output, err := oc.Run("exec").Args(command...).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("%v.3) check the output if it contains the following patterns: %s", step, strings.Join(patterns, ", ")))
			for _, pattern := range patterns {
				o.Expect(output).Should(o.ContainSubstring(pattern))
			}
			// increment step
			step++
		}
	})

	// author: dpunia@redhat.com
	g.It("Longduration-NonPreRelease-Author:dpunia-High-44596-SNO kube-apiserver can fall back to last good revision well when failing to roll out in SNO env [Disruptive]", func() {
		if !isSNOCluster(oc) {
			g.Skip("This is not a SNO cluster, skip.")
		}

		var (
			keyWords = "Stopped container|Removed container"
		)

		nodes, nodeGetError := exutil.GetAllNodes(oc)
		o.Expect(nodeGetError).NotTo(o.HaveOccurred())

		e2e.Logf("Check openshift-kube-apiserver pods current revision before changes")
		out, revisionChkError := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
		o.Expect(revisionChkError).NotTo(o.HaveOccurred())
		PreRevision, _ := strconv.Atoi(out)
		e2e.Logf("Current revision Count: %v", PreRevision)

		defer func() {
			g.By("Roll Out Step 1 Changes")
			patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": null}]`
			rollOutError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
			o.Expect(rollOutError).NotTo(o.HaveOccurred())

			g.By("7) Check Kube-apiserver operator Roll Out with new revision count")
			rollOutError = wait.Poll(100*time.Second, 900*time.Second, func() (bool, error) {
				Output, operatorChkError := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if operatorChkError == nil {
					matched, _ := regexp.MatchString("True.*False.*False", Output)
					if matched {
						out, revisionChkErr := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
						PostRevision, _ := strconv.Atoi(out)
						o.Expect(revisionChkErr).NotTo(o.HaveOccurred())
						o.Expect(PostRevision).Should(o.BeNumerically(">", PreRevision), "Validation failed as PostRevision value not greater than PreRevision")
						e2e.Logf("Kube-apiserver operator Roll Out Successfully with new revision count")
						e2e.Logf("Step 7, Test Passed")
						return true, nil
					}
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(rollOutError, "Step 7, Test Failed: Kube-apiserver operator failed to Roll Out with new revision count")
		}()

		g.By("1) Add invalid configuration to kube-apiserver to make it failed")
		patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": {"apiServerArguments":{"foo":["bar"]}}}]`
		configError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
		o.Expect(configError).NotTo(o.HaveOccurred())

		g.By("2) Check new startup-monitor pod created & running under openshift-kube-apiserver project")
		podChkError := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
			out, runError := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=app=installer", "-o", `jsonpath='{.items[?(@.status.phase=="Running")].status.phase}'`).Output()
			if runError == nil {
				if matched, _ := regexp.MatchString("Running", out); matched {
					e2e.Logf("Step 2, Test Passed: Startup-monitor pod created & running under openshift-kube-apiserver project")
					return true, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(podChkError, "Step 2, Test Failed: Failed to Create startup-monitor pod")

		g.By("3) Check kube-apiserver to fall back to previous good revision")
		fallbackError := wait.Poll(100*time.Second, 900*time.Second, func() (bool, error) {
			annotations, fallbackErr := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", `jsonpath={.items[*].metadata.annotations.startup-monitor\.static-pods\.openshift\.io/fallback-for-revision}`).Output()
			if fallbackErr == nil {
				failedRevision, _ := strconv.Atoi(annotations)
				o.Expect(failedRevision - 1).Should(o.BeNumerically("==", PreRevision))
				g.By("Check created soft-link kube-apiserver-last-known-good to the last good revision")
				out, fileChkError := exutil.DebugNodeWithOptionsAndChroot(oc, nodes[0], []string{"-n", "default"}, "bash", "-c", "ls -l /etc/kubernetes/static-pod-resources/kube-apiserver-last-known-good")
				o.Expect(fileChkError).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("kube-apiserver-pod.yaml"))
				e2e.Logf("Step 3, Test Passed: Cluster is fall back to last good revision")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(fallbackError, "Step 3, Test Failed: Failed to start kube-apiserver with previous good revision")

		g.By("4: Check startup-monitor pod was created during fallback and currently in Stopped/Removed state")
		cmd := fmt.Sprintf("journalctl -u crio --since '10min ago'| grep 'startup-monitor' | egrep %v", keyWords)
		out, journalctlErr := exutil.DebugNodeWithOptionsAndChroot(oc, nodes[0], []string{"-n", "default"}, cmd)
		o.Expect(journalctlErr).NotTo(o.HaveOccurred())
		o.Expect(out).ShouldNot(o.BeEmpty())
		e2e.Logf("Step 4, Test Passed : Startup-monitor pod was created and Stopped/Removed state")

		g.By("5) Check kube-apiserver operator status changed to degraded")
		expectedStatus := map[string]string{"Degraded": "True"}
		operatorChkErr := waitCoBecomes(oc, "kube-apiserver", 900, expectedStatus)
		exutil.AssertWaitPollNoErr(operatorChkErr, "Step 5, Test Failed: kube-apiserver operator failed to Degraded")

		g.By("6) Check kubeapiserver operator nodeStatuses show lastFallbackCount info correctly")
		out, revisionChkErr := oc.WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", "jsonpath='{.status.nodeStatuses[*].lastFailedRevisionErrors}'").Output()
		o.Expect(revisionChkErr).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("fallback to last-known-good revision %v took place", PreRevision)))
		e2e.Logf("Step 6, Test Passed")
	})

	// author: jmekkatt@redhat.com
	g.It("PreChkUpgrade-NonPreRelease-Author:jmekkatt-High-50362-Prepare Upgrade checks when cluster has bad admission webhooks [Serial]", func() {
		var (
			namespace                  = "ocp-50362"
			serviceName                = "example-service"
			serviceNamespace           = "example-namespace"
			badValidatingWebhookName   = "test-validating-cfg"
			badMutatingWebhookName     = "test-mutating-cfg"
			badCrdWebhookName          = "testcrdwebhooks.tests.com"
			badValidatingWebhook       = getTestDataFilePath("ValidatingWebhookConfigurationTemplate.yaml")
			badMutatingWebhook         = getTestDataFilePath("MutatingWebhookConfigurationTemplate.yaml")
			badCrdWebhook              = getTestDataFilePath("CRDWebhookConfigurationTemplate.yaml")
			kubeApiserverCoStatus      = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			webHookErrorConditionTypes = []string{`ValidatingAdmissionWebhookConfigurationError`, `MutatingAdmissionWebhookConfigurationError`, `CRDConversionWebhookConfigurationError`, `VirtualResourceAdmissionError`}
			reason                     = "WebhookServiceNotFound"
			status                     = "True"
		)

		validatingWebHook := admissionWebhook{
			name:             badValidatingWebhookName,
			webhookname:      "test.validating.com",
			servicenamespace: serviceNamespace,
			servicename:      serviceName,
			namespace:        namespace,
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         badValidatingWebhook,
		}

		mutatingWebHook := admissionWebhook{
			name:             badMutatingWebhookName,
			webhookname:      "test.mutating.com",
			servicenamespace: serviceNamespace,
			servicename:      serviceName,
			namespace:        namespace,
			apigroups:        "authorization.k8s.io",
			apiversions:      "v1",
			operations:       "*",
			resources:        "subjectaccessreviews",
			template:         badMutatingWebhook,
		}

		crdWebHook := admissionWebhook{
			name:             badCrdWebhookName,
			webhookname:      "tests.com",
			servicenamespace: serviceNamespace,
			servicename:      serviceName,
			namespace:        namespace,
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         badCrdWebhook,
		}

		g.By("1) Create a custom namespace for admission hook references.")
		err := oc.WithoutNamespace().Run("new-project").Args(namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2) Create a bad ValidatingWebhookConfiguration with invalid service and namespace references.")
		validatingWebHook.createAdmissionWebhookFromTemplate(oc)
		CheckIfResourceAvailable(oc, "ValidatingWebhookConfiguration", []string{badValidatingWebhookName}, "")

		g.By("3) Create a bad MutatingWebhookConfiguration with invalid service and namespace references.")
		mutatingWebHook.createAdmissionWebhookFromTemplate(oc)
		CheckIfResourceAvailable(oc, "MutatingWebhookConfiguration", []string{badMutatingWebhookName}, "")

		g.By("4) Create a bad CRDWebhookConfiguration with invalid service and namespace references.")
		crdWebHook.createAdmissionWebhookFromTemplate(oc)
		CheckIfResourceAvailable(oc, "crd", []string{badCrdWebhookName}, "")

		g.By("5) Check for information error message on kube-apiserver cluster w.r.t bad resource reference for admission webhooks")
		for _, webHookErrorConditionType := range webHookErrorConditionTypes {
			webhookError, err := oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="`+webHookErrorConditionType+`")]}'`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("kube-apiserver reports the admission webhook errors as \n %s ", string(webhookError))
			if strings.Contains(webhookError, "VirtualResourceAdmissionError") {
				reason = "AdmissionWebhookMatchesVirtualResource"
			}
			o.Expect(webhookError).Should(o.And(
				o.MatchRegexp(`"reason":"%s"`, reason),
				o.MatchRegexp(`"status":"%s"`, status),
				o.MatchRegexp(`"type":"%s"`, webHookErrorConditionType)), "Mismatch in admission errors reported")
		}

		e2e.Logf("Step 5 has passed")
		g.By("6) Check for kube-apiserver operator status after bad validating webhook added.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Step 6 has passed. Test case has passed.")

	})

	// author: jmekkatt@redhat.com
	g.It("PstChkUpgrade-NonPreRelease-Author:jmekkatt-High-50362-Post Upgrade checks when cluster has bad admission webhooks [Serial]", func() {

		var (
			namespace                  = "ocp-50362"
			badValidatingWebhookName   = "test-validating-cfg"
			badMutatingWebhookName     = "test-mutating-cfg"
			badCrdWebhookName          = "testcrdwebhooks.tests.com"
			kubeApiserverCoStatus      = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			webHookErrorConditionTypes = []string{`ValidatingAdmissionWebhookConfigurationError`, `MutatingAdmissionWebhookConfigurationError`, `CRDConversionWebhookConfigurationError`, `VirtualResourceAdmissionError`}
			reason                     = "WebhookServiceNotFound"
			status                     = "True"
		)

		defer func() {
			oc.Run("delete").Args("ValidatingWebhookConfiguration", badValidatingWebhookName, "--ignore-not-found").Execute()
			oc.Run("delete").Args("MutatingWebhookConfiguration", badMutatingWebhookName, "--ignore-not-found").Execute()
			oc.Run("delete").Args("crd", badCrdWebhookName, "--ignore-not-found").Execute()
			oc.WithoutNamespace().Run("delete").Args("project", namespace, "--ignore-not-found").Execute()
		}()

		g.By("1) Check presence of admission webhooks created in pre-upgrade steps.")
		e2e.Logf("Check availability of ValidatingWebhookConfiguration")
		CheckIfResourceAvailable(oc, "ValidatingWebhookConfiguration", []string{badValidatingWebhookName}, "")
		e2e.Logf("Check availability of MutatingWebhookConfiguration.")
		CheckIfResourceAvailable(oc, "MutatingWebhookConfiguration", []string{badMutatingWebhookName}, "")
		e2e.Logf("Check availability of CRDWebhookConfiguration.")
		CheckIfResourceAvailable(oc, "crd", []string{badCrdWebhookName}, "")

		g.By("2) Check for information message after upgrade on kube-apiserver cluster when bad admission webhooks are present.")
		for _, webHookErrorConditionType := range webHookErrorConditionTypes {
			webhookError, err := oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="`+webHookErrorConditionType+`")]}'`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("kube-apiserver reports the admission webhook errors as \n %s ", string(webhookError))
			if strings.Contains(webhookError, "VirtualResourceAdmissionError") {
				reason = "AdmissionWebhookMatchesVirtualResource"
			}
			o.Expect(webhookError).Should(o.And(
				o.MatchRegexp(`"reason":"%s"`, reason),
				o.MatchRegexp(`"status":"%s"`, status),
				o.MatchRegexp(`"type":"%s"`, webHookErrorConditionType)), "Mismatch in admission errors reported")
		}

		g.By("3) Check for kube-apiserver operator status after upgrade when cluster has bad webhooks present.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Step 3 has passed , as kubeapiserver is in expected status.")

		g.By("4) Delete all bad webhooks from upgraded cluster.")
		oc.Run("delete").Args("ValidatingWebhookConfiguration", badValidatingWebhookName, "--ignore-not-found").Execute()
		oc.Run("delete").Args("MutatingWebhookConfiguration", badMutatingWebhookName, "--ignore-not-found").Execute()
		oc.Run("delete").Args("crd", badCrdWebhookName, "--ignore-not-found").Execute()

		g.By("5) Check for informational error message presence after deletion of bad webhooks in upgraded cluster.")
		status = "False"
		for _, webHookErrorConditionType := range webHookErrorConditionTypes {
			webhookError, err := oc.Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath='{.status.conditions[?(@.type=="`+webHookErrorConditionType+`")]}'`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("kube-apiserver reports the admission webhook errors as \n %s ", string(webhookError))
			o.Expect(webhookError).Should(o.And(
				o.MatchRegexp(`"type":"%s"`, webHookErrorConditionType),
				o.MatchRegexp(`"status":"%s"`, status)), "Mismatch in admission errors reported")
		}
		e2e.Logf("Step 5 has passed , as no error related to webhooks are in cluster.")
		g.By("6) Check for kube-apiserver operator status after deletion of bad webhooks in upgraded cluster.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		e2e.Logf("Step 6 has passed. Test case has passed.")
	})

	// author: rgangwar@redhat.com
	g.It("NonPreRelease-Author:rgangwar-High-47633-[API-1361] [Apiserver] Update existing alert ExtremelyHighIndividualControlPlaneCPU [Slow] [Disruptive]", func() {
		var (
			alert             = "ExtremelyHighIndividualControlPlaneCPU"
			alertBudget       = "KubeAPIErrorBudgetBurn"
			runbookURL        = "https://github.com/openshift/runbooks/blob/master/alerts/cluster-kube-apiserver-operator/ExtremelyHighIndividualControlPlaneCPU.md"
			runbookBudgetURL  = "https://github.com/openshift/runbooks/blob/master/alerts/cluster-kube-apiserver-operator/KubeAPIErrorBudgetBurn.md"
			alertTimeWarning  = "5m"
			alertTimeCritical = "1h"
			severity          = []string{"warning", "critical"}
		)
		g.By("1.Check with cluster installed OCP 4.10 and later release, the following changes for existing alerts " + alert + " have been applied.")
		output, alertSevErr := oc.Run("get").Args("prometheusrule/cpu-utilization", "-n", "openshift-kube-apiserver", "-o", `jsonpath='{.spec.groups[?(@.name=="control-plane-cpu-utilization")].rules[?(@.alert=="`+alert+`")].labels.severity}'`).Output()
		o.Expect(alertSevErr).NotTo(o.HaveOccurred())
		chkStr := fmt.Sprintf("%s %s", severity[0], severity[1])
		o.Expect(output).Should(o.ContainSubstring(chkStr), fmt.Sprintf("Not have new alert %s with severity :: %s : %s", alert, severity[0], severity[1]))
		e2e.Logf("Have new alert %s with severity :: %s : %s", alert, severity[0], severity[1])

		e2e.Logf("Check reduce severity to %s and %s for :: %s : %s", severity[0], severity[1], alertTimeWarning, alertTimeCritical)
		output, alertTimeErr := oc.Run("get").Args("prometheusrule/cpu-utilization", "-n", "openshift-kube-apiserver", "-o", `jsonpath='{.spec.groups[?(@.name=="control-plane-cpu-utilization")].rules[?(@.alert=="`+alert+`")].for}'`).Output()
		o.Expect(alertTimeErr).NotTo(o.HaveOccurred())
		chkStr = fmt.Sprintf("%s %s", alertTimeWarning, alertTimeCritical)
		o.Expect(output).Should(o.ContainSubstring(chkStr), fmt.Sprintf("Not Have reduce severity to %s and %s for :: %s : %s", severity[0], severity[1], alertTimeWarning, alertTimeCritical))
		e2e.Logf("Have reduce severity to %s and %s for :: %s : %s", severity[0], severity[1], alertTimeWarning, alertTimeCritical)

		e2e.Logf("Check a run book url for %s", alert)
		output, alertRunbookErr := oc.Run("get").Args("prometheusrule/cpu-utilization", "-n", "openshift-kube-apiserver", "-o", `jsonpath='{.spec.groups[?(@.name=="control-plane-cpu-utilization")].rules[?(@.alert=="`+alert+`")].annotations.runbook_url}'`).Output()
		o.Expect(alertRunbookErr).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring(runbookURL), fmt.Sprintf("%s Runbook url not found :: %s", alert, runbookURL))
		e2e.Logf("Have a run book url for %s :: %s", alert, runbookURL)

		g.By("2. Provide run book url for " + alertBudget)
		output, alertKubeBudgetErr := oc.Run("get").Args("PrometheusRule", "-n", "openshift-kube-apiserver", "kube-apiserver-slos-basic", "-o", `jsonpath='{.spec.groups[?(@.name=="kube-apiserver-slos-basic")].rules[?(@.alert=="`+alertBudget+`")].annotations.runbook_url}`).Output()
		o.Expect(alertKubeBudgetErr).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring(runbookBudgetURL), fmt.Sprintf("%s runbookUrl not found :: %s", alertBudget, runbookBudgetURL))
		e2e.Logf("Run book url for %s :: %s", alertBudget, runbookBudgetURL)

		g.By("3. Test the ExtremelyHighIndividualControlPlaneCPU alerts firing")
		e2e.Logf("Check how many cpus are there in the master node")
		masterNode, masterErr := exutil.GetFirstMasterNode(oc)
		o.Expect(masterErr).NotTo(o.HaveOccurred())
		e2e.Logf("Master node is %v : ", masterNode)
		cmd := `lscpu | grep '^CPU(s):'`
		cpuCores, cpuErr := exutil.DebugNodeWithOptionsAndChroot(oc, masterNode, []string{"-n", "default"}, "bash", "-c", cmd)
		o.Expect(cpuErr).NotTo(o.HaveOccurred())
		regexStr := regexp.MustCompile(`CPU\S+\s+\S+`)
		cpuCore := strings.Split(regexStr.FindString(cpuCores), ":")
		noofCPUCore := strings.TrimSpace(cpuCore[1])
		e2e.Logf("Number of cpu :: %v", noofCPUCore)

		e2e.Logf("Run script to add cpu workload to one kube-apiserver pod on the master.")
		labelString := "apiserver"
		masterPods, err := exutil.GetAllPodsWithLabel(oc, "openshift-kube-apiserver", labelString)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masterPods).ShouldNot(o.BeEmpty(), "Not able to get pod")
		defer func() {
			err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-kube-apiserver", masterPods[0], "--", "/bin/sh", "-c", `ps -ef | grep md5sum | grep -v grep | awk '{print $2}' | xargs kill -HUP`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		_, _, _, execPodErr := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-kube-apiserver", masterPods[0], "--", "/bin/sh", "-c", `seq `+noofCPUCore+` | xargs -P0 -n1 md5sum /dev/zero`).Background()
		o.Expect(execPodErr).NotTo(o.HaveOccurred())

		e2e.Logf("Check alert ExtremelyHighIndividualControlPlaneCPU firing")
		errWatcher := wait.Poll(60*time.Second, 500*time.Second, func() (bool, error) {
			alertOutput, alertErr := GetAlertsByName(oc, "ExtremelyHighIndividualControlPlaneCPU")
			o.Expect(alertErr).NotTo(o.HaveOccurred())
			jqCmd := fmt.Sprintf(`echo '%s' | jq -r '.data.alerts[] | select( .labels.alertname | contains("%s"))' | jq -r 'select( .labels.severity| contains("%s")) | .state'`, alertOutput, alert, severity[0])
			jqOutput, jqErr := exec.Command("bash", "-c", jqCmd).Output()
			o.Expect(jqErr).NotTo(o.HaveOccurred())
			if strings.Contains(string(jqOutput), "firing") {
				e2e.Logf("%s with %s is firing", alert, severity[0])
				jqCmd = fmt.Sprintf(`echo '%s' | jq -r '.data.alerts[] | select( .labels.alertname | contains("%s"))' | jq -r 'select( .labels.severity| contains("%s")) | .state'`, alertOutput, alert, severity[1])
				jqOutput, jqErr = exec.Command("bash", "-c", jqCmd).Output()
				o.Expect(jqErr).NotTo(o.HaveOccurred())
				o.Expect(jqOutput).Should(o.ContainSubstring("pending"), fmt.Sprintf("%s with %s is not pending", alert, severity[1]))
				e2e.Logf("%s with %s is pending", alert, severity[1])
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errWatcher, fmt.Sprintf("%s with %s is not firing", alert, severity[0]))
	})
	// author: jmekkatt@redhat.com
	g.It("Author:jmekkatt-high-50223-Checks on different bad admission webhook errors, status of kube-apiserver [Serial]", func() {
		var (
			validatingWebhookNameNotFound     = "test-validating-notfound-cfg"
			mutatingWebhookNameNotFound       = "test-mutating-notfound-cfg"
			crdWebhookNameNotFound            = "testcrdwebhooks.tests.com"
			validatingWebhookNameNotReachable = "test-validating-notreachable-cfg2"
			mutatingWebhookNameNotReachable   = "test-mutating-notreachable-cfg2"
			crdWebhookNameNotReachable        = "testcrdwebhoks.tsts.com"
			validatingWebhookTemplate         = getTestDataFilePath("ValidatingWebhookConfigurationTemplate.yaml")
			mutatingWebhookTemplate           = getTestDataFilePath("MutatingWebhookConfigurationTemplate.yaml")
			crdWebhookTemplate                = getTestDataFilePath("CRDWebhookConfigurationCustomTemplate.yaml")
			serviceTemplate                   = getTestDataFilePath("ServiceTemplate.yaml")
			serviceName                       = "example-service"
			ServiceNameNotFound               = "service-unknown"
			kubeApiserverCoStatus             = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			webHookConditionErrors            = []string{`ValidatingAdmissionWebhookConfigurationError`, `MutatingAdmissionWebhookConfigurationError`, `CRDConversionWebhookConfigurationError`}
		)

		g.By("1) Create new namespace for the tests.")
		oc.SetupProject()

		validatingWebHook := admissionWebhook{
			name:             validatingWebhookNameNotFound,
			webhookname:      "test.validating.com",
			servicenamespace: oc.Namespace(),
			servicename:      serviceName,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         validatingWebhookTemplate,
		}

		mutatingWebHook := admissionWebhook{
			name:             mutatingWebhookNameNotFound,
			webhookname:      "test.mutating.com",
			servicenamespace: oc.Namespace(),
			servicename:      serviceName,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         mutatingWebhookTemplate,
		}

		crdWebHook := admissionWebhook{
			name:             crdWebhookNameNotFound,
			webhookname:      "tests.com",
			servicenamespace: oc.Namespace(),
			servicename:      serviceName,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			singularname:     "testcrdwebhooks",
			pluralname:       "testcrdwebhooks",
			kind:             "TestCrdWebhook",
			shortname:        "tcw",
			version:          "v1beta1",
			template:         crdWebhookTemplate,
		}

		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotFound, "--ignore-not-found").Execute()
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotFound, "--ignore-not-found").Execute()
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotFound, "--ignore-not-found").Execute()

		}()
		g.By("2) Create a bad ValidatingWebhookConfiguration with invalid service and namespace references.")
		validatingWebHook.createAdmissionWebhookFromTemplate(oc)

		g.By("3) Create a bad MutatingWebhookConfiguration with invalid service and namespace references.")
		mutatingWebHook.createAdmissionWebhookFromTemplate(oc)

		g.By("4) Create a bad CRDWebhookConfiguration with invalid service and namespace references.")
		crdWebHook.createAdmissionWebhookFromTemplate(oc)

		e2e.Logf("Check availability of ValidatingWebhookConfiguration")
		CheckIfResourceAvailable(oc, "ValidatingWebhookConfiguration", []string{validatingWebhookNameNotFound}, "")
		e2e.Logf("Check availability of MutatingWebhookConfiguration.")
		CheckIfResourceAvailable(oc, "MutatingWebhookConfiguration", []string{mutatingWebhookNameNotFound}, "")
		e2e.Logf("Check availability of CRDWebhookConfiguration.")
		CheckIfResourceAvailable(oc, "crd", []string{crdWebhookNameNotFound}, "")

		g.By("5) Check for information error message 'WebhookServiceNotFound' on kube-apiserver cluster w.r.t bad admissionwebhook points to invalid service.")
		compareAPIServerWebhookConditions(oc, "WebhookServiceNotFound", "True", webHookConditionErrors)
		g.By("6) Check for kubeapiserver operator status when bad admissionwebhooks configured.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)

		g.By("7) Create services and check service presence for test steps")
		service := service{
			name:      serviceName,
			clusterip: "172.30.1.1",
			namespace: oc.Namespace(),
			template:  serviceTemplate,
		}
		defer oc.AsAdmin().Run("delete").Args("service", serviceName, "-n", oc.Namespace(), "--ignore-not-found").Execute()
		service.createServiceFromTemplate(oc)
		out, err := oc.AsAdmin().Run("get").Args("services", serviceName, "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.ContainSubstring(serviceName), "Service object is not listed as expected")

		g.By("8) Check for error 'WebhookServiceConnectionError' on kube-apiserver cluster w.r.t bad admissionwebhook points to unreachable service.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		compareAPIServerWebhookConditions(oc, "WebhookServiceConnectionError", "True", webHookConditionErrors)

		g.By("9) Creation of additional webhooks that holds unknown service defintions.")
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotReachable, "--ignore-not-found").Execute()
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotReachable, "--ignore-not-found").Execute()
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotReachable, "--ignore-not-found").Execute()

		}()

		validatingWebHookUnknown := admissionWebhook{
			name:             validatingWebhookNameNotReachable,
			webhookname:      "test.validating2.com",
			servicenamespace: oc.Namespace(),
			servicename:      ServiceNameNotFound,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         validatingWebhookTemplate,
		}

		mutatingWebHookUnknown := admissionWebhook{
			name:             mutatingWebhookNameNotReachable,
			webhookname:      "test.mutating2.com",
			servicenamespace: oc.Namespace(),
			servicename:      ServiceNameNotFound,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			template:         mutatingWebhookTemplate,
		}

		crdWebHookUnknown := admissionWebhook{
			name:             crdWebhookNameNotReachable,
			webhookname:      "tsts.com",
			servicenamespace: oc.Namespace(),
			servicename:      ServiceNameNotFound,
			namespace:        oc.Namespace(),
			apigroups:        "",
			apiversions:      "v1",
			operations:       "CREATE",
			resources:        "pods",
			singularname:     "testcrdwebhoks",
			pluralname:       "testcrdwebhoks",
			kind:             "TestCrdwebhok",
			shortname:        "tcwk",
			version:          "v1beta1",
			template:         crdWebhookTemplate,
		}

		g.By("9.1) Create a bad ValidatingWebhookConfiguration with unknown service references.")
		validatingWebHookUnknown.createAdmissionWebhookFromTemplate(oc)

		g.By("9.2) Create a bad MutatingWebhookConfiguration with unknown service references.")
		mutatingWebHookUnknown.createAdmissionWebhookFromTemplate(oc)

		g.By("9.3) Create a bad CRDWebhookConfiguration with unknown service and namespace references.")
		crdWebHookUnknown.createAdmissionWebhookFromTemplate(oc)

		g.By("10) Check for kube-apiserver operator status.")
		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)

		g.By("11) Check for error 'WebhookServiceNotReady' on kube-apiserver cluster w.r.t bad admissionwebhook points both unknown and unreachable services.")
		compareAPIServerWebhookConditions(oc, "WebhookServiceNotReady", "True", webHookConditionErrors)

		g.By("12) Delete all bad webhooks and check kubeapiserver operators and errors")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotReachable).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotReachable).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotReachable).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotFound).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotFound).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotFound).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		checkCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
		compareAPIServerWebhookConditions(oc, "", "False", webHookConditionErrors)
		g.By("Test case steps are passed")

	})

	// author: zxiao@redhat.com
	g.It("PstChkUpgrade-Author:zxiao-High-44597-Upgrade SNO clusters given kube-apiserver implements startup-monitor mechanism", func() {
		g.By("1) Check if cluster is SNO.")
		if !isSNOCluster(oc) {
			g.Skip("This is not a SNO cluster, skip.")
		}

		g.By("2) Get a master node.")
		masterNode, getFirstMasterNodeErr := exutil.GetFirstMasterNode(oc)
		o.Expect(getFirstMasterNodeErr).NotTo(o.HaveOccurred())
		o.Expect(masterNode).NotTo(o.Equal(""))

		g.By("3) Check the kube-apiserver-last-known-good link file exists and is linked to a good version.")
		cmd := "ls -l /etc/kubernetes/static-pod-resources/kube-apiserver-last-known-good"
		output, debugNodeWithChrootErr := exutil.DebugNodeWithOptionsAndChroot(oc, masterNode, []string{"-n", "default"}, "bash", "-c", cmd)
		o.Expect(debugNodeWithChrootErr).NotTo(o.HaveOccurred())

		g.By("3.1) Check kube-apiserver-last-known-good file exists.")
		o.Expect(output).Should(o.ContainSubstring("kube-apiserver-last-known-good"))
		g.By("3.2) Check file is linked to another file.")
		o.Expect(output).Should(o.ContainSubstring("->"))
		g.By("3.3) Check linked file exists.")
		o.Expect(output).Should(o.ContainSubstring("kube-apiserver-pod.yaml"))

		g.By("4) Check cluster operator kube-apiserver is normal, not degraded, and does not contain abnormal statuses.")
		state, checkClusterOperatorConditionErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-apiserver", "-o", "jsonpath={.status.conditions[?(@.type==\"Available\")].status}{.status.conditions[?(@.type==\"Progressing\")].status}{.status.conditions[?(@.type==\"Degraded\")].status}").Output()
		o.Expect(checkClusterOperatorConditionErr).NotTo(o.HaveOccurred())
		o.Expect(state).To(o.ContainSubstring("TrueFalseFalse"))

		g.By("5) Check kubeapiserver operator is normal, not degraded, and does not contain abnormal statuses.")
		state, checkKubeapiserverOperatorConditionErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeapiserver.operator", "cluster", "-o", "jsonpath={.status.nodeStatuses[?(@.lastFailedRevisionErrors)]}").Output()
		o.Expect(checkKubeapiserverOperatorConditionErr).NotTo(o.HaveOccurred())
		o.Expect(state).Should(o.BeEmpty())
	})
})
