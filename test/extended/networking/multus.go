package networking

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-multus", exutil.KubeConfigPath())

	// author: weliang@redhat.com
	g.It("Author:weliang-Medium-46387-[BZ 1896533] network operator degraded due to additionalNetwork in non-existent namespace. [Disruptive]", func() {
		var (
			patchSResource = "networks.operator.openshift.io/cluster"
			patchInfo      = fmt.Sprintf("{\"spec\":{\"additionalNetworks\": [{\"name\": \"secondary\",\"namespace\":\"ocp-46387\",\"simpleMacvlanConfig\": {\"ipamConfig\": {\"staticIPAMConfig\": {\"addresses\": [{\"address\": \"10.1.1.0/24\"}] },\"type\": \"static\"}},\"type\": \"SimpleMacvlan\"}]}}")
		)

		g.By("create new namespace")
		namespace := fmt.Sprintf("ocp-46387")
		err := oc.Run("new-project").Args(namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("delete").Args("project", namespace, "--ignore-not-found").Execute()

		g.By("Configure network-attach-definition through network operator")
		patchResourceAsAdmin(oc, patchSResource, patchInfo)
		defer oc.AsAdmin().WithoutNamespace().Run("patch").Args(patchSResource, "-p", `[{"op": "remove", "path": "/spec/additionalNetworks"}]`, "--type=json").Execute()

		g.By("Check NetworkOperatorStatus")
		checkNetworkOperatorDEGRADEDState(oc)

		g.By("Delete the namespace")
		nsErr := oc.AsAdmin().Run("delete").Args("project", namespace, "--ignore-not-found").Execute()
		o.Expect(nsErr).NotTo(o.HaveOccurred())

		g.By("Check NetworkOperatorStatus after deleting namespace")
		checkNetworkOperatorDEGRADEDState(oc)
	})
})
