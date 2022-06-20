package networking

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-services", exutil.KubeConfigPath())
	// author: huirwang@redhat.com
	g.It("Author:huirwang-High-50347-internalTrafficPolicy set Local for pod/node to service access", func() {
		var (
			buildPruningBaseDir    = exutil.FixturePath("testdata", "networking")
			pingPodNodeTemplate    = filepath.Join(buildPruningBaseDir, "ping-for-pod-specific-node-template.yaml")
			genericServiceTemplate = filepath.Join(buildPruningBaseDir, "service-generic-template.yaml")
		)

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(nodeList.Items) < 2 {
			g.Skip("This case requires 2 nodes, but the cluster has less than two nodes")
		}
		g.By("Create a namespace")
		oc.SetupProject()
		ns1 := oc.Namespace()

		g.By("create 1st hello pod in ns1")

		pod1 := pingPodResourceNode{
			name:      "hello-pod1",
			namespace: ns1,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod1.createPingPodNode(oc)
		waitPodReady(oc, ns1, pod1.name)

		g.By("Create a test service which is in front of the above pods")
		svc := genericServiceResource{
			servicename:           "test-service",
			namespace:             ns1,
			protocol:              "TCP",
			selector:              "hello-pod",
			serviceType:           "ClusterIP",
			ipFamilyPolicy:        "",
			internalTrafficPolicy: "Local",
			externalTrafficPolicy: "", //This no value parameter will be ignored
			template:              genericServiceTemplate,
		}
		svc.ipFamilyPolicy = "SingleStack"
		svc.createServiceFromParams(oc)

		g.By("Create second namespace")
		oc.SetupProject()
		ns2 := oc.Namespace()

		g.By("Create a pod hello-pod2 in second namespace, pod located the same node")
		pod2 := pingPodResourceNode{
			name:      "hello-pod2",
			namespace: ns2,
			nodename:  nodeList.Items[0].Name,
			template:  pingPodNodeTemplate,
		}
		pod2.createPingPodNode(oc)
		waitPodReady(oc, ns2, pod2.name)

		g.By("Create second pod hello-pod3 in second namespace, pod located on the different node")
		pod3 := pingPodResourceNode{
			name:      "hello-pod3",
			namespace: ns2,
			nodename:  nodeList.Items[1].Name,
			template:  pingPodNodeTemplate,
		}
		pod3.createPingPodNode(oc)
		waitPodReady(oc, ns2, pod3.name)

		g.By("curl from hello-pod2 to service:port")
		CurlPod2SvcPass(oc, ns2, ns1, "hello-pod2", "test-service")

		g.By("curl from hello-pod3 to service:port should be failling")
		CurlPod2SvcFail(oc, ns2, ns1, "hello-pod3", "test-service")

		g.By("Curl from node0 to service:port")
		//Due to bug 2078691,skip below step for now.
		//CurlNode2SvcPass(oc, pod1.nodename, ns1,"test-service")
		g.By("Curl from node1 to service:port")
		CurlNode2SvcFail(oc, nodeList.Items[1].Name, ns1, "test-service")

		ipStackType := checkIPStackType(oc)

		if ipStackType == "dualstack" {
			g.By("Delete testservice from ns")
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("svc", "test-service", "-n", ns1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Checking pod to svc:port behavior now on with PreferDualStack Service")
			svc.ipFamilyPolicy = "PreferDualStack"
			svc.createServiceFromParams(oc)
			g.By("curl from hello-pod2 to service:port")
			CurlPod2SvcPass(oc, ns2, ns1, "hello-pod2", "test-service")

			g.By("curl from hello-pod3 to service:port should be failling")
			CurlPod2SvcFail(oc, ns2, ns1, "hello-pod3", "test-service")

			g.By("Curl from node0 to service:port")
			//Due to bug 2078691,skip below step for now.
			//CurlNode2SvcPass(oc, pod1.nodename, ns1,"test-service")
			g.By("Curl from node1 to service:port")
			CurlNode2SvcFail(oc, nodeList.Items[1].Name, ns1, "test-service")

		}
	})
})
