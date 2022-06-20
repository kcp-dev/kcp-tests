package router

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	//e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-env", exutil.KubeConfigPath())

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-40677-Ingresscontroller with endpointPublishingStrategy of nodePort allows PROXY protocol for source forwarding", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np-PROXY.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp40677",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a NP ingresscontroller with PROXY protocol set")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Check the router env to verify the PROXY variable is applied")
		podname := getRouterPod(oc, "ocp40677")
		dssearch := readRouterPodEnv(oc, podname, "ROUTER_USE_PROXY_PROTOCOL")
		o.Expect(dssearch).To(o.ContainSubstring(`ROUTER_USE_PROXY_PROTOCOL=true`))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-OCP-40675-Ingresscontroller with endpointPublishingStrategy of hostNetwork allows PROXY protocol for source forwarding [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-hn-PROXY.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp40675",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("check whether there are more than two worker nodes present for testing hostnetwork")
		workerNodeCount, _ := exactNodeDetails(oc)
		if workerNodeCount <= 2 {
			g.Skip("Skipping as we need more than two worker nodes")
		}

		g.By("Create a hostNetwork ingresscontroller with PROXY protocol set")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Check the router env to verify the PROXY variable is applied")
		routername := getRouterPod(oc, "ocp40675")
		dssearch := readRouterPodEnv(oc, routername, "ROUTER_USE_PROXY_PROTOCOL")
		o.Expect(dssearch).To(o.ContainSubstring(`ROUTER_USE_PROXY_PROTOCOL=true`))
	})

	//author: jechen@redhat.com
	g.It("Author:jechen-Medium-42878-Errorfile stanzas and dummy default html files have been added to the router", func() {
		g.By("Get pod (router) in openshift-ingress namespace")
		podname := getRouterPod(oc, "default")

		g.By("Check if there are default 404 and 503 error pages on the router")
		searchOutput := readRouterPodData(oc, podname, "ls -l", "error-page")
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-404.http`))
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-503.http`))

		g.By("Check if errorfile stanzas have been added into haproxy-config.template")
		searchOutput = readRouterPodData(oc, podname, "cat haproxy-config.template", "errorfile")
		o.Expect(searchOutput).To(o.ContainSubstring(`ROUTER_ERRORFILE_404`))
		o.Expect(searchOutput).To(o.ContainSubstring(`ROUTER_ERRORFILE_503`))
	})

	//author: jechen@redhat.com
	g.It("Author:jechen-High-43115-Configmap mounted on router volume after ingresscontroller has spec field HttpErrorCodePage populated with configmap name", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43115",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("1. create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		originalRouterpod := getRouterPod(oc, ingctrl.name)

		g.By("2.  Configure a customized error page configmap from files in openshift-config namespace")
		configmapName := "custom-43115-error-code-pages"
		cmFile1 := filepath.Join(buildPruningBaseDir, "error-page-503.http")
		cmFile2 := filepath.Join(buildPruningBaseDir, "error-page-404.http")
		_, error := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", configmapName, "--from-file="+cmFile1, "--from-file="+cmFile2, "-n", "openshift-config").Output()
		o.Expect(error).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("configmap", configmapName, "-n", "openshift-config").Output()

		g.By("3. Check if configmap is successfully configured in openshift-config namesapce")
		err = checkConfigMap(oc, "openshift-config", configmapName)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %v not found", configmapName))

		g.By("4. Patch the configmap created above to the custom ingresscontroller in openshift-ingress namespace")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"httpErrorCodePages\":{\"name\":\"custom-43115-error-code-pages\"}}}")

		g.By("5. Check if configmap is successfully patched into openshift-ingress namesapce, configmap with name ingctrl.name-errorpages should be created")
		expectedCmName := ingctrl.name + `-errorpages`
		err = checkConfigMap(oc, "openshift-ingress", expectedCmName)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %v not found", expectedCmName))

		g.By("6. Obtain new router pod created, and check if error_code_pages directory is created on it")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+originalRouterpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+originalRouterpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("Check /var/lib/haproxy/conf directory to see if error_code_pages subdirectory is created on the router")
		searchOutput := readRouterPodData(oc, newrouterpod, "ls -al /var/lib/haproxy/conf", "error_code_pages")
		o.Expect(searchOutput).To(o.ContainSubstring(`error_code_pages`))

		g.By("7. Check if custom error code pages have been mounted")
		searchOutput = readRouterPodData(oc, newrouterpod, "ls -al /var/lib/haproxy/conf/error_code_pages", "error")
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-503.http -> ..data/error-page-503.http`))
		o.Expect(searchOutput).To(o.ContainSubstring(`error-page-404.http -> ..data/error-page-404.http`))

		searchOutput = readRouterPodData(oc, newrouterpod, "cat /var/lib/haproxy/conf/error_code_pages/error-page-503.http", "Unavailable")
		o.Expect(searchOutput).To(o.ContainSubstring(`HTTP/1.0 503 Service Unavailable`))
		o.Expect(searchOutput).To(o.ContainSubstring(`Custom:Application Unavailable`))

		searchOutput = readRouterPodData(oc, newrouterpod, "cat /var/lib/haproxy/conf/error_code_pages/error-page-404.http", "Not Found")
		o.Expect(searchOutput).To(o.ContainSubstring(`HTTP/1.0 404 Not Found`))
		o.Expect(searchOutput).To(o.ContainSubstring(`Custom:Not Found`))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43105-The tcp client/server fin and default timeout for the ingresscontroller can be modified via tuningOptions parameterss", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43105",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Verify the default server/client fin and default timeout values")
		checkoutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerpod, "--", "bash", "-c", `cat haproxy.config | grep -we "timeout client" -we "timeout client-fin" -we "timeout server" -we "timeout server-fin"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(checkoutput).To(o.ContainSubstring(`timeout client 30s`))
		o.Expect(checkoutput).To(o.ContainSubstring(`timeout client-fin 1s`))
		o.Expect(checkoutput).To(o.ContainSubstring(`timeout server 30s`))
		o.Expect(checkoutput).To(o.ContainSubstring(`timeout server-fin 1s`))

		g.By("Patch ingresscontroller with new timeout options")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"tuningOptions\" :{\"clientFinTimeout\": \"3s\",\"clientTimeout\":\"33s\",\"serverFinTimeout\":\"3s\",\"serverTimeout\":\"33s\"}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("verify the timeout variables from the new router pods")
		checkenv := readRouterPodEnv(oc, newrouterpod, "TIMEOUT")
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_CLIENT_FIN_TIMEOUT=3s`))
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_DEFAULT_CLIENT_TIMEOUT=33s`))
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_DEFAULT_SERVER_TIMEOUT=33`))
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_DEFAULT_SERVER_FIN_TIMEOUT=3s`))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43113-Tcp inspect-delay for the haproxy pod can be modified via the TuningOptions parameters in the ingresscontroller", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43113",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Verify the default tls values")
		checkoutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerpod, "--", "bash", "-c", `cat haproxy.config | grep -w "inspect-delay"| uniq`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(checkoutput).To(o.ContainSubstring(`tcp-request inspect-delay 5s`))

		g.By("Patch ingresscontroller with a tls inspect timeout option")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"tuningOptions\" :{\"tlsInspectDelay\": \"15s\"}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("verify the new tls inspect timeout value in the router pod")
		checkenv := readRouterPodEnv(oc, newrouterpod, "ROUTER_INSPECT_DELAY")
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_INSPECT_DELAY=15s`))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43112-timeout tunnel parameter for the haproxy pods an be modified with TuningOptions option in the ingresscontroller", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43112",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Verify the default tls values")
		checkoutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerpod, "--", "bash", "-c", `cat haproxy.config | grep -w "timeout tunnel"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(checkoutput).To(o.ContainSubstring(`timeout tunnel 1h`))

		g.By("Patch ingresscontroller with a tunnel timeout option")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"tuningOptions\" :{\"tunnelTimeout\": \"2h\"}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("verify the new tls inspect timeout value in the router pod")
		checkenv := readRouterPodEnv(oc, newrouterpod, "ROUTER_DEFAULT_TUNNEL_TIMEOUT")
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_DEFAULT_TUNNEL_TIMEOUT=2h`))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Medium-43111-The tcp client/server and tunnel timeouts for ingresscontroller will remain unchanged for negative values", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43111",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Patch ingresscontroller with negative values for the tuningOptions settings and check the ingress operator config post the change")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"tuningOptions\" :{\"clientFinTimeout\": \"-7s\",\"clientTimeout\": \"-33s\",\"serverFinTimeout\": \"-3s\",\"serverTimeout\": \"-27s\",\"tlsInspectDelay\": \"-11s\",\"tunnelTimeout\": \"-1h\"}}}")
		output := fetchJSONPathValue(oc, "openshift-ingress-operator", "ingresscontroller/"+ingctrl.name, ".spec.tuningOptions")
		o.Expect(output).To(o.ContainSubstring("{\"clientFinTimeout\":\"-7s\",\"clientTimeout\":\"-33s\",\"serverFinTimeout\":\"-3s\",\"serverTimeout\":\"-27s\",\"tlsInspectDelay\":\"-11s\",\"tunnelTimeout\":\"-1h\"}"))

		g.By("Check the timeout option set in the haproxy pods post the changes applied")
		checktimeout := readRouterPodData(oc, routerpod, "cat haproxy.config", "timeout")
		o.Expect(checktimeout).To(o.ContainSubstring("timeout connect 5s"))
		o.Expect(checktimeout).To(o.ContainSubstring("timeout client 30s"))
		o.Expect(checktimeout).To(o.ContainSubstring("timeout client-fin 1s"))
		o.Expect(checktimeout).To(o.ContainSubstring("timeout server 30s"))
		o.Expect(checktimeout).To(o.ContainSubstring("timeout server-fin 1s"))
		o.Expect(checktimeout).To(o.ContainSubstring("timeout tunnel 1h"))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43414-The logEmptyRequests ingresscontroller parameter set to Ignore add the dontlognull option in the haproxy configuration", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43414",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Patch ingresscontroller with logEmptyRequests set to Ignore option")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"logging\":{\"access\":{\"destination\":{\"type\":\"Container\"},\"logEmptyRequests\":\"Ignore\"}}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Router  %v failed to fully terminate", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("verify the Dontlog variable inside the  router pod")
		checkenv := readRouterPodEnv(oc, newrouterpod, "ROUTER_DONT_LOG_NULL")
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_DONT_LOG_NULL=true`))

		g.By("Verify the parameter set in the haproxy configuration of the router pod")
		checkoutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", newrouterpod, "--", "bash", "-c", `cat haproxy.config | grep -w "dontlognull"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(checkoutput).To(o.ContainSubstring(`option dontlognull`))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43416-httpEmptyRequestsPolicy ingresscontroller parameter set to ignore adds the http-ignore-probes option in the haproxy configuration", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "43416",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Patch ingresscontroller with logEmptyRequests set to Ignore option")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"httpEmptyRequestsPolicy\":\"Ignore\"}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Router  %v failed to fully terminate", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("verify the Dontlog variable inside the  router pod")
		checkenv := readRouterPodEnv(oc, newrouterpod, "ROUTER_HTTP_IGNORE_PROBES")
		o.Expect(checkenv).To(o.ContainSubstring(`ROUTER_HTTP_IGNORE_PROBES=true`))

		g.By("Verify the parameter set in the haproxy configuration of the router pod")
		checkoutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", newrouterpod, "--", "bash", "-c", `cat haproxy.config | grep -w "http-ignore-probes"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(checkoutput).To(o.ContainSubstring(`option http-ignore-probes`))
	})

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-High-46571-Setting ROUTER_ENABLE_COMPRESSION and ROUTER_COMPRESSION_MIME in HAProxy", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "46571",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Patch ingresscontroller with httpCompression option")
		ingctrlResource := "ingresscontrollers/" + ingctrl.name
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, "{\"spec\":{\"httpCompression\":{\"mimeTypes\":[\"text/html\",\"text/css; charset=utf-8\",\"application/json\"]}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Router  %v failed to fully terminate", "pod/"+routerpod))
		newrouterpod := getRouterPod(oc, ingctrl.name)

		g.By("check the env variable of the router pod")
		checkenv1 := readRouterPodEnv(oc, newrouterpod, "ROUTER_ENABLE_COMPRESSION")
		o.Expect(checkenv1).To(o.ContainSubstring(`ROUTER_ENABLE_COMPRESSION=true`))
		checkenv2 := readRouterPodEnv(oc, newrouterpod, "ROUTER_COMPRESSION_MIME")
		o.Expect(checkenv2).To(o.ContainSubstring(`ROUTER_COMPRESSION_MIME=text/html "text/css; charset=utf-8" application/json`))

		g.By("check the haproxy config on the router pod for compression algorithm")
		algo := readRouterPodData(oc, newrouterpod, "cat haproxy.config", "compression")
		o.Expect(algo).To(o.ContainSubstring(`compression algo gzip`))
		o.Expect(algo).To(o.ContainSubstring(`compression type text/html "text/css; charset=utf-8" application/json`))
	})

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Low-46898-Setting wrong data in ROUTER_ENABLE_COMPRESSION and ROUTER_COMPRESSION_MIME in HAProxy", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "46898",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create a custom ingresscontroller, and get its router name")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))
		routerpod := getRouterPod(oc, ingctrl.name)

		g.By("Patch ingresscontroller with wrong httpCompression data and check whether it is configurable")
		output, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("ingresscontroller/46898", "-p", "{\"spec\":{\"httpCompression\":{\"mimeTypes\":[\"text/\",\"text/css; charset=utf-8\",\"//\"]}}}", "--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output).To(o.ContainSubstring("Invalid value: \"text/\": spec.httpCompression.mimeTypes in body should match"))
		o.Expect(output).To(o.ContainSubstring("application|audio|image|message|multipart|text|video"))

		g.By("check the env variable of the router pod")
		output1, _ := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerpod, "--", "bash", "-c", "/usr/bin/env | grep ROUTER_ENABLE_COMPRESSION").Output()
		o.Expect(output1).NotTo(o.ContainSubstring(`ROUTER_ENABLE_COMPRESSION=true`))

		g.By("check the haproxy config on the router pod for compression algorithm")
		output2, _ := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerpod, "--", "bash", "-c", "cat haproxy.config | grep compression").Output()
		o.Expect(output2).NotTo(o.ContainSubstring(`compression algo gzip`))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-Low-49131-check haproxy's version", func() {
		var expVersion = "2.2.24"
		g.By("rsh to a default router pod and get the HAProxy's version")
		haproxyVer := getHAProxyVersion(oc)
		g.By("show haproxy version(" + haproxyVer + "), and check if it is updated successfully")
		o.Expect(haproxyVer).To(o.ContainSubstring(expVersion))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-High-50074-Allow Ingress to be modified on the settings of livenessProbe and readinessProbe", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		timeout5 := "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"router\",\"livenessProbe\":{\"timeoutSeconds\":5},\"readinessProbe\":{\"timeoutSeconds\":5}}]}}}}"
		timeoutmax := "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"router\",\"livenessProbe\":{\"timeoutSeconds\":2147483647},\"readinessProbe\":{\"timeoutSeconds\":2147483647}}]}}}}"
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp50074",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		ingressErr := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(ingressErr, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("check the default liveness probe and readiness probe parameters in the json outut of the router deployment")
		routerDeploymentName := "router-" + ingctrl.name
		podname := getRouterPod(oc, ingctrl.name)
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..livenessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":1"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..readinessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":1"))

		g.By("patch livenessProbe and readinessProbe with 5s to the router deployment")
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("deployment", routerDeploymentName, "--type=strategic", "--patch="+timeout5, "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+podname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+podname))
		err = waitForPodWithLabelReady(oc, "openshift-ingress", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller="+ingctrl.name)
		exutil.AssertWaitPollNoErr(err, "new router pod failed to be ready state within allowed time!")
		podname = getRouterPod(oc, ingctrl.name)

		g.By("check liveness probe and readiness probe 5s in the json output of the router deployment")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..livenessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":5"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..readinessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":5"))

		g.By("patch livenessProbe and readinessProbe with max 2147483647s to the router deployment")
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("deployment", routerDeploymentName, "--type=strategic", "--patch="+timeoutmax, "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+podname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+podname))
		err = waitForPodWithLabelReady(oc, "openshift-ingress", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller="+ingctrl.name)
		exutil.AssertWaitPollNoErr(err, "new router pod failed to be ready state within allowed time!")
		podname = getRouterPod(oc, ingctrl.name)

		g.By("check liveness probe and readiness probe max 2147483647s in the description of the router deployment")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment", routerDeploymentName, "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Liveness:   http-get http://:1936/healthz delay=0s timeout=2147483647s"))
		o.Expect(output).To(o.ContainSubstring("Readiness:  http-get http://:1936/healthz/ready delay=0s timeout=2147483647s"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..livenessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":2147483647"))

		g.By("check liveness probe and readiness probe max 2147483647s in the json output of the router deployment")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..livenessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":2147483647"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", routerDeploymentName, "-o=jsonpath={..readinessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":2147483647"))

		g.By("check liveness probe and readiness probe max 2147483647s in the json output of the router pod")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podname, "-o=jsonpath={..livenessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":2147483647"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podname, "-o=jsonpath={..readinessProbe}", "-n", "openshift-ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"timeoutSeconds\":2147483647"))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-Low-50075-Negative test of allow Ingress to be modified on the settings of livenessProbe and readinessProbe", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		timeoutMinus := "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"router\",\"livenessProbe\":{\"timeoutSeconds\":-1},\"readinessProbe\":{\"timeoutSeconds\":-1}}]}}}}"
		timeoutString := "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"router\",\"livenessProbe\":{\"timeoutSeconds\":\"abc\"},\"readinessProbe\":{\"timeoutSeconds\":\"abc\"}}]}}}}"
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp50075",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		ingressErr := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(ingressErr, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("try to patch livenessProbe and readinessProbe with a minus number -1 to the router deployment")
		routerDeploymentName := "router-" + ingctrl.name
		output, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("deployment", routerDeploymentName, "--type=strategic", "--patch="+timeoutMinus, "-n", "openshift-ingress").Output()
		o.Expect(output).To(o.ContainSubstring("spec.template.spec.containers[0].livenessProbe.timeoutSeconds: Invalid value: -1: must be greater than or equal to 0"))
		o.Expect(output).To(o.ContainSubstring("spec.template.spec.containers[0].readinessProbe.timeoutSeconds: Invalid value: -1: must be greater than or equal to 0"))

		g.By("try to patch livenessProbe and readinessProbe with string type of value to the router deployment")
		output, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args("deployment", routerDeploymentName, "--type=strategic", "--patch="+timeoutString, "-n", "openshift-ingress").Output()
		o.Expect(output).To(o.ContainSubstring("The request is invalid: patch: Invalid value: \"map[spec:map[template:map[spec:map[containers:[map[livenessProbe:map[timeoutSeconds:abc] name:router readinessProbe:map[timeoutSeconds:abc]]]]]]]\": unrecognized type: int32"))
	})

	// author: shudili@redhat.com
	g.It("Author:shudili-Medium-42940-User can customize HAProxy 2.0 Error Page", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			customTemp          = filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
			srvrcInfo           = "web-server-rc"
			srvName             = "service-unsecure"
			clientPod           = filepath.Join(buildPruningBaseDir, "test-client-pod.yaml")
			cltPodName          = "hello-pod"
			cltPodLabel         = "app=hello-pod"
			http404page         = filepath.Join(buildPruningBaseDir, "error-page-404.http")
			http503page         = filepath.Join(buildPruningBaseDir, "error-page-503.http")
			cmName              = "my-custom-error-code-pages-42940"
			patchHTTPErrorPage  = "{\"spec\": {\"httpErrorCodePages\": {\"name\": \"" + cmName + "\"}}}"
			ingctrl             = ingctrlNodePortDescription{
				name:      "ocp42940",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
			ingctrlResource = "ingresscontrollers/" + ingctrl.name
		)

		g.By("Create a ConfigMap with custom 404 and 503 error pages")
		cmCrtErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", cmName, "--from-file="+http404page, "--from-file="+http503page, "-n", "openshift-config").Execute()
		o.Expect(cmCrtErr).NotTo(o.HaveOccurred())
		defer deleteConfigMap(oc, "openshift-config", cmName)
		cmOutput, cmErr := oc.WithoutNamespace().AsAdmin().Run("get").Args("configmap", "-n", "openshift-config").Output()
		o.Expect(cmErr).NotTo(o.HaveOccurred())
		o.Expect(cmOutput).To(o.ContainSubstring(cmName))
		cmOutput, cmErr = oc.WithoutNamespace().AsAdmin().Run("get").Args("configmap", cmName, "-o=jsonpath={.data}", "-n", "openshift-config").Output()
		o.Expect(cmErr).NotTo(o.HaveOccurred())
		o.Expect(cmOutput).To(o.ContainSubstring("error-page-404.http"))
		o.Expect(cmOutput).To(o.ContainSubstring("Custom error page:The requested document was not found"))
		o.Expect(cmOutput).To(o.ContainSubstring("error-page-503.http"))
		o.Expect(cmOutput).To(o.ContainSubstring("Custom error page:The requested application is not available"))

		g.By("Create one custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("patch the custom ingresscontroller with the http error code pages")
		podname := getRouterPod(oc, ingctrl.name)
		patchResourceAsAdmin(oc, ingctrl.namespace, ingctrlResource, patchHTTPErrorPage)
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+podname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+podname))
		err = waitForPodWithLabelReady(oc, "openshift-ingress", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller="+ingctrl.name)
		exutil.AssertWaitPollNoErr(err, "new router pod failed to be ready state within allowed time!")

		g.By("get one custom ingress-controller router pod's IP")
		podname = getRouterPod(oc, ingctrl.name)
		podIP := getPodv4Address(oc, podname, "openshift-ingress")

		g.By("Deploy a project with a client pod, a backend pod and its service resources")
		oc.SetupProject()
		project1 := oc.Namespace()
		g.By("create a client pod")
		createResourceFromFile(oc, project1, clientPod)
		err = waitForPodWithLabelReady(oc, project1, cltPodLabel)
		exutil.AssertWaitPollNoErr(err, "A client pod failed to be ready state within allowed time!")
		g.By("create an unsecure service and its backend pod")
		createResourceFromFile(oc, project1, testPodSvc)
		err = waitForPodWithLabelReady(oc, project1, "name="+srvrcInfo)
		exutil.AssertWaitPollNoErr(err, "backend server pod failed to be ready state within allowed time!")

		g.By("Expose an route with the unsecure service inside the project")
		routehost := srvName + "-" + project1 + "." + ingctrl.domain
		output, SrvErr := oc.Run("expose").Args("service", srvName, "--hostname="+routehost).Output()
		o.Expect(SrvErr).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(srvName))

		g.By("curl a normal route from the client pod")
		toDst := routehost + ":80:" + podIP
		output, err = oc.Run("exec").Args(cltPodName, "--", "curl", "-i", "http://"+routehost, "--resolve", toDst).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("200 OK"))

		g.By("curl a non-existing route, expect to get custom http 404 Not Found error")
		notExistRoute := "notexistroute" + "-" + project1 + "." + ingctrl.domain
		toDst2 := notExistRoute + ":80:" + podIP
		output, err = oc.Run("exec").Args(cltPodName, "--", "curl", "-v", "http://"+notExistRoute, "--resolve", toDst2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("404 Not Found"))
		o.Expect(output).To(o.ContainSubstring("Custom error page:The requested document was not found"))

		g.By("delete the backend pod and try to curl the route, expect to get custom http 503 Service Unavailable")
		podname, err = oc.Run("get").Args("pods", "-l", "name="+srvrcInfo, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("delete").Args("replicationcontroller", srvrcInfo).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForResourceToDisappear(oc, project1, "pod/"+podname)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+podname))
		output, err = oc.Run("exec").Args(cltPodName, "--", "curl", "-v", "http://"+routehost, "--resolve", toDst).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("503 Service Unavailable"))
		o.Expect(output).To(o.ContainSubstring("Custom error page:The requested application is not available"))
	})
})
