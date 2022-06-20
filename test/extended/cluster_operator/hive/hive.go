package hive

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// [Test Case Naming Rule Add-on]
// - For long duration run such as clusterpool/clusterdeployment, need to add "NonPreRelease"
// - platform specific case, need to add "[platform type]"
// - Add submodule like "ClusterPool", "ClusterDeployment" then we can run all cases for the submodule only

// [Test Resource Naming Rule]
// - Add test case Id into the resource name especially cluster-level resource to avoid name conflict in parallel run
// - Make the resource names in good correlation, the following is the rule example
//  	ClusterPool name:  poolName = pool-<test case Id>
//		Its linked ClusterImageSet name: imageSetName = poolName + "-imageset"
//		ClusterClaim name from the ClusterPool: claimName = poolName + "-claim" (This is to trim "-claim" directly to get the pool name and check if its claimed clusterdeployment delete done when deleting the clusterclaim)

var _ = g.Describe("[sig-hive] Cluster_Operator hive should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("hive-"+getRandomString(), exutil.KubeConfigPath())
		ns           hiveNameSpace
		og           operatorGroup
		sub          subscription
		hc           hiveconfig
		testDataDir  string
		iaasPlatform string
	)
	g.BeforeEach(func() {
		testDataDir = exutil.FixturePath("testdata", "cluster_operator/hive")
		nsTemp := filepath.Join(testDataDir, "namespace.yaml")
		ogTemp := filepath.Join(testDataDir, "operatorgroup.yaml")
		subTemp := filepath.Join(testDataDir, "subscription.yaml")
		hcTemp := filepath.Join(testDataDir, "hiveconfig.yaml")

		ns = hiveNameSpace{
			name:     HiveNamespace,
			template: nsTemp,
		}

		og = operatorGroup{
			name:      "hive-og",
			namespace: HiveNamespace,
			template:  ogTemp,
		}

		sub = subscription{
			name:            "hive-sub",
			namespace:       HiveNamespace,
			channel:         "alpha",
			approval:        "Automatic",
			operatorName:    "hive-operator",
			sourceName:      "community-operators",
			sourceNamespace: "openshift-marketplace",
			startingCSV:     "",
			currentCSV:      "",
			installedCSV:    "",
			template:        subTemp,
		}

		hc = hiveconfig{
			logLevel:        "debug",
			targetNamespace: HiveNamespace,
			template:        hcTemp,
		}

		// get IaaS platform
		iaasPlatform = exutil.CheckPlatform(oc)

		//Create Hive Resources if not exist
		g.By("Create Hive NameSpace...")
		ns.createIfNotExist(oc)
		g.By("Create OperatorGroup...")
		og.createIfNotExist(oc)
		g.By("Create Subscription...")
		sub.createIfNotExist(oc)
		g.By("Create hiveconfig !!!")
		hc.createIfNotExist(oc)

	})

	//author: lwan@redhat.com
	g.It("ConnectedOnly-Author:lwan-Critical-29670-install/uninstall hive operator from OperatorHub", func() {
		g.By("Check Subscription...")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "AllCatalogSourcesHealthy", ok, DefaultTimeout, []string{"sub", sub.name, "-n",
			sub.namespace, "-o=jsonpath={.status.conditions[0].reason}"}).check(oc)

		g.By("Check Hive Operator pods are created !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hive-operator", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=hive-operator",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check Hive Operator pods are in running state !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=hive-operator", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Hive Operator sucessfully installed !!! ")

		g.By("Check hive-clustersync pods are created !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hive-clustersync", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=clustersync",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-clustersync pods are in running state !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=clustersync", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hive-controllers pods are created !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hive-controllers", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=controller-manager",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hive-controllers pods are in running state !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=controller-manager", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		g.By("Check hiveadmission pods are created !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hiveadmission", ok, DefaultTimeout, []string{"pod", "--selector=app=hiveadmission",
			"-n", sub.namespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		g.By("Check hiveadmission pods are in running state !!!")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running Running", ok, DefaultTimeout, []string{"pod", "--selector=app=hiveadmission", "-n",
			sub.namespace, "-o=jsonpath={.items[*].status.phase}"}).check(oc)
		g.By("Hive controllers,clustersync and hiveadmission sucessfully installed !!! ")
	})

	//author: jshu@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "33832"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-Medium-33832-[aws]Hive supports ClusterPool [Serial]", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 33832 is for AWS - skipping test ...")
		}
		testCaseID := "33832"
		poolName := "pool-" + testCaseID
		imageSetName := poolName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP49ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		g.By("Check if ClusterImageSet was created successfully")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, imageSetName, ok, DefaultTimeout, []string{"ClusterImageSet"}).check(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and aws-creds to target namespace for the pool
		g.By("Copy AWS platform credentials...")
		createAWSCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create ClusterPool...")
		poolTemp := filepath.Join(testDataDir, "clusterpool.yaml")
		pool := clusterPool{
			name:           poolName,
			namespace:      oc.Namespace(),
			fake:           "false",
			baseDomain:     AWSBaseDomain,
			imageSetRef:    imageSetName,
			platformType:   "aws",
			credRef:        AWSCreds,
			region:         AWSRegion,
			pullSecretRef:  PullSecret,
			size:           1,
			maxSize:        1,
			runningCount:   0,
			maxConcurrent:  1,
			hibernateAfter: "360m",
			template:       poolTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterPool", oc.Namespace(), poolName})
		pool.create(oc)
		g.By("Check if ClusterPool created successfully and become ready")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, poolName, ok, DefaultTimeout, []string{"ClusterPool", "-n", oc.Namespace()}).check(oc)
		//runningCount is 0 so pool status should be standby: 1, ready: 0
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "1", ok, ClusterInstallTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.status.standby}"}).check(oc)

		g.By("Create ClusterClaim...")
		claimTemp := filepath.Join(testDataDir, "clusterclaim.yaml")
		claimName := poolName + "-claim"
		claim := clusterClaim{
			name:            claimName,
			namespace:       oc.Namespace(),
			clusterPoolName: poolName,
			template:        claimTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterClaim", oc.Namespace(), claimName})
		claim.create(oc)
		g.By("Check if ClusterClaim created successfully and become running")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, claimName, ok, DefaultTimeout, []string{"ClusterClaim", "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Running", ok, ClusterResumeTimeout, []string{"ClusterClaim", "-n", oc.Namespace()}).check(oc)
	})

	//author: jshu@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "25310"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-Medium-25310-[aws]Hive ClusterDeployment Check installed and version [Serial]", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 25310 is for AWS - skipping test ...")
		}
		testCaseID := "25310"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP49ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and aws-creds to target namespace for the pool
		g.By("Copy AWS platform credentials...")
		createAWSCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "aws-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		installConfigSecret := installConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: AWSBaseDomain,
			name2:      cdName,
			region:     AWSRegion,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create ClusterDeployment...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment.yaml")
		cluster := clusterDeployment{
			fake:                "false",
			name:                cdName,
			namespace:           oc.Namespace(),
			baseDomain:          AWSBaseDomain,
			clusterName:         cdName,
			platformType:        "aws",
			credRef:             AWSCreds,
			region:              AWSRegion,
			imageSetRef:         imageSetName,
			installConfigSecret: installConfigSecretName,
			pullSecretRef:       PullSecret,
			template:            clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		cluster.create(oc)

		g.By("Create worker and infra MachinePool ...")
		workermachinepoolAWSTemp := filepath.Join(testDataDir, "machinepool-worker-aws.yaml")
		inframachinepoolAWSTemp := filepath.Join(testDataDir, "machinepool-infra-aws.yaml")
		workermp := machinepool{
			namespace:   oc.Namespace(),
			clusterName: cdName,
			template:    workermachinepoolAWSTemp,
		}
		inframp := machinepool{
			namespace:   oc.Namespace(),
			clusterName: cdName,
			template:    inframachinepoolAWSTemp,
		}

		defer cleanupObjects(oc,
			objectTableRef{"MachinePool", oc.Namespace(), cdName + "-worker"},
			objectTableRef{"MachinePool", oc.Namespace(), cdName + "-infra"},
		)
		workermp.create(oc)
		inframp.create(oc)

		g.By("Check if ClusterDeployment created successfully and become Provisioned")
		e2e.Logf("test OCP-25310")
		//newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, ClusterInstallTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)
		e2e.Logf("test OCP-33374")
		ocpVersion := extractRelfromImg(OCP49ReleaseImage)
		if ocpVersion == "" {
			g.Fail("Case failed because no OCP version extracted from Image")
		}

		if ocpVersion != "" {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, ocpVersion, ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.metadata.labels}"}).check(oc)
		}
		e2e.Logf("test OCP-39747")
		if ocpVersion != "" {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, ocpVersion, ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.installVersion}"}).check(oc)
		}

		g.By("OCP-23165:Hive supports remote Machine Set Management for AWS")
		tmpDir := "/tmp/" + cdName + "-" + getRandomString()
		err := os.MkdirAll(tmpDir, 0777)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpDir)
		getClusterKubeconfig(oc, cdName, oc.Namespace(), tmpDir)
		kubeconfig := tmpDir + "/kubeconfig"
		e2e.Logf("Check worker machinepool .status.replicas = 3")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "3", ok, DefaultTimeout, []string{"MachinePool", cdName + "-worker", "-n", oc.Namespace(), "-o=jsonpath={.status.replicas}"}).check(oc)
		e2e.Logf("Check infra machinepool .status.replicas = 1 ")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "1", ok, DefaultTimeout, []string{"MachinePool", cdName + "-infra", "-n", oc.Namespace(), "-o=jsonpath={.status.replicas}"}).check(oc)
		machinesetsname := getResource(oc, asAdmin, withoutNamespace, "MachinePool", cdName+"-infra", "-n", oc.Namespace(), "-o=jsonpath={.status.machineSets[?(@.replicas==1)].name}")
		o.Expect(machinesetsname).NotTo(o.BeEmpty())
		e2e.Logf("Remote cluster machineset list: %s", machinesetsname)
		e2e.Logf("Check machineset %s created on remote cluster", machinesetsname)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, machinesetsname, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].metadata.name}"}).check(oc)
		e2e.Logf("Check only 1 machineset up")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "1", ok, 5*DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].status.availableReplicas}"}).check(oc)
		e2e.Logf("Check only one machines in Running status")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=infra", "-o=jsonpath={.items[*].status.phase}"}).check(oc)
		e2e.Logf("Patch infra machinepool .spec.replicas to 3")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"MachinePool", cdName + "-infra", "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"replicas": 3}}`}).check(oc)
		machinesetsname = getResource(oc, asAdmin, withoutNamespace, "MachinePool", cdName+"-infra", "-n", oc.Namespace(), "-o=jsonpath={.status.machineSets[?(@.replicas==1)].name}")
		o.Expect(machinesetsname).NotTo(o.BeEmpty())
		e2e.Logf("Remote cluster machineset list: %s", machinesetsname)
		e2e.Logf("Check machineset %s created on remote cluster", machinesetsname)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, machinesetsname, ok, 5*DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].metadata.name}"}).check(oc)
		e2e.Logf("Check machinesets scale up to 3")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "1 1 1", ok, 5*DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].status.availableReplicas}"}).check(oc)
		e2e.Logf("Check 3 machines in Running status")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running Running Running", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=infra", "-o=jsonpath={.items[*].status.phase}"}).check(oc)
		e2e.Logf("Patch infra machinepool .spec.replicas to 2")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"MachinePool", cdName + "-infra", "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"replicas": 2}}`}).check(oc)
		machinesetsname = getResource(oc, asAdmin, withoutNamespace, "MachinePool", cdName+"-infra", "-n", oc.Namespace(), "-o=jsonpath={.status.machineSets[?(@.replicas==1)].name}")
		o.Expect(machinesetsname).NotTo(o.BeEmpty())
		e2e.Logf("Remote cluster machineset list: %s", machinesetsname)
		e2e.Logf("Check machineset %s created on remote cluster", machinesetsname)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, machinesetsname, ok, 5*DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].metadata.name}"}).check(oc)
		e2e.Logf("Check machinesets scale down to 2")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "1 1", ok, 5*DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "MachineSet", "-n", "openshift-machine-api", "-l", "hive.openshift.io/machine-pool=infra", "-o=jsonpath={.items[?(@.spec.replicas==1)].status.availableReplicas}"}).check(oc)
		e2e.Logf("Check 2 machines in Running status")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running Running", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Machine", "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machine-role=infra", "-o=jsonpath={.items[*].status.phase}"}).check(oc)
	})

	//author: jshu@redhat.com
	//OCP-44945, OCP-37528, OCP-37527
	//example: ./bin/extended-platform-tests run all --dry-run|grep "44945"|./bin/extended-platform-tests run --timeout 90m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-Medium-44945-[aws]Hive supports ClusterPool runningCount and hibernateAfter[Serial]", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 44945 is for AWS - skipping test ...")
		}
		testCaseID := "44945"
		poolName := "pool-" + testCaseID
		imageSetName := poolName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP49ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		e2e.Logf("Check if ClusterImageSet was created successfully")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, imageSetName, ok, DefaultTimeout, []string{"ClusterImageSet"}).check(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and aws-creds to target namespace for the pool
		g.By("Copy AWS platform credentials...")
		createAWSCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create ClusterPool...")
		poolTemp := filepath.Join(testDataDir, "clusterpool.yaml")
		pool := clusterPool{
			name:           poolName,
			namespace:      oc.Namespace(),
			fake:           "false",
			baseDomain:     AWSBaseDomain,
			imageSetRef:    imageSetName,
			platformType:   "aws",
			credRef:        AWSCreds,
			region:         AWSRegion,
			pullSecretRef:  PullSecret,
			size:           2,
			maxSize:        2,
			runningCount:   0,
			maxConcurrent:  2,
			hibernateAfter: "10m",
			template:       poolTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterPool", oc.Namespace(), poolName})
		pool.create(oc)
		e2e.Logf("Check if ClusterPool created successfully and become ready")
		//runningCount is 0 so pool status should be standby: 2, ready: 0
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "2", ok, ClusterInstallTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.status.standby}"}).check(oc)

		e2e.Logf("OCP-44945, step 2: check all cluster are in Hibernating status")
		cdListStr := getCDlistfromPool(oc, poolName)
		var cdArray []string
		cdArray = strings.Split(strings.TrimSpace(cdListStr), "\n")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, ClusterResumeTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i]}).check(oc)
		}

		e2e.Logf("OCP-37528, step 3: check hibernateAfter and powerState fields")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.spec.powerState}"}).check(oc)
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "10m", ok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.spec.hibernateAfter}"}).check(oc)
		}

		g.By("OCP-44945, step 5: Patch .spec.runningCount=1...")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"runningCount":1}}`}).check(oc)

		e2e.Logf("OCP-44945, step 6: Check the unclaimed clusters in the pool, CD whose creationTimestamp is the oldest becomes Running")
		var oldestCD, oldestCDTimestamp string
		oldestCDTimestamp = ""
		for i := range cdArray {
			creationTimestamp := getResource(oc, asAdmin, withoutNamespace, "ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.metadata.creationTimestamp}")
			e2e.Logf("CD %d is %s, creationTimestamp is %s", i, cdArray[i], creationTimestamp)
			if strings.Compare(oldestCDTimestamp, "") == 0 || strings.Compare(oldestCDTimestamp, creationTimestamp) > 0 {
				oldestCDTimestamp = creationTimestamp
				oldestCD = cdArray[i]
			}
		}
		e2e.Logf("The CD with the oldest creationTimestamp is %s", oldestCD)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Running", ok, ClusterResumeTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD}).check(oc)

		g.By("OCP-44945, step 7: Patch pool.spec.runningCount=3...")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"runningCount":3}}`}).check(oc)

		e2e.Logf("OCP-44945, step 7: check runningCount=3 but pool size is still 2")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "3", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.spec.runningCount}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "2", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.spec.size}"}).check(oc)

		e2e.Logf("OCP-44945, step 7: All CDs in the pool become Running")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Running", ok, ClusterResumeTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i]}).check(oc)
		}

		g.By("OCP-44945, step 8: Claim a CD from the pool...")
		claimTemp := filepath.Join(testDataDir, "clusterclaim.yaml")
		claimName := poolName + "-claim"
		claim := clusterClaim{
			name:            claimName,
			namespace:       oc.Namespace(),
			clusterPoolName: poolName,
			template:        claimTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterClaim", oc.Namespace(), claimName})
		claim.create(oc)

		e2e.Logf("OCP-44945, step 8: Check the claimed CD is the one whose creationTimestamp is the oldest")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, oldestCD, ok, ClusterResumeTimeout, []string{"ClusterClaim", claimName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("OCP-44945, step 9: Check CD's ClaimedTimestamp is set")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "claimedTimestamp", ok, DefaultTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "-o=jsonpath={.spec.clusterPoolRef}"}).check(oc)

		e2e.Logf("OCP-37528, step 5: Check the claimed CD is in Running status")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Running", ok, DefaultTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "-o=jsonpath={.spec.powerState}"}).check(oc)
		e2e.Logf("OCP-37528, step 6: Check the claimed CD is in Hibernating status due to hibernateAfter=10m")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, ClusterResumeTimeout+5*DefaultTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "-o=jsonpath={.spec.powerState}"}).check(oc)

		g.By("OCP-37527, step 4: patch the CD to Running...")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "--type", "merge", "-p", `{"spec":{"powerState": "Running"}}`}).check(oc)
		e2e.Logf("Wait for CD to be Running")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Running", ok, ClusterResumeTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "-o=jsonpath={.spec.powerState}"}).check(oc)
		e2e.Logf("OCP-37527, step 5: CD becomes Hibernating again due to hibernateAfter=10m")
		//patch makes CD to be Running soon but it needs more time to get back from Hibernation actually so overall timer is ClusterResumeTimeout + hibernateAfter
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, ClusterResumeTimeout+5*DefaultTimeout, []string{"ClusterDeployment", oldestCD, "-n", oldestCD, "-o=jsonpath={.spec.powerState}"}).check(oc)
	})

	//author: jshu@redhat.com lwan@redhat.com
	//OCP-23040, OCP-42113, OCP-34719, OCP-41250, OCP-24000, OCP-25334, OCP-23876
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "23040"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-High-23040-Hive to create SyncSet resource[Serial]", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 23040 is for AWS - skipping test ...")
		}
		testCaseID := "23040"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and aws-creds to target namespace for the pool
		g.By("Copy AWS platform credentials...")
		createAWSCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "aws-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		installConfigSecret := installConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: AWSBaseDomain,
			name2:      cdName,
			region:     AWSRegion,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create ClusterDeployment...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment.yaml")
		cluster := clusterDeployment{
			fake:                "false",
			name:                cdName,
			namespace:           oc.Namespace(),
			baseDomain:          AWSBaseDomain,
			clusterName:         cdName,
			platformType:        "aws",
			credRef:             AWSCreds,
			region:              AWSRegion,
			imageSetRef:         imageSetName,
			installConfigSecret: installConfigSecretName,
			pullSecretRef:       PullSecret,
			template:            clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		cluster.create(oc)

		g.By("Create SyncSet for resource apply......")
		syncSetName := testCaseID + "-syncset1"
		configMapName := testCaseID + "-configmap1"
		configMapNamespace := testCaseID + "-" + getRandomString() + "-hive1"
		resourceMode := "Sync"
		syncTemp := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource := syncSetResource{
			name:        syncSetName,
			namespace:   oc.Namespace(),
			namespace2:  configMapNamespace,
			cdrefname:   cdName,
			cmname:      configMapName,
			cmnamespace: configMapNamespace,
			ramode:      resourceMode,
			template:    syncTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName})
		syncResource.create(oc)
		e2e.Logf("Check ClusterDeployment is installed.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, ClusterInstallTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)

		tmpDir := "/tmp/" + cdName + "-" + getRandomString()
		err := os.MkdirAll(tmpDir, 0777)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpDir)
		getClusterKubeconfig(oc, cdName, oc.Namespace(), tmpDir)
		kubeconfig := tmpDir + "/kubeconfig"

		e2e.Logf("Check if syncSet is created successfully.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetName, ok, DefaultTimeout, []string{"SyncSet", syncSetName, "-n", oc.Namespace()}).check(oc)

		g.By("Test Syncset Resource part......")
		e2e.Logf("OCP-34719, step 3: Check if clustersync and clustersynclease are created successfully.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"ClusterSyncLease", cdName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("OCP-42113: Check if there is STATUS in clustersync tabular output.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "STATUS", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "MESSAGE", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), "-o", "wide"}).check(oc)
		e2e.Logf("OCP-34719, step 4: Check clustersync will record all syncsets first success time.")
		successMessage := "All SyncSets and SelectorSyncSets have been applied to the cluster"
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, successMessage, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Success", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].result}", syncSetName)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "", nok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.firstSuccessTime}"}).check(oc)
		e2e.Logf("OCP-34719, step 5: Check firstSuccessTime won't be changed when there are new syncset created.")
		firstSuccessTime, err := time.Parse(time.RFC3339, getResource(oc, asAdmin, withoutNamespace, "ClusterSync", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.firstSuccessTime}"))
		o.Expect(err).NotTo(o.HaveOccurred())
		syncSetName2 := testCaseID + "-syncset2"
		configMapName2 := testCaseID + "-configmap2"
		configMapNamespace2 := testCaseID + "-" + getRandomString() + "-hive2"
		syncTemp2 := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource2 := syncSetResource{
			name:        syncSetName2,
			namespace:   oc.Namespace(),
			namespace2:  configMapNamespace2,
			cdrefname:   cdName,
			ramode:      resourceMode,
			cmname:      configMapName2,
			cmnamespace: configMapNamespace2,
			template:    syncTemp2,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName2})
		syncResource2.create(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetName2, ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Success", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].result}", syncSetName2)}).check(oc)
		updatedFirstSuccessTime, err := time.Parse(time.RFC3339, getResource(oc, asAdmin, withoutNamespace, "ClusterSync", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.firstSuccessTime}"))
		o.Expect(err).NotTo(o.HaveOccurred())
		if !updatedFirstSuccessTime.Equal(firstSuccessTime) {
			e2e.Failf("firstSuccessTime changed when new SyncSet is created")
		}
		e2e.Logf("Check if configMaps are stored in resourcesToDelete field in ClusterSync CR and they are applied on the target cluster.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName, "-n", configMapNamespace}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete[?(.kind==\"ConfigMap\")].name}", syncSetName)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName2, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName2, "-n", configMapNamespace2}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName2, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete[?(.kind==\"ConfigMap\")].name}", syncSetName2)}).check(oc)
		e2e.Logf("OCP-34719, step 6: Check Resource can be deleted from target cluster via SyncSet when resourceApplyMode is Sync.")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"resourceApplyMode": "Sync"}}`}).check(oc)
		patchYaml := `
