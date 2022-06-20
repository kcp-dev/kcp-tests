package networking

import (
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-networking] SDN sriov", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("sriov-"+getRandomString(), exutil.KubeConfigPath())
	)
	g.BeforeEach(func() {
		// for now skip sriov cases in temp in order to avoid cases always show failed in CI since sriov operator is not setup . will add install operator function after that
		_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get("openshift-sriov-network-operator", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				g.Skip("the cluster do not install sriov operator")
			}
		}

	})

	g.It("NonPreRelease-Author:yingwang-Medium-Longduration-42253-Pod with sriov interface should be created successfully with empty pod.ObjectMeta.Namespace in body [Disruptive]", func() {
		var (
			networkBaseDir = exutil.FixturePath("testdata", "networking")
			sriovBaseDir   = filepath.Join(networkBaseDir, "sriov")

			sriovNetPolicyName = "netpolicy42253"
			sriovNetDeviceName = "netdevice42253"
			sriovOpNs          = "openshift-sriov-network-operator"
			podName1           = "sriov-42253-testpod1"
			podName2           = "sriov-42253-testpod2"
			pfName             = "ens2f0"
			deviceID           = "1015"
			ipv4Addr1          = "192.168.2.5/24"
			ipv6Addr1          = "2002::5/64"
			ipv4Addr2          = "192.168.2.6/24"
			ipv6Addr2          = "2002::6/64"
			sriovIntf          = "net1"
			podTempfile        = "sriov-testpod-template.yaml"
			serviceAccount     = "deployer"
		)

		sriovNetworkPolicyTmpFile := filepath.Join(sriovBaseDir, "netpolicy42253-template.yaml")
		sriovNetworkPolicy := sriovNetResource{
			name:      sriovNetPolicyName,
			namespace: sriovOpNs,
			tempfile:  sriovNetworkPolicyTmpFile,
			kind:      "SriovNetworkNodePolicy",
		}

		sriovNetworkAttachTmpFile := filepath.Join(sriovBaseDir, "netdevice42253-template.yaml")
		sriovNetwork := sriovNetResource{
			name:      sriovNetDeviceName,
			namespace: sriovOpNs,
			tempfile:  sriovNetworkAttachTmpFile,
			kind:      "SriovNetwork",
		}

		g.By("1) ####### Check openshift-sriov-network-operator is running well ##########")
		chkSriovOperatorStatus(oc, sriovOpNs)

		g.By("Check the deviceID if exist on the cluster worker")
		if !checkDeviceIDExist(oc, sriovOpNs, deviceID) {
			g.Skip("the cluster do not contain the sriov card. skip this testing!")
		}
		//make sure the pf and sriov network policy name are not occupied
		rmSriovNetworkPolicy(oc, sriovNetworkPolicy.name, sriovNetworkPolicy.namespace)
		rmSriovNetwork(oc, sriovNetwork.name, sriovNetwork.namespace)

		oc.SetupProject()
		g.By("2) ####### Create sriov network policy ############")

		sriovNetworkPolicy.create(oc, "PFNAME="+pfName, "DEVICEID="+deviceID, "SRIOVNETPOLICY="+sriovNetworkPolicy.name)
		defer rmSriovNetworkPolicy(oc, sriovNetworkPolicy.name, sriovNetworkPolicy.namespace)
		waitForSriovPolicyReady(oc, sriovNetworkPolicy.namespace)

		g.By("3) ######### Create sriov network attachment ############")

		e2e.Logf("create sriov network attachment via template")
		sriovNetwork.create(oc, "TARGETNS="+oc.Namespace(), "SRIOVNETNAME="+sriovNetwork.name, "SRIOVNETPOLICY="+sriovNetworkPolicy.name)

		defer sriovNetwork.delete(oc) // ensure the resource is deleted whether the case exist normally or not.

		g.By("4) ########### Create Pod and attach sriov interface using cli ##########")
		podTempFile1 := filepath.Join(sriovBaseDir, podTempfile)
		testPod1 := sriovPod{
			name:         podName1,
			namespace:    oc.Namespace(),
			tempfile:     podTempFile1,
			ipv4addr:     ipv4Addr1,
			ipv6addr:     ipv6Addr1,
			intfname:     sriovIntf,
			intfresource: sriovNetDeviceName,
		}
		podsLog := testPod1.createPod(oc)
		defer testPod1.deletePod(oc) // ensure the resource is deleted whether the case exist normally or not.
		testPod1.waitForPodReady(oc)
		intfInfo1 := testPod1.getSriovIntfonPod(oc)
		o.Expect(intfInfo1).Should(o.MatchRegexp(testPod1.intfname))
		o.Expect(intfInfo1).Should(o.MatchRegexp(testPod1.ipv4addr))
		o.Expect(intfInfo1).Should(o.MatchRegexp(testPod1.ipv6addr))
		e2e.Logf("Check pod %s sriov interface and ip address PASS.", testPod1.name)

		g.By("5) ########### Create Pod via url without namespace ############")
		podTempFile2 := filepath.Join(sriovBaseDir, podTempfile)
		testPod2 := sriovPod{
			name:         podName2,
			namespace:    oc.Namespace(),
			tempfile:     podTempFile2,
			ipv4addr:     ipv4Addr2,
			ipv6addr:     ipv6Addr2,
			intfname:     sriovIntf,
			intfresource: sriovNetDeviceName,
		}
		e2e.Logf("extract curl reqeust command from logs of creating pod via cli")
		re := regexp.MustCompile("(curl.+-XPOST.+kubectl-create')")
		match := re.FindStringSubmatch(podsLog)
		curlCmd := match[1]
		e2e.Logf("Extracted curl from pod creating logs is %s", curlCmd)
		//creating pod via curl request
		testPod2.sendHTTPRequest(oc, serviceAccount, curlCmd)
		defer testPod2.deletePod(oc)
		testPod2.waitForPodReady(oc)
		intfInfo2 := testPod2.getSriovIntfonPod(oc)
		o.Expect(intfInfo2).Should(o.MatchRegexp(testPod2.intfname))
		o.Expect(intfInfo2).Should(o.MatchRegexp(testPod2.ipv4addr))
		o.Expect(intfInfo2).Should(o.MatchRegexp(testPod2.ipv6addr))
		e2e.Logf("Check pod %s sriov interface and ip address PASS.", testPod2.name)

	})

	g.It("Author:zzhao-Medium-Longduration-25321-Check intel dpdk works well [Disruptive]", func() {
		var (
			buildPruningBaseDir            = exutil.FixturePath("testdata", "networking/sriov")
			sriovNetworkNodePolicyTemplate = filepath.Join(buildPruningBaseDir, "sriovnetworkpolicy-template.yaml")
			sriovNeworkTemplate            = filepath.Join(buildPruningBaseDir, "sriovnetwork-template.yaml")
			sriovTestPodTemplate           = filepath.Join(buildPruningBaseDir, "sriov-dpdk-template.yaml")
			sriovOpNs                      = "openshift-sriov-network-operator"
		)
		sriovPolicy := sriovNetworkNodePolicy{
			policyName:   "intel710",
			deviceType:   "vfio-pci",
			deviceID:     "158b",
			pfName:       "ens1f0",
			vondor:       "8086",
			numVfs:       2,
			resourceName: "intel710dpdk",
			template:     sriovNetworkNodePolicyTemplate,
			namespace:    sriovOpNs,
		}

		g.By("check the sriov operator is running")
		chkSriovOperatorStatus(oc, sriovOpNs)

		g.By("Check the deviceID if exist on the cluster worker")
		if !checkDeviceIDExist(oc, sriovOpNs, sriovPolicy.deviceID) {
			g.Skip("the cluster do not contain the sriov card. skip this testing!")
		}

		g.By("setup one namespace")
		oc.SetupProject()

		g.By("Create sriovnetworkpolicy to init VF and check they are inited successfully")
		sriovPolicy.createPolicy(oc)
		defer rmSriovNetworkPolicy(oc, sriovPolicy.policyName, sriovOpNs)
		waitForSriovPolicyReady(oc, sriovOpNs)

		g.By("Create sriovNetwork to generate net-attach-def on the target namespace")
		sriovnetwork := sriovNetwork{
			name:             sriovPolicy.policyName,
			resourceName:     sriovPolicy.resourceName,
			networkNamespace: oc.Namespace(),
			template:         sriovNeworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovnetwork.createSriovNetwork(oc)
		defer rmSriovNetwork(oc, sriovnetwork.name, sriovOpNs)

		g.By("Create test pod on the target namespace")

		sriovTestPod := sriovTestPod{
			name:        "sriovdpdk",
			namespace:   oc.Namespace(),
			networkName: sriovnetwork.name,
			template:    sriovTestPodTemplate,
		}
		sriovTestPod.createSriovTestPod(oc)
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sriov-dpdk")
		exutil.AssertWaitPollNoErr(err, "this pod with label name=sriov-dpdk not ready")

		g.By("Check testpmd running well")
		pciAddress := getPciAddress(sriovTestPod.namespace, sriovTestPod.name)
		command := "testpmd -l 2-3 --in-memory -w " + pciAddress + " --socket-mem 1024 -n 4 --proc-type auto --file-prefix pg -- --disable-rss --nb-cores=1 --rxq=1 --txq=1 --auto-start --forward-mode=mac"
		testpmdOutput, err := e2e.RunHostCmd(sriovTestPod.namespace, sriovTestPod.name, command)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(testpmdOutput).Should(o.MatchRegexp("forwards packets on 1 streams"))

	})

	g.It("Author:zzhao-Medium-Longduration-49213- VF with large number can be inited for intel card [Disruptive]", func() {
		var (
			buildPruningBaseDir            = exutil.FixturePath("testdata", "networking/sriov")
			sriovNetworkNodePolicyTemplate = filepath.Join(buildPruningBaseDir, "sriovnetworkpolicy-template.yaml")
			sriovOpNs                      = "openshift-sriov-network-operator"
			sriovNodeLabel                 = "feature.node.kubernetes.io/sriov-capable=true"
		)
		sriovPolicy := sriovNetworkNodePolicy{
			policyName:   "intel710",
			deviceType:   "netdevice",
			deviceID:     "158b",
			pfName:       "ens1f0",
			vondor:       "8086",
			numVfs:       40,
			resourceName: "intel710net",
			template:     sriovNetworkNodePolicyTemplate,
			namespace:    sriovOpNs,
		}

		g.By("check the sriov operator is running")
		chkSriovOperatorStatus(oc, sriovOpNs)

		g.By("Check the deviceID if exist on the cluster worker")
		if !checkDeviceIDExist(oc, sriovOpNs, sriovPolicy.deviceID) {
			g.Skip("the cluster do not contain the sriov card. skip this testing!")
		}

		g.By("Create sriovnetworkpolicy to init VF and check they are inited successfully")
		sriovPolicy.createPolicy(oc)
		defer rmSriovNetworkPolicy(oc, sriovPolicy.policyName, sriovOpNs)
		waitForSriovPolicyReady(oc, sriovOpNs)

		g.By("check the link show the correct VF")
		sriovNode := getSriovNode(oc, sriovOpNs, sriovNodeLabel)
		output, err := exutil.DebugNodeWithChroot(oc, sriovNode, "bash", "-c", "ip l | grep ens1f0v | wc -l")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("40"))
	})
})
