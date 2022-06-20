package router

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("route-whitelist", exutil.KubeConfigPath())

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Medium-42230-route can be configured to whitelist more than 61 ips/CIDRs", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			output              string
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		)
		g.By("create project, pod, svc resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "the pod with name=web-server-rc Ready status not met")

		g.By("expose a service in the project")
		exposeRoute(oc, oc.Namespace(), "svc/service-unsecure")
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("service-unsecure"))

		g.By("annotate the route with haproxy.router.openshift.io/ip_whitelist and verify")
		setAnnotation(oc, oc.Namespace(), "route/service-unsecure", "haproxy.router.openshift.io/ip_whitelist=192.168.0.0/24 192.168.1.0/24 192.168.2.0/24 192.168.3.0/24 192.168.4.0/24 192.168.5.0/24 192.168.6.0/24 192.168.7.0/24 192.168.8.0/24 192.168.9.0/24 192.168.10.0/24 192.168.11.0/24 192.168.12.0/24 192.168.13.0/24 192.168.14.0/24 192.168.15.0/24 192.168.16.0/24 192.168.17.0/24 192.168.18.0/24 192.168.19.0/24 192.168.20.0/24 192.168.21.0/24 192.168.22.0/24 192.168.23.0/24 192.168.24.0/24 192.168.25.0/24 192.168.26.0/24 192.168.27.0/24 192.168.28.0/24 192.168.29.0/24 192.168.30.0/24 192.168.31.0/24 192.168.32.0/24 192.168.33.0/24 192.168.34.0/24 192.168.35.0/24 192.168.36.0/24 192.168.37.0/24 192.168.38.0/24 192.168.39.0/24 192.168.40.0/24 192.168.41.0/24 192.168.42.0/24 192.168.43.0/24 192.168.44.0/24 192.168.45.0/24 192.168.46.0/24 192.168.47.0/24 192.168.48.0/24 192.168.49.0/24 192.168.50.0/24 192.168.51.0/24 192.168.52.0/24 192.168.53.0/24 192.168.54.0/24 192.168.55.0/24 192.168.56.0/24 192.168.57.0/24 192.168.58.0/24 192.168.59.0/24 192.168.60.0/24 192.168.61.0/24 192.168.62.0/24 192.168.63.0/24 192.168.64.0/24")
		output, err = oc.Run("get").Args("route", "service-unsecure", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/ip_whitelist"))

		g.By("Verify the acl whitelist parameter inside router pod")
		podName := getRouterPod(oc, "default")
		//backendName is the leading context of the route
		backendName := "be_http:" + oc.Namespace() + ":service-unsecure"
		output = readHaproxyConfig(oc, podName, backendName, "-A10", "acl whitelist")
		o.Expect(output).To(o.ContainSubstring(`acl whitelist src`))
		o.Expect(output).To(o.ContainSubstring(`tcp-request content reject if !whitelist`))
	})

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-High-45399-ingress controller continue to function normally with unexpected high timeout value", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			output              string
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		)
		g.By("create project, pod, svc resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "the pod with name=web-server-rc Ready status not met")

		g.By("expose a service in the project")
		exposeRoute(oc, oc.Namespace(), "svc/service-secure")
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("service-secure"))

		g.By("annotate the route with haproxy.router.openshift.io/timeout annotation to high value and verify")
		setAnnotation(oc, oc.Namespace(), "route/service-secure", "haproxy.router.openshift.io/timeout=9999d")
		output, err = oc.Run("get").Args("route", "service-secure", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(`haproxy.router.openshift.io/timeout":"9999d`))

		g.By("Verify the haproxy configuration for the set timeout value")
		podName := getRouterPod(oc, "default")
		output = readHaproxyConfig(oc, podName, oc.Namespace(), "-A6", `timeout`)
		o.Expect(output).To(o.ContainSubstring(`timeout server  2147483647ms`))

		g.By("Verify the pod logs to see any timer overflow error messages")
		log, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-ingress", podName, "-c", "router").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(log).NotTo(o.ContainSubstring(`timer overflow`))
	})

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-High-49802-HTTPS redirect happens even if there is a more specific http-only", func() {
		var (
			output              string
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "test-client-pod.yaml")
			testEdge            = filepath.Join(buildPruningBaseDir, "49802-route.yaml")
		)

		g.By("create project and a 'Hello' pod")
		baseDomain := getBaseDomain(oc)
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "app=hello-pod")
		exutil.AssertWaitPollNoErr(err, "the pod with name=hello-pod, Ready status not met")

		g.By("create a clusterip service")
		_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("service", "clusterip", "hello-pod", "--tcp=80:8080", "--tcp=443:8443", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.Run("get").Args("service").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("hello-pod"))
		podName := getPodName(oc, oc.Namespace(), "app=hello-pod")

		g.By("create http and https routes")
		createResourceFromFile(oc, oc.Namespace(), testEdge)

		g.By("check the reachability of the secure route")
		curlCmd := fmt.Sprintf("curl -I -k https://hello-pod-%s.apps.%s", oc.Namespace(), baseDomain)
		statsOut, err := exutil.RemoteShPod(oc, oc.Namespace(), podName[0], "sh", "-c", curlCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statsOut).Should(o.ContainSubstring("HTTP/1.1 200 OK"))

		g.By("check the reachability of the secure route with redirection")
		curlCmd1 := fmt.Sprintf("curl -I http://hello-pod-%s.apps.%s", oc.Namespace(), baseDomain)
		statsOut1, err := exutil.RemoteShPod(oc, oc.Namespace(), podName[0], "sh", "-c", curlCmd1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statsOut1).Should(o.ContainSubstring("HTTP/1.1 302 Found"))
		o.Expect(statsOut1).Should(o.ContainSubstring("location: https://hello-pod-%s.apps.%s", oc.Namespace(), baseDomain))

		g.By("check the reachability of the insecure routes")
		curlCmd2 := fmt.Sprintf(`curl -I http://hello-pod-http-%s.apps.%s/test/`, oc.Namespace(), baseDomain)
		statsOut2, err := exutil.RemoteShPod(oc, oc.Namespace(), podName[0], "sh", "-c", curlCmd2)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statsOut2).Should(o.ContainSubstring("HTTP/1.1 200 OK"))
	})
})