spec:
  resources:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: ` + configMapNamespace2
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		e2e.Logf("Check if ConfigMap %s has deleted from target cluster and clusterSync CR.", configMapName2)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName2, nok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", "-n", configMapNamespace2}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapName2, nok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete[?(.kind==\"ConfigMap\")].name}", syncSetName2)}).check(oc)
		e2e.Logf("OCP-41250: Check Resource won't be deleted from target cluster via SyncSet when resourceApplyMode is Upsert.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapNamespace2, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Namespace", configMapNamespace2}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapNamespace2, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete[?(.kind==\"Namespace\")].name}", syncSetName2)}).check(oc)
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"resourceApplyMode": "Upsert"}}`}).check(oc)
		e2e.Logf("Check if resourcesToDelete field is gone in ClusterSync CR.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete}", syncSetName2)}).check(oc)
		e2e.Logf("Delete Namespace CR from SyncSet, check if Namespace is still exit in target cluster")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace(), "--type", "json", "-p", `[{"op": "replace", "path": "/spec/resources", "value":[]}]`}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapNamespace2, nok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.status.syncSets[?(@.name==\"%s\")].resourcesToDelete[?(.kind==\"Namespace\")].name}", syncSetName2)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapNamespace2, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Namespace", configMapNamespace2}).check(oc)
		e2e.Logf("OCP-34719, step 8: Create a bad SyncSet, check if there will be error message in ClusterSync CR.")
		syncSetName3 := testCaseID + "-syncset3"
		configMapName3 := testCaseID + "-configmap3"
		configMapNamespace3 := testCaseID + "-" + getRandomString() + "-hive3"
		syncTemp3 := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource3 := syncSetResource{
			name:        syncSetName3,
			namespace:   oc.Namespace(),
			namespace2:  configMapNamespace3,
			cdrefname:   cdName,
			ramode:      resourceMode,
			cmname:      configMapName3,
			cmnamespace: "namespace-non-exist",
			template:    syncTemp3,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName3})
		syncResource3.create(oc)
		errorMessage := fmt.Sprintf("SyncSet %s is failing", syncSetName3)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetName3, ok, DefaultTimeout, []string{"SyncSet", syncSetName3, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, errorMessage, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), `-o=jsonpath={.status.conditions[?(@.type=="Failed")].message}`}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "True", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace(), `-o=jsonpath={.status.conditions[?(@.type=="Failed")].status}`}).check(oc)

		g.By("OCP-23876: Test Syncset Patch part......")
		e2e.Logf("Create a test ConfigMap CR on target cluster.")
		configMapNameInRemote := testCaseID + "-patch-test"
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("--kubeconfig="+kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("--kubeconfig="+kubeconfig, "configmap", configMapNameInRemote, "--from-literal=foo=bar", "-n", configMapNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, configMapNameInRemote, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "bar", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace, "-o=jsonpath={.data.foo}"}).check(oc)
		syncSetPatchName := testCaseID + "-syncset-patch"
		syncPatchTemp := filepath.Join(testDataDir, "syncset-patch.yaml")
		patchContent := `{ "data": { "foo": "baz-strategic" } }`
		patchType := "strategic"
		syncPatch := syncSetPatch{
			name:        syncSetPatchName,
			namespace:   oc.Namespace(),
			cdrefname:   cdName,
			cmname:      configMapNameInRemote,
			cmnamespace: configMapNamespace,
			pcontent:    patchContent,
			patchType:   patchType,
			template:    syncPatchTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetPatchName})
		syncPatch.create(oc)
		e2e.Logf("Check if SyncSetPatch is created successfully.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetPatchName, ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("Check if SyncSetPatch works well when in strategic patch type.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "strategic", ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.spec.patches[?(@.name==\"%s\")].patchType}", configMapNameInRemote)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "baz-strategic", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace, "-o=jsonpath={.data.foo}"}).check(oc)
		e2e.Logf("Check if SyncSetPatch works well when in merge patch type.")
		patchYaml = `
spec:
  patches:
  - apiVersion: v1
    kind: ConfigMap
    name: ` + configMapNameInRemote + `
    namespace: ` + configMapNamespace + `
    patch: |-
      { "data": { "foo": "baz-merge" } }
    patchType: merge`
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "merge", ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.spec.patches[?(@.name==\"%s\")].patchType}", configMapNameInRemote)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "baz-merge", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace, "-o=jsonpath={.data.foo}"}).check(oc)
		e2e.Logf("Check if SyncSetPatch works well when in json patch type.")
		patchYaml = `
spec:
  patches:
  - apiVersion: v1
    kind: ConfigMap
    name: ` + configMapNameInRemote + `
    namespace: ` + configMapNamespace + `
    patch: |-
      [ { "op": "replace", "path": "/data/foo", "value": "baz-json" } ]
    patchType: json`
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "json", ok, DefaultTimeout, []string{"SyncSet", syncSetPatchName, "-n", oc.Namespace(), fmt.Sprintf("-o=jsonpath={.spec.patches[?(@.name==\"%s\")].patchType}", configMapNameInRemote)}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "baz-json", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapNameInRemote, "-n", configMapNamespace, "-o=jsonpath={.data.foo}"}).check(oc)

		g.By("OCP-25334: Test Syncset SecretReference part......")
		syncSetSecretName := testCaseID + "-syncset-secret"
		syncSecretTemp := filepath.Join(testDataDir, "syncset-secret.yaml")
		sourceName := testCaseID + "-secret"
		e2e.Logf("Create temp Secret in current namespace.")
		defer cleanupObjects(oc, objectTableRef{"Secret", oc.Namespace(), sourceName})
		err = oc.Run("create").Args("secret", "generic", sourceName, "--from-literal=testkey=testvalue", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, sourceName, ok, DefaultTimeout, []string{"Secret", sourceName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("Check Secret won't exit on target cluster before syncset-secret created.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, sourceName, nok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Secret", "-n", configMapNamespace}).check(oc)
		syncSecret := syncSetSecret{
			name:       syncSetSecretName,
			namespace:  oc.Namespace(),
			cdrefname:  cdName,
			sname:      sourceName,
			snamespace: oc.Namespace(),
			tname:      sourceName,
			tnamespace: configMapNamespace,
			template:   syncSecretTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetSecretName})
		syncSecret.create(oc)
		e2e.Logf("Check if syncset-secret is created successfully.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetSecretName, ok, DefaultTimeout, []string{"SyncSet", syncSetSecretName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("Check if the Secret is copied to the target cluster.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, sourceName, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "Secret", sourceName, "-n", configMapNamespace}).check(oc)
	})

	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-High-25447-Hive API support for Azure[Serial]", func() {
		if iaasPlatform != "azure" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 25447 is for Azure - skipping test ...")
		}
		testCaseID := "25447"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and azure-credentials to target namespace for the cluster
		g.By("Copy Azure platform credentials...")
		createAzureCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create Azure Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "azure-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		installConfigSecret := azureInstallConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: AzureBaseDomain,
			name2:      cdName,
			region:     AzureRegion,
			resGroup:   AzureRESGroup,
			azureType:  AzurePublic,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create Azure ClusterDeployment...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment-azure.yaml")
		cluster := azureClusterDeployment{
			fake:                "false",
			name:                cdName,
			namespace:           oc.Namespace(),
			baseDomain:          AzureBaseDomain,
			clusterName:         cdName,
			platformType:        "azure",
			credRef:             AzureCreds,
			region:              AzureRegion,
			resGroup:            AzureRESGroup,
			azureType:           AzurePublic,
			imageSetRef:         imageSetName,
			installConfigSecret: installConfigSecretName,
			pullSecretRef:       PullSecret,
			template:            clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		cluster.create(oc)
		g.By("Check Azure ClusterDeployment installed flag is true")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, ClusterInstallTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)
	})

	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-Medium-33854-Hive supports Azure ClusterPool [Serial]", func() {
		if iaasPlatform != "azure" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 33854 is for Azure - skipping test ...")
		}
		testCaseID := "33854"
		poolName := "pool-" + testCaseID
		imageSetName := poolName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		g.By("Check if ClusterImageSet was created successfully")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, imageSetName, ok, DefaultTimeout, []string{"ClusterImageSet"}).check(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and azure-credentials to target namespace for the cluster
		g.By("Copy Azure platform credentials...")
		createAzureCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create ClusterPool...")
		poolTemp := filepath.Join(testDataDir, "clusterpool-azure.yaml")
		pool := azureClusterPool{
			name:           poolName,
			namespace:      oc.Namespace(),
			fake:           "false",
			baseDomain:     AzureBaseDomain,
			imageSetRef:    imageSetName,
			platformType:   "azure",
			credRef:        AzureCreds,
			region:         AzureRegion,
			resGroup:       AzureRESGroup,
			pullSecretRef:  PullSecret,
			size:           1,
			maxSize:        1,
			runningCount:   0,
			maxConcurrent:  1,
			hibernateAfter: "360m",
			template:       poolTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterPool", oc.Namespace(), poolName})
		pool.create(oc)
		g.By("Check if Azure ClusterPool created successfully and become ready")
		//runningCount is 0 so pool status should be standby: 1, ready: 0
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "1", ok, ClusterInstallTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.status.standby}"}).check(oc)

		g.By("Check if CD is Hibernating")
		cdListStr := getCDlistfromPool(oc, poolName)
		var cdArray []string
		cdArray = strings.Split(strings.TrimSpace(cdListStr), "\n")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, ClusterResumeTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i]}).check(oc)
		}

		g.By("Patch pool.spec.lables.test=test...")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"labels":{"test":"test"}}}`}).check(oc)

		g.By("The existing CD in the pool has no test label")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "test", nok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.metadata.labels}"}).check(oc)
		}

		g.By("The new CD in the pool should have the test label")
		e2e.Logf("Delete the old CD in the pool")
		newCheck("expect", "delete", asAdmin, withoutNamespace, contain, "delete", ok, ClusterUninstallTimeout, []string{"ClusterDeployment", cdArray[0], "-n", cdArray[0]}).check(oc)
		e2e.Logf("Get the CD list from the pool again.")
		cdListStr = getCDlistfromPool(oc, poolName)
		cdArray = strings.Split(strings.TrimSpace(cdListStr), "\n")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "test", ok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.metadata.labels}"}).check(oc)
		}
	})

	//For simplicity, replace --simulate-bootstrap-failure with not copying aws-creds to make install failed
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:jshu-Medium-35990-Hive support limiting install attempt[Serial]", func() {
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 35990 is for AWS - skipping test ...")
		}
		testCaseID := "35990"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		e2e.Logf("Don't copy AWS platform credentials to make install failed.")

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "aws-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		installConfigSecret := installConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: AWSBaseDomain,
			name2:      cdName,
			region:     AWSRegion,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create ClusterDeployment with installAttemptsLimit=0...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment.yaml")
		clusterLimit0 := clusterDeployment{
			fake:                 "false",
			name:                 cdName,
			namespace:            oc.Namespace(),
			baseDomain:           AWSBaseDomain,
			clusterName:          cdName,
			platformType:         "aws",
			credRef:              AWSCreds,
			region:               AWSRegion,
			imageSetRef:          imageSetName,
			installConfigSecret:  installConfigSecretName,
			pullSecretRef:        PullSecret,
			installAttemptsLimit: 0,
			template:             clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		clusterLimit0.create(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "InstallAttemptsLimitReached", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"ProvisionStopped\")].reason}"}).check(oc)
		o.Expect(checkResourceNumber(oc, "pods", cdName)).To(o.Equal(0))
		g.By("Delete the ClusterDeployment and recreate it with installAttemptsLimit=1...")
		cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		clusterLimit1 := clusterDeployment{
			fake:                 "false",
			name:                 cdName,
			namespace:            oc.Namespace(),
			baseDomain:           AWSBaseDomain,
			clusterName:          cdName,
			platformType:         "aws",
			credRef:              AWSCreds,
			region:               AWSRegion,
			imageSetRef:          imageSetName,
			installConfigSecret:  installConfigSecretName,
			pullSecretRef:        PullSecret,
			installAttemptsLimit: 1,
			template:             clusterTemp,
		}
		clusterLimit1.create(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "InstallAttemptsLimitReached", nok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"ProvisionStopped\")].reason}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"pods", "-n", oc.Namespace()}).check(oc)
	})

	//author: lwan@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "41777"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:lwan-High-41777-Hive API support for GCP[Serial]", func() {
		if iaasPlatform != "gcp" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 41777 is for GCP - skipping test ...")
		}
		testCaseID := "41777"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and gcp-credentials to target namespace for the clusterdeployment
		g.By("Copy GCP platform credentials...")
		createGCPCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create GCP Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "gcp-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		projectID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure/cluster", "-o=jsonpath={.status.platformStatus.gcp.projectID}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(projectID).NotTo(o.BeEmpty())
		installConfigSecret := gcpInstallConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: GCPBaseDomain,
			name2:      cdName,
			region:     GCPRegion,
			projectid:  projectID,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create GCP ClusterDeployment...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment-gcp.yaml")
		cluster := gcpClusterDeployment{
			fake:                "false",
			name:                cdName,
			namespace:           oc.Namespace(),
			baseDomain:          GCPBaseDomain,
			clusterName:         cdName,
			platformType:        "gcp",
			credRef:             GCPCreds,
			region:              GCPRegion,
			imageSetRef:         imageSetName,
			installConfigSecret: installConfigSecretName,
			pullSecretRef:       PullSecret,
			template:            clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		cluster.create(oc)
		g.By("Check GCP ClusterDeployment installed flag is true")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, ClusterInstallTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)
	})

	//author: lwan@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "33872"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:lwan-Medium-33872-[gcp]Hive supports ClusterPool [Serial]", func() {
		if iaasPlatform != "gcp" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 33872 is for GCP - skipping test ...")
		}
		testCaseID := "33872"
		poolName := "pool-" + testCaseID
		imageSetName := poolName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		g.By("Check if ClusterImageSet was created successfully")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, imageSetName, ok, DefaultTimeout, []string{"ClusterImageSet"}).check(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and gcp-credentials to target namespace for the pool
		g.By("Copy GCP platform credentials...")
		createGCPCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create ClusterPool...")
		poolTemp := filepath.Join(testDataDir, "clusterpool-gcp.yaml")
		pool := gcpClusterPool{
			name:           poolName,
			namespace:      oc.Namespace(),
			fake:           "false",
			baseDomain:     GCPBaseDomain,
			imageSetRef:    imageSetName,
			platformType:   "gcp",
			credRef:        GCPCreds,
			region:         GCPRegion,
			pullSecretRef:  PullSecret,
			size:           1,
			maxSize:        1,
			runningCount:   0,
			maxConcurrent:  1,
			hibernateAfter: "360m",
			template:       poolTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterPool", oc.Namespace(), poolName})
		pool.create(oc)
		g.By("Check if GCP ClusterPool created successfully and become ready")
		//runningCount is 0 so pool status should be standby: 1, ready: 0
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "1", ok, ClusterInstallTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "-o=jsonpath={.status.standby}"}).check(oc)

		g.By("Check if CD is Hibernating")
		cdListStr := getCDlistfromPool(oc, poolName)
		var cdArray []string
		cdArray = strings.Split(strings.TrimSpace(cdListStr), "\n")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "Hibernating", ok, ClusterResumeTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i]}).check(oc)
		}

		g.By("Patch pool.spec.lables.test=test...")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterPool", poolName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"spec":{"labels":{"test":"test"}}}`}).check(oc)

		g.By("The existing CD in the pool has no test label")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "test", nok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.metadata.labels}"}).check(oc)
		}

		g.By("The new CD in the pool should have the test label")
		e2e.Logf("Delete the old CD in the pool")
		newCheck("expect", "delete", asAdmin, withoutNamespace, contain, "delete", ok, ClusterUninstallTimeout, []string{"ClusterDeployment", cdArray[0], "-n", cdArray[0]}).check(oc)
		e2e.Logf("Get the CD list from the pool again.")
		cdListStr = getCDlistfromPool(oc, poolName)
		cdArray = strings.Split(strings.TrimSpace(cdListStr), "\n")
		for i := range cdArray {
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, "test", ok, DefaultTimeout, []string{"ClusterDeployment", cdArray[i], "-n", cdArray[i], "-o=jsonpath={.metadata.labels}"}).check(oc)
		}
	})

	//author: lwan@redhat.com
	//default duration is 15m for extended-platform-tests and 35m for jenkins job, need to reset for ClusterPool and ClusterDeployment cases
	//example: ./bin/extended-platform-tests run all --dry-run|grep "41499"|./bin/extended-platform-tests run --timeout 60m -f -
	g.It("Longduration-NonPreRelease-ConnectedOnly-Author:lwan-High-41499-High-34404-High-25333-Hive syncset test for paused and multi-modes[Serial]", func() {
		if iaasPlatform != "gcp" {
			g.Skip("IAAS platform is " + iaasPlatform + " while 41499, 34404 or 25333 is for GCP - skipping test ...")
		}
		testCaseID := "41499"
		cdName := "cluster-" + testCaseID
		imageSetName := cdName + "-imageset"
		imageSetTemp := filepath.Join(testDataDir, "clusterimageset.yaml")
		imageSet := clusterImageSet{
			name:         imageSetName,
			releaseImage: OCP410ReleaseImage,
			template:     imageSetTemp,
		}

		g.By("Create ClusterImageSet...")
		defer cleanupObjects(oc, objectTableRef{"ClusterImageSet", "", imageSetName})
		imageSet.create(oc)

		oc.SetupProject()
		//secrets can be accessed by pod in the same namespace, so copy pull-secret and gcp-credentials to target namespace for the clusterdeployment
		g.By("Copy GCP platform credentials...")
		createGCPCreds(oc, oc.Namespace())

		g.By("Copy pull-secret...")
		createPullSecret(oc, oc.Namespace())

		g.By("Create GCP Install-Config Secret...")
		installConfigTemp := filepath.Join(testDataDir, "gcp-install-config.yaml")
		installConfigSecretName := cdName + "-install-config"
		projectID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure/cluster", "-o=jsonpath={.status.platformStatus.gcp.projectID}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(projectID).NotTo(o.BeEmpty())
		installConfigSecret := gcpInstallConfig{
			name1:      installConfigSecretName,
			namespace:  oc.Namespace(),
			baseDomain: GCPBaseDomain,
			name2:      cdName,
			region:     GCPRegion,
			projectid:  projectID,
			template:   installConfigTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"secret", oc.Namespace(), installConfigSecretName})
		installConfigSecret.create(oc)

		g.By("Create GCP ClusterDeployment...")
		clusterTemp := filepath.Join(testDataDir, "clusterdeployment-gcp.yaml")
		cluster := gcpClusterDeployment{
			fake:                "false",
			name:                cdName,
			namespace:           oc.Namespace(),
			baseDomain:          GCPBaseDomain,
			clusterName:         cdName,
			platformType:        "gcp",
			credRef:             GCPCreds,
			region:              GCPRegion,
			imageSetRef:         imageSetName,
			installConfigSecret: installConfigSecretName,
			pullSecretRef:       PullSecret,
			template:            clusterTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"ClusterDeployment", oc.Namespace(), cdName})
		cluster.create(oc)
		g.By("Check GCP ClusterDeployment installed flag is true")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "true", ok, ClusterInstallTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.spec.installed}"}).check(oc)

		tmpDir := "/tmp/" + cdName + "-" + getRandomString()
		err = os.MkdirAll(tmpDir, 0777)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpDir)
		getClusterKubeconfig(oc, cdName, oc.Namespace(), tmpDir)
		kubeconfig := tmpDir + "/kubeconfig"

		g.By("OCP-41499: Add condition in ClusterDeployment status for paused syncset")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace()}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "False", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"SyncSetFailed\")].status}"}).check(oc)
		e2e.Logf("Add \"hive.openshift.io/syncset-pause\" annotation in ClusterDeployment, and delete ClusterSync CR")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"metadata": {"annotations": {"hive.openshift.io/syncset-pause": "true"}}}`}).check(oc)
		newCheck("expect", "delete", asAdmin, withoutNamespace, contain, "delete", ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace()}).check(oc)
		e2e.Logf("Check ClusterDeployment condition=SyncSetFailed")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "True", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"SyncSetFailed\")].status}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "SyncSetPaused", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"SyncSetFailed\")].reason}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "SyncSet is paused. ClusterSync will not be created", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"SyncSetFailed\")].message}"}).check(oc)
		e2e.Logf("Check ClusterSync won't be created.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, nok, DefaultTimeout, []string{"ClusterSync", "-n", oc.Namespace()}).check(oc)
		e2e.Logf("Remove annotation, check ClusterSync will be created again.")
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "--type", "merge", "-p", `{"metadata": {"annotations": {"hive.openshift.io/syncset-pause": "false"}}}`}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "False", ok, DefaultTimeout, []string{"ClusterDeployment", cdName, "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"SyncSetFailed\")].status}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, cdName, ok, DefaultTimeout, []string{"ClusterSync", cdName, "-n", oc.Namespace()}).check(oc)

		g.By("OCP-34404: Hive adds muti-modes for syncset to handle applying resources too large")
		e2e.Logf("Create SyncSet with default applyBehavior.")
		syncSetName := testCaseID + "-syncset-1"
		configMapName := testCaseID + "-configmap-1"
		configMapNamespace := testCaseID + "-" + getRandomString() + "-hive-1"
		resourceMode := "Sync"
		syncTemp := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource := syncSetResource{
			name:        syncSetName,
			namespace:   oc.Namespace(),
			namespace2:  configMapNamespace,
			cdrefname:   cdName,
			cmname:      configMapName,
			cmnamespace: configMapNamespace,
			ramode:      resourceMode,
			template:    syncTemp,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName})
		syncResource.create(oc)
		e2e.Logf("Check ConfigMap is created on target cluster and have a last-applied-config annotation.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName, "-n", configMapNamespace, "-o=jsonpath={.data}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "kubectl.kubernetes.io/last-applied-configuration", ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName, "-n", configMapNamespace, "-o=jsonpath={.metadata.annotations}"}).check(oc)
		e2e.Logf("Patch syncset resource.")
		patchYaml := `
