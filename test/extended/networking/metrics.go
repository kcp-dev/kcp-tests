package networking

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-metrics", exutil.KubeConfigPath())

	g.It("Author:weliang-Medium-47524-Metrics for ovn-appctl stopwatch/show command.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}
		leaderNodeIP := getLeaderInfo(oc, namespace, cmName, networkType)
		prometheusURL := "https://" + leaderNodeIP + ":9105/metrics"
		metricName1 := "ovn_controller_if_status_mgr_run_total_samples"
		metricName2 := "ovn_controller_if_status_mgr_run_long_term_avg"
		metricName3 := "ovn_controller_bfd_run_total_samples"
		metricName4 := "ovn_controller_bfd_run_long_term_avg"
		metricName5 := "ovn_controller_flow_installation_total_samples"
		metricName6 := "ovn_controller_flow_installation_long_term_avg"
		metricName7 := "ovn_controller_if_status_mgr_run_total_samples"
		metricName8 := "ovn_controller_if_status_mgr_run_long_term_avg"
		metricName9 := "ovn_controller_if_status_mgr_update_total_samples"
		metricName10 := "ovn_controller_if_status_mgr_update_long_term_avg"
		metricName11 := "ovn_controller_flow_generation_total_samples"
		metricName12 := "ovn_controller_flow_generation_long_term_avg"
		metricName13 := "ovn_controller_pinctrl_run_total_samples"
		metricName14 := "ovn_controller_pinctrl_run_long_term_avg"
		metricName15 := "ovn_controller_ofctrl_seqno_run_total_samples"
		metricName16 := "ovn_controller_ofctrl_seqno_run_long_term_avg"
		metricName17 := "ovn_controller_patch_run_total_samples"
		metricName18 := "ovn_controller_patch_run_long_term_avg"
		metricName19 := "ovn_controller_ct_zone_commit_total_samples"
		metricName20 := "ovn_controller_ct_zone_commit_long_term_avg"
		checkSDNMetrics(oc, prometheusURL, metricName1)
		checkSDNMetrics(oc, prometheusURL, metricName2)
		checkSDNMetrics(oc, prometheusURL, metricName3)
		checkSDNMetrics(oc, prometheusURL, metricName4)
		checkSDNMetrics(oc, prometheusURL, metricName5)
		checkSDNMetrics(oc, prometheusURL, metricName6)
		checkSDNMetrics(oc, prometheusURL, metricName7)
		checkSDNMetrics(oc, prometheusURL, metricName8)
		checkSDNMetrics(oc, prometheusURL, metricName9)
		checkSDNMetrics(oc, prometheusURL, metricName10)
		checkSDNMetrics(oc, prometheusURL, metricName11)
		checkSDNMetrics(oc, prometheusURL, metricName12)
		checkSDNMetrics(oc, prometheusURL, metricName13)
		checkSDNMetrics(oc, prometheusURL, metricName14)
		checkSDNMetrics(oc, prometheusURL, metricName15)
		checkSDNMetrics(oc, prometheusURL, metricName16)
		checkSDNMetrics(oc, prometheusURL, metricName17)
		checkSDNMetrics(oc, prometheusURL, metricName18)
		checkSDNMetrics(oc, prometheusURL, metricName19)
		checkSDNMetrics(oc, prometheusURL, metricName20)
	})

	g.It("Author:weliang-Medium-47471-Record update to cache versus port binding.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}
		leaderNodeIP := getLeaderInfo(oc, namespace, cmName, networkType)
		prometheusURL := "https://" + leaderNodeIP + ":9102/metrics"
		metricName1 := "ovnkube_master_pod_first_seen_lsp_created_duration_seconds_count"
		metricName2 := "ovnkube_master_pod_lsp_created_port_binding_duration_seconds_count"
		metricName3 := "ovnkube_master_pod_port_binding_port_binding_chassis_duration_seconds_count"
		metricName4 := "ovnkube_master_pod_port_binding_chassis_port_binding_up_duration_seconds_count"
		checkSDNMetrics(oc, prometheusURL, metricName1)
		checkSDNMetrics(oc, prometheusURL, metricName2)
		checkSDNMetrics(oc, prometheusURL, metricName3)
		checkSDNMetrics(oc, prometheusURL, metricName4)
	})

	g.It("Author:weliang-Medium-45841-Add OVN flow count metric.", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}
		leaderNodeIP := getLeaderInfo(oc, namespace, cmName, networkType)
		prometheusURL := "https://" + leaderNodeIP + ":9105/metrics"
		metricName := "ovn_controller_integration_bridge_openflow"
		checkSDNMetrics(oc, prometheusURL, metricName)
	})

	g.It("Author:weliang-Medium-45688-Metrics for egress firewall. [Disruptive]", func() {
		var (
			ovnnamespace        = "openshift-ovn-kubernetes"
			ovncmName           = "ovn-kubernetes-master"
			sdnnamespace        = "openshift-sdn"
			sdncmName           = "openshift-network-controller"
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking/metrics")
			egressFirewall      = filepath.Join(buildPruningBaseDir, "OVN-Rules.yaml")
			egressNetworkpolicy = filepath.Join(buildPruningBaseDir, "SDN-Rules.yaml")
		)
		g.By("create new namespace")
		oc.SetupProject()
		ns := oc.Namespace()

		networkType := checkNetworkType(oc)
		if networkType == "ovnkubernetes" {
			g.By("get the metrics of ovnkube_master_num_egress_firewall_rules before configuration")
			leaderNodeIP := getLeaderInfo(oc, ovnnamespace, ovncmName, networkType)
			prometheusURL := "https://" + leaderNodeIP + ":9102/metrics"
			output := getOVNMetrics(oc, prometheusURL)
			metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep ovnkube_master_num_egress_firewall_rules | awk 'NR==3{print $2}'").Output()
			metricValue := strings.TrimSpace(string(metricOutput))
			e2e.Logf("The output of the ovnkube_master_num_egress_firewall_rules metrics is : %v", metricValue)
			o.Expect(metricValue).To(o.ContainSubstring("0"))

			g.By("create egressfirewall rules in OVN cluster")
			fwErr := oc.AsAdmin().Run("create").Args("-n", ns, "-f", egressFirewall).Execute()
			o.Expect(fwErr).NotTo(o.HaveOccurred())
			defer oc.AsAdmin().Run("delete").Args("-n", ns, "-f", egressFirewall).Execute()
			fwOutput, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("egressfirewall", "-n", ns).Output()
			o.Expect(fwOutput).To(o.ContainSubstring("EgressFirewall Rules applied"))

			g.By("get the metrics of ovnkube_master_num_egress_firewall_rules after configuration")
			output1 := getOVNMetrics(oc, prometheusURL)
			metricOutput1, _ := exec.Command("bash", "-c", "cat "+output1+" | grep ovnkube_master_num_egress_firewall_rules | awk 'NR==3{print $2}'").Output()
			metricValue1 := strings.TrimSpace(string(metricOutput1))
			e2e.Logf("The output of the ovnkube_master_num_egress_firewall_rules metrics is : %v", metricValue1)
			o.Expect(metricValue1).To(o.ContainSubstring("3"))
		}
		if networkType == "openshiftsdn" {
			g.By("get the metrics of sdn_controller_num_egress_firewalls before configuration")
			leaderPodName := getLeaderInfo(oc, sdnnamespace, sdncmName, networkType)
			output := getSDNMetrics(oc, leaderPodName)
			metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep sdn_controller_num_egress_firewall_rules | awk 'NR==3{print $2}'").Output()
			metricValue := strings.TrimSpace(string(metricOutput))
			e2e.Logf("The output of the sdn_controller_num_egress_firewall_rules metrics is : %v", metricValue)
			o.Expect(metricValue).To(o.ContainSubstring("0"))

			g.By("create egressNetworkpolicy rules in SDN cluster")
			fwErr := oc.AsAdmin().Run("create").Args("-n", ns, "-f", egressNetworkpolicy).Execute()
			o.Expect(fwErr).NotTo(o.HaveOccurred())
			defer oc.AsAdmin().Run("delete").Args("-n", ns, "-f", egressNetworkpolicy).Execute()
			fwOutput, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("egressnetworkpolicy", "-n", ns).Output()
			o.Expect(fwOutput).To(o.ContainSubstring("sdn-egressnetworkpolicy"))

			g.By("get the metrics of sdn_controller_num_egress_firewalls after configuration")
			output1 := getSDNMetrics(oc, leaderPodName)
			metricOutput1, _ := exec.Command("bash", "-c", "cat "+output1+" | grep sdn_controller_num_egress_firewall_rules | awk 'NR==3{print $2}'").Output()
			metricValue1 := strings.TrimSpace(string(metricOutput1))
			e2e.Logf("The output of the sdn_controller_num_egress_firewall_rules metrics is : %v", metricValue1)
			o.Expect(metricValue1).To(o.ContainSubstring("2"))
		}
	})

	g.It("Author:weliang-Medium-45842-Metrics for IPSec enabled/disabled", func() {
		var (
			namespace = "openshift-ovn-kubernetes"
			cmName    = "ovn-kubernetes-master"
		)
		networkType := checkNetworkType(oc)
		if !strings.Contains(networkType, "ovn") {
			g.Skip("Skip testing on non-ovn cluster!!!")
		}

		ipsecState := checkIPsec(oc)
		e2e.Logf("The ipsec state is : %v", ipsecState)
		leaderNodeIP := getLeaderInfo(oc, namespace, cmName, networkType)
		prometheusURL := "https://" + leaderNodeIP + ":9102/metrics"
		output := getOVNMetrics(oc, prometheusURL)
		metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep ovnkube_master_ipsec_enabled | awk 'NR==3{print $2}'").Output()
		metricValue := strings.TrimSpace(string(metricOutput))
		e2e.Logf("The output of the ovnkube_master_ipsec_enabled metrics is : %v", metricValue)
		if metricValue == "1" && ipsecState == "{}" {
			e2e.Logf("The IPsec is enabled in the cluster")
		} else if metricValue == "0" && ipsecState == "" {
			e2e.Logf("The IPsec is disabled in the cluster")
		} else {
			e2e.Failf("Testing fail to get the correct metrics of ovnkube_master_ipsec_enabled")
		}
	})

	g.It("Author:weliang-Medium-45687-Metrics for egress router", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking/metrics")
			egressrouterPod     = filepath.Join(buildPruningBaseDir, "egressrouter.yaml")
		)
		g.By("create new namespace")
		oc.SetupProject()
		ns := oc.Namespace()

		g.By("create a test pod")
		podErr1 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", egressrouterPod, "-n", ns).Execute()
		o.Expect(podErr1).NotTo(o.HaveOccurred())
		podErr2 := waitForPodWithLabelReady(oc, oc.Namespace(), "app=egress-router-cni")
		exutil.AssertWaitPollNoErr(podErr2, "egressrouterPod is not running")

		podName := getPodName(oc, "openshift-multus", "app=multus-admission-controller")
		output, err := oc.AsAdmin().Run("exec").Args("-n", "openshift-multus", podName[1], "--", "curl", "localhost:9091/metrics").OutputToFile("metrics.txt")
		o.Expect(err).NotTo(o.HaveOccurred())
		metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep egress-router | awk '{print $2}'").Output()
		metricValue := strings.TrimSpace(string(metricOutput))
		e2e.Logf("The output of the egress-router metrics is : %v", metricValue)
		o.Expect(metricValue).To(o.ContainSubstring("1"))
	})

	g.It("Author:weliang-Medium-45685-Metrics for Metrics for egressIP. [Disruptive]", func() {
		var (
			ovnnamespace        = "openshift-ovn-kubernetes"
			ovncmName           = "ovn-kubernetes-master"
			sdnnamespace        = "openshift-sdn"
			sdncmName           = "openshift-network-controller"
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking")
			egressIPTemplate    = filepath.Join(buildPruningBaseDir, "egressip-config1-template.yaml")
		)

		platform := checkPlatform(oc)
		if !strings.Contains(platform, "vsphere") {
			g.Skip("Skip for un-expected platform, egreeIP testing need to be executed on a vsphere cluster!")
		}
		networkType := checkNetworkType(oc)

		if networkType == "ovnkubernetes" {
			g.By("create new namespace")
			oc.SetupProject()
			ns := oc.Namespace()

			g.By("get the metrics of ovnkube_master_num_egress_ips before egress_ips configurations")
			leaderNodeIP := getLeaderInfo(oc, ovnnamespace, ovncmName, networkType)
			prometheusURL := "https://" + leaderNodeIP + ":9102/metrics"
			output := getOVNMetrics(oc, prometheusURL)
			metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep ovnkube_master_num_egress_ips | awk 'NR==3{print $2}'").Output()
			metricValue := strings.TrimSpace(string(metricOutput))
			e2e.Logf("The output of the ovnkube_master_num_egress_ips is : %v", metricValue)
			o.Expect(metricValue).To(o.ContainSubstring("0"))

			g.By("Label EgressIP node")
			var EgressNodeLabel = "k8s.ovn.org/egress-assignable"
			nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
			if err != nil {
				e2e.Logf("Unexpected error occurred: %v", err)
			}
			g.By("Apply EgressLabel Key on one node.")
			e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel, "true")
			defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, EgressNodeLabel)

			g.By("Apply label to namespace")
			_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns, "name=test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns, "name-").Output()

			g.By("Create an egressip object")
			sub1, _ := getDefaultSubnet(oc)
			ips := findUnUsedIPs(oc, sub1, 2)
			egressip1 := egressIPResource1{
				name:      "egressip-45685",
				template:  egressIPTemplate,
				egressIP1: ips[0],
				egressIP2: ips[1],
			}
			egressip1.createEgressIPObject1(oc)
			defer egressip1.deleteEgressIPObject1(oc)

			g.By("get the metrics of ovnkube_master_num_egress_ips after egress_ips configurations")
			output1 := getOVNMetrics(oc, prometheusURL)
			metricOutput1, _ := exec.Command("bash", "-c", "cat "+output1+" | grep ovnkube_master_num_egress_ips | awk 'NR==3{print $2}'").Output()
			metricValue1 := strings.TrimSpace(string(metricOutput1))
			e2e.Logf("The output of the ovnkube_master_num_egress_ips is : %v", metricValue1)
			o.Expect(metricValue1).To(o.ContainSubstring("1"))
		}

		if networkType == "openshiftsdn" {
			g.By("create new namespace")
			oc.SetupProject()
			ns := oc.Namespace()
			ip := "192.168.249.145"

			g.By("get the metrics of sdn_controller_num_egress_ips before egress_ips configurations")
			leaderPodName := getLeaderInfo(oc, sdnnamespace, sdncmName, networkType)
			output := getSDNMetrics(oc, leaderPodName)
			metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep sdn_controller_num_egress_ips | awk 'NR==3{print $2}'").Output()
			metricValue := strings.TrimSpace(string(metricOutput))
			e2e.Logf("The output of the sdn_controller_num_egress_ips is : %v", metricValue)
			o.Expect(metricValue).To(o.ContainSubstring("0"))

			patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[\""+ip+"\"]}")
			defer patchResourceAsAdmin(oc, "netnamespace/"+ns, "{\"egressIPs\":[]}")

			nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
			o.Expect(err).NotTo(o.HaveOccurred())
			egressNode := nodeList.Items[0].Name
			patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressIPs\":[\""+ip+"\"]}")
			defer patchResourceAsAdmin(oc, "hostsubnet/"+egressNode, "{\"egressIPs\":[]}")

			g.By("get the metrics of sdn_controller_num_egress_ips after egress_ips configurations")
			output1 := getSDNMetrics(oc, leaderPodName)
			metricOutput1, _ := exec.Command("bash", "-c", "cat "+output1+" | grep sdn_controller_num_egress_ips | awk 'NR==3{print $2}'").Output()
			metricValue1 := strings.TrimSpace(string(metricOutput1))
			e2e.Logf("The output of the sdn_controller_num_egress_ips is : %v", metricValue1)
			o.Expect(metricValue1).To(o.ContainSubstring("1"))
		}
	})

	g.It("Author:weliang-Medium-45689-Metrics for idling enable/disabled.", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking")
			testPodFile         = filepath.Join(buildPruningBaseDir, "metrics/metrics-pod.json")
			testSvcFile         = filepath.Join(buildPruningBaseDir, "testpod.yaml")
			testPodName         = "hello-pod"
		)

		g.By("create new namespace")
		oc.SetupProject()
		ns := oc.Namespace()

		g.By("create a service")
		createResourceFromFile(oc, ns, testSvcFile)
		ServiceOutput, serviceErr := oc.WithoutNamespace().Run("get").Args("service", "-n", ns).Output()
		o.Expect(serviceErr).NotTo(o.HaveOccurred())
		o.Expect(ServiceOutput).To(o.ContainSubstring("test-service"))

		g.By("create a test pod")
		createResourceFromFile(oc, ns, testPodFile)
		podErr := waitForPodWithLabelReady(oc, ns, "name=hello-pod")
		exutil.AssertWaitPollNoErr(podErr, "hello-pod is not running")

		g.By("get test service ip address")
		testServiceIP, _ := getSvcIP(oc, ns, "test-service") //This case is check metrics not svc testing, do not need use test-service dual-stack address

		g.By("test-pod can curl service ip address:port")
		ipStackType := checkIPStackType(oc)
		//Need curl serverice several times, otherwise casue curl: (7) Failed to connect to 172.30.248.18 port 27017
		//after 0 ms: Connection refused\ncommand terminated with exit code 7\n\nerror:\nexit status 7"
		if ipStackType == "ipv6single" {
			for i := 0; i < 6; i++ {
				e2e.RunHostCmd(ns, testPodName, "curl ["+testServiceIP+"]:27017 --connect-timeout 5")
			}
			_, svcerr := e2e.RunHostCmd(ns, testPodName, "curl ["+testServiceIP+"]:27017 --connect-timeout 5")
			o.Expect(svcerr).NotTo(o.HaveOccurred())
		}
		if ipStackType == "ipv4single" || ipStackType == "dualstack" {
			for i := 0; i < 6; i++ {
				e2e.RunHostCmd(ns, testPodName, "curl "+testServiceIP+":27017 --connect-timeout 5")
			}
			_, svcerr := e2e.RunHostCmd(ns, testPodName, "curl "+testServiceIP+":27017 --connect-timeout 5")
			o.Expect(svcerr).NotTo(o.HaveOccurred())
		}

		g.By("idle test-service")
		_, idleerr := oc.Run("idle").Args("-n", ns, "test-service").Output()
		o.Expect(idleerr).NotTo(o.HaveOccurred())

		g.By("test pod can curl service address:port again to unidle the svc")
		//Need curl serverice several times, otherwise casue curl: (7) Failed to connect to 172.30.248.18 port 27017
		//after 0 ms: Connection refused\ncommand terminated with exit code 7\n\nerror:\nexit status 7"
		if ipStackType == "ipv6single" {
			for i := 0; i < 6; i++ {
				e2e.RunHostCmd(ns, testPodName, "curl ["+testServiceIP+"]:27017 --connect-timeout 5")
			}
			_, svcerr := e2e.RunHostCmd(ns, testPodName, "curl ["+testServiceIP+"]:27017 --connect-timeout 5")
			o.Expect(svcerr).NotTo(o.HaveOccurred())
		} else {
			for i := 0; i < 6; i++ {
				e2e.RunHostCmd(ns, testPodName, "curl "+testServiceIP+":27017 --connect-timeout 5")
			}
			_, svcerr := e2e.RunHostCmd(ns, testPodName, "curl "+testServiceIP+":27017 --connect-timeout 5")
			o.Expect(svcerr).NotTo(o.HaveOccurred())
		}

		g.By("get controller-managert service ip address")
		ManagertServiceIP, _ := getSvcIP(oc, "openshift-controller-manager", "controller-manager") //Right now, the cluster svc only get single IP
		var prometheusURL string
		if ipStackType == "ipv6single" {
			prometheusURL = "https://[" + ManagertServiceIP + "]/metrics"
		} else {
			prometheusURL = "https://" + ManagertServiceIP + "/metrics"
		}
		//Because Bug 2064786: Not always can get the metrics of openshift_unidle_events_total
		//Need curl several times to get the metrics of openshift_unidle_events_total
		metricsErr := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output := getOVNMetrics(oc, prometheusURL)
			metricOutput, _ := exec.Command("bash", "-c", "cat "+output+" | grep openshift_unidle_events_total | awk 'NR==3{print $2}'").Output()
			metricValue := strings.TrimSpace(string(metricOutput))
			e2e.Logf("The output of openshift_unidle_events metrics is : %v", metricValue)
			if strings.Contains(metricValue, "1") {
				return true, nil
			}
			e2e.Logf("Can't get correct metrics of openshift_unidle_events and try again")
			return false, nil

		})
		exutil.AssertWaitPollNoErr(metricsErr, fmt.Sprintf("Fail to get metric and the error is:%s", metricsErr))
	})
})
