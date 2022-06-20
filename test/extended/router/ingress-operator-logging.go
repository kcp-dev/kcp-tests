package router

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	// e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("ingress-operator-logging", exutil.KubeConfigPath())

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-High-46287-ingresscontroller supports to update maxlength for syslog message", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-syslog.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp46287",
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

		g.By("check the env variable of the router pod to verify the default log length")
		newrouterpod := getRouterPod(oc, "ocp46287")
		logLength := readRouterPodEnv(oc, newrouterpod, "ROUTER_LOG_MAX_LENGTH")
		o.Expect(logLength).To(o.ContainSubstring(`ROUTER_LOG_MAX_LENGTH=1024`))

		g.By("check the haproxy config on the router pod to verify the default log length is enabled")
		checkoutput := readRouterPodData(oc, newrouterpod, "cat haproxy.config", "1024")
		o.Expect(checkoutput).To(o.ContainSubstring(`log 1.2.3.4:514 len 1024 local1 info`))

		g.By("patch the existing custom ingress controller with minimum log length value")
		routerpod := getRouterPod(oc, "ocp46287")
		patchResourceAsAdmin(oc, ingctrl.namespace, "ingresscontroller/ocp46287", "{\"spec\":{\"logging\":{\"access\":{\"destination\":{\"syslog\":{\"maxLength\":480}}}}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+routerpod))

		g.By("check the env variable of the router pod to verify the minimum log length")
		newrouterpod = getRouterPod(oc, "ocp46287")
		minimumlogLength := readRouterPodEnv(oc, newrouterpod, "ROUTER_LOG_MAX_LENGTH")
		o.Expect(minimumlogLength).To(o.ContainSubstring(`ROUTER_LOG_MAX_LENGTH=480`))

		g.By("patch the existing custom ingress controller with maximum log length value")
		routerpod = getRouterPod(oc, "ocp46287")
		patchResourceAsAdmin(oc, ingctrl.namespace, "ingresscontroller/ocp46287", "{\"spec\":{\"logging\":{\"access\":{\"destination\":{\"syslog\":{\"maxLength\":4096}}}}}}")
		err = waitForResourceToDisappear(oc, "openshift-ingress", "pod/"+routerpod)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %v does not disapper", "pod/"+routerpod))

		g.By("check the env variable of the router pod to verify the maximum log length")
		newrouterpod = getRouterPod(oc, "ocp46287")
		maximumlogLength := readRouterPodEnv(oc, newrouterpod, "ROUTER_LOG_MAX_LENGTH")
		o.Expect(maximumlogLength).To(o.ContainSubstring(`ROUTER_LOG_MAX_LENGTH=4096`))
	})

	// author: mjoseph@redhat.com
	g.It("Author:mjoseph-Low-46288-ingresscontroller should deny invalid maxlengh value for syslog message", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ingresscontroller-syslog.yaml")
		var (
			ingctrl = ingctrlNodePortDescription{
				name:      "ocp46288",
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

		g.By("patch the existing custom ingress controller with log length value less than minimum threshold")
		output1, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("ingresscontroller/ocp46288", "-p", "{\"spec\":{\"logging\":{\"access\":{\"destination\":{\"syslog\":{\"maxLength\":479}}}}}}", "--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output1).To(o.ContainSubstring("Invalid value: 479: spec.logging.access.destination.syslog.maxLength in body should be greater than or equal to 480"))

		g.By("patch the existing custom ingress controller with log length value more than maximum threshold")
		output2, _ := oc.AsAdmin().WithoutNamespace().Run("patch").Args("ingresscontroller/ocp46288", "-p", "{\"spec\":{\"logging\":{\"access\":{\"destination\":{\"syslog\":{\"maxLength\":4097}}}}}}", "--type=merge", "-n", ingctrl.namespace).Output()
		o.Expect(output2).To(o.ContainSubstring("Invalid value: 4097: spec.logging.access.destination.syslog.maxLength in body should be less than or equal to 4096"))

		g.By("check the haproxy config on the router pod to verify the default log length is enabled")
		routerpod := getRouterPod(oc, "ocp46288")
		checkoutput := readRouterPodData(oc, routerpod, "cat haproxy.config", "1024")
		o.Expect(checkoutput).To(o.ContainSubstring(`log 1.2.3.4:514 len 1024 local1 info`))
	})
})