spec:
  resources:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: ` + configMapNamespace + `
  - apiVersion: v1
    data:
      foo1: bar1
    kind: ConfigMap
    metadata:
      name: ` + configMapName + `
      namespace: ` + configMapNamespace
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		e2e.Logf("Check data field in ConfigMap on target cluster should update.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo1":"bar1"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName, "-n", configMapNamespace, "-o=jsonpath={.data}"}).check(oc)

		e2e.Logf("Create SyncSet with applyBehavior=CreateOnly.")
		syncSetName2 := testCaseID + "-syncset-2"
		configMapName2 := testCaseID + "-configmap-2"
		configMapNamespace2 := testCaseID + "-" + getRandomString() + "-hive-2"
		applyBehavior := "CreateOnly"
		syncTemp2 := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource2 := syncSetResource{
			name:          syncSetName2,
			namespace:     oc.Namespace(),
			namespace2:    configMapNamespace2,
			cdrefname:     cdName,
			cmname:        configMapName2,
			cmnamespace:   configMapNamespace2,
			ramode:        resourceMode,
			applybehavior: applyBehavior,
			template:      syncTemp2,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName2})
		syncResource2.create(oc)
		e2e.Logf("Check ConfigMap is created on target cluster and should not have the last-applied-config annotation.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName2, "-n", configMapNamespace2, "-o=jsonpath={.data}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "kubectl.kubernetes.io/last-applied-configuration", nok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName2, "-n", configMapNamespace2, "-o=jsonpath={.metadata.annotations}"}).check(oc)
		e2e.Logf("Patch syncset resource.")
		patchYaml = `
