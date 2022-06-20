package workloads

import (
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cli] Workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("ocmirror", exutil.KubeConfigPath())
	)
	g.It("Author:yinzhou-Medium-46517-List operator content with different options", func() {
		dirname := "/tmp/case46517"
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(dirname)

		dockerCreFile, homePath, err := locateDockerCred(oc, dirname)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			os.RemoveAll(dockerCreFile)
			_, err = os.Stat(homePath + "/.docker/config.json.back")
                        if err == nil {
				copyFile(homePath+"/.docker/config.json.back", homePath+"/.docker/config.json")
                        }
                }()


		out, err := oc.WithoutNamespace().WithoutKubeconf().Run("mirror").Args("list", "operators", "--version=4.11").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		checkMessage := []string{
			"registry.redhat.io/redhat/redhat-operator-index:v4.11",
			"registry.redhat.io/redhat/certified-operator-index:v4.11",
			"registry.redhat.io/redhat/community-operator-index:v4.11",
			"registry.redhat.io/redhat/redhat-marketplace-index:v4.11",
		}
		for _, v := range checkMessage {
			o.Expect(out).To(o.ContainSubstring(v))
		}
		out, err = oc.WithoutNamespace().WithoutKubeconf().Run("mirror").Args("list", "operators", "--version=4.11", "--catalog=registry.redhat.io/redhat/redhat-operator-index:v4.11").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		checkMessage = []string{
			"3scale-operator",
			"amq-online",
			"amq-streams",
			"amq7-interconnect-operator",
			"ansible-automation-platform-operator",
			"ansible-cloud-addons-operator",
			"apicast-operator",
			"businessautomation-operator",
			"cincinnati-operator",
			"cluster-logging",
			"codeready-workspaces",
			"codeready-workspaces2",
			"compliance-operator",
			"container-security-operator",
			"costmanagement-metrics-operator",
			"cryostat-operator",
			"datagrid",
			"devworkspace-operator",
			"eap",
			"elasticsearch-operator",
			"external-dns-operator",
			"file-integrity-operator",
			"fuse-apicurito",
			"fuse-console",
			"fuse-online",
			"gatekeeper-operator-product",
			"integration-operator",
			"jaeger-product",
			"jws-operator",
			"kiali-ossm",
			"kubevirt-hyperconverged",
			"mcg-operator",
			"mtc-operator",
			"mtv-operator",
			"node-healthcheck-operator",
			"node-maintenance-operator",
			"ocs-operator",
			"odf-csi-addons-operator",
			"odf-lvm-operator",
			"odf-multicluster-orchestrator",
			"odf-operator",
			"odr-cluster-operator",
			"odr-hub-operator",
			"openshift-cert-manager-operator",
			"openshift-gitops-operator",
			"openshift-pipelines-operator-rh",
			"openshift-secondary-scheduler-operator",
			"opentelemetry-product",
			"poison-pill-manager",
			"quay-bridge-operator",
			"quay-operator",
			"red-hat-camel-k",
			"redhat-oadp-operator",
			"rh-service-binding-operator",
			"rhacs-operator",
			"rhpam-kogito-operator",
			"rhsso-operator",
			"sandboxed-containers-operator",
			"serverless-operator",
			"service-registry-operator",
			"servicemeshoperator",
			"skupper-operator",
			"submariner",
			"web-terminal",
		}

		for _, v := range checkMessage {
			o.Expect(out).To(o.ContainSubstring(v))
		}
		err = oc.WithoutNamespace().WithoutKubeconf().Run("mirror").Args("list", "operators", "--catalog=registry.redhat.io/redhat/redhat-operator-index:v4.11", "--package=cluster-logging", "--channel=stable").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().WithoutKubeconf().Run("mirror").Args("list", "operators", "--catalog=registry.redhat.io/redhat/redhat-operator-index:v4.11", "--package=cluster-logging").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	})

})
