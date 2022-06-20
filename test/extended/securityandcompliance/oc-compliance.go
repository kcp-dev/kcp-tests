package securityandcompliance

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance The OC Compliance plugin makes compliance operator easy to use", func() {
	defer g.GinkgoRecover()

	var (
		oc                         = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
		dr                         = make(describerResrouce)
		buildPruningBaseDir        string
		ogCoTemplate               string
		catsrcCoTemplate           string
		subCoTemplate              string
		scansettingTemplate        string
		scansettingbindingTemplate string
		tprofileWithoutVarTemplate string
		catSrc                     catalogSourceDescription
		ogD                        operatorGroupDescription
		subD                       subscriptionDescription
	)

	g.BeforeEach(func() {
		buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
		ogCoTemplate = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcCoTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subCoTemplate = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		scansettingTemplate = filepath.Join(buildPruningBaseDir, "oc-compliance-scansetting.yaml")
		scansettingbindingTemplate = filepath.Join(buildPruningBaseDir, "oc-compliance-scansettingbinding.yaml")
		tprofileWithoutVarTemplate = filepath.Join(buildPruningBaseDir, "tailoredprofile-withoutvariable.yaml")

		catSrc = catalogSourceDescription{
			name:        "compliance-operator",
			namespace:   "",
			displayName: "openshift-compliance-operator",
			publisher:   "Red Hat",
			sourceType:  "grpc",
			address:     "",
			template:    catsrcCoTemplate,
		}
		ogD = operatorGroupDescription{
			name:      "openshift-compliance",
			namespace: "",
			template:  ogCoTemplate,
		}
		subD = subscriptionDescription{
			subName:                "compliance-operator",
			namespace:              "",
			channel:                "release-0.1",
			ipApproval:             "Automatic",
			operatorPackage:        "compliance-operator",
			catalogSourceName:      "compliance-operator",
			catalogSourceNamespace: "",
			startingCSV:            "",
			currentCSV:             "",
			installedCSV:           "",
			template:               subCoTemplate,
			singleNamespace:        true,
		}

		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	g.Context("When the compliance-operator is installed", func() {

		var itName string

		g.BeforeEach(func() {
			oc.SetupProject()
			catSrc.namespace = oc.Namespace()
			catSrc.address = getIndexFromURL("compliance")
			ogD.namespace = oc.Namespace()
			subD.namespace = oc.Namespace()
			subD.catalogSourceName = catSrc.name
			subD.catalogSourceNamespace = catSrc.namespace
			itName = g.CurrentGinkgoTestDescription().TestText
			g.By("Create catalogSource !!!")
			e2e.Logf("Here catsrc namespace : %v\n", catSrc.namespace)
			catSrc.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catSrc.name, "-n", catSrc.namespace,
				"-o=jsonpath={.status..lastObservedState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "-n", catSrc.namespace,
				"-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Create operatorGroup !!!")
			ogD.create(oc, itName, dr)

			g.By("Create subscription for above catalogsource !!!")
			subD.create(oc, itName, dr)
			e2e.Logf("Here subscp namespace : %v\n", subD.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, []string{"sub", subD.subName, "-n",
				subD.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("Check CSV is created sucessfully !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subD.installedCSV, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check Compliance Operator & profileParser pods are created !!!")
			newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator", ok, []string{"pod", "--selector=name=compliance-operator",
				"-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos4-e2e-test-compliance", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check Compliance Operator & profileParser pods are in running state !!!")
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=name=compliance-operator", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=ocp4", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, compare, "Running", ok, []string{"pod", "--selector=profile-bundle=rhcos4", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Compliance Operator sucessfully installed !!! ")
		})

		g.AfterEach(func() {
			g.By("Remove compliance-operator default objects")
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			cleanupObjects(oc,
				objectTableRef{"profilebundle.compliance", subD.namespace, "ocp4"},
				objectTableRef{"profilebundle.compliance", subD.namespace, "rhcos4"},
				objectTableRef{"deployment", subD.namespace, "compliance-operator"})
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-40681-The oc compliance plugin rerun set of scans on command [Slow]", func() {

			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "master-scansetting",
					namespace:             "",
					roles1:                "master",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis-node",
					scansettingname: "master-scansetting",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name})

			g.By("Check default profiles name ocp4-cis-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis-node")

			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name

			g.By("Create scansetting !!!\n")
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "master-scansetting", ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status, name and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteName(oc, "co-requirement")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check ComplianceSuite status, name and result after first rescan.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			subD.complianceSuiteName(oc, "co-requirement")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			_, err1 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())

			g.By("Check ComplianceSuite status, name and result after second rescan.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			subD.complianceSuiteName(oc, "co-requirement")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			_, err2 := OcComplianceCLI().Run("rerun-now").Args("compliancescan", "ocp4-cis-node-master", "-n", subD.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())

			g.By("Check ComplianceScan status, name and result after third rescan.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "RUNNING", ok, []string{"compliancescan", "ocp4-cis-node-master", "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", "ocp4-cis-node-master", "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			subD.complianceScanName(oc, "ocp4-cis-node-master")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("The ocp-40681 The oc compliance plugin has performed rescan on command successfully... !!!!\n ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-NonPreRelease-High-41185-The oc compliance controls command reports the compliance standards and controls that is benchmark fulfil for profiles [Slow]", func() {

			g.By("Check default profilebundles name and status.. !!!\n")
			subD.getProfileBundleNameandStatus(oc, "ocp4", "VALID")
			subD.getProfileBundleNameandStatus(oc, "rhcos4", "VALID")

			g.By("Check default profiles name.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-cis-node")
			subD.getProfileName(oc, "ocp4-e8")
			subD.getProfileName(oc, "ocp4-moderate")
			subD.getProfileName(oc, "ocp4-moderate-node")
			subD.getProfileName(oc, "ocp4-nerc-cip")
			subD.getProfileName(oc, "ocp4-nerc-cip-node")
			subD.getProfileName(oc, "rhcos4-e8")
			subD.getProfileName(oc, "rhcos4-moderate")
			subD.getProfileName(oc, "rhcos4-nerc-cip")

			g.By("Check profile standards and controls.. !!!\n")
			assertCheckProfileControls(oc, "ocp4-cis", [...]string{"CIS-OCP     | 1.2.1", "NIST-800-53 | AC-2"})
			assertCheckProfileControls(oc, "ocp4-cis-node", [...]string{"NIST-800-53 | CM-6", "CIS-OCP     | 1.1.1"})
			assertCheckProfileControls(oc, "ocp4-e8", [...]string{"CIS-OCP     | 1.2.34", "NIST-800-53 | AC-2(1)"})
			assertCheckProfileControls(oc, "ocp4-moderate", [...]string{"CIS-OCP     | 1.2.1", "NIST-800-53 | AC-12"})
			assertCheckProfileControls(oc, "ocp4-moderate-node", [...]string{"NERC-CIP    | CIP-003-8 R1.3", "PCI-DSS     | Req-10.5.2"})
			assertCheckProfileControls(oc, "ocp4-nerc-cip", [...]string{"NERC-CIP    | CIP-003-8 R1.3", "PCI-DSS     | Req-1.1.4"})
			assertCheckProfileControls(oc, "ocp4-nerc-cip-node", [...]string{"PCI-DSS     | Req-10.5.2", "NERC-CIP    | CIP-003-8 R1.3"})
			assertCheckProfileControls(oc, "rhcos4-e8", [...]string{"NIST-800-53", "AC-17(2)"})
			assertCheckProfileControls(oc, "rhcos4-moderate", [...]string{"NIST-800-53", "AC-12"})
			assertCheckProfileControls(oc, "rhcos4-nerc-cip", [...]string{"NERC-CIP    | CIP-002-5 R1.1", "PCI-DSS     | Req-10.1"})

			g.By("The ocp-41185 Successfully verify compliance standards and controls for all profiles ... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41190-The view result command of oc compliance plugin exposes more information about a compliance result [Slow]", func() {

			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "master-scansetting",
					namespace:             "",
					roles1:                "master",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					scansettingname: "master-scansetting",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name})

			g.By("Check default profiles name ocp4-cis.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")

			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name

			g.By("Create scansetting !!!\n")
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "master-scansetting", ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify rules status and result through oc-compliance view-result command.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-cis-api-server-admission-control-plugin-alwaysadmit", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			assertRuleResult(oc, "ocp4-cis-api-server-admission-control-plugin-alwaysadmit", subD.namespace,
				[...]string{"Status               | PASS", "Result Object Name   | ocp4-cis-api-server-admission-control-plugin-alwaysadmit"})

			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-cis-audit-log-forwarding-enabled", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			assertRuleResult(oc, "ocp4-cis-audit-log-forwarding-enabled", subD.namespace,
				[...]string{"Status               | FAIL", "Result Object Name   | ocp4-cis-audit-log-forwarding-enabled"})

			g.By("The ocp-41190 Successfully verify oc-compliance view-result reports result in details... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41182-The bind command of oc compliance plugin will take the given parameters and create a ScanSettingBinding object [Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "master-scansetting",
					namespace:             "",
					roles1:                "master",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				tp = tailoredProfileWithoutVarDescription{
					name:         "ocp4-cis-custom",
					namespace:    "",
					extends:      "ocp4-cis",
					title:        "new profile from scratch",
					description:  "new profile with specific rules",
					enrulename1:  "ocp4-scc-limit-root-containers",
					enrulename2:  "ocp4-scheduler-no-bind-address",
					disrulename1: "ocp4-api-server-encryption-provider-cipher",
					disrulename2: "ocp4-scc-drop-container-capabilities",
					template:     tprofileWithoutVarTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, "my-binding"},
				objectTableRef{"tailoredprofile", subD.namespace, tp.name},
				objectTableRef{"scansetting", subD.namespace, ss.name})

			g.By("Check default profiles name ocp4-cis.. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			_, err := OcComplianceCLI().Run("bind").Args("-N", "my-binding", "profile/ocp4-moderate", "-n", subD.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verify scansettingbinding, ScanSetting, profile objects created..!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "my-binding", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-moderate", ok, []string{"scansettingbinding", "my-binding", "-n", subD.namespace,
				"-o=jsonpath={.profiles}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "default", ok, []string{"scansettingbinding", "my-binding", "-n", subD.namespace,
				"-o=jsonpath={.settingsRef}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", "my-binding", "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, "my-binding", "NON-COMPLIANT")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, "my-binding"})

			g.By("Create scansetting and tailoredProfile objects.. !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			tp.namespace = subD.namespace
			tp.create(oc, itName, dr)
			_, err1 := OcComplianceCLI().Run("bind").Args("-N", "my-binding", "-S", "master-scansetting", "tailoredprofile/ocp4-cis-custom", "-n", subD.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())

			g.By("Verify scansettingbinding, ScanSetting, profile objects created..!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "my-binding", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ocp4-cis-custom", ok, []string{"scansettingbinding", "my-binding", "-n", subD.namespace,
				"-o=jsonpath={.profiles}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "master-scansetting", ok, []string{"scansettingbinding", "my-binding", "-n", subD.namespace,
				"-o=jsonpath={.settingsRef}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			checkComplianceSuiteStatus(oc, "my-binding", subD.namespace, "DONE")
			subD.complianceSuiteResult(oc, "my-binding", "NON-COMPLIANT")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			assertDryRunBind(oc, "profile/ocp4-cis", subD.namespace, "name: ocp4-cis")
			assertDryRunBind(oc, "profile/ocp4-cis-node", subD.namespace, "name: ocp4-cis-node")
			assertDryRunBind(oc, "tailoredprofile/ocp4-cis-custom", subD.namespace, "name: ocp4-cis-custom")

			g.By("The ocp-41182 verify oc-compliance bind command works as per desing... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-40714-The oc compliance helps to download the raw compliance results from the Persistent Volume [Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "master-scansetting",
					namespace:             "",
					roles1:                "master",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					scansettingname: "master-scansetting",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name})

			g.By("Check default profiles name ocp4-cis.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")

			g.By("Create scansetting !!!\n")
			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "master-scansetting", ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			assertfetchRawResult(oc, ssb.name, subD.namespace)

			g.By("The ocp-40714 fetches raw result of scan successfully... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41195-The oc compliance plugin fetches the fixes or remediations from a rule profile or remediation objects [Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "master-scansetting",
					namespace:             "",
					roles1:                "master",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					scansettingname: "master-scansetting",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name})

			g.By("Check default profiles name ocp4-cis.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")

			g.By("Create scansetting !!!\n")
			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "master-scansetting", ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "co-requirement", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			assertfetchFixes(oc, "rule", "ocp4-api-server-encryption-provider-cipher", subD.namespace)
			assertfetchFixes(oc, ssb.profilekind1, ssb.profilename1, subD.namespace)

			g.By("The ocp-41195 fetches fixes from a rule profile or remediation objects successfully... !!!!\n ")
		})
	})
})
