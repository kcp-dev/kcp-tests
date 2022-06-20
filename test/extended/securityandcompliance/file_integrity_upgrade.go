package securityandcompliance

import (
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance Pre-check and post-check for file integrity operator upgrade", func() {
	defer g.GinkgoRecover()
	const (
		ns1 = "openshift-file-integrity"
		ns2 = "openshift-file-integrity2"
	)
	var (
		oc = exutil.NewCLI("file-integrity-"+getRandomString(), exutil.KubeConfigPath())
	)

	g.Context("When the file-integrity-operator is installed", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
			fioTemplate         = filepath.Join(buildPruningBaseDir, "fileintegrity.yaml")
			fi1                 = fileintegrity{
				name:              "example-fileintegrity",
				namespace:         "",
				configname:        "",
				configkey:         "",
				graceperiod:       15,
				debug:             false,
				nodeselectorkey:   "node.openshift.io/os_id",
				nodeselectorvalue: "rhcos",
				template:          fioTemplate,
			}
		)

		g.BeforeEach(func() {
			g.By("Check csv and pods for ns1 !!!")
			rsCsvName := getResourceNameWithKeywordForNamespace(oc, "csv", "file-integrity-operator", ns1)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", rsCsvName, "-n", ns1, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "file-integrity-operator", ok, []string{"pod", "--selector=name=file-integrity-operator", "-n",
				ns1, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Check file-integrity Operator pod is in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=file-integrity-operator", "-n",
				ns1, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Check csv and pods for ns2 !!!")
			rsCsvNames := getResourceNameWithKeywordForNamespace(oc, "csv", "file-integrity-operator", ns2)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", rsCsvNames, "-n", ns2, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "file-integrity-operator", ok, []string{"pod", "--selector=name=file-integrity-operator", "-n",
				ns2, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Check file-integrity Operator pod is in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=file-integrity-operator", "-n",
				ns2, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-High-39254-Critical-42663-Critical-45366-precheck for file integrity operator", func() {
			g.By("Create file integrity object  !!!\n")
			fi1.namespace = ns1
			err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace, "GRACEPERIOD="+strconv.Itoa(fi1.graceperiod),
				"DEBUG="+strconv.FormatBool(fi1.debug), "NODESELECTORKEY="+fi1.nodeselectorkey, "NODESELECTORVALUE="+fi1.nodeselectorvalue)
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, fi1.name, ok, []string{"fileintegrity", "-n", fi1.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check aid pod and file integrity object status.. !!!\n")
			aidpodNames, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l app=aide-example-fileintegrity", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			aidpodName := strings.Fields(aidpodNames)
			for _, v := range aidpodName {
				newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", v, "-n", fi1.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			}
			fionodeNames, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			fionodeName := strings.Fields(fionodeNames)
			for _, v := range fionodeName {
				fi1.checkFileintegritynodestatus(oc, v, "Succeeded")
			}
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-High-39254-Critical-42663-Critical-45366-postcheck for file integrity operator", func() {
			fi1.namespace = ns1
			defer cleanupObjects(oc,
				objectTableRef{"fileintegrity", ns2, fi1.name},
				objectTableRef{"project", ns1, ns1},
				objectTableRef{"project", ns2, ns2})

			g.By("Check aid pod and file integrity object status after upgrade.. !!!\n")
			aidpodNames, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l app=aide-example-fileintegrity", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			aidpodName := strings.Fields(aidpodNames)
			for _, v := range aidpodName {
				newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", v, "-n", fi1.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			}
			fionodeNames, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			fionodeName := strings.Fields(fionodeNames)
			for _, v := range fionodeName {
				fi1.checkFileintegritynodestatus(oc, v, "Succeeded")
			}

			g.By("Delete file integrity object ns1 !!!\n")
			cleanupObjects(oc, objectTableRef{"fileintegrity", ns1, fi1.name})

			g.By("Create file integrity object ns2 !!!\n")
			fi1.namespace = ns2
			err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace, "GRACEPERIOD="+strconv.Itoa(fi1.graceperiod),
				"DEBUG="+strconv.FormatBool(fi1.debug), "NODESELECTORKEY="+fi1.nodeselectorkey, "NODESELECTORVALUE="+fi1.nodeselectorvalue)
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, fi1.name, ok, []string{"fileintegrity", "-n", fi1.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check aid pod and file integrity object status.. !!!\n")
			aidepodNames, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l app=aide-example-fileintegrity", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			aidepodName := strings.Fields(aidepodNames)
			for _, v := range aidepodName {
				newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", v, "-n", fi1.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			}
			fioNodeNames, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-n", fi1.namespace,
				"-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			fioNodeName := strings.Fields(fioNodeNames)
			for _, v := range fioNodeName {
				fi1.checkFileintegritynodestatus(oc, v, "Succeeded")
			}
		})
	})
})