spec:
  resources:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: ` + configMapNamespace2 + `
  - apiVersion: v1
    data:
      foo1: bar1
    kind: ConfigMap
    metadata:
      name: ` + configMapName2 + `
      namespace: ` + configMapNamespace2
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName2, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		e2e.Logf("Check data field in ConfigMap on target cluster should not update.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName2, "-n", configMapNamespace2, "-o=jsonpath={.data}"}).check(oc)

		e2e.Logf("Create SyncSet with applyBehavior=CreateOrUpdate.")
		syncSetName3 := testCaseID + "-syncset-3"
		configMapName3 := testCaseID + "-configmap-3"
		configMapNamespace3 := testCaseID + "-" + getRandomString() + "-hive-3"
		applyBehavior = "CreateOrUpdate"
		syncTemp3 := filepath.Join(testDataDir, "syncset-resource.yaml")
		syncResource3 := syncSetResource{
			name:          syncSetName3,
			namespace:     oc.Namespace(),
			namespace2:    configMapNamespace3,
			cdrefname:     cdName,
			cmname:        configMapName3,
			cmnamespace:   configMapNamespace3,
			ramode:        resourceMode,
			applybehavior: applyBehavior,
			template:      syncTemp3,
		}
		defer cleanupObjects(oc, objectTableRef{"SyncSet", oc.Namespace(), syncSetName3})
		syncResource3.create(oc)
		e2e.Logf("Check ConfigMap is created on target cluster and should not have the last-applied-config annotation.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName3, "-n", configMapNamespace3, "-o=jsonpath={.data}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "kubectl.kubernetes.io/last-applied-configuration", nok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName3, "-n", configMapNamespace3, "-o=jsonpath={.metadata.annotations}"}).check(oc)
		e2e.Logf("Patch syncset resource.")
		patchYaml = `
