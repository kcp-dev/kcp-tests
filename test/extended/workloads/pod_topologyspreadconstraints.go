package workloads

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-scheduling] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Critical-33836-Critical-33845-High-33767-Check Validate Pod with only one TopologySpreadConstraint topologyKey node [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podSelectorT := filepath.Join(buildPruningBaseDir, "pod_nodeselect.yaml")
		podSinglePtsT := filepath.Join(buildPruningBaseDir, "pod_singlepts.yaml")
		podSinglePtsNodeSelectorT := filepath.Join(buildPruningBaseDir, "pod_singlepts_nodeselect.yaml")

		var kz = "zone"
		var kn = "node"

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz, "zoneA")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn, "node1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz, "zoneA")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn, "node2")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz, "zoneB")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn, "node3")
		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn)

		g.By("Test for case OCP-33836")
		g.By("create new namespace")
		oc.SetupProject()

		pod1 := podNodeSelector{
			name:       "mypod1-33836",
			namespace:  oc.Namespace(),
			nodeKey:    "node",
			nodeValue:  "node1",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod2 := podNodeSelector{
			name:       "mypod2-33836",
			namespace:  oc.Namespace(),
			nodeKey:    "node",
			nodeValue:  "node2",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod3 := podSinglePts{
			name:       "mypod3-33836",
			namespace:  oc.Namespace(),
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			template:   podSinglePtsT,
		}
		g.By("Trying to launch a pod with a label to node1")
		pod1.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod1nodename := pod1.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod1nodename)

		g.By("Trying to launch a pod with a label to node2")
		pod2.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2nodename := pod2.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod2nodename)

		g.By("In this case, the new coming pod only scheduler to node3")
		pod3.createPodSinglePts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod3nodename := pod3.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod3nodename)

		g.By("Test for case OCP-33845")
		g.By("create new namespace")
		oc.SetupProject()

		pod338451 := podNodeSelector{
			name:       "mypod1-33845",
			namespace:  oc.Namespace(),
			nodeKey:    "node",
			nodeValue:  "node1",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338452 := podNodeSelector{
			name:       "mypod2-33845",
			namespace:  oc.Namespace(),
			nodeKey:    "node",
			nodeValue:  "node2",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338453 := podNodeSelector{
			name:       "mypod3-33845",
			namespace:  oc.Namespace(),
			nodeKey:    "node",
			nodeValue:  "node3",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338454 := podSinglePts{
			name:       "mypod4-33845",
			namespace:  oc.Namespace(),
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "zone",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    2,
			template:   podSinglePtsT,
		}

		g.By("Trying to launch a pod with a label to node1")
		pod338451.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338451nodename := pod338451.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod338451nodename)

		g.By("Trying to launch a pod with a label to node2")
		pod338452.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338452nodename := pod338452.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod338452nodename)

		g.By("Trying to launch a pod with a label to node3")
		pod338453.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338453nodename := pod338453.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod338453nodename)

		g.By("In this case, the new coming pod could scheduler to node1-node3")
		pod338454.createPodSinglePts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338454nodename := pod338454.getPodNodeName(oc)
		o.Expect(pod338454nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name, nodeList.Items[2].Name}))

		g.By("Test for case OCP-33767")
		g.By("create new namespace")
		oc.SetupProject()

		pod337671 := podSinglePtsNodeSelector{
			name:       "mypod1-33767",
			namespace:  oc.Namespace(),
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			nodeKey:    "zone",
			nodeValue:  "zoneA",
			template:   podSinglePtsNodeSelectorT,
		}

		pod337672 := podSinglePtsNodeSelector{
			name:       "mypod2-33767",
			namespace:  oc.Namespace(),
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "node",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			nodeKey:    "zone",
			nodeValue:  "zoneA",
			template:   podSinglePtsNodeSelectorT,
		}

		g.By("Trying to launch a pod with a label to zoneA node1 or node2")
		pod337671.createPodSinglePtsNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod337671nodename := pod337671.getPodNodeName(oc)
		o.Expect(pod337671nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name}))

		g.By("In this case, the new coming pod could scheduler to zoneA,but not same node with pod337671")
		pod337672.createPodSinglePtsNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod337672nodename := pod337672.getPodNodeName(oc)
		o.Expect(pod337672nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name}))
		o.Expect(pod337672nodename).NotTo(o.Equal(pod337671nodename))
	})
	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-34019-Check validate TopologySpreadConstraints ignored the node without the label [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		deploySinglePtsT := filepath.Join(buildPruningBaseDir, "deploy_single_pts.yaml")
		var ktz = "testzone"
		var ktn = "testnode"

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		expectNodeList := []string{nodeList.Items[0].Name, nodeList.Items[1].Name}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, ktz, "testzoneA")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, ktn, "testnode1")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, ktz, "testzoneB")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, ktn, "testnode2")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, ktz, "testzoneC")

		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, ktz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, ktn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, ktz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, ktn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, ktz)

		g.By("Test for case OCP-34019")
		g.By("create new namespace")
		oc.SetupProject()

		deploy34019 := deploySinglePts{
			dName:      "d34019",
			namespace:  oc.Namespace(),
			replicaNum: 2,
			labelKey:   "foo",
			labelValue: "bar",
			ptsKeyName: "testnode",
			ptsPolicy:  "DoNotSchedule",
			skewNum:    1,
			template:   deploySinglePtsT,
		}

		g.By("Trying to launch a deploy with a label to node with testnode label")
		deploy34019.createDeploySinglePts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Geting the node list where pods running")
		podNodeList := getPodNodeListByLabel(oc, deploy34019.namespace, deploy34019.labelKey)

		g.By("Checking all the pods scheduled to node with testnode label")
		for _, nodeName := range podNodeList {
			o.Expect(nodeName).Should(o.BeElementOf(expectNodeList))
		}

		g.By("Scale up the deploy")
		_, err = oc.WithoutNamespace().Run("scale").Args("deploy", "-n", deploy34019.namespace, deploy34019.dName, "--replicas="+strconv.Itoa(5)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the deploy scale up")
		err = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.WithoutNamespace().Run("get").Args("-n", deploy34019.namespace, "deploy", deploy34019.dName, "-o=jsonpath={.status.replicas}").Output()
			if err != nil {
				e2e.Logf("Fail to get podnum: %s, error: %s and try again", deploy34019.dName, err)
				return false, nil
			}
			if output == "5" {
				e2e.Logf("Get expected pod num: %s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("depoly %s is not scale up to 5", deploy34019.dName))

		g.By("Geting the node list where pods running")
		podNodeList = getPodNodeListByLabel(oc, deploy34019.namespace, deploy34019.labelKey)

		g.By("Checking all the pods scheduled to node with testnode label")
		for _, nodeName := range podNodeList {
			o.Expect(nodeName).Should(o.BeElementOf(expectNodeList))
		}
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-33824-Check Validate TopologySpreadConstraint with podAffinity and podAntiAffinity [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podSelectorT := filepath.Join(buildPruningBaseDir, "pod_nodeselect.yaml")
		podAffinityPreferredPtsT := filepath.Join(buildPruningBaseDir, "pod_singlepts_prefer.yaml")
		podAffinityRequiredPtsT := filepath.Join(buildPruningBaseDir, "pod_singlepts_required.yaml")

		var kz = "zone33824"
		var kn = "node33824"

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz, "zone33824A")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn, "node338241")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz, "zone33824B")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn, "node338242")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz, "zone33824B")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn, "node338243")
		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn)

		g.By("Test for case OCP-33824")
		g.By("create new namespace")
		oc.SetupProject()

		pod338241 := podNodeSelector{
			name:       "mypod1-33824",
			namespace:  oc.Namespace(),
			nodeKey:    "node33824",
			nodeValue:  "node338241",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		pod338242 := podNodeSelector{
			name:       "mypod2-33824",
			namespace:  oc.Namespace(),
			nodeKey:    "node33824",
			nodeValue:  "node338243",
			labelKey:   "security",
			labelValue: "S1",
			template:   podSelectorT,
		}

		pod338243 := podAffinityPreferredPts{
			name:           "mypod3-33824",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "zone33824",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			affinityMethod: "podAntiAffinity",
			weigthNum:      100,
			keyName:        "security",
			valueName:      "S1",
			operatorName:   "In",
			template:       podAffinityPreferredPtsT,
		}

		pod338244 := podAffinityRequiredPts{
			name:           "mypod4-33824",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "zone33824",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			affinityMethod: "podAffinity",
			keyName:        "security",
			valueName:      "S1",
			operatorName:   "In",
			template:       podAffinityRequiredPtsT,
		}

		pod338245 := podAffinityRequiredPts{
			name:           "mypod5-33824",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "zone33824",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			affinityMethod: "podAffinity",
			keyName:        "security",
			valueName:      "S1",
			operatorName:   "In",
			template:       podAffinityRequiredPtsT,
		}

		g.By("Trying to launch a pod with a label to the frist node")
		pod338241.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338241nodename := pod338241.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod338241nodename)

		g.By("Trying to launch a pod with a label to the third node")
		pod338242.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338242nodename := pod338242.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod338242nodename)

		g.By("Trying to launch a pod with podAntiAffinity to the second node")
		pod338243.createPodAffinityPreferredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338243nodename := pod338243.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod338243nodename)

		g.By("Trying to launch a pod with podAffinity to the third node")
		pod338244.createPodAffinityRequiredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338244nodename := pod338244.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[2].Name, pod338244nodename)

		g.By("Trying to launch a pod with podAffinity to the third node again, but will failed to scheduler")
		pod338245.createPodAffinityRequiredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod338245describestatus := describePod(oc, pod338245.namespace, pod338245.name)
		o.Expect(pod338245describestatus).Should(o.ContainSubstring("FailedScheduling"))
	})

	// author: knarra@redhat.com
	g.It("Author:knarra-High-34017-TopologySpreadConstraints do not work on cross namespaced pods [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podSelectorT := filepath.Join(buildPruningBaseDir, "pod_nodeselect.yaml")
		podNodeAffinityRequiredPtsT := filepath.Join(buildPruningBaseDir, "pod_pts_nodeaffinity_required.yaml")

		var kz = "zone34017"
		var kn = "node34017"

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz, "zone34017A")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn, "node340171")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz, "zone34017A")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn, "node340172")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz, "zone34017B")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn, "node340173")

		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn)

		g.By("Test for case OCP-34017")
		g.By("create new namespace")
		oc.SetupProject()

		pod1 := podNodeSelector{
			name:       "pod1-34017",
			namespace:  oc.Namespace(),
			nodeKey:    "node34017",
			nodeValue:  "node340171",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}
		pod2 := podNodeSelector{
			name:       "pod2-34017",
			namespace:  oc.Namespace(),
			nodeKey:    "node34017",
			nodeValue:  "node340172",
			labelKey:   "foo",
			labelValue: "bar",
			template:   podSelectorT,
		}

		g.By("Trying to launch a pod with a label to node1")
		pod1.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod1nodename := pod1.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[0].Name, pod1nodename)

		g.By("Trying to launch a pod with a label to node2")
		pod2.createPodNodeSelector(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2nodename := pod2.getPodNodeName(oc)
		e2e.ExpectEqual(nodeList.Items[1].Name, pod2nodename)

		g.By("create second namespace")
		oc.SetupProject()

		pod3 := podNodeAffinityRequiredPts{
			name:           "pod3-34017",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "zone34017",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			ptsKey2Name:    "node34017",
			ptsPolicy2:     "DoNotSchedule",
			skewNum2:       1,
			affinityMethod: "nodeAffinity",
			keyName:        "zone34017",
			operatorName:   "NotIn",
			valueName:      "zone34017B",
			template:       podNodeAffinityRequiredPtsT,
		}

		g.By("Trying to launch a pod with nodeAffinity not to second node")
		pod3.createpodNodeAffinityRequiredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod3nodename := pod3.getPodNodeName(oc)
		e2e.ExpectNotEqual(nodeList.Items[2].Name, pod3nodename)
	})

	// author: knarra@redhat.com
	g.It("Author:knarra-High-34014-Validate TopologySpreadConstraints with NodeAffinity [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podSingleNodeAffinityRequiredPtsT := filepath.Join(buildPruningBaseDir, "pod_single_nodeaffinity_required.yaml")

		var kz = "zone34014"
		var kn = "node34014"

		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			e2e.Logf("Unexpected error occurred: %v", err)
		}
		g.By("Apply dedicated Key for this test on the 3 nodes.")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz, "zone34014A")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn, "node340141")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz, "zone34014A")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn, "node340142")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz, "zone34014B")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn, "node340143")

		g.By("Remove dedicated Key for this test on the 3 nodes.")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, kn)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kz)
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[2].Name, kn)

		g.By("Test for case OCP-34014")
		g.By("create new namespace")
		oc.SetupProject()

		pod340141 := podSingleNodeAffinityRequiredPts{
			name:           "pod1-34014",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "node34014",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			affinityMethod: "nodeAffinity",
			keyName:        "zone34014",
			operatorName:   "NotIn",
			valueName:      "zone34014B",
			template:       podSingleNodeAffinityRequiredPtsT,
		}

		pod340142 := podSingleNodeAffinityRequiredPts{
			name:           "pod2-34014",
			namespace:      oc.Namespace(),
			labelKey:       "foo",
			labelValue:     "bar",
			ptsKeyName:     "node34014",
			ptsPolicy:      "DoNotSchedule",
			skewNum:        1,
			affinityMethod: "nodeAffinity",
			keyName:        "zone34014",
			operatorName:   "NotIn",
			valueName:      "zone34014B",
			template:       podSingleNodeAffinityRequiredPtsT,
		}

		g.By("Trying to launch a pod with a label to node1 or node2")
		pod340141.createpodSingleNodeAffinityRequiredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod340141nodename := pod340141.getPodNodeName(oc)
		o.Expect(pod340141nodename).Should(o.BeElementOf([]string{nodeList.Items[0].Name, nodeList.Items[1].Name}))

		g.By("Validate pod340141 nodename is not equal to pod340142 nodename")
		pod340142.createpodSingleNodeAffinityRequiredPts(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod340142nodename := pod340142.getPodNodeName(oc)
		e2e.ExpectNotEqual(pod340141nodename, pod340142nodename)
	})

})
