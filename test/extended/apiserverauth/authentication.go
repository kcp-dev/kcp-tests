package apiserverauth

import (
	"crypto/tls"
	base64 "encoding/base64"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-auth] Authentication", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// author: xxia@redhat.com
	// It is destructive case, will make co/authentical Available=False for a while, so adding [Disruptive]
	// If the case duration is greater than 10 minutes and is executed in serial (labelled Serial or Disruptive), add Longduration
	g.It("Longduration-Author:xxia-Medium-29917-Deleted authentication resources can come back immediately [Disruptive]", func() {
		g.By("Delete namespace openshift-authentication")
		err := oc.WithoutNamespace().Run("delete").Args("ns", "openshift-authentication").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for the namespace back, it should be back immediate enough. If it is not back immediately, it is bug")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("ns", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get namespace openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("openshift-authentication.*Active", output); matched {
				e2e.Logf("Namespace is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "openshift-authentication is not back")

		g.By("Waiting for oauth-openshift pods back")
		// It needs some time to wait for pods recreated and Running, so the Poll parameters are a little larger
		err = wait.Poll(15*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get pods under openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("oauth-openshift.*Running", output); matched {
				e2e.Logf("Pods are back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of openshift-authentication is not back")

		g.By("Waiting for the clusteroperator back to normal")
		// It needs more time to wait for clusteroperator back to normal. In test, the max time observed is up to 4 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("co", "authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator authentication is back to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator authentication is not back to normal")

		g.By("Delete authentication.operator cluster")
		err = oc.WithoutNamespace().Run("delete").Args("authentication.operator", "cluster").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for authentication.operator back")
		// It needs more time to wait for authentication.operator back. In test, the max time observed is up to 4 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 360*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("authentication.operator", "--no-headers").Output()
			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("^cluster ", output); matched {
				e2e.Logf("authentication.operator cluster is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "authentication.operator cluster is not back")

		g.By("Delete project openshift-authentication-operator")
		err = oc.WithoutNamespace().Run("delete").Args("project", "openshift-authentication-operator").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Waiting for project openshift-authentication-operator back")
		// It needs more time to wait for project openshift-authentication-operator back. In test, the max time observed is up to 6 mins, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 480*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("project", "openshift-authentication-operator").Output()
			if err != nil {
				e2e.Logf("Fail to get project openshift-authentication-operator, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("openshift-authentication-operator.*Active", output); matched {
				e2e.Logf("project openshift-authentication-operator is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "project openshift-authentication-operator  is not back")

		g.By("Waiting for the authentication-operator pod back")
		// It needs some time to wait for pods recreated and Running, so the Poll parameters are a little larger
		err = wait.Poll(15*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication-operator").Output()
			if err != nil {
				e2e.Logf("Fail to get pod under openshift-authentication-operator, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("authentication-operator.*Running", output); matched {
				e2e.Logf("Pod is back:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of  openshift-authentication-operator is not back")
	})

	// author: pmali@redhat.com
	// It is destructive case, will make co/authentical Available=False for a while, so adding [Disruptive]

	g.It("Author:pmali-High-33390-Network Stability check every level of a managed route [Disruptive] [Flaky]", func() {
		g.By("Check pods under openshift-authentication namespace is available")
		err := wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication").Output()
			if err != nil {
				e2e.Logf("Fail to get pods under openshift-authentication, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("oauth-openshift.*Running", output); matched {
				e2e.Logf("Pods are in Running state:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod of openshift-authentication is not Running")

		// Check authentication operator, If its UP and running that means route and service is also working properly. No need to check seperately Service and route endpoints.
		g.By("Check authentication operator is available")
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].status}").Output()
			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("False", output); matched {
				e2e.Logf("authentication.operator cluster is UP:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "authentication.operator cluster is not UP")

		//Check service endpoint is showing correct error

		buildPruningBaseDir := exutil.FixturePath("testdata", "apiserverauth")

		g.By("Check service endpoint is showing correct error")
		networkPolicyAllow := filepath.Join(buildPruningBaseDir, "allow-same-namespace.yaml")

		g.By("Create AllowNetworkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-authentication", "-f"+networkPolicyAllow).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyAllow).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check authentication operator after allow network policy change
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].message}").Output()

			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "OAuthServiceEndpointsCheckEndpointAccessibleControllerDegraded") {
				e2e.Logf("Allow network policy applied successfully:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Allow network policy applied failure")

		g.By("Delete allow-same-namespace Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyAllow).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		//Deny all trafic for route
		g.By("Check route is showing correct error")

		networkPolicyDeny := filepath.Join(buildPruningBaseDir, "deny-network-policy.yaml")

		g.By("Create Deny-all Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check authentication operator after network policy change
		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "authentication", "-o=jsonpath={.status.conditions[0].message}").Output()

			if err != nil {
				e2e.Logf("Fail to get authentication.operator cluster, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "OAuthRouteCheckEndpointAccessibleControllerDegraded") {
				e2e.Logf("Deny network policy applied:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Deny network policy not applied")

		g.By("Delete Networkpolicy")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-authentication", "-f="+networkPolicyDeny).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: ytripath@redhat.com
	g.It("NonPreRelease-Longduration-Author:ytripath-Medium-20804-Support ConfigMap injection controller [Disruptive] [Slow]", func() {
		oc.SetupProject()

		// Check the pod service-ca is running in namespace openshift-service-ca
		podDetails, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		matched, _ := regexp.MatchString("service-ca-.*Running", podDetails)
		o.Expect(matched).Should(o.Equal(true))

		// Create a configmap --from-literal and annotating it with service.beta.openshift.io/inject-cabundle=true
		err = oc.Run("create").Args("configmap", "my-config", "--from-literal=key1=config1", "--from-literal=key2=config2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("annotate").Args("configmap", "my-config", "service.beta.openshift.io/inject-cabundle=true").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for service-ca.crt to be created in configmap
		err = wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			output, err := oc.Run("get").Args("configmap", "my-config", `-o=json`).Output()
			if err != nil {
				e2e.Logf("Failed to get configmap, error: %s. Trying again", err)
				return false, nil
			}
			if strings.Contains(output, "service-ca.crt") {
				e2e.Logf("service-ca injected into configmap successfully\n")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "service-ca.crt not found in configmap")

		oldCert, err := oc.Run("get").Args("configmap", "my-config", `-o=jsonpath={.data.service-ca\.crt}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Delete secret signing-key in openshift-service-ca project
		podOldUID, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", "-o=jsonpath={.items[0].metadata.uid}").Output()
		err = oc.AsAdmin().Run("delete").WithoutNamespace().Args("-n", "openshift-service-ca", "secret", "signing-key").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			// sleep for 200 seconds to make sure the pod is restarted
			time.Sleep(200 * time.Second)
			var podStatus string
			err := wait.Poll(15*time.Second, 15*time.Minute, func() (bool, error) {
				e2e.Logf("Check if all pods are in Completed or Running state across all namespaces")
				podStatus, err = oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-A", `--field-selector=metadata.namespace!=openshift-kube-apiserver,status.phase==Pending`).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf(podStatus)
				if podStatus == "No resources found" {
					// Sleep for 100 seconds then double-check if all pods are up and running
					time.Sleep(100 * time.Second)
					podStatus, err = oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-A", `--field-selector=metadata.namespace!=openshift-kube-apiserver,status.phase==Pending`).Output()
					if err == nil {
						if podStatus == "No resources found" {
							e2e.Logf("No pending pods found")
							return true, nil
						}
						return false, err
					}
				}
				return false, err
			})
			exutil.AssertWaitPollNoErr(err, "These pods are still not back up after waiting for 15 minutes\n"+podStatus)
		}()

		// Waiting for the pod to be Ready, after several minutes(10 min ?) check the cert data in the configmap
		g.By("Waiting for service-ca to be ready, then check if cert data is updated")
		err = wait.Poll(15*time.Second, 5*time.Minute, func() (bool, error) {
			podStatus, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", `-o=jsonpath={.items[0].status.containerStatuses[0].ready}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			podUID, err := oc.AsAdmin().Run("get").WithoutNamespace().Args("po", "-n", "openshift-service-ca", "-o=jsonpath={.items[0].metadata.uid}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if podStatus == `true` && podOldUID != podUID {
				// We need use AsAdmin() otherwise it will frequently hit "error: You must be logged in to the server (Unauthorized)"
				// before the affected components finish pod restart after the secret deletion, like kube-apiserver, oauth-apiserver etc.
				// Still researching if this is a bug
				newCert, _ := oc.AsAdmin().Run("get").Args("configmap", "my-config", `-o=jsonpath={.data.service-ca\.crt}`).Output()
				matched, _ := regexp.MatchString(oldCert, newCert)
				if !matched {
					g.By("Cert data has been updated")
					return true, nil
				}
			}
			return false, err
		})
		exutil.AssertWaitPollNoErr(err, "Cert data not updated after waiting for 5 mins")
	})

	// author: rugong@redhat.com
	// It is destructive case, will change scc restricted, so adding [Disruptive]
	g.It("Author:rugong-Medium-20052-New field forbiddenSysctls for SCC [Disruptive]", func() {
		// In 4.11 and above, we should use SCC "restricted-v2"
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("scc", "restricted-v2", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		re := regexp.MustCompile("(?m)[\r\n]+^  (uid|resourceVersion):.*$")
		output = re.ReplaceAllString(output, "")
		path := "/tmp/scc_restricted_20052.yaml"
		err = ioutil.WriteFile(path, []byte(output), 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(path)
		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", "-p", `{"allowedUnsafeSysctls":["kernel.msg*"]}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Restoring the restricted SCC before exiting the scenario")
			err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", path).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		oc.SetupProject()
		BaseDir := exutil.FixturePath("testdata", "apiserverauth")
		podYaml := filepath.Join(BaseDir, "pod_with_sysctls.yaml")
		err = oc.Run("create").Args("-f", podYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", "-p", `{"forbiddenSysctls":["kernel.msg*"]}`, "--type=merge").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("sysctl overlaps with kernel.msg"))
		e2e.Logf("oc patch scc failed, this is expected.")

		err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", path).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Restore the SCC successfully.")

		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", "-p", `{"forbiddenSysctls":["kernel.msg*"]}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.Run("delete").Args("po", "busybox").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", podYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", "-p", `{"allowedUnsafeSysctls":["kernel.msg*"]}`, "--type=merge").Output()
		o.Expect(err).To(o.HaveOccurred())
		e2e.Logf("oc patch scc failed, this is expected.")

		err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", path).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Restore the SCC successfully.")

		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", "-p", `{"forbiddenSysctls":["kernel.shm_rmid_forced"]}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.Run("delete").Args("po", "busybox").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.Run("create").Args("-f", podYaml).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("unable to validate against any security context constraint"))
		e2e.Logf("Failed to create pod, this is expected.")
	})

	// author: rugong@redhat.com
	// It is destructive case, will change scc restricted, so adding [Disruptive]
	g.It("Author:rugong-Medium-20050-New field allowedUnsafeSysctls for SCC [Disruptive]", func() {
		// In 4.11 and above, we should use SCC "restricted-v2"
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("scc", "restricted-v2", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// uid and resourceVersion must be removed, otherwise "Operation cannot be fulfilled" error will occur when running oc replace in later steps
		re := regexp.MustCompile("(?m)[\r\n]+^  (uid|resourceVersion):.*$")
		output = re.ReplaceAllString(output, "")
		path := "/tmp/scc_restricted_20050.yaml"
		err = ioutil.WriteFile(path, []byte(output), 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(path)

		oc.SetupProject()
		BaseDir := exutil.FixturePath("testdata", "apiserverauth")
		podYaml := filepath.Join(BaseDir, "pod-with-msgmax.yaml")
		output, err = oc.Run("create").Args("-f", podYaml).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(`unsafe sysctl "kernel.msgmax" is not allowed`))
		e2e.Logf("Failed to create pod, this is expected.")

		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("scc", "restricted-v2", `--type=json`, `-p=[{"op": "add", "path": "/allowedUnsafeSysctls", "value":["kernel.msg*"]}]`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Restoring the restricted SCC before exiting the scenario")
			err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", path).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		output, err = oc.Run("create").Args("-f", podYaml).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: rugong@redhat.com
	// It is destructive case, will change oauth cluster and the case execution duration is greater than 5 min, so adding [Disruptive] and [NonPreRelease]
	g.It("NonPreRelease-Author:rugong-Medium-22434-RequestHeader IDP consumes header values from requests of auth proxy [Disruptive]", func() {
		configMapPath, err := os.MkdirTemp("/tmp/", "tmp_22434")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(configMapPath)
		caFileName := "/ca-bundle.crt"
		configMapName := "my-request-header-idp-configmap"
		err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("-n", "openshift-config", "cm/admin-kubeconfig-client-ca", "--confirm", "--to="+configMapPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(configMapPath + caFileName)
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", configMapName, "--from-file=ca.crt="+configMapPath+caFileName, "-n", "openshift-config").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Removing configmap before exiting the scenario.")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", configMapName, "-n", "openshift-config").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		oauthClusterYamlPath := "/tmp/oauth_cluster_22434.yaml"
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("oauth", "cluster", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// uid and resourceVersion must be removed, otherwise "Operation cannot be fulfilled" error will occur when running oc replace in later steps
		re := regexp.MustCompile("(?m)[\r\n]+^  (uid|resourceVersion):.*$")
		output = re.ReplaceAllString(output, "")
		err = ioutil.WriteFile(oauthClusterYamlPath, []byte(output), 0644)
		defer os.Remove(oauthClusterYamlPath)
		baseDir := exutil.FixturePath("testdata", "apiserverauth")
		idpPath := filepath.Join(baseDir, "RequestHeader_IDP.yaml")
		idpStr, err := ioutil.ReadFile(idpPath)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Replacing oauth cluster yaml [spec] part.")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("oauth", "cluster", "--type=merge", "-p", string(idpStr)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Restoring oauth cluster yaml before exiting the scenario.")
			err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", oauthClusterYamlPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		expectedStatus := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "authentication", 240, expectedStatus)
		exutil.AssertWaitPollNoErr(err, `authentication status has not yet changed to {"Progressing": "True"} in 240 seconds`)
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "authentication", 240, expectedStatus)
		exutil.AssertWaitPollNoErr(err, `authentication status has not yet changed to {"Available": "True", "Progressing": "False", "Degraded": "False"} in 240 seconds`)
		e2e.Logf("openshift-authentication pods are all running.")

		g.By("Preparing file client.crt and client.key")
		output, err = oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "--context", "admin", "--minify", "--raw").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		reg := regexp.MustCompile("(?m)[\r\n]+^    (client-certificate-data):.*$")
		clientCertificateData := reg.FindString(output)
		reg = regexp.MustCompile("(?m)[\r\n]+^    (client-key-data):.*$")
		clientKeyData := reg.FindString(output)
		reg = regexp.MustCompile("[^ ]+$")
		crtEncode := reg.FindString(clientCertificateData)
		o.Expect(crtEncode).NotTo(o.BeEmpty())
		keyEncode := reg.FindString(clientKeyData)
		o.Expect(keyEncode).NotTo(o.BeEmpty())
		crtDecodeByte, err := base64.StdEncoding.DecodeString(crtEncode)
		o.Expect(err).NotTo(o.HaveOccurred())
		keyDecodeByte, err := base64.StdEncoding.DecodeString(keyEncode)
		o.Expect(err).NotTo(o.HaveOccurred())
		crtPath := "/tmp/client_22434.crt"
		keyPath := "/tmp/client_22434.key"
		err = ioutil.WriteFile(crtPath, crtDecodeByte, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(crtPath)
		err = ioutil.WriteFile(keyPath, keyDecodeByte, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(keyPath)
		e2e.Logf("File client.crt and client.key are prepared.")

		// generate first request
		cert, err := tls.LoadX509KeyPair(crtPath, keyPath)
		o.Expect(err).NotTo(o.HaveOccurred())
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					Certificates:       []tls.Certificate{cert},
				},
			},
			// if the client follows redirects automatically, it will encounter this error "http: no Location header in response"
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		oauthRouteHost, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "oauth-openshift", "-n", "openshift-authentication", "-o", "jsonpath={.spec.host}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		requestURL := "https://" + oauthRouteHost + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client"
		request, err := http.NewRequest("GET", requestURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		ssoUser1 := "testUser1"
		xRemoteUserDisplayName := "testDisplayName1"
		request.Header.Add("SSO-User", ssoUser1)
		request.Header.Add("X-Remote-User-Display-Name", xRemoteUserDisplayName)
		g.By("First request is sending, waiting a response.")
		response1, err := client.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer response1.Body.Close()
		// check user & identity & oauthaccesstoken
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("user", ssoUser1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("identity", "my-request-header-idp:"+ssoUser1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("oauthaccesstoken").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(ssoUser1))
		e2e.Logf("First response is gotten, user & identity & oauthaccesstoken are expected.")
		defer func() {
			g.By("Removing user " + ssoUser1)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("user", ssoUser1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Removing identity my-request-header-idp:" + ssoUser1)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("identity", "my-request-header-idp:"+ssoUser1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Logging in with access_token.")
		location := response1.Header.Get("Location")
		u, err := url.Parse(location)
		o.Expect(err).NotTo(o.HaveOccurred())
		subStrArr := strings.Split(u.Fragment, "&")
		accessToken := ""
		for i := 0; i < len(subStrArr); i++ {
			if strings.Contains(subStrArr[i], "access_token") {
				accessToken = strings.Replace(subStrArr[i], "access_token=", "", 1)
				break
			}
		}
		o.Expect(accessToken).NotTo(o.BeEmpty())
		// The --token command modifies the file pointed to the env KUBECONFIG, so I need a temporary file for it to modify
		oc.SetupProject()
		err = oc.Run("login").Args("--token", accessToken).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Log in with access_token successfully.")

		// generate second request
		requestURL = "https://" + oauthRouteHost + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client"
		request, err = http.NewRequest("GET", requestURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		ssoUser2 := "testUser2"
		xRemoteUserLogin := "testUserLogin"
		request.Header.Add("SSO-User", ssoUser2)
		request.Header.Add("X-Remote-User-Login", xRemoteUserLogin)
		g.By("Second request is sending, waiting a response.")
		response2, err := client.Do(request)
		defer response2.Body.Close()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("user", xRemoteUserLogin).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("identity", "my-request-header-idp:"+ssoUser2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Second response is gotten, user & identity are expected.")
		defer func() {
			g.By("Removing user " + xRemoteUserLogin)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("user", xRemoteUserLogin).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Removing identity my-request-header-idp:" + ssoUser2)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("identity", "my-request-header-idp:"+ssoUser2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					Certificates:       []tls.Certificate{cert},
				},
			},
		}
		// generate third request
		requestURL = "https://" + oauthRouteHost + "/oauth/token/request"
		request, err = http.NewRequest("GET", requestURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		xRemoteUser := "testUser3"
		request.Header.Add("X-Remote-User", xRemoteUser)
		g.By("Third request is sending, waiting a response.")
		response3, err := client.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer response3.Body.Close()
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("user", xRemoteUser).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("identity", "my-request-header-idp:"+xRemoteUser).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		bodyByte, err := ioutil.ReadAll(response3.Body)
		respBody := string(bodyByte)
		o.Expect(respBody).To(o.ContainSubstring("Display Token"))
		e2e.Logf("Third response is gotten, user & identity & display_token are expected.")
		defer func() {
			g.By("Removing user " + xRemoteUser)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("user", xRemoteUser).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Removing identity my-request-header-idp:" + xRemoteUser)
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("identity", "my-request-header-idp:"+xRemoteUser).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
	})

	// author: rugong@redhat.com
	g.It("Author:rugong-Low-37697-Allow Users To Manage Their Own Tokens", func() {
		oc.SetupProject()
		user1Name := oc.Username()
		userOauthAccessTokenYamlPath, err := os.MkdirTemp("/tmp/", "tmp_37697")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(userOauthAccessTokenYamlPath)
		userOauthAccessTokenYamlName := "userOauthAccessToken.yaml"
		userOauthAccessTokenName1, err := oc.Run("get").Args("useroauthaccesstokens", "-ojsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		userOauthAccessTokenYaml, err := oc.Run("get").Args("useroauthaccesstokens", userOauthAccessTokenName1, "-o", "yaml").Output()
		err = ioutil.WriteFile(userOauthAccessTokenYamlPath+"/"+userOauthAccessTokenYamlName, []byte(userOauthAccessTokenYaml), 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", userOauthAccessTokenYamlPath+"/"+userOauthAccessTokenYamlName).Execute()
		o.Expect(err).To(o.HaveOccurred())
		e2e.Logf("User cannot create useroauthaccesstokens by yaml file of his own, this is expected.")

		// switch to another user, try to get and delete previous user's useroauthaccesstokens
		oc.SetupProject()
		err = oc.Run("get").Args("useroauthaccesstokens", userOauthAccessTokenName1).Execute()
		o.Expect(err).To(o.HaveOccurred())
		e2e.Logf("User cannot list other user's useroauthaccesstokens, this is expected.")
		err = oc.Run("delete").Args("useroauthaccesstoken", userOauthAccessTokenName1).Execute()
		o.Expect(err).To(o.HaveOccurred())
		e2e.Logf("User cannot delete other user's useroauthaccesstoken, this is expected.")

		baseDir := exutil.FixturePath("testdata", "apiserverauth")
		clusterRoleTestSudoer := filepath.Join(baseDir, "clusterrole-test-sudoer.yaml")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", clusterRoleTestSudoer).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterroles", "test-sudoer-37697").Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("clusterrolebinding", "test-sudoer-37697", "--clusterrole=test-sudoer-37697", "--user="+oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebinding", "test-sudoer-37697").Execute()
		e2e.Logf("Clusterroles and clusterrolebinding were created successfully.")

		err = oc.Run("get").Args("useroauthaccesstokens", "--as="+user1Name, "--as-group=system:authenticated:oauth").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("delete").Args("useroauthaccesstoken", userOauthAccessTokenName1, "--as="+user1Name, "--as-group=system:authenticated:oauth").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("A user of 'impersonate' permission can get and delete other user's useroauthaccesstoken, this is expected.")

		shaToken, err := oc.Run("whoami").Args("-t").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		userOauthAccessTokenName2, err := oc.Run("get").Args("useroauthaccesstokens", "-ojsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("delete").Args("useroauthaccesstokens", userOauthAccessTokenName2).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Need wait a moment to ensure the token really becomes invalidated
		err = wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			err = oc.Run("login").Args("--token=" + shaToken).Execute()
			if err != nil {
				e2e.Logf("The token is now invalidated after its useroauthaccesstoken is deleted for a while.")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Timed out in invalidating a token after its useroauthaccesstoken is deleted for a while")
	})
})
