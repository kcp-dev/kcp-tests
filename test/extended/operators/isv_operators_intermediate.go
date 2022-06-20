package operators

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const DEFAULT_STATUS_QUERY = "-o=jsonpath={.status.conditions[0].type}"
const DEFAULT_EXPECTED_BEHAVIOR = "Ready"

var _ = g.Describe("[sig-operators] ISV_Operators [Suite:openshift/isv]", func() {
	var oc = exutil.NewCLI("operators", exutil.KubeConfigPath())

	g.It("ConnectedOnly-Author:bandrade-Medium-23955-[Intermediate] Operator amq-streams should work properly", func() {

		kafkaCR := "Kafka"
		kafkaClusterName := "my-cluster"
		kafkaPackageName := "amq-streams"
		kafkaFile := "kafka.yaml"
		namespace := "amq-streams"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(kafkaPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, kafkaFile, oc)
		CheckCR(currentPackage, kafkaCR, kafkaClusterName, DEFAULT_STATUS_QUERY, DEFAULT_EXPECTED_BEHAVIOR, oc)
		RemoveCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:kuiwang-Medium-25880-[Intermediate] Operator portworx-certified should work properly", func() {

		packageName := "portworx-certified"
		crdName := "storagenode"
		crName := "storagenode-example"
		crFile := "portworx-snode-cr.yaml"
		namespace := "portworx-certified"
		jsonPath := "-o=json"
		expectedMsg := "storagenode-example"

		defer RemoveNamespace(namespace, oc)
		g.By("install operator")
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)
		g.By("create CR")
		CreateFromYAML(currentPackage, crFile, oc)
		g.By("check CR")
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)
		g.By("remvoe operator")
		RemoveCR(currentPackage, crdName, crName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:kuiwang-Medium-25414-[Intermediate] Operator couchbase-enterprise-certified should work properly", func() {

		packageName := "couchbase-enterprise-certified"
		crdName := "CouchbaseCluster"
		crName := "cb-example"
		crFile := "couchbase-enterprise-cr.yaml"
		namespace := "couchbase-enterprise-certified"
		jsonPath := "-o=json"
		expectedMsg := "Running"

		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, crFile, oc)
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)
		RemoveCR(currentPackage, crdName, crName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-26057-[Intermediate] Operator jaeger-product should work properly", func() {

		jaegerPackageName := "jaeger-product"
		jaegerCR := "Jaeger"
		jaegerCRClusterName := "jaeger-all-in-one-inmemory"
		namespace := "openshift-operators"

		currentPackage := CreateSubscriptionSpecificNamespace(jaegerPackageName, oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, "jaeger.yaml", oc)
		CheckCR(currentPackage, jaegerCR, jaegerCRClusterName,
			"-o=jsonpath={.status.phase}", "Running", oc)
		RemoveCR(currentPackage, jaegerCR, jaegerCRClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-26945-[Intermediate] Operator keycloak-operator should work properly", func() {

		keycloakCR := "Keycloak"
		keycloakCRName := "example-keycloak"
		keycloakPackageName := "keycloak-operator"
		keycloakFile := "keycloak-cr.yaml"
		namespace := "keycloak"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(keycloakPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, keycloakFile, oc)
		CheckCR(currentPackage, keycloakCR, keycloakCRName, "-o=jsonpath={.status.ready}", "true", oc)
		RemoveCR(currentPackage, keycloakCR, keycloakCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:tbuskey-Medium-26944-[Intermediate] Operator spark-gcp should work properly", func() {
		packageName := "spark-gcp" // spark-operator in OperatorHub
		namespace := "spark-gcp"
		crFile := "spark-gcp-sparkapplication-cr.yaml"
		sparkgcpCR := "sparkapp"
		sparkgcpName := "spark-pi"
		crPodname := "spark-pi-driver"
		jsonPath := "-o=jsonpath={.status.applicationState.state}"
		expectedMsg := "COMPLETE"
		searchMsg := "Pi is roughly "
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, crFile, oc)
		CheckCR(currentPackage, sparkgcpCR, sparkgcpName, jsonPath, expectedMsg, oc)
		msg, err := oc.WithoutNamespace().AsAdmin().Run("logs").Args(crPodname, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(searchMsg))
		e2e.Logf("STEP PASS %v", searchMsg)
		RemoveCR(currentPackage, sparkgcpCR, sparkgcpName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)
	})

	// author: tbuskey@redhat.com OCPQE-2169-Intermediate
	g.It("ConnectedOnly-Author:tbuskey-Medium-27313-[Intermediate] Operator radanalytics-spark should work properly", func() {
		var (
			itName                   = g.CurrentGinkgoTestDescription().TestText
			buildPruningBaseDir      = exutil.FixturePath("testdata", "olm")
			ogTemplate               = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
			subFile                  = filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
			buildIntermediateBaseDir = exutil.FixturePath("testdata", "operators")
			radSparkCluster          = filepath.Join(buildIntermediateBaseDir, "radanalytics-spark-sparkcluster-cr.yaml")
			radSparkApplication      = filepath.Join(buildIntermediateBaseDir, "radanalytics-spark-sparkapplication-cr.yaml")

			appPodName   = ""
			csvName      = ""
			caseID       = CaseIDISVOperators["radanalytics-spark"]
			err          error
			msg          string
			packageName  = "radanalytics-spark" // spark-operator in OperatorHub
			pkgAvailable = true
			re           = regexp.MustCompile(`Pi is roughly 3\.[0-9]+`)
			searchMsg    = "Pi is roughly "
			waitErr      error
		)

		oc.SetupProject()

		var (
			og = operatorGroupDescription{
				name:      packageName + "-" + caseID,
				namespace: oc.Namespace(),
				template:  ogTemplate,
			}
			sub = subscriptionDescription{
				subName:                packageName + "-" + caseID,
				namespace:              oc.Namespace(),
				catalogSourceName:      "community-operators",
				catalogSourceNamespace: "openshift-marketplace",
				ipApproval:             "Automatic",
				channel:                "alpha",
				operatorPackage:        packageName,
				startingCSV:            "sparkoperator.v1.1.0",
				installedCSV:           "sparkoperator.v1.1.0",
				singleNamespace:        true,
				template:               subFile,
			}
			clusterCrd = customResourceDescription{
				name:      "my-sparkcluster",
				namespace: oc.Namespace(),
				typename:  "SparkCluster",
				template:  radSparkCluster,
			}
			appCrd = customResourceDescription{
				name:      "my-spark-app",
				namespace: oc.Namespace(),
				typename:  "SparkApplication",
				template:  radSparkApplication,
			}
		)

		dr := make(describerResrouce)
		itName = g.CurrentGinkgoTestDescription().TestText
		dr.addIr(itName)

		g.By("Check " + packageName + " availability")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-n", "openshift-marketplace", packageName, "--no-headers").Output()
		if err != nil {
			pkgAvailable = false
			e2e.Logf("!!! Could not query packagemanifest for %v operator, probably will fail: %v %v\n", packageName, err, msg)
		}
		if !strings.Contains(msg, "Community Operators") {
			e2e.Logf("!!! Could not find %v operator in Community Operators, probably will fail: %v %v\n", packageName, err, msg)
			pkgAvailable = false
		}
		if pkgAvailable {
			e2e.Logf("Package %v is available", packageName)
		} else {
			e2e.Logf("\n\nFAIL: %v was not available. \n", packageName)
			o.Expect(pkgAvailable)
		}

		g.By("Create og")
		og.createwithCheck(oc, itName, dr)

		g.By("Create sub")
		sub.createWithoutCheck(oc, itName, dr)

		g.By("Wait for csv")
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", oc.Namespace(), "--no-headers").Output()
			if strings.Contains(msg, sub.installedCSV) {
				e2e.Logf("found csv %v", msg)
				msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", sub.installedCSV, "-n", oc.Namespace(), "-o=jsonpath={.status.phase}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(msg, "Succeeded") {
					csvName = sub.installedCSV
					return true, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("expected csv %s not Succeeded", sub.installedCSV))
		o.Expect(csvName).NotTo(o.BeEmpty())

		g.By("Create Sparkcluster")
		clusterCrd.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "ready", ok, []string{clusterCrd.typename, clusterCrd.name, "-n", oc.Namespace(), "-o=jsonpath={.status.state}"}).check(oc)

		g.By("Create SparkApplication")
		appCrd.create(oc, itName, dr)
		newCheck("expect", asAdmin, withoutNamespace, compare, "ready", ok, []string{appCrd.typename, appCrd.name, "-n", oc.Namespace(), "-o=jsonpath={.status.state}"}).check(oc)

		g.By("Wait for application pod to appear")
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace(), "--no-headers").Output()
			if strings.Contains(msg, "my-spark-app") {
				if strings.Contains(msg, "driver") {
					appPodName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace(), "--selector=spark-role=driver", "-o=jsonpath={.items..metadata.name}").Output()
					e2e.Logf("found app pod %v", appPodName)
					return true, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "my-spark-app pod not not found")
		o.Expect(appPodName).NotTo(o.BeEmpty())

		g.By("Wait for SparkApplication to finish")
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace(), appPodName, "--no-headers").Output()
			e2e.Logf("%v", msg)
			if strings.Contains(msg, "Completed") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("expected pod %s not Completed", "SparkApplication"))
		o.Expect(msg).NotTo(o.BeEmpty())

		g.By("Check the answer in logs")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args(appPodName, "-n", oc.Namespace()).Output()
		o.Expect(waitErr).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(searchMsg))

		g.By("DONE")
		e2e.Logf("%v\n\n", re.FindString(msg))

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-26056-[Intermediate] Operator strimzi-kafka-operator should work properly", func() {

		strimziCR := "Kafka"
		strimziClusterName := "my-cluster"
		strimziPackageName := "strimzi-kafka-operator"
		strimziFile := "strimzi-cr.yaml"
		namespace := "strimzi"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(strimziPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, strimziFile, oc)
		CheckCR(currentPackage, strimziCR, strimziClusterName, DEFAULT_STATUS_QUERY, DEFAULT_EXPECTED_BEHAVIOR, oc)
		RemoveCR(currentPackage, strimziCR, strimziClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-27311-[Intermediate] Operator resource-locker-operator should work properly", func() {

		packageName := "resource-locker-operator"
		crdName := "ResourceLocker"
		crName := "locked-configmap-foo-bar-configmap"
		crFile := "resourcelocker-cr.yaml"
		jsonPath := "-o=jsonpath={.status.conditions..reason}"
		expectedMsg := "LastReconcileCycleSucceded"
		rolesFile := "resourcelocker-role.yaml"
		sa := "resource-locker-test-sa"

		g.By("install operator")
		currentPackage := CreateSubscription(packageName, oc, INSTALLPLAN_AUTOMATIC_MODE)
		defer RemoveOperatorDependencies(currentPackage, oc, false)

		defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("sa", sa, "-n", currentPackage.Namespace).Output()

		_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", sa, "-n", currentPackage.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)

		g.By("create CR")
		defer RemoveFromYAML(currentPackage, rolesFile, oc)
		CreateFromYAML(currentPackage, rolesFile, oc)
		CreateFromYAML(currentPackage, crFile, oc)

		g.By("check CR")
		CheckCR(currentPackage, crdName, crName, jsonPath, expectedMsg, oc)

		g.By("remove CR")
		RemoveCR(currentPackage, crdName, crName, oc)

	})

	g.It("ConnectedOnly-Author:kuiwang-Medium-25885-[Intermediate] Operator storageos2 should work properly", func() {

		packageName := "storageos2"
		crdName1 := "StorageOSCluster"
		crdName2 := "storageosupgrade"
		crName1 := "storageoscluster-example"
		crName2 := "storageosupgrade-example"
		crFile1 := "storageoscluster-cr.yaml"
		crFile2 := "storageosupgrade-cr.yaml"
		secretFile := "storageos-secret.yaml"
		namespace := "storageos"
		jsonPath := "-o=json"
		expectedMsg1 := "storageoscluster-example"
		expectedMsg2 := "storageosupgrade-example"

		defer RemoveNamespace(namespace, oc)
		g.By("create secret")
		buildPruningBaseDirsecret := exutil.FixturePath("testdata", "operators")
		secret := filepath.Join(buildPruningBaseDirsecret, secretFile)
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", secret).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", "storageos-api-isv", "-n", "openshift-operators").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("install operator")
		currentPackage := CreateSubscriptionSpecificNamespace(packageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		g.By("check deployment of operator")
		CheckDeployment(currentPackage, oc)
		g.By("create CR1")
		CreateFromYAML(currentPackage, crFile1, oc)
		g.By("create CR2")
		CreateFromYAML(currentPackage, crFile2, oc)
		g.By("check CR1")
		CheckCR(currentPackage, crdName1, crName1, jsonPath, expectedMsg1, oc)
		g.By("check CR2")
		CheckCR(currentPackage, crdName2, crName2, jsonPath, expectedMsg2, oc)
		g.By("remvoe operator")
		RemoveCR(currentPackage, crdName1, crName1, oc)
		RemoveCR(currentPackage, crdName2, crName2, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-27312-[Intermediate] Operator argocd-operator should work properly", func() {
		argoCR := "ArgoCD"
		argoCRName := "example-argocd"
		argoPackageName := "argocd-operator"
		argoFile := "argocd-cr.yaml"
		namespace := "argocd"
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(argoPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, argoFile, oc)
		CheckCR(currentPackage, argoCR, argoCRName, "-o=jsonpath={.status.phase}", "Available", oc)
		RemoveCR(currentPackage, argoCR, argoCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:bandrade-Medium-27301-[Intermediate] Operator kiali-ossm should work properly", func() {
		kialiCR := "Kiali"
		kialiCRName := "kiali-27301"
		kialiPackageName := "kiali-ossm"
		kialiFile := "kiali-cr.yaml"
		namespace := "openshift-operators"
		kialiNamespace := "istio-system"
		CreateNamespaceWithoutPrefix(kialiNamespace, oc)
		defer RemoveNamespace(kialiNamespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(kialiPackageName, oc, false, false, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, kialiFile, oc)
		CheckCR(currentPackage, kialiCR, kialiCRName, "-o=jsonpath={.status.conditions..reason}", "Running", oc)
		RemoveCR(currentPackage, kialiCR, kialiCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})

	g.It("ConnectedOnly-Author:scolange-Medium-27782-[Intermediate] Operator crunchy ossm should work properly", func() {
		crunchyCR := "Pgcluster"
		crunchyCRName := "example"
		crunchyPackageName := "crunchy-postgres-operator"
		crunchyFile := "crunchy-cr.yaml"
		namespace := "crunchy"
		//CreateNamespaceWithoutPrefix(namespace, oc)
		defer RemoveNamespace(namespace, oc)
		currentPackage := CreateSubscriptionSpecificNamespace(crunchyPackageName, oc, true, true, namespace, INSTALLPLAN_AUTOMATIC_MODE)
		CheckDeployment(currentPackage, oc)
		CreateFromYAML(currentPackage, crunchyFile, oc)
		CheckCR(currentPackage, crunchyCR, crunchyCRName, "-o=jsonpath={.status.state}", "pgcluster Processed", oc)
		RemoveCR(currentPackage, crunchyCR, crunchyCRName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)

	})
})

//the method is to create CR with yaml file in the namespace of the installed operator
func CreateFromYAML(p Packagemanifest, filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "operators")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cr, "-n", p.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to create CR with yaml file in the namespace of the installed operator
func RemoveFromYAML(p Packagemanifest, filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "operators")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", cr, "-n", p.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to delete CR of kind CRName with name instanceName in the namespace of the installed operator
func RemoveCR(p Packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args(CRName, instanceName, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring("deleted"))
}

//the method is to check if the CR is expected.
//the content is got by jsonpath.
//if it is expected, nothing happen
//if it is not expected, it will delete CR and the resource of the installed operator, for example sub, csv and possible ns
func CheckCR(p Packagemanifest, CRName string, instanceName string, jsonPath string, expectedMessage string, oc *exutil.CLI) {

	poolErr := wait.Poll(10*time.Second, 600*time.Second, func() (bool, error) {
		msg, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args(CRName, instanceName, "-n", p.Namespace, jsonPath).Output()
		e2e.Logf(msg)
		if strings.Contains(msg, expectedMessage) {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		e2e.Logf("Could not get CR " + CRName + " for " + p.CsvVersion)
		RemoveCR(p, CRName, instanceName, oc)
		RemoveOperatorDependencies(p, oc, false)
		g.Fail("Could not get CR " + CRName + " for " + p.CsvVersion)
	}
}
