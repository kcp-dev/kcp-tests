package router

import (
	"fmt"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLI("awslb", exutil.KubeConfigPath())
		operatorNamespace = "aws-load-balancer-operator"
		operatorPodLabel  = "control-plane=controller-manager"
	)

	g.BeforeEach(func() {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", "qe-app-registry").Output()
		if strings.Contains(output, "NotFound") {
			g.Skip("Skip since catalogsource/qe-app-registry is not installed")
		}
		output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform, it requires AWS")
		}
		output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", operatorNamespace, "pod", "-l", operatorPodLabel).Output()
		if !strings.Contains(output, "Running") {
			createAWSLoadBalancerOperator(oc)
		}
	})

	// author: hongli@redhat.com
	g.It("ConnectedOnly-Author:hongli-High-51189-Support aws-load-balancer-operator", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router", "awslb")
			AWSLBController     = filepath.Join(buildPruningBaseDir, "awslbcontroller.yaml")
			podsvc              = filepath.Join(buildPruningBaseDir, "podsvc.yaml")
			ingress             = filepath.Join(buildPruningBaseDir, "ingress-test.yaml")
			operandCRName       = "cluster"
			operandPodLabel     = "app.kubernetes.io/name=aws-load-balancer-operator"
		)

		g.By("Ensure the operartor pod is ready")
		waitErr := waitForPodWithLabelReady(oc, operatorNamespace, operatorPodLabel)
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the aws-load-balancer-operator pod is not ready"))

		g.By("Create CR AWSLoadBalancerController")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("awsloadbalancercontroller", operandCRName).Output()
		_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", AWSLBController).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = waitForPodWithLabelReady(oc, operatorNamespace, operandPodLabel)
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the aws-load-balancer controller pod is not ready"))

		g.By("Create user project, pod and NodePort service")
		oc.SetupProject()
		createResourceFromFile(oc, oc.Namespace(), podsvc)
		waitErr = waitForPodWithLabelReady(oc, oc.Namespace(), "name=web-server")
		exutil.AssertWaitPollNoErr(waitErr, "the pod web-server is not ready")

		g.By("create ingress with alb annotation in the project and ensure the alb is provsioned")
		// need to ensure the ingress is deleted before deleting the CR AWSLoadBalancerController
		// otherwise the lb resources cannot be cleared
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ingress/ingress-test", "-n", oc.Namespace()).Output()
		createResourceFromFile(oc, oc.Namespace(), ingress)
		output, err := oc.Run("get").Args("ingress").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("ingress-test"))
		waitForLoadBalancerProvision(oc, oc.Namespace(), "ingress-test")
	})
})
