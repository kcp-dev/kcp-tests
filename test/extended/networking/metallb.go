//MetalLB operator tests
package networking

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN metallb", func() {
	defer g.GinkgoRecover()

	var (
		oc          = exutil.NewCLI("networking-metallb", exutil.KubeConfigPath())
		opNamespace = "metallb-system"
		opName      = "metallb-operator"
		testDataDir = exutil.FixturePath("testdata", "networking/metallb")
	)

	g.BeforeEach(func() {
		platform := checkPlatform(oc)
		if !strings.Contains(platform, "vsphere") {
			g.Skip("Skipping for unsupported platform, not vsphere!")
		}
		namespaceTemplate := filepath.Join(testDataDir, "namespace-template.yaml")
		operatorGroupTemplate := filepath.Join(testDataDir, "operatorgroup-template.yaml")
		subscriptionTemplate := filepath.Join(testDataDir, "subscription-template.yaml")
		sub := subscriptionResource{
			name:             "metallb-operator-sub",
			namespace:        opNamespace,
			operatorName:     opName,
			channel:          "stable",
			catalog:          "qe-app-registry",
			catalogNamespace: "openshift-marketplace",
			template:         subscriptionTemplate,
		}
		ns := namespaceResource{
			name:     opNamespace,
			template: namespaceTemplate,
		}
		og := operatorGroupResource{
			name:             opName,
			namespace:        opNamespace,
			targetNamespaces: opNamespace,
			template:         operatorGroupTemplate,
		}

		operatorInstall(oc, sub, ns, og)

	})

	g.It("Author:asood-High-43074-MetalLB-Operator installation ", func() {
		g.By("Checking metalLB operator installation")
		e2e.Logf("Operator install check successfull as part of setup !!!!!")
		g.By("SUCCESS - MetalLB operator installed")

	})

	g.It("Author:asood-High-46560-MetalLB-CR All Workers Creation [Serial]", func() {

		g.By("Creating metalLB CR on all the worker nodes in cluster")
		metallbCRTemplate := filepath.Join(testDataDir, "metallb-cr-template.yaml")
		metallbCR := metalLBCRResource{
			name:      "metallb",
			namespace: opNamespace,
			template:  metallbCRTemplate,
		}
		defer deleteMetalLBCR(oc, metallbCR)
		result := createMetalLBCR(oc, metallbCR, metallbCRTemplate)
		o.Expect(result).To(o.BeTrue())

		g.By("SUCCESS - MetalLB CR Created")
		g.By("Validate speaker pods scheduled on worker nodes")
		result = validateAllWorkerNodeMCR(oc, opNamespace)
		o.Expect(result).To(o.BeTrue())

		g.By("SUCCESS - Speaker pods are scheduled on worker nodes")

	})

	g.It("Author:asood-High-43075-Create L2 LoadBalancer Service [Serial]", func() {
		var ns string

		g.By("1. Create MetalLB CR")
		metallbCRTemplate := filepath.Join(testDataDir, "metallb-cr-template.yaml")
		metallbCR := metalLBCRResource{
			name:      "metallb",
			namespace: opNamespace,
			template:  metallbCRTemplate,
		}
		defer deleteMetalLBCR(oc, metallbCR)
		result := createMetalLBCR(oc, metallbCR, metallbCRTemplate)
		o.Expect(result).To(o.BeTrue())

		g.By("SUCCESS - MetalLB CR Created")

		g.By("2. Create Layer2 addresspool")
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		var addresses []string
		var addressIPv4 string
		for i := 0; i <= (len(nodeList.Items) - 1); i++ {
			addressIPv4 = getNodeIPv4(oc, opNamespace, nodeList.Items[i].Name)
			addresses = append(addresses, addressIPv4+"-"+addressIPv4)
		}

		addresspoolTemplate := filepath.Join(testDataDir, "addresspool-template.yaml")
		addresspool := addressPoolResource{
			name:      "addresspool-l2",
			namespace: opNamespace,
			protocol:  "layer2",
			addresses: addresses,
			template:  addresspoolTemplate,
		}
		defer deleteAddressPool(oc, addresspool)
		result = createAddressPoolCR(oc, addresspool, addresspoolTemplate)
		o.Expect(result).To(o.BeTrue())
		g.By("SUCCESS - Layer2 addresspool")

		g.By("3. Create LoadBalancer services using Layer 2 addresses")
		g.By("3.1 Create a namespace")
		loadBalancerServiceTemplate := filepath.Join(testDataDir, "loadbalancer-svc-template.yaml")
		oc.SetupProject()
		ns = oc.Namespace()

		g.By("3.2 Create a service with extenaltrafficpolicy local")
		svc1 := loadBalancerServiceResource{
			name:                  "hello-world-local",
			namespace:             ns,
			externaltrafficpolicy: "Local",
			template:              loadBalancerServiceTemplate,
		}
		result = createLoadBalancerService(oc, svc1, loadBalancerServiceTemplate)
		o.Expect(result).To(o.BeTrue())

		g.By("3.3 Create a service with extenaltrafficpolicy Cluster")
		svc2 := loadBalancerServiceResource{
			name:                  "hello-world-cluster",
			namespace:             ns,
			externaltrafficpolicy: "Cluster",
			template:              loadBalancerServiceTemplate,
		}
		result = createLoadBalancerService(oc, svc2, loadBalancerServiceTemplate)
		o.Expect(result).To(o.BeTrue())

		g.By("3.3 SUCCESS - Services created successfully")

		g.By("3.4 Validate LoadBalancer services")
		err = checkLoadBalancerSvcStatus(oc, svc1.namespace, svc1.name)
		o.Expect(err).NotTo(o.HaveOccurred())

		svcIP := getLoadBalancerSvcIP(oc, svc1.namespace, svc1.name)
		e2e.Logf("The service %s External IP is %q", svc1.name, svcIP)
		result = validateService(oc, nodeList.Items[2].Name, svcIP)
		o.Expect(result).To(o.BeTrue())

		err = checkLoadBalancerSvcStatus(oc, svc2.namespace, svc2.name)
		o.Expect(err).NotTo(o.HaveOccurred())

		svcIP = getLoadBalancerSvcIP(oc, svc2.namespace, svc2.name)
		e2e.Logf("The service %s External IP is %q", svc2.name, svcIP)
		result = validateService(oc, nodeList.Items[2].Name, svcIP)
		o.Expect(result).To(o.BeTrue())

	})

})