spec:
  resources:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: ` + configMapNamespace3 + `
  - apiVersion: v1
    data:
      foo2: bar2
    kind: ConfigMap
    metadata:
      name: ` + configMapName3 + `
      namespace: ` + configMapNamespace3
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName3, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		e2e.Logf("Check data field in ConfigMap on target cluster should update and contain both foo and foo2.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar","foo2":"bar2"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName3, "-n", configMapNamespace3, "-o=jsonpath={.data}"}).check(oc)
		e2e.Logf("Patch syncset resource.")
		patchYaml = `
spec:
  resources:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: ` + configMapNamespace3 + `
  - apiVersion: v1
    data:
      foo: bar-test
      foo3: bar3
    kind: ConfigMap
    metadata:
      name: ` + configMapName3 + `
      namespace: ` + configMapNamespace3
		newCheck("expect", "patch", asAdmin, withoutNamespace, contain, "patched", ok, DefaultTimeout, []string{"SyncSet", syncSetName3, "-n", oc.Namespace(), "--type", "merge", "-p", patchYaml}).check(oc)
		e2e.Logf("Check data field in ConfigMap on target cluster should update, patch foo and add foo3.")
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, `{"foo":"bar-test","foo2":"bar2","foo3":"bar3"}`, ok, DefaultTimeout, []string{"--kubeconfig=" + kubeconfig, "ConfigMap", configMapName3, "-n", configMapNamespace3, "-o=jsonpath={.data}"}).check(oc)

		g.By("OCP-25333: Changing apiGroup for ClusterRoleBinding in SyncSet doesn't delete the CRB")
		e2e.Logf("Create SyncSet with invalid apiGroup in resource CR.")
		syncSetName4 := testCaseID + "-syncset-4"
		syncsetYaml := `
apiVersion: hive.openshift.io/v1
kind: SyncSet
metadata:
  name: ` + syncSetName4 + `
spec:
  clusterDeploymentRefs:
  - name: ` + cdName + `
  - namespace: ` + oc.Namespace() + `
  resourceApplyMode: Sync
  resources:
  - apiVersion: authorization.openshift.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: dedicated-admins-cluster
    subjects:
    - kind: Group
      name: dedicated-admins
    - kind: Group
      name: system:serviceaccounts:dedicated-admin
    roleRef:
      name: dedicated-admins-cluster`
		var filename = testCaseID + "-syncset-crb.yaml"
		err = ioutil.WriteFile(filename, []byte(syncsetYaml), 0644)
		defer os.Remove(filename)
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filename).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(`Invalid value: "authorization.openshift.io/v1": must use kubernetes group for this resource kind`))
		e2e.Logf("oc create syncset failed, this is expected.")
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, syncSetName4, nok, DefaultTimeout, []string{"SyncSet", "-n", oc.Namespace()}).check(oc)
	})
})
