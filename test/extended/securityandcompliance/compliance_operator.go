package securityandcompliance

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-isc] Security_and_Compliance The Compliance Operator automates compliance check for OpenShift and CoreOS", func() {
	defer g.GinkgoRecover()

	var (
		oc                               = exutil.NewCLI("compliance-"+getRandomString(), exutil.KubeConfigPath())
		dr                               = make(describerResrouce)
		buildPruningBaseDir              string
		ogCoTemplate                     string
		catsrcCoTemplate                 string
		subCoTemplate                    string
		csuiteTemplate                   string
		csuiteRemTemplate                string
		csuitetpcmTemplate               string
		csuitetaintTemplate              string
		csuitenodeTemplate               string
		csuiteSCTemplate                 string
		cscanTemplate                    string
		cscantaintTemplate               string
		cscantaintsTemplate              string
		cscanSCTemplate                  string
		tprofileTemplate                 string
		tprofileWithoutVarTemplate       string
		scansettingTemplate              string
		scansettingSingleTemplate        string
		scansettingbindingTemplate       string
		scansettingbindingSingleTemplate string
		profilebundleTemplate            string
		pvextractpodYAML                 string
		podModifyTemplate                string
		storageClassTemplate             string
		fioTemplate                      string
		fluentdCmYAML                    string
		fluentdDmYAML                    string
		clusterLogForYAML                string
		clusterLoggingYAML               string
		ldapConfigMapYAML                string
		motdConfigMapYAML                string
		consoleNotificationYAML          string
		networkPolicyYAML                string
		machineConfigPoolYAML            string
		prometheusAuditRuleYAML          string
		wordpressRouteYAML               string
		resourceQuotaYAML                string
		tprofileWithoutDescriptionYAML   string
		tprofileWithoutTitleYAML         string
		catSrc                           catalogSourceDescription
		ogD                              operatorGroupDescription
		subD                             subscriptionDescription
		podModifyD                       podModify
		storageClass                     storageClassDescription
	)

	g.BeforeEach(func() {
		buildPruningBaseDir = exutil.FixturePath("testdata", "securityandcompliance")
		ogCoTemplate = filepath.Join(buildPruningBaseDir, "operator-group.yaml")
		catsrcCoTemplate = filepath.Join(buildPruningBaseDir, "catalogsource-image.yaml")
		subCoTemplate = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		csuiteTemplate = filepath.Join(buildPruningBaseDir, "compliancesuite.yaml")
		csuiteRemTemplate = filepath.Join(buildPruningBaseDir, "compliancesuiterem.yaml")
		csuitetpcmTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitetpconfmap.yaml")
		csuitetaintTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitetaint.yaml")
		csuitenodeTemplate = filepath.Join(buildPruningBaseDir, "compliancesuitenodes.yaml")
		csuiteSCTemplate = filepath.Join(buildPruningBaseDir, "compliancesuiteStorageClass.yaml")
		cscanTemplate = filepath.Join(buildPruningBaseDir, "compliancescan.yaml")
		cscantaintTemplate = filepath.Join(buildPruningBaseDir, "compliancescantaint.yaml")
		cscantaintsTemplate = filepath.Join(buildPruningBaseDir, "compliancescantaints.yaml")
		cscanSCTemplate = filepath.Join(buildPruningBaseDir, "compliancescanStorageClass.yaml")
		tprofileTemplate = filepath.Join(buildPruningBaseDir, "tailoredprofile.yaml")
		tprofileWithoutVarTemplate = filepath.Join(buildPruningBaseDir, "tailoredprofile-withoutvariable.yaml")
		scansettingTemplate = filepath.Join(buildPruningBaseDir, "scansetting.yaml")
		scansettingSingleTemplate = filepath.Join(buildPruningBaseDir, "scansettingsingle.yaml")
		scansettingbindingTemplate = filepath.Join(buildPruningBaseDir, "scansettingbinding.yaml")
		scansettingbindingSingleTemplate = filepath.Join(buildPruningBaseDir, "oc-compliance-scansettingbinding.yaml")
		profilebundleTemplate = filepath.Join(buildPruningBaseDir, "profilebundle.yaml")
		pvextractpodYAML = filepath.Join(buildPruningBaseDir, "pv-extract-pod.yaml")
		podModifyTemplate = filepath.Join(buildPruningBaseDir, "pod_modify.yaml")
		storageClassTemplate = filepath.Join(buildPruningBaseDir, "storage_class.yaml")
		fioTemplate = filepath.Join(buildPruningBaseDir, "fileintegrity.yaml")
		fluentdCmYAML = filepath.Join(buildPruningBaseDir, "fluentdConfigMap.yaml")
		fluentdDmYAML = filepath.Join(buildPruningBaseDir, "fluentdDeployment.yaml")
		clusterLogForYAML = filepath.Join(buildPruningBaseDir, "ClusterLogForwarder.yaml")
		clusterLoggingYAML = filepath.Join(buildPruningBaseDir, "ClusterLogging.yaml")
		ldapConfigMapYAML = filepath.Join(buildPruningBaseDir, "ldap_configmap.yaml")
		motdConfigMapYAML = filepath.Join(buildPruningBaseDir, "motd_configmap.yaml")
		consoleNotificationYAML = filepath.Join(buildPruningBaseDir, "consolenotification.yaml")
		networkPolicyYAML = filepath.Join(buildPruningBaseDir, "network-policy.yaml")
		machineConfigPoolYAML = filepath.Join(buildPruningBaseDir, "machineConfigPool.yaml")
		prometheusAuditRuleYAML = filepath.Join(buildPruningBaseDir, "prometheus-audit.yaml")
		wordpressRouteYAML = filepath.Join(buildPruningBaseDir, "wordpress-route.yaml")
		resourceQuotaYAML = filepath.Join(buildPruningBaseDir, "resource-quota.yaml")
		tprofileWithoutDescriptionYAML = filepath.Join(buildPruningBaseDir, "tailoredprofile-withoutdescription.yaml")
		tprofileWithoutTitleYAML = filepath.Join(buildPruningBaseDir, "tailoredprofile-withouttitle.yaml")
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
		podModifyD = podModify{
			name:      "",
			namespace: "",
			nodeName:  "",
			args:      "",
			template:  podModifyTemplate,
		}
		storageClass = storageClassDescription{
			name:              "",
			provisioner:       "",
			reclaimPolicy:     "",
			volumeBindingMode: "",
			template:          storageClassTemplate,
		}

		itName := g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)
	})

	g.AfterEach(func() {
		itName := g.CurrentGinkgoTestDescription().TestText
		dr.getIr(itName).cleanup()
		dr.rmIr(itName)
	})

	// author: pdhamdhe@redhat.com
	g.It("Author:pdhamdhe-Critical-34378-Install the Compliance Operator through olm using CatalogSource and Subscription", func() {

		var itName = g.CurrentGinkgoTestDescription().TestText
		oc.SetupProject()
		catSrc.namespace = oc.Namespace()
		catSrc.address = getIndexFromURL("compliance")
		ogD.namespace = oc.Namespace()
		subD.namespace = oc.Namespace()
		subD.catalogSourceName = catSrc.name
		subD.catalogSourceNamespace = catSrc.namespace

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

		// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
		// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
		defer cleanupObjects(oc,
			objectTableRef{"profilebundle.compliance", subD.namespace, "ocp4"},
			objectTableRef{"profilebundle.compliance", subD.namespace, "rhcos4"},
			objectTableRef{"deployment", subD.namespace, "compliance-operator"})

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
			g.By("Label the namespace  !!!\n")
			labelNameSpace(oc, subD.namespace, "openshift.io/cluster-monitoring=true")
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
		g.It("Author:pdhamdhe-Critical-27649-The ComplianceSuite reports the scan result as Compliant or Non-Compliant [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
				csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name},
				objectTableRef{"compliancesuite", subD.namespace, csuiteMD.name})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			csuiteMD.namespace = subD.namespace
			g.By("Create master-compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteMD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteResult(oc, csuiteMD.name, "NON-COMPLIANT")
			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check worker-compliancesuite name and result..!!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-27649 ComplianceScan has performed successfully... !!!!\n ")

		})

		/* Disabling the test case, it might be required in future release
		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-32082-The ComplianceSuite shows the scan result NOT-APPLICABLE after all rules are skipped to scan", func() {

			var (
				csuite = complianceSuiteDescription{
					name:                "worker-compliancesuite",
					namespace:           "",
					scanname:            "worker-scan",
					profile:             "xccdf_org.ssgproject.content_profile_ncp",
					content:             "ssg-rhel7-ds.xml",
					contentImage:        "quay.io/complianceascode/ocp4:latest",
					noExternalResources: true,
					nodeSelector:        "wscan",
					template:            csuiteTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuite.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuite.create(oc, itName, dr)

			g.By("Check complianceSuite Status !!!\n")
			csuite.checkComplianceSuiteStatus(oc, "DONE")

			g.By("Check worker scan pods status !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuite.name, "NOT-APPLICABLE")

			g.By("Check rule status through complianceCheckResult.. !!!\n")
			subD.getRuleStatus(oc, "SKIP")

			g.By("The ocp-32082 complianceScan has performed successfully....!!!\n")

		})*/

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33398-The Compliance Operator supports to variables in tailored profile [Slow]", func() {
			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-tailoredprofile",
					namespace:    "",
					extends:      "rhcos4-moderate",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-audit-rules-dac-modification-chmod",
					disrulename2: "rhcos4-audit-rules-etc-group-open",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, "rhcos-tailoredprofile"})

			tprofileD.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			tprofileD.create(oc, itName, dr)
			g.By("Check tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, tprofileD.name)

			g.By("Verify the tailoredprofile details through configmap ..!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_rule_sshd_disable_root_login", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_rule_audit_rules_dac_modification_chmod", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "xccdf_org.ssgproject.content_value_var_selinux_state", ok,
				[]string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace, "-o=jsonpath={.data}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "permissive", ok, []string{"configmap", "rhcos-tailoredprofile-tp", "-n", subD.namespace,
				"-o=jsonpath={.data}"}).check(oc)

			g.By("ocp-33398 The Compliance Operator supported variables in tailored profile... !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-32840-The ComplianceSuite generates through ScanSetting CR [Slow]", func() {
			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-tp",
					namespace:    "",
					extends:      "rhcos4-e8",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-no-empty-passwords",
					disrulename2: "rhcos4-audit-rules-dac-modification-chown",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "myss",
					namespace:             "",
					roles1:                "master",
					roles2:                "worker",
					rotation:              10,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "co-requirement",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "rhcos-tp",
					profilename2:    "ocp4-moderate",
					scansettingname: "",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name},
				objectTableRef{"tailoredprofile", subD.namespace, ssb.profilename1})

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, tprofileD.extends)

			tprofileD.namespace = subD.namespace
			ssb.namespace = subD.namespace
			ss.namespace = subD.namespace
			ssb.scansettingname = ss.name
			g.By("Create tailoredprofile rhcos-tp !!!\n")
			tprofileD.create(oc, itName, dr)
			g.By("Verify tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, ssb.profilename1)

			g.By("Create scansetting !!!\n")
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ss.name, ok, []string{"scansetting", "-n", ss.namespace, ss.name,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the disable rules are not available in compliancecheckresult.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-32840 The ComplianceSuite generated successfully using scansetting... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33381-Verify the ComplianceSuite could be generated from Tailored profiles [Slow]", func() {
			var (
				tprofileD = tailoredProfileDescription{
					name:         "rhcos-e8-tp",
					namespace:    "",
					extends:      "rhcos4-e8",
					enrulename1:  "rhcos4-sshd-disable-root-login",
					disrulename1: "rhcos4-no-empty-passwords",
					disrulename2: "rhcos4-audit-rules-dac-modification-chown",
					varname:      "rhcos4-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				csuiteD = complianceSuiteDescription{
					name:               "rhcos-csuite",
					namespace:          "",
					scanname:           "rhcos-scan",
					profile:            "xccdf_compliance.openshift.io_profile_rhcos-e8-tp",
					content:            "ssg-rhcos4-ds.xml",
					contentImage:       "quay.io/complianceascode/ocp4:latest",
					nodeSelector:       "wscan",
					tailoringConfigMap: "rhcos-e8-tp-tp",
					template:           csuitetpcmTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name},
				objectTableRef{"tailoredprofile", subD.namespace, tprofileD.name})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, tprofileD.extends)
			tprofileD.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			tprofileD.create(oc, itName, dr)
			g.By("Check tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, tprofileD.name)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check rhcos-csuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")
			g.By("Check rhcos-csuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the enable and disable rules.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-sshd-disable-root-login", ok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-audit-rules-dac-modification-chown", nok, []string{"compliancecheckresult", "-n", subD.namespace}).check(oc)

			g.By("ocp-33381 The ComplianceSuite performed scan successfully using tailored profile... !!!\n")

		})
		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-33611-Verify the tolerations could work for compliancescan when there is more than one taint on node [Exclusive]", func() {
			var (
				cscanD = complianceScanDescription{
					name:         "example-compliancescan3",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					key1:         "key2",
					value1:       "value2",
					operator1:    "Equal",
					nodeSelector: "wscan",
					template:     cscantaintsTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint value to one worker node.. !!!\n")
			nodeName := getOneWorkerNodeName(oc)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule", "key2=value2:NoExecute")
			labelTaintNode(oc, "node", nodeName, "taint=true")
			defer func() {
				taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-", "key2=value2:NoExecute-")
				labelTaintNode(oc, "node", nodeName, "taint-")
			}()

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if scan pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, cscanD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and recover tainted worker node.. !!!\n")
			cscanD.delete(itName, dr)

			g.By("ocp-33611 Verify the tolerations could work for compliancescan when there is more than one taints on node successfully.. !!!\n")

		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-37121-The ComplianceSuite generates through ScanSettingBinding CR with cis profile and default scansetting [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name ocp4-cis .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			g.By("Check default profiles name ocp4-cis-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, subD.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37121 The ComplianceSuite generated successfully using scansetting CR and cis profile and default scansetting... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33713-The ComplianceSuite reports the scan result as Error", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_coreos-ncp",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "ERROR")

			g.By("Check complianceScan result through configmap exit-code and result from error-msg..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "1")
			subD.getScanResultFromConfigmap(oc, "No profile matching suffix \"xccdf_org.ssgproject.content_profile_coreos-ncp\" was found.")

			g.By("The ocp-33713 complianceScan has performed successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-27705-The ComplianceScan reports the scan result Compliant or Non-Compliant", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}

				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancescan", subD.namespace, "worker-scan"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create worker-scan !!!\n")
			cscanD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check worker-scan name and result..!!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check worker-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-27705 ComplianceScan has performed successfully... !!!! ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-27762-The ComplianceScan reports the scan result Error", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_coreos-ncp",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "worker-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceScan name and result..!!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "ERROR")

			g.By("Check complianceScan result through configmap exit-code and result from error-msg..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "1")
			subD.getScanResultFromConfigmap(oc, "No profile matching suffix \"xccdf_org.ssgproject.content_profile_coreos-ncp\" was found.")

			g.By("The ocp-27762 complianceScan has performed successfully....!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-27968-Perform scan only on a subset of nodes using ComplianceScan object [Slow]", func() {
			var (
				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					template:     cscanTemplate,
				}
			)
			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, cscanMD.name})

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan !!!\n")
			cscanMD.create(oc, itName, dr)
			checkComplianceScanStatus(oc, cscanMD.name, subD.namespace, "DONE")

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, cscanMD.name)
			subD.complianceScanResult(oc, "NON-COMPLIANT")
			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-27968 ComplianceScan has performed successfully on a subset of nodes... !!!! ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33230-The compliance-operator raw result storage size is configurable", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					size:         "2Gi",
					template:     csuiteTemplate,
				}
				cscanMD = complianceScanDescription{
					name:         "master-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "master",
					size:         "3Gi",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Check pvc name and storage size for worker-scan.. !!!\n")
			subD.getPVCName(oc, "worker-scan")
			subD.getPVCSize(oc, "2Gi")

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check master-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check pvc name and storage size for master-scan ..!!!\n")
			subD.getPVCName(oc, "master-scan")
			subD.getPVCSize(oc, "3Gi")

			g.By("The ocp-33230 complianceScan has performed successfully and storage size verified ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33609-Verify the tolerations could work for compliancesuite [Exclusive]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					nodeSelector: "wscan",
					template:     csuitetaintTemplate,
				}
				csuite = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "",
					operator:     "Exists",
					nodeSelector: "wscan",
					template:     csuitetaintTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint to one worker node.. !!!\n")
			//	setTaintLabelToWorkerNode(oc)
			//	setTaintToWorkerNodeWithValue(oc)
			nodeName := getOneWorkerNodeName(oc)
			defer func() {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
				if strings.Contains(output, "value1") {
					taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
				}
				output1, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints[0].key}").Output()
				if strings.Contains(output1, "key1") {
					taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
				}
				output2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.labels.taint}").Output()
				if strings.Contains(output2, "true") {
					labelTaintNode(oc, "node", nodeName, "taint-")
				}
			}()
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")
			labelTaintNode(oc, "node", nodeName, "taint=true")

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite.. !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove csuite and taint label from worker node.. !!!\n")
			csuiteD.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")

			g.By("Taint worker node without value.. !!!\n")
			/*	defer func() {
				taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			}()*/
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule")

			csuite.namespace = subD.namespace
			g.By("Create compliancesuite.. !!!\n")
			csuite.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuite.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuite.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove csuite, taint label and key from worker node.. !!!\n")
			csuite.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")
			//	removeTaintKeyFromWorkerNode(oc)
			//	removeTaintLabelFromWorkerNode(oc)

			g.By("ocp-33609 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33610-Verify the tolerations could work for compliancescan [Exclusive]", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "value1",
					operator:     "Equal",
					nodeSelector: "wscan",
					template:     cscantaintTemplate,
				}
				cscan = complianceScanDescription{
					name:         "worker-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					key:          "key1",
					value:        "",
					operator:     "Exists",
					nodeSelector: "wscan",
					template:     cscantaintTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Label and set taint value to one worker node.. !!!\n")
			nodeName := getOneWorkerNodeName(oc)
			defer func() {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
				if strings.Contains(output, "value1") {
					taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
				}
				output1, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints[0].key}").Output()
				if strings.Contains(output1, "key1") {
					taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
				}
				output2, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.labels.taint}").Output()
				if strings.Contains(output2, "true") {
					labelTaintNode(oc, "node", nodeName, "taint-")
				}
			}()
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")
			labelTaintNode(oc, "node", nodeName, "taint=true")

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if scan pod generated for tainted node.. !!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and recover tainted worker node.. !!!\n")
			cscanD.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")

			g.By("Set taint to worker node without value.. !!!\n")
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule")

			cscan.namespace = subD.namespace
			g.By("Create compliancescan.. !!!\n")
			cscan.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscan.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceScan name and result.. !!!\n")
			subD.complianceScanName(oc, "worker-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify if the scan pod generated for tainted node...!!!\n")
			assertCoPodNumerEqualNodeNumber(oc, subD.namespace, "node-role.kubernetes.io/wscan=")

			g.By("Remove compliancescan object and taint label and key from worker node.. !!!\n")
			cscan.delete(itName, dr)
			taintNode(oc, "taint", "node", nodeName, "key1=:NoSchedule-")
			labelTaintNode(oc, "node", nodeName, "taint-")

			g.By("ocp-33610 The compliance scan performed on tained node successfully.. !!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-28949-The complianceSuite and ComplianeScan perform scan using Platform scan type", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
				cscanMD = complianceScanDescription{
					name:         "platform-new-scan",
					namespace:    "",
					scanType:     "platform",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     cscanTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "platform-new-scan"})

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create platform-new-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")

			g.By("Check platform-new-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-new-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Check platform-new-scan name and result..!!!\n")
			subD.complianceScanName(oc, "platform-new-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check platform-new-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-28949 complianceScan for platform has performed successfully ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-36988-The ComplianceScan could be triggered for cis profile for platform scanType", func() {

			var (
				cscanMD = complianceScanDescription{
					name:         "platform-new-scan",
					namespace:    "",
					scanType:     "platform",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     cscanTemplate,
				}
			)

			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "platform-new-scan"})

			cscanMD.namespace = subD.namespace
			g.By("Create platform-new-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-new-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-new-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-new-scan name and result..!!!\n")
			subD.complianceScanName(oc, "platform-new-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check platform-new-scan result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-36988 complianceScan for platform has performed successfully ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-36990-The ComplianceSuite could be triggered for cis profiles for platform scanType", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
			)

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "platform-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By(" ocp-36990 The complianceSuite object successfully performed platform scan for cis profile ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Critical-37063-The ComplianceSuite could be triggered for cis profiles for node scanType", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}

				csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}

				csuiteRD = complianceSuiteDescription{
					name:         "rhcos-compliancesuite",
					namespace:    "",
					scanname:     "rhcos-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuitenodeTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "rhcos-compliancesuite"},
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-kubelet-configure-event-creation", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=FAIL", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"worker-scan-kubelet-configure-event-creation", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Remove worker-compliancesuite object.. !!!\n")
			csuiteD.delete(itName, dr)

			csuiteMD.namespace = subD.namespace
			g.By("Create master-compliancesuite !!!\n")
			csuiteMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "master-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteMD.name, "NON-COMPLIANT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"master-scan-etcd-unique-ca", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Remove master-compliancesuite object.. !!!\n")
			csuiteMD.delete(itName, dr)

			csuiteRD.namespace = subD.namespace
			g.By("Create rhcos-compliancesuite !!!\n")
			csuiteRD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteRD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check master-compliancesuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "rhcos-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteRD.name, "INCONSISTENT")

			g.By("Check master-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-etcd-unique-ca", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=INCONSISTENT", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "INCONSISTENT", ok, []string{"compliancecheckresult",
				"rhcos-scan-etcd-unique-ca", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By(" ocp-37063 The complianceSuite object successfully triggered scan for cis node profile.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-Longduration-High-32120-The ComplianceSuite performs schedule scan for Platform scan type [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "platform-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanType:     "platform",
					scanname:     "platform-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "platform-compliancesuite"},
			)

			csuiteD.namespace = subD.namespace
			g.By("Create platform-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-scan pod.. !!!\n")
			subD.scanPodName(oc, "platform-scan-api-checks-pod")

			g.By("Check platform scan pods status.. !!! \n")
			subD.scanPodStatus(oc, "Succeeded")
			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")
			g.By("Check platform-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			newCheck("expect", asAdmin, withoutNamespace, contain, "platform-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "platform-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			checkComplianceSuiteStatus(oc, csuiteD.name, subD.namespace, "RUNNING")
			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l=workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check platform-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("The ocp-32120 The complianceScan object performed Platform schedule scan successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33418-Medium-44062-The ComplianceSuite performs the schedule scan through cron job and also verify the suitererunner resources are doubled [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "worker-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			checkComplianceSuiteStatus(oc, csuiteD.name, subD.namespace, "RUNNING")
			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify the suitererunner resource requests and limits doubled through jobs.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "{\"cpu\":\"50m\",\"memory\":\"100Mi\"}", ok, []string{"jobs", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].spec.template.spec.containers[0].resources.limits}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "{\"cpu\":\"10m\",\"memory\":\"20Mi\"}", ok, []string{"jobs", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].spec.template.spec.containers[0].resources.requests}"}).check(oc)

			g.By("Verify the suitererunner resource requests and limits doubled through pods.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "{\"cpu\":\"50m\",\"memory\":\"100Mi\"}", ok, []string{"pods", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].spec.containers[0].resources.limits}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "{\"cpu\":\"10m\",\"memory\":\"20Mi\"}", ok, []string{"pods", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].spec.containers[0].resources.requests}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("ocp-33418 ocp-44062 The ComplianceSuite object performed schedule scan and verify the suitererunner resources requests & limit successfully.. !!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-NonPreRelease-Longduration-Medium-33456-The Compliance-Operator edits the scheduled cron job to scan from ComplianceSuite [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "example-compliancesuite1",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name},
			)

			// adding label to rhcos worker node to skip non-rhcos worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			newCheck("expect", asAdmin, withoutNamespace, contain, csuiteD.name+"-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", csuiteD.name + "-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			g.By("edit schedule by patch.. !!!\n")
			patch := fmt.Sprintf("{\"spec\":{\"schedule\":\"*/4 * * * *\"}}")
			patchResource(oc, asAdmin, withoutNamespace, "compliancesuites", csuiteD.name, "-n", csuiteD.namespace, "--type", "merge", "-p", patch)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/4 * * * *", ok, []string{"cronjob", csuiteD.name + "-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			checkComplianceSuiteStatus(oc, csuiteD.name, subD.namespace, "RUNNING")
			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("The ocp-33456-The Compliance-Operator could edit scheduled cron job to scan from ComplianceSuite successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-Longduration-High-33453-The Compliance Operator rotates the raw scan results [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					schedule:     "*/3 * * * *",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					rotation:     2,
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name},
				objectTableRef{"pod", subD.namespace, "pv-extract"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			//Verifying rotation policy and cronjob
			newCheck("expect", asAdmin, withoutNamespace, contain, "2", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.spec.scans[0].rawResultStorage.rotation}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-compliancesuite-rerunner", ok, []string{"cronjob", "-n",
				subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "*/3 * * * *", ok, []string{"cronjob", "worker-compliancesuite-rerunner",
				"-n", subD.namespace, "-o=jsonpath={.spec.schedule}"}).check(oc)

			//Second round of scan and check
			checkComplianceSuiteStatus(oc, csuiteD.name, subD.namespace, "RUNNING")
			newCheck("expect", asAdmin, withoutNamespace, contain, "1", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			//Third round of scan and check
			checkComplianceSuiteStatus(oc, csuiteD.name, subD.namespace, "RUNNING")
			newCheck("expect", asAdmin, withoutNamespace, contain, "2", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.scanStatuses[*].currentIndex}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Succeeded", ok, []string{"pod", "-l workload=suitererunner", "-n",
				subD.namespace, "-o=jsonpath={.items[1].status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			g.By("Check worker-compliancesuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Create pv-extract pod and verify arfReport result directories.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", pvextractpodYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pod", "pv-extract", "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			commands := []string{"exec", "pod/pv-extract", "--", "ls", "/workers-scan-results"}
			arfReportDir, err := oc.AsAdmin().Run(commands...).Args().Output()
			e2e.Logf("The arfReport result dir:\n%v", arfReportDir)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(arfReportDir, "0") && (strings.Contains(arfReportDir, "1") && strings.Contains(arfReportDir, "2")) {
				g.By("The ocp-33453 The ComplianceSuite object performed schedule scan and rotates the raw scan results successfully.. !!!\n")
			}
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-33660-Verify the differences in nodes from the same role could be handled [Serial]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_direct_root_logins",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			var pod = podModifyD
			pod.namespace = oc.Namespace()
			nodeName := getOneRhcosWorkerNodeName(oc)
			pod.name = "pod-modify"
			pod.nodeName = nodeName
			pod.args = "touch /hostroot/etc/securetty"
			defer func() {
				pod.name = "pod-recover"
				pod.nodeName = nodeName
				pod.args = "rm -rf /hostroot/etc/securetty"
				pod.doActionsOnNode(oc, "Succeeded", dr)
			}()
			pod.doActionsOnNode(oc, "Succeeded", dr)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "INCONSISTENT")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-direct-root-logins", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/inconsistent-check", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "INCONSISTENT", ok, []string{"compliancecheckresult",
				"worker-scan-no-direct-root-logins", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-33660 The compliance scan successfully handled the differences from the same role nodes ...!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-32814-High-45729-The compliance operator by default creates ProfileBundles and profiles", func() {
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
			subD.getProfileName(oc, "ocp4-pci-dss")
			subD.getProfileName(oc, "ocp4-pci-dss-node")
			subD.getProfileName(oc, "rhcos4-e8")
			subD.getProfileName(oc, "rhcos4-moderate")
			subD.getProfileName(oc, "rhcos4-nerc-cip")

			g.By("The Compliance Operator by default created ProfileBundles and profiles are verified successfully.. !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33431-Verify compliance check result shows in ComplianceCheckResult label for compliancesuite", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"})
			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			csuiteD.namespace = subD.namespace
			g.By("Create compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "0")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-severity=medium", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=PASS", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/scan-name=worker-scan", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "worker-scan-no-netrc-files", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/suite=worker-compliancesuite", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("ocp-33431 The compliance scan result verified through ComplianceCheckResult label successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33435-Verify the compliance scan result shows in ComplianceCheckResult label for compliancescan", func() {

			var (
				cscanD = complianceScanDescription{
					name:         "rhcos-scan",
					namespace:    "",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wscan",
					template:     cscanTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc, objectTableRef{"compliancescan", subD.namespace, "rhcos-scan"})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			cscanD.namespace = subD.namespace
			g.By("Create compliancescan !!!\n")
			cscanD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceScanName(oc, "rhcos-scan")
			subD.complianceScanResult(oc, "NON-COMPLIANT")

			g.By("Check complianceScan result exit-code through configmap...!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify compliance scan result compliancecheckresult through label ...!!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-severity=high", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/check-status=FAIL", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "rhcos-scan-no-empty-passwords", ok, []string{"compliancecheckresult",
				"--selector=compliance.openshift.io/scan-name=rhcos-scan", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("ocp-33435 The compliance scan result verified through ComplianceCheckResult label successfully....!!!\n")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-33449-The compliance-operator raw results store in ARF format on a PVC", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"pod", subD.namespace, "pv-extract"})

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite !!!\n")
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")

			g.By("Check worker-compliancesuite result through exit-code ..!!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Create pv-extract pod and check status.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", pvextractpodYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pod", "pv-extract", "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check ARF report generates in xml format.. !!!\n")
			subD.getARFreportFromPVC(oc, ".xml.bzip2")

			g.By("The ocp-33449 complianceScan raw result successfully stored in ARF format on the PVC... !!!!\n")

		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-37171-Check compliancesuite status when there are multiple rhcos4 profiles added in scansettingbinding object [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "rhcos4",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "rhcos4-e8",
					profilename2:    "rhcos4-moderate",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name rhcos4-e8 .. !!!\n")
			subD.getProfileName(oc, ssb.profilename1)
			g.By("Check default profiles name rhcos4-moderate .. !!!\n")
			subD.getProfileName(oc, ssb.profilename2)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", ssb.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, ssb.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37171 Check compliancesuite status when there are multiple rhcos4 profiles added in scansettingbinding object successfully... !!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-37084-The ComplianceSuite generates through ScanSettingBinding CR with tailored cis profile", func() {
			var (
				tp = tailoredProfileWithoutVarDescription{
					name:         "ocp4-cis-custom",
					namespace:    "",
					extends:      "ocp4-cis",
					title:        "My little profile",
					description:  "cis profile rules",
					enrulename1:  "ocp4-scc-limit-root-containers",
					rationale1:   "None",
					enrulename2:  "ocp4-scheduler-no-bind-address",
					rationale2:   "Platform",
					disrulename1: "ocp4-api-server-encryption-provider-cipher",
					drationale1:  "Platform",
					disrulename2: "ocp4-scc-drop-container-capabilities",
					drationale2:  "None",
					template:     tprofileWithoutVarTemplate,
				}
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "myss",
					namespace:             "",
					roles1:                "master",
					roles2:                "worker",
					rotation:              5,
					schedule:              "0 1 * * *",
					size:                  "2Gi",
					template:              scansettingTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "my-companys-compliance-requirements",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "ocp4-cis-custom",
					profilename2:    "ocp4-cis-node",
					scansettingname: "myss",
					template:        scansettingbindingTemplate,
				}
			)

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"scansetting", subD.namespace, ss.name},
				objectTableRef{"tailoredprofile", subD.namespace, tp.name})

			g.By("Check default profiles name ocp4-cis and ocp4-cis-node.. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-cis-node")

			tp.namespace = subD.namespace
			g.By("Create tailoredprofile !!!\n")
			tp.create(oc, itName, dr)
			subD.getTailoredProfileNameandStatus(oc, tp.name)

			g.By("Create scansetting !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "myss", ok, []string{"scansetting", ss.name, "-n", ss.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "my-companys-compliance-requirements", ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-37084 The ComplianceSuite generates through ScanSettingBinding CR with tailored cis profile successfully... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-34928-Storage class and access modes are configurable through ComplianceSuite and ComplianceScan", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:             "worker-compliancesuite",
					namespace:        "",
					scanname:         "worker-scan",
					profile:          "xccdf_org.ssgproject.content_profile_moderate",
					content:          "ssg-rhcos4-ds.xml",
					contentImage:     "quay.io/complianceascode/ocp4:latest",
					rule:             "xccdf_org.ssgproject.content_rule_no_netrc_files",
					nodeSelector:     "worker",
					storageClassName: "gold",
					pvAccessModes:    "ReadWriteOnce",
					template:         csuiteSCTemplate,
				}
				cscanMD = complianceScanDescription{
					name:             "master-scan",
					namespace:        "",
					profile:          "xccdf_org.ssgproject.content_profile_e8",
					content:          "ssg-rhcos4-ds.xml",
					contentImage:     "quay.io/complianceascode/ocp4:latest",
					rule:             "xccdf_org.ssgproject.content_rule_accounts_no_uid_except_zero",
					nodeSelector:     "master",
					storageClassName: "gold",
					pvAccessModes:    "ReadWriteOnce",
					template:         cscanSCTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "worker-compliancesuite"},
				objectTableRef{"compliancescan", subD.namespace, "master-scan"},
				objectTableRef{"storageclass", subD.namespace, "gold"})

			g.By("Get the default storageClass provisioner & volumeBindingMode from cluster .. !!!\n")
			storageClass.name = "gold"
			storageClass.provisioner = getStorageClassProvisioner(oc)
			storageClass.reclaimPolicy = "Delete"
			storageClass.volumeBindingMode = getStorageClassVolumeBindingMode(oc)
			storageClass.create(oc, itName, dr)

			csuiteD.namespace = subD.namespace
			g.By("Create worker-compliancesuite.. !!!\n")
			e2e.Logf("Here namespace : %v\n", catSrc.namespace)
			csuiteD.create(oc, itName, dr)

			cscanMD.namespace = subD.namespace
			g.By("Create master-scan.. !!!\n")
			cscanMD.create(oc, itName, dr)

			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancescan", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check worker-compliancesuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT INCONSISTENT")

			g.By("Check pvc name and storage size for worker-scan.. !!!\n")
			subD.getPVCName(oc, "worker-scan")
			newCheck("expect", asAdmin, withoutNamespace, contain, "gold", ok, []string{"pvc", csuiteD.scanname, "-n",
				subD.namespace, "-o=jsonpath={.spec.storageClassName}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ReadWriteOnce", ok, []string{"pvc", csuiteD.scanname, "-n",
				subD.namespace, "-o=jsonpath={.status.accessModes[]}"}).check(oc)

			g.By("Check master-scan name and result..!!!\n")
			subD.complianceScanName(oc, "master-scan")
			subD.complianceScanResult(oc, "COMPLIANT")

			g.By("Check pvc name and storage size for master-scan ..!!!\n")
			subD.getPVCName(oc, "master-scan")
			newCheck("expect", asAdmin, withoutNamespace, contain, "gold", ok, []string{"pvc", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.spec.storageClassName}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ReadWriteOnce", ok, []string{"pvc", cscanMD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.accessModes[]}"}).check(oc)

			g.By("ocp-34928 Storage class and access modes are successfully configurable through ComplianceSuite and ComplianceScan ..!!!\n")
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-40372-Use a separate SA for resultserver", func() {
			var csuiteMD = complianceSuiteDescription{
				name:         "master-compliancesuite",
				namespace:    "",
				scanname:     "master-scan",
				profile:      "xccdf_org.ssgproject.content_profile_moderate",
				content:      "ssg-rhcos4-ds.xml",
				contentImage: "quay.io/complianceascode/ocp4:latest",
				rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
				nodeSelector: "master",
				template:     csuiteTemplate,
			}

			// These are special steps to overcome problem which are discussed in [1] so that namespace should not stuck in 'Terminating' state
			// [1] https://bugzilla.redhat.com/show_bug.cgi?id=1858186
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, "master-compliancesuite"})

			g.By("check role resultserver")
			rsRoleName := getResourceNameWithKeyword(oc, "role", "resultserver")
			e2e.Logf("rs role name: %v\n", rsRoleName)
			newCheck("expect", asAdmin, withoutNamespace, contain, "[{\"apiGroups\":[\"security.openshift.io\"],\"resourceNames\":[\"restricted\"],\"resources\":[\"securitycontextconstraints\"],\"verbs\":[\"use\"]}]",
				ok, []string{"role", rsRoleName, "-n", subD.namespace, "-o=jsonpath={.rules}"}).check(oc)

			g.By("create compliancesuite")
			csuiteMD.namespace = subD.namespace
			csuiteMD.create(oc, itName, dr)

			g.By("check the scc and securityContext for the rs pod")
			rsPodName := getResourceNameWithKeywordFromResourceList(oc, "pod", "rs")
			//could not use newCheck as rs pod will be deleted soon
			checkKeyWordsForRspod(oc, rsPodName, [...]string{"restricted", "fsGroup", "resultserver"})
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-40280-The infrastructure feature should show the Compliance operator when the disconnected filter gets applied", func() {
			g.By("check the infrastructure-features for csv!!!\n")
			csvName := getResource(oc, asAdmin, withoutNamespace, "csv", "-n", oc.Namespace(), "-o=jsonpath={.items[0].metadata.name}")
			newCheck("expect", asAdmin, withoutNamespace, contain, "[\"disconnected\", \"fips\", \"proxy-aware\"]", ok, []string{"csv",
				csvName, "-n", oc.Namespace(), "-o=jsonpath={.metadata.annotations.operators\\.openshift\\.io/infrastructure-features}"}).check(oc)
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-41769-The compliance operator could get HTTP_PROXY and HTTPS_PROXY environment from OpenShift has global proxy settings	", func() {
			g.By("Get the httpPoxy and httpsProxy info!!!\n")
			httpProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-n", oc.Namespace(), "-o=jsonpath={.spec.httpProxy}")
			httpsProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-n", oc.Namespace(), "-o=jsonpath={.spec.httpsProxy}")

			if len(httpProxy) == 0 && len(httpsProxy) == 0 {
				g.Skip("Skip for non-proxy cluster! This case intentionally runs nothing!")
			} else {
				g.By("check the proxy info for the compliance-operator deployment!!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "\"name\":\"HTTP_PROXY\",\"value\":\""+httpProxy+"\"", ok, []string{"deployment", "compliance-operator",
					"-n", oc.Namespace(), "-o=jsonpath={.spec.template.spec.containers[0].env}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "\"name\":\"HTTPS_PROXY\",\"value\":\""+httpsProxy+"\"", ok, []string{"deployment", "compliance-operator",
					"-n", oc.Namespace(), "-o=jsonpath={.spec.template.spec.containers[0].env}"}).check(oc)

				g.By("create a compliancesuite!!!\n")
				var csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_cis-node",
					content:      "ssg-ocp4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}
				defer cleanupObjects(oc,
					objectTableRef{"compliancesuite", subD.namespace, "master-compliancesuite"})
				csuiteMD.namespace = subD.namespace
				g.By("Create master-compliancesuite !!!\n")
				csuiteMD.create(oc, itName, dr)
				newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n",
					subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

				g.By("check the https proxy in the configmap!!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, httpsProxy, ok, []string{"cm", csuiteMD.scanname + "-openscap-env-map", "-n",
					subD.namespace, "-o=jsonpath={.data.HTTPS_PROXY}"}).check(oc)
			}
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-33859-Verify if the profileparser enables to get content updates when the image digest updated [Slow]", func() {
			var (
				pb = profileBundleDescription{
					name:         "test1",
					namespace:    "",
					contentimage: "quay.io/openshifttest/ocp4-openscap-content@sha256:392b0a67e4386a7450b0bb0c9353231563b7ab76056d215f10e6f5ffe0a2cbad",
					contentfile:  "ssg-rhcos4-ds.xml",
					template:     profilebundleTemplate,
				}
				tprofileD = tailoredProfileDescription{
					name:         "example-tailoredprofile",
					namespace:    "",
					extends:      "test1-moderate",
					enrulename1:  "test1-account-disable-post-pw-expiration",
					disrulename1: "test1-account-unique-name",
					disrulename2: "test1-account-use-centralized-automated-auth",
					varname:      "test1-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				tprofileD2 = tailoredProfileDescription{
					name:         "example-tailoredprofile2",
					namespace:    "",
					extends:      "test1-moderate",
					enrulename1:  "test1-wireless-disable-in-bios",
					disrulename1: "test1-account-unique-name",
					disrulename2: "test1-account-use-centralized-automated-auth",
					varname:      "test1-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"profilebundle", subD.namespace, pb.name},
				objectTableRef{"tailoredprofile", subD.namespace, tprofileD.name},
				objectTableRef{"tailoredprofile", subD.namespace, tprofileD2.name})

			g.By("create profilebundle!!!\n")
			pb.namespace = subD.namespace
			pb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, pb.name, ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "test1-chronyd-no-chronyc-network", ok, []string{"rules", "test1-chronyd-no-chronyc-network",
				"-n", oc.Namespace()}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "test1-chronyd-client-only", ok, []string{"rules", "test1-chronyd-client-only",
				"-n", oc.Namespace()}).check(oc)

			g.By("check the default profilebundle !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "ocp4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "rhcos4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("Create tailoredprofile !!!\n")
			tprofileD.namespace = subD.namespace
			tprofileD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "READY", ok, []string{"tailoredprofile", tprofileD.name, "-n", tprofileD.namespace,
				"-o=jsonpath={.status.state}"}).check(oc)

			g.By("Patch the profilebundle with a different image !!!\n")
			patch := fmt.Sprintf("{\"spec\":{\"contentImage\":\"quay.io/openshifttest/ocp4-openscap-content@sha256:3778c668f462424552c15c6c175704b64270ea06183fd034aa264736f1ec45a9\"}}")
			patchResource(oc, asAdmin, withoutNamespace, "profilebundle", pb.name, "-n", pb.namespace, "--type", "merge", "-p", patch)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Pending", ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"rules", "test1-chronyd-no-chronyc-network", "-n", oc.Namespace()}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "test1-chronyd-client-only", ok, []string{"rules", "test1-chronyd-client-only",
				"-n", oc.Namespace()}).check(oc)

			g.By("Create tailoredprofile !!!\n")
			tprofileD2.namespace = subD.namespace
			tprofileD2.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "READY", ok, []string{"tailoredprofile", tprofileD2.name, "-n", tprofileD.namespace,
				"-o=jsonpath={.status.state}"}).check(oc)
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-33578-Verify if the profileparser enables to get content updates when add a new ProfileBundle [Slow]", func() {
			var (
				pb = profileBundleDescription{
					name:         "test1",
					namespace:    "",
					contentimage: "quay.io/openshifttest/ocp4-openscap-content@sha256:392b0a67e4386a7450b0bb0c9353231563b7ab76056d215f10e6f5ffe0a2cbad",
					contentfile:  "ssg-rhcos4-ds.xml",
					template:     profilebundleTemplate,
				}
				pb2 = profileBundleDescription{
					name:         "test2",
					namespace:    "",
					contentimage: "quay.io/openshifttest/ocp4-openscap-content@sha256:3778c668f462424552c15c6c175704b64270ea06183fd034aa264736f1ec45a9",
					contentfile:  "ssg-rhcos4-ds.xml",
					template:     profilebundleTemplate,
				}
				tprofileD = tailoredProfileDescription{
					name:         "example-tailoredprofile",
					namespace:    "",
					extends:      "test1-moderate",
					enrulename1:  "test1-account-disable-post-pw-expiration",
					disrulename1: "test1-account-unique-name",
					disrulename2: "test1-account-use-centralized-automated-auth",
					varname:      "test1-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
				tprofileD2 = tailoredProfileDescription{
					name:         "example-tailoredprofile2",
					namespace:    "",
					extends:      "test2-moderate",
					enrulename1:  "test2-wireless-disable-in-bios",
					disrulename1: "test2-account-unique-name",
					disrulename2: "test2-account-use-centralized-automated-auth",
					varname:      "test2-var-selinux-state",
					value:        "permissive",
					template:     tprofileTemplate,
				}
			)

			defer cleanupObjects(oc,
				objectTableRef{"profilebundle", subD.namespace, pb.name},
				objectTableRef{"profilebundle", subD.namespace, pb2.name},
				objectTableRef{"tailoredprofile", subD.namespace, tprofileD.name},
				objectTableRef{"tailoredprofile", subD.namespace, tprofileD2.name})

			g.By("create profilebundle!!!\n")
			pb.namespace = subD.namespace
			pb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, pb.name, ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", pb.name, "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("check the default profilebundle !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "ocp4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "rhcos4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("Create tailoredprofile !!!\n")
			tprofileD.namespace = subD.namespace
			tprofileD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "READY", ok, []string{"tailoredprofile", tprofileD.name, "-n", tprofileD.namespace,
				"-o=jsonpath={.status.state}"}).check(oc)

			g.By("Create another profilebundle with a different image !!!\n")
			pb2.namespace = subD.namespace
			pb2.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, pb2.name, ok, []string{"profilebundle", pb2.name, "-n", pb2.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", pb2.name, "-n", pb2.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("check the default profilebundle !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "ocp4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Valid", ok, []string{"profilebundle", "rhcos4", "-n", pb.namespace,
				"-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

			g.By("Create tailoredprofile !!!\n")
			tprofileD2.namespace = subD.namespace
			tprofileD2.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "READY", ok, []string{"tailoredprofile", tprofileD2.name, "-n", tprofileD.namespace,
				"-o=jsonpath={.status.state}"}).check(oc)
		})

		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-High-33429-The Compliance Operator performs scan successfully on taint node without tolerations [Exclusive] [Slow]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "example-compliancesuite",
					namespace:    "",
					scanname:     "rhcos-scan",
					schedule:     "* 1 * * *",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wscan",
					size:         "2Gi",
					template:     csuiteTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Get one worker node.. !!!\n")
			nodeName := getOneRhcosWorkerNodeName(oc)

			g.By("cleanup !!!\n")
			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name})
			defer func() {
				taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule-")
			}()

			g.By("Taint node and create compliancesuite.. !!!\n")
			taintNode(oc, "taint", "node", nodeName, "key1=value1:NoSchedule")
			csuiteD.namespace = subD.namespace
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "NON-COMPLIANT", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.result}"}).check(oc)

			g.By("Check complianceScan result exit-code through configmap.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2 unschedulable")
		})

		/* Disabling the test, it may be needed in future
		// author: xiyuan@redhat.com
		g.It("Author:xiyuan-Medium-40226-NOT APPLICABLE rule should report NOT APPLICABLE status in 'ComplianceCheckResult' instead of SKIP [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "NOT-APPLICABLE", ok, []string{"compliancecheckresult", "ocp4-cis-node-worker-etcd-unique-ca", "-n", ssb.namespace,
				"-o=jsonpath={.status}"}).check(oc)

			g.By("Check the number of compliancecheckresult in NOT-APPLICABLE !!!\n")
			checkResourceNumber(oc, 37, "compliancecheckresult", "-l", "compliance.openshift.io/check-status=NOT-APPLICABLE", "--no-headers", "-n", subD.namespace)

			g.By("Check the warnings of NOT-APPLICABLE rules !!!\n")
			checkWarnings(oc, "rule is only applicable", "compliancecheckresult", "-l", "compliance.openshift.io/check-status=NOT-APPLICABLE", "--no-headers", "-n", subD.namespace)
		})*/

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41861-Verify fips mode checking rules are working as expected [Slow]", func() {

			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_enable_fips_mode",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}

				csuiteMD = complianceSuiteDescription{
					name:         "master-compliancesuite",
					namespace:    "",
					scanname:     "master-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_enable_fips_mode",
					nodeSelector: "master",
					template:     csuiteTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"compliancesuite", subD.namespace, csuiteMD.name},
				objectTableRef{"compliancesuite", subD.namespace, csuiteD.name})

			// adding label to rhcos worker node to skip rhel worker node if any
			g.By("Label all rhcos worker nodes as wscan.. !!!\n")
			setLabelToNode(oc)

			g.By("Create compliancesuite objects !!!\n")
			csuiteD.namespace = subD.namespace
			csuiteD.create(oc, itName, dr)
			csuiteMD.namespace = subD.namespace
			csuiteMD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, csuiteD.name, ok, []string{"compliancesuite", "-n", csuiteD.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, csuiteMD.name, ok, []string{"compliancesuite", "-n", csuiteMD.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n", csuiteD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteMD.name, "-n", csuiteMD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			fipsOut := checkFipsStatus(oc)
			if strings.Contains(fipsOut, "FIPS mode is enabled.") {
				g.By("Check complianceSuite result.. !!!\n")
				subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
				subD.complianceSuiteResult(oc, csuiteMD.name, "COMPLIANT")
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult", "master-scan-enable-fips-mode", "-n", oc.Namespace(), "-o=jsonpath={.status}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult", "worker-scan-enable-fips-mode", "-n", oc.Namespace(), "-o=jsonpath={.status}"}).check(oc)

			} else {
				g.By("Check complianceSuite result.. !!!\n")
				subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")
				subD.complianceSuiteResult(oc, csuiteMD.name, "NON-COMPLIANT")
				newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult", "master-scan-enable-fips-mode", "-n", oc.Namespace(), "-o=jsonpath={.status}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult", "worker-scan-enable-fips-mode", "-n", oc.Namespace(), "-o=jsonpath={.status}"}).check(oc)
			}

			g.By("ocp-41861 Successfully verify fips mode checking rules are working as expected ..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41093-Medium-44944-The instructions should be available for all rules in cis profiles and The nodeName shows in target and fact:identifier elements of complianceScan XCCDF format result [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-instruction",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check the total number of CIS Manual rules.. !!!\n")
			checkResourceNumber(oc, 24, "compliancecheckresult", "-l", "compliance.openshift.io/check-status=MANUAL", "--no-headers", "-n", subD.namespace)
			checkCisRulesInstruction(oc)

			g.By("Verify the nodeName shows in target & fact:identifier elements of complianceScan XCCDF format result.. !!!\n")
			extractResultFromConfigMap(oc, "worker", ssb.namespace)
			extractResultFromConfigMap(oc, "master", ssb.namespace)
			g.By("All CIS rules has instructions & nodeName is available in target & identifier elements of complianceScan XCCDF format result..!!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-47044-Verify the ocp4 moderate profiles perform scan as expected with default scanSettings [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					profilename2:    "ocp4-moderate-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")
			g.By("Check default profiles name ocp4-moderate-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, subD.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-47044 The ocp4 moderate profiles perform scan as expected with default scanSettings... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Low-42695-Verify the manual remediation for rule ocp4-moderate-compliancesuite-exists works as expected [Slow]", func() {

			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify 'ocp4-moderate-compliancesuite-exists' rule status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-compliancesuite-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("The ocp-42695 The ComplianceSuite object exist and operator is successfully installed... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-CPaasrunOnly-NonPreRelease-Low-42719-Low-42810-Low-42834-Check manual remediation works for TokenMaxAge TokenInactivityTimeout and no-ldap-insecure rules for oauth cluster object [Disruptive][Slow]", func() {

			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove TokenMaxAge, TokenInactivityTimeout parameters and ldap configuration by patching.. !!!\n")
				patch1 := fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/spec/tokenConfig/accessTokenMaxAgeSeconds\"}]")
				patch2 := fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/spec/tokenConfig/accessTokenInactivityTimeout\"}]")
				patch3 := fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/spec/identityProviders/1\",\"value\":{\"ldap\":{\"attributes\":{\"id\":[\"dn\"],\"name\":[\"cn\"],\"preferredUsername\":[\"uid\"]},\"bindDN\":\"\",\"bindPassword\":{\"name\":\"\"},\"ca\":{\"name\":\"ad-ldap\"},\"insecure\":false,\"url\":\"ldaps://10.66.147.179/cn=users,dc=ad-example,dc=com?uid\"},\"mappingMethod\":\"claim\",\"name\":\"AD_ldaps_provider\",\"type\":\"LDAP\"}}]")
				patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "json", "-p", patch1)
				patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "json", "-p", patch2)
				patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type=json", "-p", patch3)
				newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"oauth", "cluster", "-o=jsonpath={.spec.tokenConfig.accessTokenMaxAgeSeconds}"}).check(oc)
				newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"oauth", "cluster", "-o=jsonpath={.spec.tokenConfig.accessTokenInactivityTimeout}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "ldap", nok, []string{"oauth", "cluster", "-o=jsonpath={.spec.identityProviders}"}).check(oc)
				checkOauthPodsStatus(oc)
				cleanupObjects(oc, objectTableRef{"configmap", "openshift-config", "ca.crt"})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			}()

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify TokenMaxAge, TokenInactivityTimeout and no-ldap-insecure rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-token-maxage", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-inactivity-timeout", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-ocp-no-ldap-insecure", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Set TokenMaxAge, TokenInactivityTimeout parameters and ldap configuration by patching.. !!!\n")
			patch1 := fmt.Sprintf("[{\"op\":\"add\",\"path\":\"/spec/identityProviders/-\",\"value\":{\"ldap\":{\"attributes\":{\"email\":[\"mail\"],\"id\":[\"dn\"],\"name\":[\"uid\"],\"preferredUsername\":[\"uid\"]},\"insecure\":true,\"url\":\"ldap://10.66.147.104:389/ou=People,dc=my-domain,dc=com?uid\"},\"mappingMethod\":\"add\",\"name\":\"openldapidp\",\"type\":\"LDAP\"}}]")
			patch2 := fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/spec/identityProviders/1\",\"value\":{\"ldap\":{\"attributes\":{\"email\":[\"mail\"],\"id\":[\"dn\"],\"name\":[\"uid\"],\"preferredUsername\":[\"uid\"]},\"insecure\":true,\"url\":\"ldap://10.66.147.104:389/ou=People,dc=my-domain,dc=com?uid\"},\"mappingMethod\":\"add\",\"name\":\"openldapidp\",\"type\":\"LDAP\"}}]")
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "merge", "-p",
				"{\"spec\":{\"tokenConfig\":{\"accessTokenMaxAgeSeconds\":28800}}}")
			newCheck("expect", asAdmin, withoutNamespace, contain, "28800", ok, []string{"oauth", "cluster",
				"-o=jsonpath={.spec.tokenConfig.accessTokenMaxAgeSeconds}"}).check(oc)
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "merge", "-p",
				"{\"spec\":{\"tokenConfig\":{\"accessTokenInactivityTimeout\":\"10m0s\"}}}")
			newCheck("expect", asAdmin, withoutNamespace, contain, "10m0s", ok, []string{"oauth", "cluster",
				"-o=jsonpath={.spec.tokenConfig.accessTokenInactivityTimeout}"}).check(oc)
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type=json", "-p", patch1)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ldap", ok, []string{"oauth", "cluster", "-o=jsonpath={.spec.identityProviders}"}).check(oc)

			g.By("Check pod status from 'openshift-authentication' namespace during pod reboot.. !!!\n")
			checkOauthPodsStatus(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify TokenMaxAge, TokenInactivityTimeout and no-ldap-insecure rules status through compliancecheck result after rescan.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-token-maxage", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-inactivity-timeout", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-ocp-no-ldap-insecure", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type=json", "-p", patch2)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ldap", nok, []string{"oauth", "cluster", "-o=jsonpath={.spec.identityProviders}"}).check(oc)

			g.By("Apply secure ldap to oauth cluster object by patching.. !!!\n")
			patch3 := fmt.Sprintf("[{\"op\":\"add\",\"path\":\"/spec/identityProviders/-\",\"value\":{\"ldap\":{\"attributes\":{\"id\":[\"dn\"],\"name\":[\"cn\"],\"preferredUsername\":[\"uid\"]},\"bindDN\":\"\",\"bindPassword\":{\"name\":\"\"},\"ca\":{\"name\":\"ad-ldap\"},\"insecure\":false,\"url\":\"ldaps://10.66.147.179/cn=users,dc=ad-example,dc=com?uid\"},\"mappingMethod\":\"claim\",\"name\":\"AD_ldaps_provider\",\"type\":\"LDAP\"}}]")
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type=json", "-p", patch3)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ldap", ok, []string{"oauth", "cluster", "-o=jsonpath={.spec.identityProviders}"}).check(oc)
			g.By("Configured ldap to oauth cluster object by patching.. !!!\n")
			_, err2 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-config", "-f", ldapConfigMapYAML).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "ca.crt", ok, []string{"configmap", "-n", "openshift-config", "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err3 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Verify 'ocp4-moderate-ocp-no-ldap-insecure' rule status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-ocp-no-ldap-insecure", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-42719-42810-42834 The manual remediation successfully applied for TokenMaxAge, TokenInactivityTimeout and no-ldap-insecure rules... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Low-42960-Low-43098-Check that TokenMaxAge and TokenInactivityTimeout are configurable for oauthclient objects [Disruptive][Slow]", func() {

			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove TokenMaxAge parameter by patching oauthclient objects.. !!!\n")
				oauthclients, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("oauthclient", "-n", oc.Namespace(),
					"-o=jsonpath={.items[*].metadata.name}").Output()
				oauthclient := strings.Fields(oauthclients)
				for _, v := range oauthclient {
					patchResource(oc, asAdmin, withoutNamespace, "oauthclient", v, "--type=json", "-p",
						"[{\"op\": \"remove\",\"path\": \"/accessTokenMaxAgeSeconds\"}]")
					newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"oauthclient", v,
						"-o=jsonpath={.accessTokenMaxAgeSeconds}"}).check(oc)
					patchResource(oc, asAdmin, withoutNamespace, "oauthclient", v, "--type=json", "-p",
						"[{\"op\": \"remove\",\"path\": \"/accessTokenInactivityTimeoutSeconds\"}]")
					newCheck("present", asAdmin, withoutNamespace, notPresent, "", ok, []string{"oauthclient", v,
						"-o=jsonpath={.accessTokenInactivityTimeoutSeconds}"}).check(oc)
				}
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			}()

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify 'ocp4-moderate-oauth-or-oauthclient-token-maxage' rule status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-token-maxage", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Set TokenMaxAge parameter to console oauthclient object by patching.. !!!\n")
			patchResource(oc, asAdmin, withoutNamespace, "oauthclient", "console", "--type", "merge", "-p", "{\"accessTokenMaxAgeSeconds\":28800}")
			newCheck("expect", asAdmin, withoutNamespace, contain, "28800", ok, []string{"oauthclient", "console",
				"-o=jsonpath={.accessTokenMaxAgeSeconds}"}).check(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check ComplianceSuite status.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Verify 'ocp4-moderate-oauth-or-oauthclient-token-maxage' rule status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-token-maxage", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Set TokenMaxAge & TokenInactivityTimeout parameters to all remaining oauthclient objects by patching.. !!!\n")
			oauthclients, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("oauthclient", "-n", oc.Namespace(), "-o=jsonpath={.items[*].metadata.name}").Output()
			oauthclient := strings.Fields(oauthclients)
			for _, v := range oauthclient {
				patchResource(oc, asAdmin, withoutNamespace, "oauthclient", v, "--type", "merge", "-p", "{\"accessTokenMaxAgeSeconds\":28800}")
				newCheck("expect", asAdmin, withoutNamespace, contain, "28800", ok, []string{"oauthclient", v,
					"-o=jsonpath={.accessTokenMaxAgeSeconds}"}).check(oc)
				patchResource(oc, asAdmin, withoutNamespace, "oauthclient", v, "--type", "merge", "-p", "{\"accessTokenInactivityTimeoutSeconds\":600}")
				newCheck("expect", asAdmin, withoutNamespace, contain, "600", ok, []string{"oauthclient", v,
					"-o=jsonpath={.accessTokenInactivityTimeoutSeconds}"}).check(oc)
			}

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err1 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			g.By("Check ComplianceSuite status.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify TokenMaxAge & TokenInactivityTimeout rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-token-maxage", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-oauth-or-oauthclient-inactivity-timeout", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-42960-43098 The TokenMaxAge & TokenInactivityTimeout parameters are configured for oauthclient objects successfully... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Low-42685-Low-46927-check the manual remediation for rules file-integrity-exists and file-integrity-notification-enabled working as expected [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				og = operatorGroupDescription{
					name:      "openshift-file-integrity-qbcd",
					namespace: "",
					template:  ogCoTemplate,
				}
				sub = subscriptionDescription{
					subName:                "file-integrity-operator",
					namespace:              "",
					channel:                "release-0.1",
					ipApproval:             "Automatic",
					operatorPackage:        "file-integrity-operator",
					catalogSourceName:      "redhat-operators",
					catalogSourceNamespace: "openshift-marketplace",
					startingCSV:            "",
					currentCSV:             "",
					installedCSV:           "",
					template:               subCoTemplate,
					singleNamespace:        true,
				}
				fi1 = fileintegrity{
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

				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			fioObj, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegrities", "--all-namespaces",
				"-o=jsonpath={.items[0].status.phase}").Output()

			if strings.Contains(fioObj, "Active") {
				g.By("The fileintegrity operator is installed, let's verify the rule status through compliancecheck result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-file-integrity-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-file-integrity-notification-enabled", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
				g.By("The file-integrity-exists and file-integrity-notification-enabled rules are verified successfully... !!!!\n ")
			} else {
				g.By("\n\n Let's installed fileintegrity operator... !!!\n")

				oc.SetupProject()
				og.namespace = oc.Namespace()
				sub.namespace = og.namespace
				g.By("Create operatorGroup.. !!!\n")
				og.create(oc, itName, dr)
				og.checkOperatorgroup(oc, og.name)
				g.By("Create subscription.. !!!\n")
				sub.create(oc, itName, dr)
				g.By("check csv.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.installedCSV,
					"-n", sub.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
				sub.checkPodFioStatus(oc, "running")

				g.By("Create File Integrity object.. !!!\n")
				fi1.namespace = sub.namespace
				fi1.createFIOWithoutConfig(oc, itName, dr)

				g.By("Rerun scan and check ComplianceSuite status & result.. !!!\n")
				_, err := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssb.name, "-n", subD.namespace).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
					"-o=jsonpath={.status.phase}"}).check(oc)
				subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

				g.By("Verify 'file-integrity-exists' & 'file-integrity-notification-enabled' rules status again through compliancecheck result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-file-integrity-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-file-integrity-notification-enabled", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

				g.By("The file-integrity-exists and file-integrity-notification-enabled rules are verified successfully... !!!!\n ")
			}
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-Medium-40660-Low-42874-Check whether the audit logs are getting forwarded using TLS protocol [Disruptive][Slow]", func() {
			var (
				ogL = operatorGroupDescription{
					name:      "openshift-logging",
					namespace: "openshift-logging",
					template:  ogCoTemplate,
				}
				subL = subscriptionDescription{
					subName:                "cluster-logging",
					namespace:              "openshift-logging",
					channel:                "stable",
					ipApproval:             "Automatic",
					operatorPackage:        "cluster-logging",
					catalogSourceName:      "redhat-operators",
					catalogSourceNamespace: "openshift-marketplace",
					startingCSV:            "",
					currentCSV:             "",
					installedCSV:           "",
					template:               subCoTemplate,
					singleNamespace:        true,
				}
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc,
				objectTableRef{"scansettingbinding", subD.namespace, ssb.name},
				objectTableRef{"ClusterLogForwarder", subL.namespace, "instance"},
				objectTableRef{"ClusterLogging", subL.namespace, "instance"},
				objectTableRef{"configmap", subL.namespace, "fluentdserver"},
				objectTableRef{"deployment", subL.namespace, "fluentdserver"},
				objectTableRef{"service", subL.namespace, "fluentdserver"},
				objectTableRef{"serviceaccount", subL.namespace, "fluentdserver"},
				objectTableRef{"secret", subL.namespace, "fluentdserver"},
				objectTableRef{"project", subL.namespace, subL.namespace})

			g.By("Check default profiles are available 'ocp4-cis' and 'ocp4-moderate' .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			subD.getProfileName(oc, "ocp4-moderate")
			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify audit rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-audit-log-forwarding-uses-tls", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-cis-audit-log-forwarding-enabled", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Check if the openshift-logging namespace is already available.. !!!\n")
			namespace, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", "-ojsonpath={.items[*].metadata.name}").Output()
			if !strings.Contains(namespace, "openshift-logging") {
				_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "openshift-logging").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("Installing cluster-logging operator in openshift-logging namespace.. !!!\n")
				ogL.namespace = "openshift-logging"
				subL.namespace = ogL.namespace
				ogL.create(oc, itName, dr)
				subL.create(oc, itName, dr)
				newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subL.installedCSV, "-n", subL.namespace,
					"-o=jsonpath={.status.phase}"}).check(oc)
				newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", "-l name=cluster-logging-operator", "-n", subL.namespace,
					"-o=jsonpath={.items[0].status.phase}"}).check(oc)
			} else {
				g.By("The openshift-logging namespace is already available.. !!!\n")
				podStat := checkOperatorPodStatus(oc, "openshift-logging")
				if !strings.Contains(podStat, "Running") {
					ogL.namespace = "openshift-logging"
					subL.namespace = ogL.namespace
					ogL.create(oc, itName, dr)
					subL.create(oc, itName, dr)
					newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", subL.installedCSV, "-n", subL.namespace,
						"-o=jsonpath={.status.phase}"}).check(oc)
					newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", "-l name=cluster-logging-operator", "-n", subL.namespace,
						"-o=jsonpath={.items[0].status.phase}"}).check(oc)
				} else {
					g.By("The cluster-logging operator is installed in openshift-logging namespace and pod is running.. !!!\n")
				}
			}

			g.By("Generate the secret key for fluentdserver.. !!!\n")
			genFluentdSecret(oc, subL.namespace, "fluentdserver")
			newCheck("expect", asAdmin, withoutNamespace, contain, "fluentdserver", ok, []string{"secret", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Create service accound for fluentd receiver.. !!!\n")
			_, err2 := oc.AsAdmin().WithoutNamespace().Run("create").Args("sa", "fluentdserver", "-n", subL.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "fluentdserver", ok, []string{"sa", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			_, err3 := oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", "system:serviceaccount:openshift-logging:fluentdserver", "-n", subL.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			g.By("Create Fluentd ConfigMap .. !!!\n")
			_, err4 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subL.namespace, "-f", fluentdCmYAML).Output()
			o.Expect(err4).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "fluentdserver", ok, []string{"cm", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Create Fluentd Deployment .. !!!\n")
			_, err5 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subL.namespace, "-f", fluentdDmYAML).Output()
			o.Expect(err5).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "fluentdserver", ok, []string{"deployment", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			g.By("Expose Fluentd Deployment .. !!!\n")
			_, err6 := oc.AsAdmin().WithoutNamespace().Run("expose").Args("deployment", "fluentdserver", "-n", subL.namespace).Output()
			o.Expect(err6).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "fluentdserver", ok, []string{"deployment", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Create ClusterForward & ClusterLogging instances.. !!!\n")
			_, err7 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subL.namespace, "-f", clusterLogForYAML).Output()
			o.Expect(err7).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "instance", ok, []string{"ClusterLogForwarder", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			_, err8 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subL.namespace, "-f", clusterLoggingYAML).Output()
			o.Expect(err8).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "instance", ok, []string{"ClusterLogging", "-n", subL.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check fluentdserver is running state.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pod", "-l logging-infra=fluentdserver", "-n",
				subL.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)

			g.By("Rerun scan and check ComplianceSuite status & result.. !!!\n")
			_, err9 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err9).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify audit rules status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-audit-log-forwarding-uses-tls", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-cis-audit-log-forwarding-enabled", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			//	csvname, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", subD.namespace, "-o=jsonpath={.items[0].metadata.name}").Output()
			//	assertCheckAuditLogsForword(oc, subL.namespace, csvname)

			g.By("ocp-40660 and ocp-42874 the audit logs are getting forwarded using TLS protocol successfully... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-Low-42700-Check that a login banner is configured and login screen customised [Disruptive][Slow]", func() {

			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove motd, ConsoleNotification and scansettingbinding objects.. !!!\n")
				patch := fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/spec/templates/login\"}]")
				patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "json", "-p", patch)
				newCheck("expect", asAdmin, withoutNamespace, contain, "login-secret", nok, []string{"oauth", "cluster", "-o=jsonpath={.spec.templates.login}"}).check(oc)
				cleanupObjects(oc, objectTableRef{"secret", "openshift-config", "login-secret"})
				cleanupObjects(oc, objectTableRef{"configmap", "openshift", "motd"})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			}()

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify motd and banner or login rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-openshift-motd-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-banner-or-login-template-set", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Create motd configMap and consoleNotification objects.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift", "-f", motdConfigMapYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "motd", ok, []string{"configmap", "-n", "openshift", "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			_, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", consoleNotificationYAML).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "classification-banner", ok, []string{"ConsoleNotification", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err2 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify motd and banner or login rules status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-openshift-motd-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-banner-or-login-template-set", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			cleanupObjects(oc, objectTableRef{"ConsoleNotification", subD.namespace, "classification-banner"})
			createLoginTemp(oc, "openshift-config")
			newCheck("expect", asAdmin, withoutNamespace, contain, "login-secret", ok, []string{"secret", "-n", "openshift-config", "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Set login-secret template to oauth cluster object by patching.. !!!\n")
			patchResource(oc, asAdmin, withoutNamespace, "oauth", "cluster", "--type", "merge", "-p", "{\"spec\":{\"templates\":{\"login\":{\"name\":\"login-secret\"}}}}")
			newCheck("expect", asAdmin, withoutNamespace, contain, "login-secret", ok, []string{"oauth", "cluster", "-o=jsonpath={.spec.templates.login}"}).check(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err3 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify ocp4-moderate-banner-or-login-template-set rule status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-banner-or-login-template-set", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-42700 The login banner is configured and login screen customised successfully... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-Low-42720-check manual remediation for rule ocp4-moderate-configure-network-policies-namespaces working as expected [Disruptive][Slow]", func() {

			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove NetworkPolicy object from all non-control plane namespace.. !!!\n")
				nsName := "ns-42720-1"
				cleanupObjects(oc, objectTableRef{"ns", nsName, nsName})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
				nonControlNamespacesList := getNonControlNamespaces(oc, "Active")
				for _, v := range nonControlNamespacesList {
					cleanupObjects(oc, objectTableRef{"NetworkPolicy", v, "allow-same-namespace"})
					newCheck("expect", asAdmin, withoutNamespace, contain, "allow-same-namespace", nok, []string{"NetworkPolicy", "-n", v, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
				}
			}()

			g.By("Check default profiles name ocp4-moderate .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")
			ssb.namespace = subD.namespace

			nonControlNamespacesTerList := getNonControlNamespaces(oc, "Terminating")
			e2e.Logf("Terminating Non Control Namespaces List: %s", nonControlNamespacesTerList)
			if len(nonControlNamespacesTerList) != 0 {
				for _, v := range nonControlNamespacesTerList {
					scanName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("scan", "-n", v, "-ojsonpath={.items[*].metadata.name}").Output()
					scans := strings.Fields(scanName)
					patch := fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]")
					if len(scans) != 0 {
						for _, scanname := range scans {
							e2e.Logf("The %s scan patched to remove finalizers from namespace %s \n", scanname, v)
							patchResource(oc, asAdmin, withoutNamespace, "scan", scanname, "--type", "json", "-p", patch, "-n", v)
						}
					}
					suiteName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("suite", "-n", v, "-ojsonpath={.items[*].metadata.name}").Output()
					suites := strings.Fields(suiteName)
					if len(suites) != 0 {
						for _, suitename := range suites {
							e2e.Logf("The %s suite patched to remove finalizers from namespace %s \n", suitename, v)
							patchResource(oc, asAdmin, withoutNamespace, "suite", suitename, "--type", "json", "-p", patch, "-n", v)
						}
					}
					profbName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("profilebundles", "-n", v, "-ojsonpath={.items[*].metadata.name}").Output()
					profb := strings.Fields(profbName)
					if len(profb) != 0 {
						for _, profbname := range profb {
							e2e.Logf("The %s profilebundle patched to remove finalizers from namespace %s \n", profbname, v)
							patchResource(oc, asAdmin, withoutNamespace, "profilebundles", profbname, "--type", "json", "-p", patch, "-n", v)
						}
					}
				}
			}

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "moderate-test", ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")
			g.By("Verify ocp4-moderate-configure-network-policies-namespaces rule status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-configure-network-policies-namespaces", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)

			nonControlNamespacesTerList1 := getNonControlNamespaces(oc, "Terminating")
			if len(nonControlNamespacesTerList1) == 0 {
				g.By("Create NetworkPolicy in all non-control plane namespace .. !!!\n")
				nonControlNamespacesList := getNonControlNamespaces(oc, "Active")
				e2e.Logf("Here namespace : %v\n", nonControlNamespacesList)
				for _, v := range nonControlNamespacesList {
					_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", v, "-f", networkPolicyYAML).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					newCheck("expect", asAdmin, withoutNamespace, contain, "allow-same-namespace", ok, []string{"NetworkPolicy", "-n", v, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
				}
			}

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", ssb.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Verify ocp4-moderate-configure-network-policies-namespaces rule status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-configure-network-policies-namespaces", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Create one more non-control plane namespace .. !!!\n")
			nsName := "ns-42720-1"
			_, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", nsName).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err2 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", ssb.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Verify motd and banner or login rules status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-configure-network-policies-namespaces", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-42720 The manual remediation works for network-policies-namespaces rule... !!!!\n ")

		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-41153-There are OpenSCAP checks created to verify that the cluster is compliant  for the section 5 of the Kubernetes CIS profile [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Check default profiles name ocp4-cis .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			cisRlueList := []string{"ocp4-cis-rbac-limit-cluster-admin", "ocp4-cis-rbac-limit-secrets-access", "ocp4-cis-rbac-wildcard-use", "ocp4-cis-rbac-pod-creation-access",
				"ocp4-cis-accounts-unique-service-account", "ocp4-cis-accounts-restrict-service-account-tokens", "ocp4-cis-scc-limit-privileged-containers", "ocp4-cis-scc-limit-process-id-namespace",
				"ocp4-cis-scc-limit-ipc-namespace", "ocp4-cis-scc-limit-network-namespace", "ocp4-cis-scc-limit-privilege-escalation", "ocp4-cis-scc-limit-root-containers",
				"ocp4-cis-scc-limit-net-raw-capability", "ocp4-cis-scc-limit-container-allowed-capabilities", "ocp4-cis-scc-drop-container-capabilities", "ocp4-cis-configure-network-policies",
				"ocp4-cis-configure-network-policies-namespaces", "ocp4-cis-secrets-no-environment-variables", "ocp4-cis-secrets-consider-external-storage", "ocp4-cis-general-configure-imagepolicywebhook",
				"ocp4-cis-general-namespaces-in-use", "ocp4-cis-general-default-seccomp-profile", "ocp4-cis-general-apply-scc", "ocp4-cis-general-default-namespace-use"}
			checkRulesExistInComplianceCheckResult(oc, cisRlueList, subD.namespace)

			g.By("ocp-41153 There are OpenSCAP checks created to verify that the cluster is compliant for the section 5 of the Kubernetes CIS profile... !!!!\n ")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-CPaasrunOnly-NonPreRelease-High-27967-High-33782-Medium-33711-Medium-47346-The ComplianceSuite performs scan on a subset of nodes with autoApplyRemediations enable and ComplianceCheckResult shows remediation rule result in details and also supports array of values for remediation [Disruptive][Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_audit_rules_dac_modification_chmod",
					nodeSelector: "wrscan",
					template:     csuiteTemplate,
				}
				csuite = complianceSuiteDescription{
					name:         "example-compliancesuite",
					namespace:    "",
					scanname:     "example-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_no_empty_passwords",
					nodeSelector: "wrscan",
					template:     csuiteRemTemplate,
				}
				csuiteCD = complianceSuiteDescription{
					name:         "chronyd-compliancesuite",
					namespace:    "",
					scanname:     "rhcos4-scan",
					profile:      "xccdf_org.ssgproject.content_profile_moderate",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					rule:         "xccdf_org.ssgproject.content_rule_chronyd_or_ntpd_specify_multiple_servers",
					nodeSelector: "wrscan",
					template:     csuiteRemTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)
			// checking all nodes are in Ready state before the test case starts
			checkNodeStatus(oc)
			// adding label to one rhcos worker node to skip rhel and other RHCOS worker nodes
			g.By("Label one rhcos worker node as wrscan.. !!!\n")
			workerNodeName := getOneRhcosWorkerNodeName(oc)
			setLabelToOneWorkerNode(oc, workerNodeName)

			defer func() {
				g.By("Remove compliancesuite, machineconfig, machineconfigpool objects.. !!!\n")
				removeLabelFromWorkerNode(oc, workerNodeName)
				checkMachineConfigPoolStatus(oc, "worker")
				cleanupObjects(oc, objectTableRef{"mc", subD.namespace, "75-worker-scan-audit-rules-dac-modification-chmod"})
				cleanupObjects(oc, objectTableRef{"mc", subD.namespace, "75-example-scan-no-empty-passwords"})
				cleanupObjects(oc, objectTableRef{"mc", subD.namespace, "75-rhcos4-scan-chronyd-or-ntpd-specify-multiple-servers"})
				cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, csuiteD.name})
				cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, csuite.name})
				cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, csuiteCD.name})
				checkMachineConfigPoolStatus(oc, "worker")
				cleanupObjects(oc, objectTableRef{"mcp", subD.namespace, csuiteD.nodeSelector})
				checkNodeStatus(oc)
			}()

			g.By("Create wrscan machineconfigpool.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", machineConfigPoolYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			checkMachineConfigPoolStatus(oc, csuiteD.nodeSelector)

			g.By("Create compliancesuite objects !!!\n")
			csuiteD.namespace = subD.namespace
			csuite.namespace = subD.namespace
			csuiteCD.namespace = subD.namespace
			csuiteD.create(oc, itName, dr)
			csuite.create(oc, itName, dr)
			csuiteCD.create(oc, itName, dr)
			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n", csuiteD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuiteD.name, "NON-COMPLIANT")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuite.name, "-n", csuite.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuite.name, "NON-COMPLIANT")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteCD.name, "-n", csuiteCD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuiteCD.name, "NON-COMPLIANT")

			g.By("Verify worker-scan-audit-rules-dac-modification-chmod rule status through compliancecheckresult & complianceremediations.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"worker-scan-audit-rules-dac-modification-chmod", "-n", csuiteD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "NotApplied", ok, []string{"complianceremediations",
				"worker-scan-audit-rules-dac-modification-chmod", "-n", csuiteD.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)

			g.By("Apply remediation by patching rule.. !!!\n")
			patch := fmt.Sprintf("{\"spec\":{\"apply\":true}}")
			patchResource(oc, asAdmin, withoutNamespace, "complianceremediations", "worker-scan-audit-rules-dac-modification-chmod", "-n", csuiteD.namespace, "--type", "merge", "-p", patch)

			g.By("Verify rules status through compliancecheckresult !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"example-scan-no-empty-passwords", "-n", csuite.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"rhcos4-scan-chronyd-or-ntpd-specify-multiple-servers", "-n", csuiteCD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Verified autoremediation applied for those rules and machineConfig gets created.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations", "worker-scan-audit-rules-dac-modification-chmod", "-n", csuiteD.namespace,
				"-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "75-worker-scan-audit-rules-dac-modification-chmod", ok, []string{"mc", "-n", csuiteD.namespace,
				"--selector=compliance.openshift.io/scan-name=worker-scan", "-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations", "example-scan-no-empty-passwords", "-n", csuiteD.namespace,
				"-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "75-example-scan-no-empty-passwords", ok, []string{"mc", "-n", csuiteD.namespace,
				"--selector=compliance.openshift.io/scan-name=example-scan", "-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations", "rhcos4-scan-chronyd-or-ntpd-specify-multiple-servers", "-n", csuiteCD.namespace,
				"-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "75-rhcos4-scan-chronyd-or-ntpd-specify-multiple-servers", ok, []string{"mc", "-n", csuiteD.namespace,
				"--selector=compliance.openshift.io/scan-name=rhcos4-scan", "-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check worker machineconfigpool status.. !!!\n")
			checkMachineConfigPoolStatus(oc, csuiteD.nodeSelector)

			g.By("Rerun scan using oc-compliance plugin.. !!!\n")
			_, err1 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", csuiteD.name, "-n", csuiteD.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			_, err2 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", csuite.name, "-n", csuite.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			_, err3 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", csuiteCD.name, "-n", csuiteCD.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n", csuiteD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuiteD.name, "COMPLIANT")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuite.name, "-n", csuite.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuite.name, "COMPLIANT")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteCD.name, "-n", csuiteCD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, csuiteCD.name, "COMPLIANT")

			g.By("Verify rules status through compliancecheck result again.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"worker-scan-audit-rules-dac-modification-chmod", "-n", csuiteD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"example-scan-no-empty-passwords", "-n", csuite.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"rhcos4-scan-chronyd-or-ntpd-specify-multiple-servers", "-n", csuiteCD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Verify the ntp settings from node contents.. !!!\n")
			contList := []string{"server 0.pool.ntp.org minpoll 4 maxpoll 10", "server 1.pool.ntp.org minpoll 4 maxpoll 10", "server 2.pool.ntp.org minpoll 4 maxpoll 10", "server 3.pool.ntp.org minpoll 4 maxpoll 10"}
			checkNodeContents(oc, workerNodeName, contList, "cat", "-n", "/etc/chrony.d/ntp-server.conf", "server")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-CPaasrunOnly-NonPreRelease-High-45421-Verify the scan scheduling option strict or not strict are configurable through scan objects [Disruptive][Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "nodestrict",
					namespace:             "",
					roles1:                "worker",
					rotation:              5,
					schedule:              "0 1 * * *",
					strictnodescan:        false,
					size:                  "2Gi",
					template:              scansettingSingleTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "rhcos4-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "rhcos4-moderate",
					scansettingname: "nodestrict",
					template:        scansettingbindingSingleTemplate,
				}
			)

			g.By("Get one worker node and mark that unschedulable.. !!!\n")
			nodeName := getOneRhcosWorkerNodeName(oc)
			defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", nodeName).Output()
			_, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("cordon", nodeName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "true", ok, []string{"node", nodeName, "-o=jsonpath={.spec.unschedulable}"}).check(oc)

			defer func() {
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
				cleanupObjects(oc, objectTableRef{"scansetting", subD.namespace, ss.name})
			}()

			g.By("Check default profiles name rhcos4-moderate.. !!!\n")
			subD.getProfileName(oc, "rhcos4-moderate")

			g.By("Create scansetting !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ss.name, ok, []string{"scansetting", ss.name, "-n", ss.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Remove scansettingbinding object !!!\n")
			cleanupObjects(oc, objectTableRef{"scansettingbinding", ssb.namespace, ssb.name})

			g.By("Patch scansetting object and verify.. !!!\n")
			patchResource(oc, asAdmin, withoutNamespace, "ss", ss.name, "--type", "merge", "-p", "{\"strictNodeScan\":true}", "-n", ss.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "true", ok, []string{"ss", ss.name, "-n", ss.namespace, "-o=jsonpath={..strictNodeScan}"}).check(oc)

			g.By("Again create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PENDING", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("ocp-45421 Successfully verify the scan scheduling option strict or not strict are configurable through scan objects... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-NonPreRelease-CPaasrunOnly-Longduration-High-45692-Verify scan and manual fix work as expected for NERC CIP profiles with default scanSettings [Disruptive][Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: false,
					name:                  "nerc-ss",
					namespace:             "",
					roles1:                "worker",
					rotation:              5,
					schedule:              "0 1 * * *",
					strictnodescan:        false,
					size:                  "2Gi",
					template:              scansettingSingleTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "nerc-ocp4-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-nerc-cip",
					profilename2:    "ocp4-nerc-cip-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				ssb1 = scanSettingBindingDescription{
					name:            "nerc-rhcos4-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "rhcos4-nerc-cip",
					scansettingname: "nerc-ss",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove motd configmap and scansettingbinding objects.. !!!\n")
				cleanupObjects(oc, objectTableRef{"configmap", "openshift", "motd"})
				cleanupObjects(oc, objectTableRef{"scansetting", subD.namespace, ss.name})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb1.name})
			}()

			g.By("Check default NERC profiles.. !!!\n")
			subD.getProfileName(oc, "ocp4-nerc-cip")
			subD.getProfileName(oc, "ocp4-nerc-cip-node")
			subD.getProfileName(oc, "rhcos4-nerc-cip")

			g.By("Create scansetting !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ss.name, ok, []string{"scansetting", ss.name, "-n", ss.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)

			g.By("Create scansettingbindings !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, subD.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Create scansettingbindings for rhcos4-nerc.. !!!\n")
			ssb1.namespace = subD.namespace
			ssb1.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb1.name, ok, []string{"scansettingbinding", ssb1.name, "-n", ssb1.namespace,
				"-o=jsonpath={.metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb1.name, subD.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb1.name)
			subD.complianceSuiteResult(oc, ssb1.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Create motd configMap object.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift", "-f", motdConfigMapYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "motd", ok, []string{"configmap", "-n", "openshift", "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Rerun scan using oc-compliance plugin.. !!")
			_, err1 := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Verify motd and banner or login rules status again through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-nerc-cip-openshift-motd-exists", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("ocp-45692 The NERC CIP compliance profiles perform scan and manual fix as expected with default scanSettings... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-46991-Check the PCI DSS compliance profiles perform scan as expected with default scanSettings [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "pci-dss-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-pci-dss",
					profilename2:    "ocp4-pci-dss-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			g.By("Check default profiles name ocp4-pci-dss .. !!!\n")
			subD.getProfileName(oc, "ocp4-pci-dss")
			g.By("Check default profiles name ocp4-pci-dss-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-pci-dss-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, subD.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("ocp-46991 The PCI DSS compliance profiles perform scan as expected with default scanSettings... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-High-43066-check the metrics and alerts are available for Compliance Operator [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "default",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			metricSsbStr := []string{"compliance_operator_compliance_scan_status_total{name=\"ocp4-cis-node-master\",phase=\"DONE\",result=\"NON-COMPLIANT\"} 1",
				"compliance_operator_compliance_scan_status_total{name=\"ocp4-cis-node-worker\",phase=\"DONE\",result=\"NON-COMPLIANT\"} 1",
				"compliance_operator_compliance_state{name=\"cis-test\"} 1"}

			newCheck("expect", asAdmin, withoutNamespace, contain, "openshift.io/cluster-monitoring", ok, []string{"namespace", subD.namespace, "-o=jsonpath={.metadata.labels}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "metrics", ok, []string{"service", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check default profiles name ocp4-cis .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis")
			g.By("Check default profiles name ocp4-cis-node .. !!!\n")
			subD.getProfileName(oc, "ocp4-cis-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssb.name, subD.namespace, "DONE")
			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteName(oc, ssb.name)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")

			checkMetric(oc, metricSsbStr, subD.namespace, "co")
			newCheck("expect", asAdmin, withoutNamespace, contain, "compliance", ok, []string{"PrometheusRule", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "NonCompliant", ok, []string{"PrometheusRule", "compliance", "-n", subD.namespace, "-ojsonpath={.spec.groups[0].rules[0].alert}"}).check(oc)

			g.By("ocp-43066 The metrics and alerts are getting reported for Compliance Operator ... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Low-43072-check the metrics and alerts are available for compliance_operator_compliance_scan_error_total [Slow]", func() {
			var (
				csuiteD = complianceSuiteDescription{
					name:         "worker-compliancesuite",
					namespace:    "",
					scanname:     "worker-scan",
					profile:      "xccdf_org.ssgproject.content_profile_coreos-ncp",
					content:      "ssg-rhcos4-ds.xml",
					contentImage: "quay.io/complianceascode/ocp4:latest",
					nodeSelector: "wscan",
					template:     csuiteTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"compliancesuite", subD.namespace, csuiteD.name})

			g.By("Label all rhcos worker nodes as wscan !!!\n")
			setLabelToNode(oc)

			metricsErr := []string{"compliance_operator_compliance_scan_error_total{error=\"I: oscap:", "compliance_operator_compliance_scan_status_total{name=\"worker-scan\",phase=\"DONE\",result=\"ERROR\"} 1",
				"compliance_operator_compliance_state{name=\"worker-compliancesuite\"} 3"}

			newCheck("expect", asAdmin, withoutNamespace, contain, "openshift.io/cluster-monitoring", ok, []string{"namespace", subD.namespace, "-o=jsonpath={.metadata.labels}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "metrics", ok, []string{"service", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Create compliancesuite !!!\n")
			csuiteD.namespace = subD.namespace
			csuiteD.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", csuiteD.name, "-n",
				subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result..!!!\n")
			subD.complianceSuiteName(oc, "worker-compliancesuite")
			subD.complianceSuiteResult(oc, csuiteD.name, "ERROR")

			checkMetric(oc, metricsErr, subD.namespace, "co")
			newCheck("expect", asAdmin, withoutNamespace, contain, "compliance", ok, []string{"PrometheusRule", "-n", subD.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "NonCompliant", ok, []string{"PrometheusRule", "compliance", "-n", subD.namespace, "-ojsonpath={.spec.groups[0].rules[0].alert}"}).check(oc)

			g.By("ocp-43072 The metrics and alerts are getting reported for Compliance Operator error ... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-CPaasrunOnly-NonPreRelease-High-46419-Compliance operator supports remediation templating by setting custom variables in the tailored profile [Disruptive][Slow]", func() {
			var (
				tprofileD = tailoredProfileDescription{
					name:         "ocp4-audit-tailored",
					namespace:    "",
					extends:      "ocp4-cis",
					enrulename1:  "ocp4-audit-profile-set",
					disrulename1: "ocp4-audit-error-alert-exists",
					disrulename2: "ocp4-audit-log-forwarding-uses-tls",
					varname:      "ocp4-var-openshift-audit-profile",
					value:        "WriteRequestBodies",
					template:     tprofileTemplate,
				}
				ssb = scanSettingBindingDescription{
					name:            "ocp4-audit-ssb",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "ocp4-audit-tailored",
					scansettingname: "default-auto-apply",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Reset the audit profile value to Default and remove resources... !!!\n")
				patchResource(oc, asAdmin, withoutNamespace, "apiservers", "cluster", "--type", "merge", "-p",
					"{\"spec\":{\"audit\":{\"profile\":\"Default\"}}}")
				newCheck("expect", asAdmin, withoutNamespace, contain, "Default", ok, []string{"apiservers", "cluster",
					"-o=jsonpath={.spec.audit.profile}"}).check(oc)
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
				cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, tprofileD.name})
			}()

			g.By("Check default profiles name ocp4-cis .. !!!\n")
			subD.getProfileName(oc, tprofileD.extends)

			g.By("Create tailoredprofile ocp4-audit-tailored !!!\n")
			tprofileD.namespace = subD.namespace
			ssb.namespace = subD.namespace
			tprofileD.create(oc, itName, dr)
			g.By("Verify tailoredprofile name and status !!!\n")
			subD.getTailoredProfileNameandStatus(oc, tprofileD.name)

			g.By("Create scansettingbinding !!!\n")
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", subD.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT INCONSISTENT")
			g.By("Check complianceSuite result through exit-code.. !!!\n")
			subD.getScanExitCodeFromConfigmap(oc, "2")

			g.By("Verify the rule status through compliancecheckresult and complianceremediations... !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-audit-tailored-audit-profile-set", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations",
				"ocp4-audit-tailored-audit-profile-set", "-n", subD.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)

			g.By("Verify the audit profile set value through complianceremediations... !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "WriteRequestBodies", ok, []string{"complianceremediations",
				"ocp4-audit-tailored-audit-profile-set", "-n", subD.namespace, "-o=jsonpath={.spec.current.object.spec.audit.profile}"}).check(oc)

			g.By("Rerun the scan and check result... !!")
			_, err := OcComplianceCLI().Run("rerun-now").Args("scansettingbinding", ssb.name, "-n", subD.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", subD.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "COMPLIANT")

			g.By("Verify the rule status through compliancecheckresult and value through apiservers cluster object... !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-audit-tailored-audit-profile-set", "-n", subD.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "WriteRequestBodies", ok, []string{"apiservers", "cluster",
				"-ojsonpath={.spec.audit.profile}"}).check(oc)

			g.By("The compliance operator supports remediation templating by setting custom variables in the tailored profile... !!!\n")
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-CPaasrunOnly-NonPreRelease-High-46100-High-46995-Verify autoremediations works for PCI-DSS and CIS profiles [Disruptive][Slow]", func() {
			var (
				ss = scanSettingDescription{
					autoapplyremediations: true,
					name:                  "auto-rem-ss",
					namespace:             "",
					roles1:                "wrscan",
					rotation:              5,
					schedule:              "0 1 * * *",
					strictnodescan:        false,
					size:                  "2Gi",
					template:              scansettingSingleTemplate,
				}
				ssbCis = scanSettingBindingDescription{
					name:            "cis-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-cis",
					profilename2:    "ocp4-cis-node",
					scansettingname: "auto-rem-ss",
					template:        scansettingbindingTemplate,
				}
				ssbPci = scanSettingBindingDescription{
					name:            "pci-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-pci-dss",
					profilename2:    "ocp4-pci-dss-node",
					scansettingname: "auto-rem-ss",
					template:        scansettingbindingTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			// checking all nodes are in Ready state before the test case starts
			checkNodeStatus(oc)
			// adding label to one rhcos worker node to skip rhel and other RHCOS worker nodes
			g.By("Label one rhcos worker node as wrscan.. !!!\n")
			workerNodeName := getOneRhcosWorkerNodeName(oc)
			setLabelToOneWorkerNode(oc, workerNodeName)

			defer func() {
				g.By("Remove scansettingbinding, machineconfig, machineconfigpool objects.. !!!\n")
				removeLabelFromWorkerNode(oc, workerNodeName)
				checkMachineConfigPoolStatus(oc, "worker")
				cleanupObjects(oc, objectTableRef{"ScanSettingBinding", ssbCis.namespace, ssbCis.name})
				cleanupObjects(oc, objectTableRef{"ScanSettingBinding", ssbPci.namespace, ssbPci.name})
				cleanupObjects(oc, objectTableRef{"ScanSetting", ss.namespace, ss.name})
				cleanupObjects(oc, objectTableRef{"kubeletconfig", ssbCis.name, "compliance-operator-kubelet-wrscan"})
				cleanupObjects(oc, objectTableRef{"mc", ssbCis.name, "75-ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-sysctl"})
				cleanupObjects(oc, objectTableRef{"mc", ssbPci.name, "75-ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-sysctl"})
				checkMachineConfigPoolStatus(oc, "worker")
				cleanupObjects(oc, objectTableRef{"mcp", ssbCis.namespace, ss.roles1})
				checkNodeStatus(oc)
			}()

			g.By("Create wrscan machineconfigpool.. !!!\n")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", subD.namespace, "-f", machineConfigPoolYAML).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			checkMachineConfigPoolStatus(oc, ss.roles1)

			remRules := []string{"wrscan-kubelet-configure-event-creation", "wrscan-kubelet-configure-tls-cipher-suites", "wrscan-kubelet-enable-iptables-util-chains", "wrscan-kubelet-enable-protect-kernel-sysctl",
				"wrscan-kubelet-enable-streaming-connections", "wrscan-kubelet-eviction-thresholds-set-hard-imagefs-available", "wrscan-kubelet-eviction-thresholds-set-hard-imagefs-inodesfree",
				"wrscan-kubelet-eviction-thresholds-set-hard-memory-available", "wrscan-kubelet-eviction-thresholds-set-hard-nodefs-available", "wrscan-kubelet-eviction-thresholds-set-hard-nodefs-inodesfree",
				"wrscan-kubelet-eviction-thresholds-set-soft-imagefs-available", "wrscan-kubelet-eviction-thresholds-set-soft-imagefs-inodesfree", "wrscan-kubelet-eviction-thresholds-set-soft-memory-available",
				"wrscan-kubelet-eviction-thresholds-set-soft-nodefs-available", "wrscan-kubelet-eviction-thresholds-set-soft-nodefs-inodesfree"}

			g.By("Create scansetting... !!!\n")
			ss.namespace = subD.namespace
			ss.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ss.name, ok, []string{"scansetting", "-n", ss.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Create scansettingbinding... !!!\n")
			ssbCis.namespace = subD.namespace
			ssbPci.namespace = subD.namespace
			ssbCis.create(oc, itName, dr)
			ssbPci.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssbCis.name, ok, []string{"scansettingbinding", "-n", ssbCis.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssbPci.name, ok, []string{"scansettingbinding", "-n", ssbPci.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			checkComplianceSuiteStatus(oc, ssbCis.name, ssbCis.namespace, "DONE")
			checkComplianceSuiteStatus(oc, ssbPci.name, ssbPci.namespace, "DONE")

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssbCis.name, "NON-COMPLIANT INCONSISTENT")
			subD.complianceSuiteResult(oc, ssbPci.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check rules and remediation status !!!\n")
			getRuleStatus(oc, remRules, "FAIL", ssbCis.profilename2, ssbCis.namespace)
			getRuleStatus(oc, remRules, "FAIL", ssbPci.profilename2, ssbPci.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations",
				"ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-sysctl", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations",
				"ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-sysctl", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "MissingDependencies", ok, []string{"complianceremediations",
				"ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "MissingDependencies", ok, []string{"complianceremediations",
				"ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)

			g.By("Check rules status and remediation status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "compliance-operator-kubelet-wrscan", ok, []string{"kubeletconfig", "-n", ssbCis.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "75-ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-sysctl", ok, []string{"mc", "-n", ssbCis.namespace,
				"--selector=compliance.openshift.io/suite=" + ssbCis.name, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "75-ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-sysctl", ok, []string{"mc", "-n", ssbPci.namespace,
				"--selector=compliance.openshift.io/suite=" + ssbPci.name, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check wrscan machineconfigpool status.. !!!\n")
			checkMachineConfigPoolStatus(oc, ss.roles1)

			g.By("Performing second scan using oc-compliance plugin.. !!!\n")
			_, err1 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssbCis.name, "-n", ssbCis.namespace).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			_, err2 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssbPci.name, "-n", ssbPci.namespace).Output()
			o.Expect(err2).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbCis.name, "-n", ssbCis.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbPci.name, "-n", ssbPci.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssbCis.name, "NON-COMPLIANT INCONSISTENT")
			subD.complianceSuiteResult(oc, ssbPci.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check rules and remediation status !!!\n")
			getRuleStatus(oc, remRules, "PASS", ssbCis.profilename2, ssbCis.namespace)
			getRuleStatus(oc, remRules, "PASS", ssbPci.profilename2, ssbPci.namespace)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations",
				"ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Applied", ok, []string{"complianceremediations",
				"ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status.applicationState}"}).check(oc)

			g.By("Check wrscan machineconfigpool status.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, compare, "0", ok, []string{"machineconfigpool", ss.roles1, "-n", ssbCis.namespace, "-o=jsonpath={.status.readyMachineCount}"}).check(oc)
			checkMachineConfigPoolStatus(oc, ss.roles1)

			g.By("Performing third scan using oc-compliance plugin.. !!!\n")
			_, err3 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssbCis.name, "-n", ssbCis.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			_, err4 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssbPci.name, "-n", ssbPci.namespace).Output()
			o.Expect(err4).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbCis.name, "-n", ssbCis.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbPci.name, "-n", ssbPci.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)

			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssbCis.name, "NON-COMPLIANT INCONSISTENT")
			subD.complianceSuiteResult(oc, ssbPci.name, "NON-COMPLIANT INCONSISTENT")

			g.By("Check rules and remediation status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-cis-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-pci-dss-node-wrscan-kubelet-enable-protect-kernel-defaults", "-n", ssbCis.namespace, "-o=jsonpath={.status}"}).check(oc)
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Longduration-CPaasrunOnly-NonPreRelease-High-47147-Check rules work if all non-openshift namespaces has resourcequota and route rate limit [Disruptive][Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer func() {
				g.By("Remove route and resourcequota objects from all non-control plane namespace.. !!!\n")
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})
				nsName := "ns-47147-1"
				cleanupObjects(oc, objectTableRef{"route", nsName, "wordpress"})
				cleanupObjects(oc, objectTableRef{"ns", nsName, nsName})
				newCheck("expect", asAdmin, withoutNamespace, contain, "wordpress", nok, []string{"route", "-n", nsName, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
				nonControlNamespacesList := getNonControlNamespaces(oc, "Active")
				for _, v := range nonControlNamespacesList {
					cleanupObjects(oc, objectTableRef{"resourcequota", v, "example"})
					newCheck("expect", asAdmin, withoutNamespace, contain, "example", nok, []string{"resourcequota", "-n", v, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
				}
			}()

			g.By("Check for default profile 'ocp4-moderate' .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create route in non-control namespace .. !!!\n")
			nsName := "ns-47147-1"
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", nsName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", nsName, "-f", wordpressRouteYAML).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, "wordpress", ok, []string{"route", "-n", nsName, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-resource-requests-quota", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
				"ocp4-moderate-routes-rate-limit", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)

			g.By("Annotate route in non-control namespace.. !!!\n")
			_, err2 := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("-n", nsName, "route", "wordpress", "haproxy.router.openshift.io/rate-limit-connections=true").Output()
			o.Expect(err2).NotTo(o.HaveOccurred())

			nonControlNamespacesTerList1 := getNonControlNamespaces(oc, "Terminating")
			if len(nonControlNamespacesTerList1) == 0 {
				nonControlNamespacesList := getNonControlNamespaces(oc, "Active")
				e2e.Logf("Here namespace : %v\n", nonControlNamespacesList)
				g.By("Create resourcequota in all non-control plane namespace .. !!!\n")
				for _, v := range nonControlNamespacesList {
					_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", v, "-f", resourceQuotaYAML).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					newCheck("expect", asAdmin, withoutNamespace, contain, "example", ok, []string{"resourcequota", "-n", v,
						"-o=jsonpath={.items[*].metadata.name}"}).check(oc)
				}
			}

			g.By("Rerun scan using oc-compliance plugin.. !!!\n")
			_, err3 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssb.name, "-n", ssb.namespace).Output()
			o.Expect(err3).NotTo(o.HaveOccurred())
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Verify rules status through compliancecheck result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-resource-requests-quota", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"ocp4-moderate-routes-rate-limit", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-47173-Verify the rule that check for API Server audit error alerts [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "ocp4-moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Check for default profile 'ocp4-moderate' .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			g.By("Check if audit prometheusrules is already available.. !!!\n")
			prometheusrules, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("prometheusrules", "-n", "openshift-kube-apiserver", "-ojsonpath={.items[*].metadata.name}").Output()
			if strings.Contains(prometheusrules, "audit-errors") {
				newCheck("expect", asAdmin, withoutNamespace, contain, "audit-errors", ok, []string{"prometheusrules", "-n", "openshift-kube-apiserver",
					"-ojsonpath={.items[*].metadata.name}"}).check(oc)
				g.By("Verify audit rule status through compliancecheck result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-audit-error-alert-exists", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
			} else {
				g.By("Verify audit rule status through compliancecheck result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "FAIL", ok, []string{"compliancecheckresult",
					"ocp4-moderate-audit-error-alert-exists", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
				g.By("Create the prometheusrules for audit.. !!!\n")
				_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", "openshift-kube-apiserver", "-f", prometheusAuditRuleYAML).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				newCheck("expect", asAdmin, withoutNamespace, contain, "audit-errors", ok, []string{"prometheusrules", "-n", "openshift-kube-apiserver",
					"-ojsonpath={.items[*].metadata.name}"}).check(oc)
				defer cleanupObjects(oc, objectTableRef{"prometheusrules", "openshift-kube-apiserver", "audit-errors"})

				g.By("Rerun scan using oc-compliance plugin.. !!!\n")
				_, err1 := OcComplianceCLI().Run("rerun-now").Args("compliancesuite", ssb.name, "-n", ssb.namespace).Output()
				o.Expect(err1).NotTo(o.HaveOccurred())
				newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
					"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
				g.By("Check ComplianceSuite status and result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
					"-o=jsonpath={.status.phase}"}).check(oc)
				subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")
				g.By("Verify audit rule status through compliancecheck result.. !!!\n")
				newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
					"ocp4-moderate-audit-error-alert-exists", "-n", ssb.namespace, "-o=jsonpath={.status}"}).check(oc)
			}
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-47373-Low-47371-Enable TailoredProfiles without extending a Profile and also validate that title and description [Slow]", func() {
			var (
				tprofileDN = tailoredProfileWithoutVarDescription{
					name:         "new-profile-node",
					namespace:    "",
					extends:      "",
					title:        "new profile with node rules from scratch",
					description:  "new profile with node rules",
					enrulename1:  "ocp4-file-groupowner-cni-conf",
					rationale1:   "Node",
					enrulename2:  "ocp4-accounts-restrict-service-account-tokens",
					rationale2:   "None",
					disrulename1: "ocp4-file-groupowner-etcd-data-dir",
					drationale1:  "Node",
					disrulename2: "ocp4-accounts-unique-service-account",
					drationale2:  "None",
					template:     tprofileWithoutVarTemplate,
				}
				tprofileDP = tailoredProfileWithoutVarDescription{
					name:         "new-profile-platform",
					namespace:    "",
					extends:      "",
					title:        "new profile with platform rules from scratch",
					description:  "new profile with platform rules",
					enrulename1:  "ocp4-api-server-admission-control-plugin-alwaysadmit",
					rationale1:   "Platform",
					enrulename2:  "ocp4-accounts-restrict-service-account-tokens",
					rationale2:   "None",
					disrulename1: "ocp4-api-server-admission-control-plugin-alwayspullimages",
					drationale1:  "Platform",
					disrulename2: "ocp4-configure-network-policies",
					drationale2:  "None",
					template:     tprofileWithoutVarTemplate,
				}
				tprofileDNP = tailoredProfileWithoutVarDescription{
					name:         "new-profile-both",
					namespace:    "",
					extends:      "",
					title:        "new profile with both checktype rules from scratch",
					description:  "new profile with both checkType rules",
					enrulename1:  "ocp4-file-groupowner-cni-conf",
					rationale1:   "Node",
					enrulename2:  "ocp4-api-server-admission-control-plugin-alwayspullimages",
					rationale2:   "Platform",
					disrulename1: "ocp4-file-groupowner-etcd-data-dir",
					drationale1:  "Node",
					disrulename2: "ocp4-api-server-admission-control-plugin-alwaysadmit",
					drationale2:  "Platform",
					template:     tprofileWithoutVarTemplate,
				}
				ssbN = scanSettingBindingDescription{
					name:            "tailor-profile-node",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "new-profile-node",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				ssbP = scanSettingBindingDescription{
					name:            "tailor-profile-platform",
					namespace:       "",
					profilekind1:    "TailoredProfile",
					profilename1:    "new-profile-platform",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)
			var errmsg = "Rule 'ocp4-api-server-admission-control-plugin-alwayspullimages' with type 'Platform' didn't match expected type: Node"
			defer func() {
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssbN.name})
				cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssbP.name})
				cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, tprofileDN.name})
				cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, tprofileDP.name})
				cleanupObjects(oc, objectTableRef{"tailoredprofile", subD.namespace, tprofileDNP.name})
			}()

			g.By("Create tailoredprofiles !!!\n")
			tprofileDN.namespace = subD.namespace
			tprofileDP.namespace = subD.namespace
			tprofileDNP.namespace = subD.namespace
			tprofileDN.create(oc, itName, dr)
			tprofileDP.create(oc, itName, dr)
			tprofileDNP.create(oc, itName, dr)
			subD.getTailoredProfileNameandStatus(oc, tprofileDN.name)
			subD.getTailoredProfileNameandStatus(oc, tprofileDP.name)
			newCheck("expect", asAdmin, withoutNamespace, contain, "ERROR", ok, []string{"tailoredprofile", tprofileDNP.name, "-n", tprofileDNP.namespace,
				"-o=jsonpath={.status.state}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, errmsg, ok, []string{"tailoredprofile", tprofileDNP.name, "-n", tprofileDNP.namespace,
				"-o=jsonpath={.status.errorMessage}"}).check(oc)

			errorMsg := []string{"The TailoredProfile \"profile-description\" is invalid: spec.description: Required value", "The TailoredProfile \"profile-title\" is invalid: spec.title: Required value"}
			verifyTailoredProfile(oc, errorMsg, subD.namespace, tprofileWithoutDescriptionYAML)
			verifyTailoredProfile(oc, errorMsg, subD.namespace, tprofileWithoutTitleYAML)

			g.By("Create scansettingbindings !!!\n")
			ssbN.namespace = subD.namespace
			ssbP.namespace = subD.namespace
			ssbN.create(oc, itName, dr)
			ssbP.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssbN.name, ok, []string{"scansettingbinding", "-n", ssbN.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssbP.name, ok, []string{"scansettingbinding", "-n", ssbP.namespace,
				"-o=jsonpath={.items[*].metadata.name}"}).check(oc)

			g.By("Check ComplianceSuite status !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbN.name, "-n", ssbN.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Check complianceSuite name and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssbP.name, "-n", ssbP.namespace, "-o=jsonpath={.status.phase}"}).check(oc)
			g.By("Check complianceSuite name and result.. !!!\n")
			subD.complianceSuiteResult(oc, ssbN.name, "COMPLIANT")
			subD.complianceSuiteResult(oc, ssbP.name, "COMPLIANT")

			g.By("Verify the rule status through compliancecheckresult... !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"new-profile-node-master-file-groupowner-cni-conf", "-n", ssbN.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "MANUAL", ok, []string{"compliancecheckresult",
				"new-profile-node-worker-accounts-restrict-service-account-tokens", "-n", ssbN.namespace, "-o=jsonpath={.status}"}).check(oc)
			newCheck("expect", asAdmin, withoutNamespace, contain, "PASS", ok, []string{"compliancecheckresult",
				"new-profile-platform-api-server-admission-control-plugin-alwaysadmit", "-n", ssbP.namespace, "-o=jsonpath={.status}"}).check(oc)
		})

		// author: pdhamdhe@redhat.com
		g.It("Author:pdhamdhe-Medium-47148-Check file and directory permissions for apiserver audit logs [Slow]", func() {
			var (
				ssb = scanSettingBindingDescription{
					name:            "ocp4-moderate-test",
					namespace:       "",
					profilekind1:    "Profile",
					profilename1:    "ocp4-moderate-node",
					scansettingname: "default",
					template:        scansettingbindingSingleTemplate,
				}
				itName = g.CurrentGinkgoTestDescription().TestText
			)

			defer cleanupObjects(oc, objectTableRef{"scansettingbinding", subD.namespace, ssb.name})

			g.By("Check for default profile 'ocp4-moderate-node' .. !!!\n")
			subD.getProfileName(oc, "ocp4-moderate-node")

			g.By("Create scansettingbinding !!!\n")
			ssb.namespace = subD.namespace
			ssb.create(oc, itName, dr)
			newCheck("expect", asAdmin, withoutNamespace, contain, ssb.name, ok, []string{"scansettingbinding", "-n", ssb.namespace,
				"-o=jsonpath={.items[0].metadata.name}"}).check(oc)
			g.By("Check ComplianceSuite status and result.. !!!\n")
			newCheck("expect", asAdmin, withoutNamespace, contain, "DONE", ok, []string{"compliancesuite", ssb.name, "-n", ssb.namespace,
				"-o=jsonpath={.status.phase}"}).check(oc)
			subD.complianceSuiteResult(oc, ssb.name, "NON-COMPLIANT")

			auditRules := []string{"master-directory-permissions-var-log-kube-audit", "master-directory-permissions-var-log-oauth-audit", "master-directory-permissions-var-log-ocp-audit",
				"master-file-ownership-var-log-kube-audit", "master-file-ownership-var-log-oauth-audit", "master-file-ownership-var-log-ocp-audit", "master-file-permissions-var-log-kube-audit",
				"master-file-permissions-var-log-oauth-audit", "master-file-permissions-var-log-ocp-audit"}

			g.By("Check audit rules status !!!\n")
			getRuleStatus(oc, auditRules, "PASS", ssb.profilename1, ssb.namespace)
			g.By("Get one master node.. !!!\n")
			masterNodeName := getOneMasterNodeName(oc)
			contList1 := []string{"drwx------.  2 root        root ", "kube-apiserver"}
			contList2 := []string{"drwx------.  2 root        root ", "oauth-apiserver"}
			contList3 := []string{"drwx------.  2 root        root ", "openshift-apiserver"}
			contList4 := []string{"-rw-------", "audit.log"}
			checkNodeContents(oc, masterNodeName, contList1, "ls", "-l", "/var/log", "kube-apiserver")
			checkNodeContents(oc, masterNodeName, contList2, "ls", "-l", "/var/log", "oauth-apiserver")
			checkNodeContents(oc, masterNodeName, contList3, "ls", "-l", "/var/log", "openshift-apiserver")
			checkNodeContents(oc, masterNodeName, contList4, "ls", "-l", "/var/log/kube-apiserver", "audit.log")
			checkNodeContents(oc, masterNodeName, contList4, "ls", "-l", "/var/log/oauth-apiserver", "audit.log")
		})
	})
})
