package networking

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"strings"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-ovnkubernetes", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		networkType := exutil.CheckNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Incompatible networkType, skipping test!!!")
		}
	})

	// author: anusaxen@redhat.com
	g.It("Author:anusaxen-Medium-49216-[BZ 2009857] ovnkube-node logs should not print api token in logs. ", func() {
		workerNode, err := exutil.GetFirstWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		ovnkubePod, err := exutil.GetPodName(oc, "openshift-ovn-kubernetes", "app=ovnkube-node", workerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		podlogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(ovnkubePod, "-n", "openshift-ovn-kubernetes", "-c", "ovnkube-node").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podlogs).NotTo(o.ContainSubstring("kube-api-token"))
		g.By("ovnkube-node logs doesn't contain api-token")
	})

})
