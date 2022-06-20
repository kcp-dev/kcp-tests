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

	var oc = exutil.NewCLI("router-hsts", exutil.KubeConfigPath())

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43476-The PreloadPolicy option can be set to be enforced strictly to be present or absent in HSTS preload header checks [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43476",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge", "service-unsecure", routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload enforced to be absent")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"RequireNoPreload\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"RequireNoPreload\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireNoPreload"))

		g.By("Annotate the edge route with preload option to verify the effect")
		output1, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).To(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("HSTS preload must not be specified"))

		g.By("Add the HSTS policy to global ingresses resource with preload enforced to be present")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"RequirePreload\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequirePreload"))

		g.By("verify the enforced policy by overwriting the route annotation to disable Preload headers")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS preload must be specified"))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43478-The PreloadPolicy option can be configured to be permissive with NoOpinion flag [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43478",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routedomain := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge", "service-unsecure", routedomain)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to NoOpinion")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("NoOpinion"))

		g.By("Annotate the edge route with preload option to verify")
		_, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())

		g.By("Annotate the edge route without preload option to verify")
		_, err2 = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Critical-43474-The includeSubDomainsPolicy parameter can configure subdomain policy to inherit the HSTS policy of parent domain [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43474",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge", "service-unsecure", routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with IncludeSubdomain enforced to be absent")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireNoIncludeSubDomains\" , \"maxAge\":{}}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireNoIncludeSubDomains\", \"maxAge\":{}}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireNoIncludeSubDomains"))

		g.By("Annotate the edge route with preload option to verify the effect")
		output1, err2 := oc.Run("annotate").Args("-n", oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;includeSubDomains", "--overwrite").Output()
		o.Expect(err2).To(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("HSTS includeSubDomains must not be specified"))

		g.By("Add the HSTS policy to global ingresses resource with IncludeSubdomain enforced to be present")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("RequireIncludeSubDomains"))

		g.By("verify the enforced policy by overwriting the route annotation to disable Preload headers")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS includeSubDomains must be specified"))
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43475-The includeSubDomainsPolicy option can be configured to be permissive with NoOpinion flag [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43475",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "project resource creation failed!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge", "service-unsecure", routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to NoOpinion")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"includeSubDomainsPolicy\" : \"RequireIncludeSubDomains\" , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{}, \"preloadPolicy\" :\"NoOpinion\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("NoOpinion"))

		g.By("Annotate the edge route with preload option to verify")
		_, err2 := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000;preload", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())

		g.By("Annotate the edge route without preload option to verify")
		_, err2 = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000", "--overwrite").Output()
		o.Expect(err2).NotTo(o.HaveOccurred())
	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43479-The Maxage HSTS policy strictly adheres to validation of route based based on largestMaxAge and smallestMaxAge parameter [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43479",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "pod failed to be ready state within allowed time!")

		g.By("Expose an edge route via the unsecure service inside project")
		var output string
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		routehost := "route-edge" + "-" + oc.Namespace() + "." + ingctrl.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge", "service-unsecure", routehost)
		output, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("route-edge"))

		g.By("Annotate the edge route with preload HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge", "haproxy.router.openshift.io/hsts_header=max-age=50000")
		output, err = oc.Run("get").Args("route", "route-edge", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Add the HSTS policy to global ingresses resource with preload option set to maxAge with lowest and highest timer option")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{\"largestMaxAge\": 40000, \"smallestMaxAge\": 100 }}]}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"] , \"maxAge\":{\"largestMaxAge\": 40000, \"smallestMaxAge\": 100 }}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("largestMaxAge"))

		g.By("verify the enforced policy by overwriting the route annotation with largestMaxAge set higher than globally defined")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS max-age is greater than maximum age 40000s"))

		g.By("verify the enforced policy by overwriting the route annotation with largestMaxAge set lower than globally defined")
		msg2, err = oc.Run("annotate").WithoutNamespace().Args("route/route-edge", "haproxy.router.openshift.io/hsts_header='max-age=50'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS max-age is less than minimum age 100s"))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43480-The HSTS domain policy can be configure with multiple domainPatterns options [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl1 = ingctrlNodePortDescription{
				name:      "ocp43480-1",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
			ingctrl2 = ingctrlNodePortDescription{
				name:      "ocp43480-2",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)
		g.By("Create first custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl1.domain = ingctrl1.name + "." + baseDomain
		defer ingctrl1.delete(oc)
		ingctrl1.create(oc)
		err1 := waitForCustomIngressControllerAvailable(oc, ingctrl1.name)
		exutil.AssertWaitPollNoErr(err1, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl1.name))

		g.By("Create second custom ingresscontroller")
		baseDomain = getBaseDomain(oc)
		ingctrl2.domain = ingctrl2.name + "." + baseDomain
		defer ingctrl2.delete(oc)
		ingctrl2.create(oc)
		err2 := waitForCustomIngressControllerAvailable(oc, ingctrl2.name)
		exutil.AssertWaitPollNoErr(err2, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl2.name))

		g.By("Deploy project with pods and service resources")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), testPodSvc)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "pod failed to be ready state within allowed time!")

		g.By("Expose an edge route via the unsecure service through ingresscontroller 1 inside project")
		var output1 string
		ingctldomain1 := getIngressctlDomain(oc, ingctrl1.name)
		routehost1 := "route-edge1" + "-" + oc.Namespace() + "." + ingctrl1.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge1", "service-unsecure", routehost1)
		output1, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("route-edge1"))

		g.By("Expose an edge route via the unsecure service through ingresscontroller 2 inside project")
		var output2 string
		ingctldomain2 := getIngressctlDomain(oc, ingctrl2.name)
		routehost2 := "route-edge2" + "-" + oc.Namespace() + "." + ingctrl2.domain
		exposeRouteEdge(oc, oc.Namespace(), "route-edge2", "service-unsecure", routehost2)
		output2, err = oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output2).To(o.ContainSubstring("route-edge2"))

		g.By("Annotate the edge route 1 to enable HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge1", "haproxy.router.openshift.io/hsts_header=max-age=4000")
		output, err := oc.Run("get").Args("route", "route-edge1", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Annotate the edge route 2 to enable HSTS header option")
		setAnnotation(oc, oc.Namespace(), "route/route-edge2", "haproxy.router.openshift.io/hsts_header=max-age=2000")
		output, err = oc.Run("get").Args("route", "route-edge2", "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("haproxy.router.openshift.io/hsts_header"))

		g.By("Set a different HSTS maxage policy for each domain in the global ingresses configuration")
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\"}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain1+"'"+"] , \"includeSubDomainsPolicy\":\"NoOpinion\",\"maxAge\":{\"largestMaxAge\":5000,\"smallestMaxAge\":1},\"preloadPolicy\":\"NoOpinion\"},{\"domainPatterns\":"+" ['*"+"."+ingctldomain2+"'"+"],\"includeSubDomainsPolicy\":\"NoOpinion\",\"maxAge\":{\"largestMaxAge\":3000,\"smallestMaxAge\":1},\"preloadPolicy\":\"NoOpinion\"}]}]")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("largestMaxAge"))

		g.By("verify the enforced policy by overwriting the annotation for route 1  with max-age set  higher than the largestMaxAge defined for the domain")
		msg1, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge1", "haproxy.router.openshift.io/hsts_header='max-age=6000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg1).To(o.ContainSubstring("HSTS max-age is greater than maximum age 5000s"))

		g.By("verify the enforced policy by overwriting the annotation for route 2  with max-age set  higher than the largestMaxAge defined for the domain")
		msg2, err := oc.Run("annotate").WithoutNamespace().Args("route/route-edge2", "haproxy.router.openshift.io/hsts_header='max-age=4000'", "--overwrite").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(msg2).To(o.ContainSubstring("HSTS max-age is greater than maximum age 3000s"))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-High-43884-lobal HSTS policy can be enforced strictly on a specific namespace using namespaceSelector for given domain pattern filtering [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		testPodSvc := filepath.Join(buildPruningBaseDir, "web-server-rc.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43884",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Deploy project 1 with pods and service resources")
		oc.SetupProject()
		project1 := oc.Namespace()
		createResourceFromFile(oc, project1, testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "pod failed to be ready state within allowed time!")

		g.By("Deploy project 2 with pods and service resources")
		oc.SetupProject()
		project2 := oc.Namespace()
		createResourceFromFile(oc, project2, testPodSvc)
		err = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server-rc")
		exutil.AssertWaitPollNoErr(err, "pod failed to be ready state within allowed time!")

		g.By("set up HSTS policy for the custom domain with namespace selector set to label of project1 namespace")
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		defer patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"remove\" , \"path\" : \"/spec/requiredHSTSPolicies\"}]")
		patchGlobalResourceAsAdmin(oc, "ingresses.config.openshift.io/cluster", "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :"+"['*"+"."+ingctldomain+"'"+"],\"includeSubDomainsPolicy\":\"NoOpinion\",\"maxAge\":{\"largestMaxAge\":5000,\"smallestMaxAge\":1},\"namespaceSelector\":{\"matchLabels\":{\"kubernetes.io/metadata.name\":\""+project1+"\"}},\"preloadPolicy\":\"NoOpinion\"}]}]")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ingresses.config.openshift.io/cluster", "-o=jsonpath={.spec.requiredHSTSPolicies[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("largestMaxAge"))

		g.By("Test for outcome by creating an edge route via the HSTS implemented domain through the project1")
		routehost1 := "route-edge" + "-" + project1 + "." + ingctrl.domain
		output, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", project1, "route", "edge", "route-edge", "--service=service-unsecure", "--hostname="+routehost1).Output()
		o.Expect(err1).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("HSTS max-age must be set correctly in HSTS annotation"))

		g.By("Test for outcome by creating an edge route via the default non-HSTS policy controlled domain through the project2")
		routehost2 := "route-edge2" + "-" + project2 + "." + ingctrl.domain
		exposeRouteEdge(oc, project2, "route-edge", "service-unsecure", routehost2)
		output2, err := oc.Run("get").Args("route").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output2).To(o.ContainSubstring("route-edge2"))

	})

	// author: aiyengar@redhat.com
	g.It("Author:aiyengar-Low-43966-Negative values for largestMaxAge and smallestMaxAge option under Maxage HSTS policy are rejected", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp43966",
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
		err := waitForCustomIngressControllerAvailable(oc, ingctrl.name)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("ingresscontroller %s conditions not available", ingctrl.name))

		g.By("Add the HSTS policy with  largestMaxAge set to negative value")
		ingctldomain := getIngressctlDomain(oc, ingctrl.name)
		patch := "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :" + "['*" + "." + ingctldomain + "'" + "] , \"maxAge\":{\"largestMaxAge\": -40000, \"smallestMaxAge\": 100 }}]}]"
		output1, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("ingresses.config.openshift.io/cluster", "--patch="+patch, "--type=json").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output1).To(o.ContainSubstring("spec.requiredHSTSPolicies.maxAge.largestMaxAge: Invalid value"))

		g.By("Add the HSTS policy with  smallestMaxAge set to negative value")
		patch = "[{\"op\":\"add\" , \"path\" : \"/spec/requiredHSTSPolicies\" , \"value\" : [{\"domainPatterns\" :" + "['*" + "." + ingctldomain + "'" + "] , \"maxAge\":{\"largestMaxAge\": 40000, \"smallestMaxAge\": -100 }}]}]"
		output2, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("ingresses.config.openshift.io/cluster", "--patch="+patch, "--type=json").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output2).To(o.ContainSubstring("spec.requiredHSTSPolicies.maxAge.smallestMaxAge: Invalid value"))

	})

})
