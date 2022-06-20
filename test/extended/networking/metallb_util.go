//MetalLB operator tests
package networking

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type subscriptionResource struct {
	name             string
	namespace        string
	operatorName     string
	channel          string
	catalog          string
	catalogNamespace string
	template         string
}
type namespaceResource struct {
	name     string
	template string
}
type operatorGroupResource struct {
	name             string
	namespace        string
	targetNamespaces string
	template         string
}

type metalLBCRResource struct {
	name      string
	namespace string
	template  string
}

type addressPoolResource struct {
	name      string
	namespace string
	protocol  string
	addresses []string
	template  string
}

type loadBalancerServiceResource struct {
	name                  string
	namespace             string
	externaltrafficpolicy string
	template              string
}

var (
	snooze time.Duration = 720
)

func operatorInstall(oc *exutil.CLI, sub subscriptionResource, ns namespaceResource, og operatorGroupResource) (status bool) {
	//Installing Operator
	g.By(" (1) INSTALLING Operator in the namespace")

	//Applying the config of necessary yaml files from templates to create metallb operator
	g.By("(1.1) Applying namespace template")
	err0 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", ns.template, "-p", "NAME="+ns.name)
	e2e.Logf("Error creating namespace %v", err0)

	g.By("(1.2)  Applying operatorgroup yaml")
	err0 = applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "TARGETNAMESPACES="+og.targetNamespaces)
	e2e.Logf("Error creating operator group %v", err0)

	g.By("(1.3) Creating subscription yaml from template")
	// no need to check for an existing subscription
	err0 = applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBSCRIPTIONNAME="+sub.name, "NAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"CATALOGSOURCE="+sub.catalog, "CATALOGSOURCENAMESPACE="+sub.catalogNamespace)
	e2e.Logf("Error creating subscription %v", err0)

	//confirming operator install
	g.By("(1.4) Verify the operator finished subscribing")
	errCheck := wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
		subState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subState, "AtLatestKnown") == 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Subscription %s in namespace %v does not have expected status", sub.name, sub.namespace))

	g.By("(1.5) Get csvName")
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", sub.namespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			e2e.Logf("CSV check complete!!!")
			return true, nil

		}
		return false, nil

	})
	exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("CSV %v in %v namespace does not have expected status", csvName, sub.namespace))

	return true
}

func createMetalLBCR(oc *exutil.CLI, metallbcr metalLBCRResource, metalLBCRTemplate string) (status bool) {
	g.By("Creating MetalLB CR from template")

	err := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", metallbcr.template, "-p", "NAME="+metallbcr.name, "NAMESPACE="+metallbcr.namespace)
	e2e.Logf("Error creating MetalLB CR %v", err)

	err = waitForPodWithLabelReady(oc, metallbcr.namespace, "component=speaker")
	exutil.AssertWaitPollNoErr(err, "The pods with label component=speaker are not ready")
	if err != nil {
		e2e.Logf("Speaker Pods did not transition to ready state %v", err)
		return false
	}
	err = waitForPodWithLabelReady(oc, metallbcr.namespace, "component=controller")
	exutil.AssertWaitPollNoErr(err, "The pod with label component=controller is not ready")
	if err != nil {
		e2e.Logf("Controller pod did not transition to ready state %v", err)
		return false
	}
	e2e.Logf("Controller and speaker pods created successfully")
	return true

}

func validateAllWorkerNodeMCR(oc *exutil.CLI, namespace string) bool {
	podList, err := exutil.GetAllPodsWithLabel(oc, namespace, "component=speaker")

	if err != nil {
		e2e.Logf("Unable to get list of speaker pods %s", err)
		return false
	}
	nodeList, err := exutil.GetClusterNodesBy(oc, "worker")
	if len(podList) != len(nodeList) {
		e2e.Logf("Speaker pods not scheduled on all worker nodes")
	}
	if err != nil {
		e2e.Logf("Unable to get nodes to determine if node is worker node  %s", err)
		return false
	}
	// Iterate over the speaker pods to validate they are scheduled on node that is worker node
	for _, pod := range podList {
		nodeName, _ := exutil.GetPodNodeName(oc, namespace, pod)
		e2e.Logf("Pod %s, node name %s", pod, nodeName)
		if isWorkerNode(oc, nodeName, nodeList) == false {
			return false
		}

	}
	return true

}

func isWorkerNode(oc *exutil.CLI, nodeName string, nodeList []string) bool {
	for i := 0; i <= (len(nodeList) - 1); i++ {
		if nodeList[i] == nodeName {
			return true
		}
	}
	return false

}

func createAddressPoolCR(oc *exutil.CLI, addresspool addressPoolResource, addressPoolTemplate string) (status bool) {
	err := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", addresspool.template, "-p", "NAME="+addresspool.name, "NAMESPACE="+addresspool.namespace, "PROTOCOL="+addresspool.protocol, "ADDRESS1="+addresspool.addresses[0], "ADDRESS2="+addresspool.addresses[1])
	if err != nil {
		e2e.Logf("Error creating addresspool %v", err)
		return false
	}
	return true

}

func createLoadBalancerService(oc *exutil.CLI, loadBalancerSvc loadBalancerServiceResource, loadBalancerServiceTemplate string) (status bool) {
	err := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", loadBalancerSvc.template, "-p", "NAME="+loadBalancerSvc.name, "NAMESPACE="+loadBalancerSvc.namespace, "EXTERNALTRAFFICPOLICY="+loadBalancerSvc.externaltrafficpolicy)
	if err != nil {
		e2e.Logf("Error creating LoadBalancerService %v", err)
		return false
	}
	return true
}

func checkLoadBalancerSvcStatus(oc *exutil.CLI, namespace string, svcName string) error {

	return wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		e2e.Logf("Checking status of service %s", svcName)
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}").Output()
		if err != nil {
			e2e.Logf("MetalLB failed to get service status, error:%s. Trying again", err)
			return false, nil
		}
		if strings.Contains(output, "Pending") {
			e2e.Logf("MetalLB failed to assign address to service, error:%s. Trying again", err)
			return false, nil
		}
		return true, nil

	})

}

func getLoadBalancerSvcIP(oc *exutil.CLI, namespace string, svcName string) string {
	svcIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("LoadBalancer service %s's, IP is :%s", svcName, svcIP)
	return svcIP
}

func validateService(oc *exutil.CLI, nodeName string, svcExternalIP string) bool {
	e2e.Logf("Validating LoadBalancer service with IP %s", svcExternalIP)
	stdout, err := exutil.DebugNode(oc, nodeName, "curl", svcExternalIP, "--connect-timeout", "30")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(stdout).Should(o.ContainSubstring("Hello OpenShift!"))
	if err != nil {
		e2e.Logf("Error %s", err)
		return false
	}
	return true

}

func deleteMetalLBCR(oc *exutil.CLI, rs metalLBCRResource) {
	e2e.Logf("delete %s %s in namespace %s", "metallb", rs.name, rs.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("metallb", rs.name, "-n", rs.namespace).Execute()
}

func deleteAddressPool(oc *exutil.CLI, rs addressPoolResource) {
	e2e.Logf("delete %s %s in namespace %s", "addresspool", rs.name, rs.namespace)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("addresspool", rs.name, "-n", rs.namespace).Execute()
}
