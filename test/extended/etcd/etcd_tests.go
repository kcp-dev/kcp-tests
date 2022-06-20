package etcd

import (
	"os/exec"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd] ETCD", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: skundu@redhat.com
	g.It("Author:skundu-Critical-43330-Ensure a safety net for the 3.4 to 3.5 etcd upgrade", func() {
		var (
			err error
			msg string
		)
		g.By("Test for case OCP-43330 Ensure a safety net for the 3.4 to 3.5 etcd upgrade")
		oc.SetupProject()

		e2e.Logf("Discover all the etcd pods")
		etcdPodList := getPodListByLabel(oc, "etcd=true")

		e2e.Logf("verify whether etcd version is 3.5")
		output, err := exutil.RemoteShPod(oc, "openshift-etcd", etcdPodList[0], "etcdctl")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(output).To(o.ContainSubstring("3.5"))

		e2e.Logf("get the Kubernetes version")
		version, err := exec.Command("bash", "-c", "oc version | grep Kubernetes |awk '{print $3}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		s_version := string(version)
		kube_ver := strings.Split(s_version, "+")[0]

		e2e.Logf("retrieve all the master node")
		masterNodeList := getNodeListByLabel(oc, "node-role.kubernetes.io/master=")

		e2e.Logf("verify the kubelet version in node details")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("node", masterNodeList[0], "-o", "custom-columns=VERSION:.status.nodeInfo.kubeletVersion").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(kube_ver))

	})
})
