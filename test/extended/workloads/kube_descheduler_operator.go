package workloads

import (
	"fmt"
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-scheduling] Workloads The Descheduler Operator automates pod evictions using different profiles", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())
	var kubeNamespace = "openshift-kube-descheduler-operator"

	buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
	operatorGroupT := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
	subscriptionT := filepath.Join(buildPruningBaseDir, "subscription.yaml")
	deschedulerT := filepath.Join(buildPruningBaseDir, "kubedescheduler.yaml")

	sub := subscription{
		name:        "cluster-kube-descheduler-operator",
		namespace:   kubeNamespace,
		channelName: "4.11",
		opsrcName:   "qe-app-registry",
		sourceName:  "openshift-marketplace",
		template:    subscriptionT,
	}

	og := operatorgroup{
		name:      "openshift-kube-descheduler-operator",
		namespace: kubeNamespace,
		template:  operatorGroupT,
	}

	deschu := kubedescheduler{
		namespace:        kubeNamespace,
		interSeconds:     60,
		imageInfo:        "registry.redhat.io/openshift4/ose-descheduler:v4.11.0",
		logLevel:         "Normal",
		operatorLogLevel: "Normal",
		profile1:         "AffinityAndTaints",
		profile2:         "TopologyAndDuplicates",
		profile3:         "LifecycleAndUtilization",
		template:         deschedulerT,
	}

	// author: knarra@redhat.com
	g.It("Author:knarra-High-21205-Low-36584-Install descheduler operator via a deployment & verify it should not violate PDB [Slow] [Disruptive] [Flaky]", func() {
		deploydpT := filepath.Join(buildPruningBaseDir, "deploy_duplicatepodsrs.yaml")

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

		g.By("Create the descheduler namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()

		patch := `[{"op":"add", "path":"/metadata/labels/openshift.io~1cluster-monitoring", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", kubeNamespace, "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the operatorgroup")
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer og.deleteOperatorGroup(oc)

		g.By("Create the subscription")
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer sub.deleteSubscription(oc)

		g.By("Wait for the descheduler operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
			e2e.Logf("Kubedescheduler operator runnnig now\n")
		}

		g.By("Create descheduler cluster")
		deschu.createKubeDescheduler(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "descheduler", "-n", kubeNamespace, "-o=jsonpath={.status.observedGeneration}").Output()
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("2", output); matched {
				e2e.Logf("deploy is up:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("observed Generation is not expected"))

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name")
		podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Validate all profiles have been enabled checking descheduler cluster logs")
		profileDetails := []string{"duplicates.go", "lownodeutilization.go", "pod_antiaffinity.go", "node_affinity.go", "node_taint.go", "toomanyrestarts.go", "pod_lifetime.go", "topologyspreadconstraint.go"}
		for _, pd := range profileDetails {
			checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(pd))
		}

		// Check descheduler_build_info from prometheus
		checkDeschedulerMetrics(oc, "DeschedulerVersion", "descheduler_build_info")

		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		testdp := deployduplicatepods{
			dName:      "d36584",
			namespace:  oc.Namespace(),
			replicaNum: 12,
			template:   deploydpT,
		}

		// Test for descheduler not violating PDB

		g.By("Cordon node1")
		err = oc.AsAdmin().Run("adm").Args("cordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()

		g.By("Create the test deploy")
		testdp.createDuplicatePods(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should running on node")
		if ok := waitForAvailableRsRunning(oc, "rs", testdp.dName, testdp.namespace, "12"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		// Create PDB
		g.By("Create PDB")
		err = oc.AsAdmin().Run("create").Args("poddisruptionbudget", testdp.dName, "--selector=app=d36584", "--min-available=11").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("delete").Args("pdb", testdp.dName).Execute()

		g.By("Set descheduler mode to Automatic")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/mode", "value":"Automatic"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubedescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patchYamlToRestore := `[{"op": "replace", "path": "/spec/mode", "value":"Predictive"}]`

		defer func() {
			e2e.Logf("Restoring descheduler mode back to Predictive")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the kubedescheduler run well")
			checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")
		}()

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name after mode is set")
		podName, err = oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Uncordon node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Error evicting pod"`)+".*"+regexp.QuoteMeta(`Cannot evict pod as it would violate the pod's disruption budget.`))

		g.By("Delete PDB")
		err = oc.AsAdmin().Run("delete").Args("pdb", testdp.dName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Delete rs from the namespace
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("rs", testdp.dName, "-n", testdp.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Make sure all the pods assoicated with replicaset are deleted")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", testdp.namespace).Output()
			if err != nil {
				e2e.Logf("Fail to get is, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("No resources found", output); matched {
				e2e.Logf("All pods associated with replicaset have been deleted:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "All pods associated with replicaset have been not deleted")

		// Test for PDB with --max-unavailable=1
		g.By("cordon node1")
		err = oc.AsAdmin().Run("adm").Args("cordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()

		g.By("Create the test deploy")
		testdp.createDuplicatePods(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should running on node")
		if ok := waitForAvailableRsRunning(oc, "rs", testdp.dName, testdp.namespace, "12"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		// Create PDB for --max-unavailable=1
		g.By("Create PDB for --max-unavailable=1")
		err = oc.AsAdmin().Run("create").Args("poddisruptionbudget", testdp.dName, "--selector=app=d36584", "--max-unavailable=1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("delete").Args("pdb", testdp.dName).Execute()

		g.By("Uncordon node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Error evicting pod"`)+".*"+regexp.QuoteMeta(`Cannot evict pod as it would violate the pod's disruption budget.`))

		// Collect PDB  metrics from prometheus
		g.By("Checking PDB metrics from prometheus")
		checkDeschedulerMetrics(oc, `"result":"error"`, "descheduler_pods_evicted")
		checkDeschedulerMetrics(oc, "RemoveDuplicatePods", "descheduler_pods_evicted")

	})

	// author: knarra@redhat.com
	g.It("Author:knarra-High-37463-High-40055-Descheduler-Validate AffinityAndTaints and TopologyAndDuplicates profile [Disruptive][Slow] [Flaky]", func() {
		deployT := filepath.Join(buildPruningBaseDir, "deploy_nodeaffinity.yaml")
		deploynT := filepath.Join(buildPruningBaseDir, "deploy_nodetaint.yaml")
		deploypT := filepath.Join(buildPruningBaseDir, "deploy_interpodantiaffinity.yaml")
		deploydpT := filepath.Join(buildPruningBaseDir, "deploy_duplicatepods.yaml")
		deployptsT := filepath.Join(buildPruningBaseDir, "deploy_podTopologySpread.yaml")
		deploydT := filepath.Join(buildPruningBaseDir, "deploy_demopod.yaml")

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		testd := deploynodeaffinity{
			dName:          "d37463",
			namespace:      oc.Namespace(),
			replicaNum:     1,
			labelKey:       "app37463",
			labelValue:     "d37463",
			affinityKey:    "e2e-az-NorthSouth",
			operatorPolicy: "In",
			affinityValue1: "e2e-az-North",
			affinityValue2: "e2e-az-South",
			template:       deployT,
		}

		testd2 := deploynodetaint{
			dName:     "d374631",
			namespace: oc.Namespace(),
			template:  deploynT,
		}

		testd3 := deployinterpodantiaffinity{
			dName:            "d3746321",
			namespace:        oc.Namespace(),
			replicaNum:       1,
			podAffinityKey:   "key3746321",
			operatorPolicy:   "In",
			podAffinityValue: "value3746321",
			template:         deploypT,
		}

		testd4 := deployinterpodantiaffinity{
			dName:            "d374632",
			namespace:        oc.Namespace(),
			replicaNum:       6,
			podAffinityKey:   "key374632",
			operatorPolicy:   "In",
			podAffinityValue: "value374632",
			template:         deploypT,
		}

		testdp := deployduplicatepods{
			dName:      "d40055",
			namespace:  oc.Namespace(),
			replicaNum: 6,
			template:   deploydpT,
		}

		testpts := deploypodtopologyspread{
			dName:     "d400551",
			namespace: oc.Namespace(),
			template:  deployptsT,
		}

		testpts1 := deploypodtopologyspread{
			dName:     "d400552",
			namespace: oc.Namespace(),
			template:  deploydT,
		}

		testpts2 := deploypodtopologyspread{
			dName:     "d4005521",
			namespace: oc.Namespace(),
			template:  deploydT,
		}

		testpts3 := deploypodtopologyspread{
			dName:     "d4005522",
			namespace: oc.Namespace(),
			template:  deploydT,
		}

		g.By("Create the descheduler namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()

		patch := `[{"op":"add", "path":"/metadata/labels/openshift.io~1cluster-monitoring", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", kubeNamespace, "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the operatorgroup")
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer og.deleteOperatorGroup(oc)

		g.By("Create the subscription")
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer sub.deleteSubscription(oc)

		g.By("Wait for the descheduler operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
			e2e.Logf("Kubedescheduler operator runnnig now\n")
		}

		g.By("Create descheduler cluster")
		deschu.createKubeDescheduler(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "descheduler", "-n", kubeNamespace, "-o=jsonpath={.status.observedGeneration}").Output()
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("2", output); matched {
				e2e.Logf("deploy is up:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("observed Generation is not expected"))

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name")
		podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set descheduler mode to Automatic")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/mode", "value":"Automatic"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubedescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patchYamlToRestore := `[{"op": "replace", "path": "/spec/mode", "value":"Predictive"}]`

		defer func() {
			e2e.Logf("Restoring descheduler mode back to Predictive")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the kubedescheduler run well")
			checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")
		}()

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name after mode is set")
		podName, err = oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test for RemovePodsViolatingNodeAffinity

		g.By("Create the test deploy")
		testd.createDeployNodeAffinity(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should be pending")
		if ok := checkPodsStatusByLabel(oc, oc.Namespace(), testd.labelKey+"="+testd.labelValue, "Pending"); ok {
			e2e.Logf("All pods are in Pending status\n")
		}

		g.By("label the node1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth", "e2e-az-North")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")

		g.By("Check all the pods should running on node1")
		waitErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", testd.namespace).Output()

			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "Running") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "not all the pods are running on node1")

		testPodName, err := oc.AsAdmin().Run("get").Args("pods", "-l", testd.labelKey+"="+testd.labelValue, "-n", testd.namespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pod37463nodename := getPodNodeName(oc, testd.namespace, testPodName)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod37463nodename)

		g.By("Remove the label from node1 and label node2 ")
		e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "e2e-az-NorthSouth")
		g.By("label removed from node1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth", "e2e-az-South")
		g.By("label Added to node2")

		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth")

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="NodeAffinity"`))

		// Collect NodeAffinity  metrics from prometheus
		g.By("Checking NodeAffinity metrics from prometheus")
		checkDeschedulerMetrics(oc, "NodeAffinity", "descheduler_pods_evicted")

		// Test for RemovePodsViolatingNodeTaints

		g.By("Create the test2 deploy")
		testd2.createDeployNodeTaint(oc)
		pod374631nodename := getPodNodeName(oc, testd2.namespace, "d374631")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Add taint to the node")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod374631nodename, "dedicated=special-user:NoSchedule").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod374631nodename, "dedicated-").Execute()

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="NodeTaint"`))

		// Collect NodeTaint  metrics from prometheus
		g.By("Checking NodeTaint metrics from prometheus")
		checkDeschedulerMetrics(oc, "NodeTaint", "descheduler_pods_evicted")

		// Test for RemovePodsViolatingInterPodAntiAffinity

		g.By("Create the test3 deploy")
		testd3.createDeployInterPodAntiAffinity(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get pod name")
		podNameIpa, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=d3746321", "-n", testd3.namespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the test4 deploy")
		testd4.createDeployInterPodAntiAffinity(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Add label to the pod")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("pod", podNameIpa, "key374632=value374632", "-n", testd3.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("pod", podNameIpa, "key374632-", "-n", testd3.namespace).Execute()

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="InterPodAntiAffinity"`))

		// Collect InterPodAntiAffinity  metrics from prometheus
		g.By("Checking InterPodAntiAffinity metrics from prometheus")
		checkDeschedulerMetrics(oc, "InterPodAntiAffinity", "descheduler_pods_evicted")

		// Perform cleanup so that next case will be executed
		g.By("Performing cleanup to execute 40055")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("deployment", testd.dName, "-n", testd.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "e2e-az-NorthSouth")

		oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod374631nodename, "dedicated-").Execute()

		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("deployment", testd4.dName, "-n", testd4.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test for RemoveDuplicates

		g.By("Cordon node1")
		err = oc.AsAdmin().Run("adm").Args("cordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()

		g.By("Create the test deploy")
		testdp.createDuplicatePods(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should running on node")
		if ok := waitForAvailableRsRunning(oc, "deploy", testdp.dName, testdp.namespace, "6"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		g.By("Uncordon node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="RemoveDuplicatePods"`))

		// Collect RemoveDuplicatePods metrics from prometheus
		g.By("Checking RemoveDuplicatePods metrics from prometheus")
		checkDeschedulerMetrics(oc, "RemoveDuplicatePods", "descheduler_pods_evicted")

		// Delete deployment from the namespace
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("deployment", testdp.dName, "-n", testdp.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test for PodTopologySpreadConstriant

		g.By("Cordon all nodes in the cluster")
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		defer func() {
			for _, v := range node {
				oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", fmt.Sprintf("%s", v)).Execute()
			}
		}()

		for _, v := range node {
			err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("cordon", fmt.Sprintf("%s", v)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Label Node1 & Node2")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "ocp40055-zone", "ocp40055zoneA")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "ocp40055-zone")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "ocp40055-zone", "ocp40055zoneB")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "ocp40055-zone")

		g.By("Uncordon Node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create three pods on node1")
		testpts.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating first demo pod")
		testpts1.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating second demo pod")
		testpts2.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("cordon Node1, uncordon Node2")
		err = oc.AsAdmin().Run("adm").Args("cordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[1].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create one pod on node2")
		testpts3.createPodTopologySpread(oc)

		g.By("uncordon Node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="PodTopologySpread"`))

		// Collect PodTopologySpread metrics from prometheus
		g.By("Checking PodTopologySpread metrics from prometheus")
		checkDeschedulerMetrics(oc, "PodTopologySpread", "descheduler_pods_evicted")

	})

	// author: knarra@redhat.com
	g.It("Longduration-NonPreRelease-Author:knarra-High-43287-High-43283-Descheduler-Descheduler operator should verify config does not conflict with scheduler and SoftTopologyAndDuplicates profile [Disruptive][Slow]", func() {
		deploysptT := filepath.Join(buildPruningBaseDir, "deploy_softPodTopologySpread.yaml")
		deploysdT := filepath.Join(buildPruningBaseDir, "deploy_softdemopod.yaml")

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

		deschu = kubedescheduler{
			namespace:        kubeNamespace,
			interSeconds:     60,
			imageInfo:        "registry.redhat.io/openshift4/ose-descheduler:v4.11.0",
			logLevel:         "Normal",
			operatorLogLevel: "Normal",
			profile1:         "EvictPodsWithPVC",
			profile2:         "SoftTopologyAndDuplicates",
			profile3:         "LifecycleAndUtilization",
			template:         deschedulerT,
		}

		g.By("Create the descheduler namespace")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patch := `[{"op":"add", "path":"/metadata/labels/openshift.io~1cluster-monitoring", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", kubeNamespace, "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the operatorgroup")
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer og.deleteOperatorGroup(oc)

		g.By("Create the subscription")
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer sub.deleteSubscription(oc)

		g.By("Wait for the descheduler operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
			e2e.Logf("Kubedescheduler operator runnnig now\n")
		}

		g.By("Create descheduler cluster")
		deschu.createKubeDescheduler(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "descheduler", "-n", kubeNamespace, "-o=jsonpath={.status.observedGeneration}").Output()
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("2", output); matched {
				e2e.Logf("deploy is up:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("observed Generation is not expected"))

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Set descheduler mode to Automatic")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/mode", "value":"Automatic"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubedescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patchYamlToRestore := `[{"op": "replace", "path": "/spec/mode", "value":"Predictive"}]`

		defer func() {
			e2e.Logf("Restoring descheduler mode back to Predictive")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the kubedescheduler run well")
			checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")
		}()

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name after mode is set")
		podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test for SoftTopologyAndDuplicates
		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		testspt := deploypodtopologyspread{
			dName:     "d432831",
			namespace: oc.Namespace(),
			template:  deploysptT,
		}

		testspt1 := deploypodtopologyspread{
			dName:     "d432832",
			namespace: oc.Namespace(),
			template:  deploysdT,
		}

		testspt2 := deploypodtopologyspread{
			dName:     "d432833",
			namespace: oc.Namespace(),
			template:  deploysdT,
		}

		testspt3 := deploypodtopologyspread{
			dName:     "d432834",
			namespace: oc.Namespace(),
			template:  deploysdT,
		}

		g.By("Cordon all nodes in the cluster")
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		defer func() {
			for _, v := range node {
				oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", fmt.Sprintf("%s", v)).Execute()
			}
		}()

		for _, v := range node {
			err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("cordon", fmt.Sprintf("%s", v)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Label Node1 & Node2")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "ocp43283-zone", "ocp43283zoneA")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "ocp43283-zone")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "ocp43283-zone", "ocp43283zoneB")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "ocp43283-zone")

		g.By("Uncordon Node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create three pods on node1")
		testspt.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating first demo pod")
		testspt1.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating second demo pod")
		testspt2.createPodTopologySpread(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("cordon Node1, uncordon Node2")
		err = oc.AsAdmin().Run("adm").Args("cordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[1].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create one pod on node2")
		testspt3.createPodTopologySpread(oc)

		g.By("uncordon Node1")
		err = oc.AsAdmin().Run("adm").Args("uncordon", nodeList.Items[0].Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see evict logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(`reason="PodTopologySpread"`))

		// Collect SoftTopologyAndDuplicate metrics from prometheus
		g.By("Checking SoftTopologyAndDuplicate metrics from prometheus")
		checkDeschedulerMetrics(oc, "PodTopologySpread", "descheduler_pods_evicted")

		// Test for config does not conflict with scheduler
		patch = `[{"op":"add", "path":"/spec/profile", "value":"HighNodeUtilization"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			patch = `[{"op":"add", "path":"/spec/profile"}]`
			oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patch).Execute()
			g.By("Check the kube-scheduler operator should be in Progressing")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "Clusteroperator kube-scheduler is not Progressing")

			g.By("Wait for the KubeScheduler operator to recover")
			err = wait.Poll(30*time.Second, 400*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "Clusteroperator kube-scheduler is not recovered to normal")

		}()

		g.By("Check the kube-scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the KubeScheduler operator to recover")
		err = wait.Poll(30*time.Second, 400*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get descheduler operator pod name")
		operatorPodName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "name=descheduler-operator", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see config error logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", operatorPodName, regexp.QuoteMeta(`"enabling Descheduler LowNodeUtilization with Scheduler HighNodeUtilization may cause an eviction/scheduling hot loop"`))

	})

	// author: knarra@redhat.com
	g.It("Author:knarra-Medium-43277-High-50941-Descheduler-Validate Predictive and Automatic mode for descheduler [Flaky][Slow]", func() {
		deschedulerpT := filepath.Join(buildPruningBaseDir, "kubedescheduler_podlifetime.yaml")

		_, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)

		deschu = kubedescheduler{
			namespace:        kubeNamespace,
			interSeconds:     60,
			imageInfo:        "registry.redhat.io/openshift4/ose-descheduler:v4.11.0",
			logLevel:         "Normal",
			operatorLogLevel: "Normal",
			profile1:         "EvictPodsWithPVC",
			profile2:         "SoftTopologyAndDuplicates",
			profile3:         "LifecycleAndUtilization",
			template:         deschedulerpT,
		}

		g.By("Create the descheduler namespace")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patch := `[{"op":"add", "path":"/metadata/labels/openshift.io~1cluster-monitoring", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", kubeNamespace, "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the operatorgroup")
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer og.deleteOperatorGroup(oc)

		g.By("Create the subscription")
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer sub.deleteSubscription(oc)

		g.By("Wait for the descheduler operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "descheduler-operator", kubeNamespace, "1"); ok {
			e2e.Logf("Kubedescheduler operator runnnig now\n")
		}

		g.By("Create descheduler cluster")
		deschu.createKubeDescheduler(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("KubeDescheduler", "--all", "-n", kubeNamespace).Execute()

		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "descheduler", "-n", kubeNamespace, "-o=jsonpath={.status.observedGeneration}").Output()
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("2", output); matched {
				e2e.Logf("deploy is up:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("observed Generation is not expected"))

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name")
		podName, err := oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test for podLifetime
		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		err = oc.Run("create").Args("deployment", "ocp43277", "--image", "quay.io/openshifttest/hello-openshift@sha256:1e70b596c05f46425c39add70bf749177d78c1e98b2893df4e5ae3883c2ffb5e").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should running")
		if ok := waitForAvailableRsRunning(oc, "deployment", "ocp43277", oc.Namespace(), "1"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		g.By("Check the descheduler deploy logs, should see config error logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod in dry run mode" `)+".*"+regexp.QuoteMeta(oc.Namespace())+".*"+regexp.QuoteMeta(`reason="PodLifeTime"`))

		// Collect PodLifetime metrics from prometheus
		g.By("Checking PodLifetime metrics from prometheus")
		checkDeschedulerMetrics(oc, "PodLifeTime", "descheduler_pods_evicted")

		// Test descheduler automatic mode
		g.By("Set descheduler mode to Automatic")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/mode", "value":"Automatic"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubedescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		patchYamlToRestore := `[{"op": "replace", "path": "/spec/mode", "value":"Predictive"}]`

		defer func() {
			e2e.Logf("Restoring descheduler mode back to Predictive")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "-n", kubeNamespace, "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the kubedescheduler run well")
			checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")
		}()

		err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "descheduler", "-n", kubeNamespace, "-o=jsonpath={.status.observedGeneration}").Output()
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("3", output); matched {
				e2e.Logf("deploy is up:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "observed Generation is not expected")

		g.By("Check the kubedescheduler run well")
		checkAvailable(oc, "deploy", "descheduler", kubeNamespace, "1")

		g.By("Get descheduler cluster pod name after mode is set")
		podName, err = oc.AsAdmin().Run("get").Args("pods", "-l", "app=descheduler", "-n", kubeNamespace, "-o=jsonpath={.items..metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the descheduler deploy logs, should see config error logs")
		checkLogsFromRs(oc, kubeNamespace, "pod", podName, regexp.QuoteMeta(`"Evicted pod"`)+".*"+regexp.QuoteMeta(oc.Namespace())+".*"+regexp.QuoteMeta(`reason="PodLifeTime"`))

		// Collect PodLifetime metrics from prometheus
		g.By("Checking PodLifetime metrics from prometheus")
		checkDeschedulerMetrics(oc, "PodLifeTime", "descheduler_pods_evicted")
	})
})
