package networking

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	netutils "k8s.io/utils/net"
)

type pingPodResource struct {
	name      string
	namespace string
	template  string
}

type pingPodResourceNode struct {
	name      string
	namespace string
	nodename  string
	template  string
}

type pingPodResourceWinNode struct {
	name      string
	namespace string
	image     string
	nodename  string
	template  string
}

type egressIPResource1 struct {
	name          string
	template      string
	egressIP1     string
	egressIP2     string
	nsLabelKey    string
	nsLabelValue  string
	podLabelKey   string
	podLabelValue string
}

type egressFirewall1 struct {
	name      string
	namespace string
	template  string
}

type egressFirewall2 struct {
	name      string
	namespace string
	ruletype  string
	cidr      string
	template  string
}

type ipBlockIngressDual struct {
	name      string
	namespace string
	cidrIpv4  string
	cidrIpv6  string
	template  string
}

type ipBlockIngressSingle struct {
	name      string
	namespace string
	cidr      string
	template  string
}

type genericServiceResource struct {
	servicename           string
	namespace             string
	protocol              string
	selector              string
	serviceType           string
	ipFamilyPolicy        string
	externalTrafficPolicy string
	internalTrafficPolicy string
	template              string
}

type windowGenericServiceResource struct {
	servicename           string
	namespace             string
	protocol              string
	selector              string
	serviceType           string
	ipFamilyPolicy        string
	externalTrafficPolicy string
	internalTrafficPolicy string
	template              string
}

type testPodMultinetwork struct {
	name      string
	namespace string
	nodename  string
	nadname   string
	labelname string
	template  string
}

func (pod *pingPodResource) createPingPod(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create pod %v", pod.name))
}

func (pod *pingPodResourceNode) createPingPodNode(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "NODENAME="+pod.nodename)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create pod %v", pod.name))
}

func (pod *pingPodResourceWinNode) createPingPodWinNode(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "IMAGE="+pod.image, "NODENAME="+pod.nodename)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create pod %v", pod.name))
}

func (pod *testPodMultinetwork) createTestPodMultinetwork(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "NODENAME="+pod.nodename, "LABELNAME="+pod.labelname, "NADNAME="+pod.nadname)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create pod %v", pod.name))
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.Run("process").Args(parameters...).OutputToFile(getRandomString() + "ping-pod.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func (egressIP *egressIPResource1) createEgressIPObject1(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", egressIP.template, "-p", "NAME="+egressIP.name, "EGRESSIP1="+egressIP.egressIP1, "EGRESSIP2="+egressIP.egressIP2)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create EgressIP %v", egressIP.name))
}

func (egressIP *egressIPResource1) deleteEgressIPObject1(oc *exutil.CLI) {
	removeResource(oc, true, true, "egressip", egressIP.name)
}

func (egressIP *egressIPResource1) createEgressIPObject2(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", egressIP.template, "-p", "NAME="+egressIP.name, "EGRESSIP1="+egressIP.egressIP1, "NSLABELKEY="+egressIP.nsLabelKey, "NSLABELVALUE="+egressIP.nsLabelValue, "PODLABELKEY="+egressIP.podLabelKey, "PODLABELVALUE="+egressIP.podLabelValue)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create EgressIP %v", egressIP.name))
}

func (egressFirewall *egressFirewall1) createEgressFWObject1(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", egressFirewall.template, "-p", "NAME="+egressFirewall.name, "NAMESPACE="+egressFirewall.namespace)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create EgressFW %v", egressFirewall.name))
}

func (egressFirewall *egressFirewall1) deleteEgressFWObject1(oc *exutil.CLI) {
	removeResource(oc, true, true, "egressfirewall", egressFirewall.name, "-n", egressFirewall.namespace)
}

func (egressFirewall *egressFirewall2) createEgressFW2Object(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", egressFirewall.template, "-p", "NAME="+egressFirewall.name, "NAMESPACE="+egressFirewall.namespace, "RULETYPE="+egressFirewall.ruletype, "CIDR="+egressFirewall.cidr)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create EgressFW2 %v", egressFirewall.name))
}

func (ipBlock_ingress_policy *ipBlockIngressDual) createipBlockIngressObjectDual(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", ipBlock_ingress_policy.template, "-p", "NAME="+ipBlock_ingress_policy.name, "NAMESPACE="+ipBlock_ingress_policy.namespace, "cidrIpv6="+ipBlock_ingress_policy.cidrIpv6, "cidrIpv4="+ipBlock_ingress_policy.cidrIpv4)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create network policy %v", ipBlock_ingress_policy.name))
}

func (ipBlock_ingress_policy *ipBlockIngressSingle) createipBlockIngressObjectSingle(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", ipBlock_ingress_policy.template, "-p", "NAME="+ipBlock_ingress_policy.name, "NAMESPACE="+ipBlock_ingress_policy.namespace, "CIDR="+ipBlock_ingress_policy.cidr)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create network policy %v", ipBlock_ingress_policy.name))
}

func (service *genericServiceResource) createServiceFromParams(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", service.template, "-p", "SERVICENAME="+service.servicename, "NAMESPACE="+service.namespace, "PROTOCOL="+service.protocol, "SELECTOR="+service.selector, "serviceType="+service.serviceType, "ipFamilyPolicy="+service.ipFamilyPolicy, "internalTrafficPolicy="+service.internalTrafficPolicy, "externalTrafficPolicy="+service.externalTrafficPolicy)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create svc %v", service.servicename))
}

func (service *windowGenericServiceResource) createWinServiceFromParams(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", service.template, "-p", "SERVICENAME="+service.servicename, "NAMESPACE="+service.namespace, "PROTOCOL="+service.protocol, "SELECTOR="+service.selector, "serviceType="+service.serviceType, "ipFamilyPolicy="+service.ipFamilyPolicy, "internalTrafficPolicy="+service.internalTrafficPolicy, "externalTrafficPolicy="+service.externalTrafficPolicy)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create svc %v", service.servicename))
}

func (egressFirewall *egressFirewall2) deleteEgressFW2Object(oc *exutil.CLI) {
	removeResource(oc, true, true, "egressfirewall", egressFirewall.name, "-n", egressFirewall.namespace)
}

func (pod *pingPodResource) deletePingPod(oc *exutil.CLI) {
	removeResource(oc, false, true, "pod", pod.name, "-n", pod.namespace)
}

func (pod *pingPodResourceNode) deletePingPodNode(oc *exutil.CLI) {
	removeResource(oc, false, true, "pod", pod.name, "-n", pod.namespace)
}

func removeResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	output, err := doAction(oc, "delete", asAdmin, withoutNamespace, parameters...)
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
			e2e.Logf("the resource is delete successfully")
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to delete resource %v", parameters))
}

func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func applyResourceFromTemplateByAdmin(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "resource.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("as admin fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.WithoutNamespace().AsAdmin().Run("apply").Args("-f", configFile).Execute()
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func getPodStatus(oc *exutil.CLI, namespace string, podName string) (string, error) {
	podStatus, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status in namespace %s is %q", podName, namespace, podStatus)
	return podStatus, err
}

func checkPodReady(oc *exutil.CLI, namespace string, podName string) (bool, error) {
	podOutPut, err := getPodStatus(oc, namespace, podName)
	status := []string{"Running", "Ready", "Complete"}
	return contains(status, podOutPut), err
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func waitPodReady(oc *exutil.CLI, namespace string, podName string) {
	err := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
		status, err1 := checkPodReady(oc, namespace, podName)
		if err1 != nil {
			e2e.Logf("the err:%v, wait for pod %v to become ready.", err1, podName)
			return status, err1
		}
		if !status {
			return status, nil
		}
		return status, nil
	})

	if err != nil {
		podDescribe := describePod(oc, namespace, podName)
		e2e.Logf("oc describe pod %v.", podName)
		e2e.Logf(podDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %v is not ready", podName))
}

func describePod(oc *exutil.CLI, namespace string, podName string) string {
	podDescribe, err := oc.WithoutNamespace().Run("describe").Args("pod", "-n", namespace, podName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podDescribe)
	return podDescribe
}

func execCommandInSpecificPod(oc *exutil.CLI, namespace string, podName string, command string) (string, error) {
	e2e.Logf("The command is: %v", command)
	command1 := []string{"-n", namespace, podName, "--", "bash", "-c", command}
	msg, err := oc.WithoutNamespace().Run("exec").Args(command1...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with  err:%v  and output is %v.", err, msg)
		return msg, err
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

func execCommandInNetworkingPod(oc *exutil.CLI, command string) (string, error) {
	networkType := checkNetworkType(oc)
	var cmd []string
	if strings.Contains(networkType, "ovn") {
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-ovn-kubernetes", "-l", "app=ovnkube-node", "-o=jsonpath={.items[0].metadata.name}").Output()
		if err != nil {
			e2e.Logf("Cannot get onv-kubernetes pods, errors: %v", err)
			return "", err
		}
		cmd = []string{"-n", "openshift-ovn-kubernetes", "-c", "ovnkube-node", podName, "--", "/bin/sh", "-c", command}
	} else if strings.Contains(networkType, "sdn") {
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-sdn", "-l", "app=sdn", "-o=jsonpath={.items[0].metadata.name}").Output()
		if err != nil {
			e2e.Logf("Cannot get openshift-sdn pods, errors: %v", err)
			return "", err
		}
		cmd = []string{"-n", "openshift-sdn", "-c", "sdn", podName, "--", "/bin/sh", "-c", command}
	}

	msg, err := oc.WithoutNamespace().AsAdmin().Run("exec").Args(cmd...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with  err:%v .", err)
		return "", err
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

func getDefaultInterface(oc *exutil.CLI) (string, error) {
	getDefaultInterfaceCmd := "/usr/sbin/ip -4 route show default"
	int1, err := execCommandInNetworkingPod(oc, getDefaultInterfaceCmd)
	if err != nil {
		e2e.Logf("Cannot get default interface, errors: %v", err)
		return "", err
	}
	defInterface := strings.Split(int1, " ")[4]
	e2e.Logf("Get the default inteface: %s", defInterface)
	return defInterface, nil
}

func getDefaultSubnet(oc *exutil.CLI) (string, error) {
	int1, _ := getDefaultInterface(oc)
	getDefaultSubnetCmd := "/usr/sbin/ip -4 -brief a show " + int1
	subnet1, err := execCommandInNetworkingPod(oc, getDefaultSubnetCmd)
	defSubnet := strings.Fields(subnet1)[2]
	if err != nil {
		e2e.Logf("Cannot get default subnet, errors: %v", err)
		return "", err
	}
	e2e.Logf("Get the default subnet: %s", defSubnet)
	return defSubnet, nil
}

//Hosts function return the host network CIDR
func Hosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	e2e.Logf("in Hosts function, ip: %v, ipnet: %v", ip, ipnet)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// remove network address and broadcast address
	return ips[1 : len(ips)-1], nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func findUnUsedIPs(oc *exutil.CLI, cidr string, number int) []string {
	ipRange, _ := Hosts(cidr)
	var ipUnused = []string{}
	//shuffle the ips slice
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ipRange), func(i, j int) { ipRange[i], ipRange[j] = ipRange[j], ipRange[i] })
	for _, ip := range ipRange {
		if len(ipUnused) < number {
			pingCmd := "ping -c4 -t1 " + ip
			_, err := execCommandInNetworkingPod(oc, pingCmd)
			if err != nil {
				e2e.Logf("%s is not used!\n", ip)
				ipUnused = append(ipUnused, ip)
			}
		} else {
			break
		}

	}
	return ipUnused
}

func ipEchoServer() string {
	return "172.31.249.80:9095"
}

func checkPlatform(oc *exutil.CLI) string {
	output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	return strings.ToLower(output)
}

func checkNetworkType(oc *exutil.CLI) string {
	output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.type}").Output()
	return strings.ToLower(output)
}

func getDefaultIPv6Subnet(oc *exutil.CLI) (string, error) {
	int1, _ := getDefaultInterface(oc)
	getDefaultSubnetCmd := "/usr/sbin/ip -6 -brief a show " + int1
	subnet1, err := execCommandInNetworkingPod(oc, getDefaultSubnetCmd)
	if err != nil {
		e2e.Logf("Cannot get default ipv6 subnet, errors: %v", err)
		return "", err
	}
	defSubnet := strings.Fields(subnet1)[2]
	e2e.Logf("Get the default ipv6 subnet: %s", defSubnet)
	return defSubnet, nil
}

func findUnUsedIPv6(oc *exutil.CLI, cidr string, number int) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	number += 2
	var ips []string
	var i = 0
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		//Not use the first two IPv6 addresses , such as 2620:52:0:4e::  , 2620:52:0:4e::1
		if i == 0 || i == 1 {
			i++
			continue
		}
		//Start to detect the IPv6 adress is used or not
		pingCmd := "ping -c4 -t1 -6 " + ip.String()
		_, err := execCommandInNetworkingPod(oc, pingCmd)
		if err != nil && i < number {
			e2e.Logf("%s is not used!\n", ip)
			ips = append(ips, ip.String())
		} else if i >= number {
			break
		}
		i++
	}

	return ips, nil
}

func ipv6EchoServer(isIPv6 bool) string {
	if isIPv6 {
		return "[2620:52:0:4974:def4:1ff:fee7:8144]:8085"
	}
	return "10.73.116.56:8085"
}

func checkIPStackType(oc *exutil.CLI) string {
	svcNetwork, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.serviceNetwork}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Count(svcNetwork, ":") >= 2 && strings.Count(svcNetwork, ".") >= 2 {
		return "dualstack"
	} else if strings.Count(svcNetwork, ":") >= 2 {
		return "ipv6single"
	} else if strings.Count(svcNetwork, ".") >= 2 {
		return "ipv4single"
	}
	return ""
}

func installSctpModule(oc *exutil.CLI, configFile string) {
	status, _ := oc.AsAdmin().Run("get").Args("machineconfigs").Output()
	if !strings.Contains(status, "load-sctp-module") {
		err := oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func checkSctpModule(oc *exutil.CLI, nodeName string) {
	err := wait.Poll(30*time.Second, 15*time.Minute, func() (bool, error) {
		// Check nodes status to make sure all nodes are up after rebooting caused by load-sctp-module
		nodesStatus, err := oc.AsAdmin().Run("get").Args("node").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc_get_nodes: %v", nodesStatus)
		status, _ := oc.AsAdmin().Run("debug").Args("node/"+nodeName, "--", "cat", "/sys/module/sctp/initstate").Output()
		if strings.Contains(status, "live") {
			e2e.Logf("stcp module is installed in the %s", nodeName)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "stcp module is installed in the nodes")
}

func getPodIPv4(oc *exutil.CLI, namespace string, podName string) string {
	podIPv4, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s IP in namespace %s is %q", podName, namespace, podIPv4)
	return podIPv4
}

func getPodIPv6(oc *exutil.CLI, namespace string, podName string, ipStack string) string {
	if ipStack == "ipv6single" {
		podIPv6, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IP in namespace %s is %q", podName, namespace, podIPv6)
		return podIPv6
	} else if ipStack == "dualstack" {
		podIPv6, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[1].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IP in namespace %s is %q", podName, namespace, podIPv6)
		return podIPv6
	}
	return ""
}

// For normal user to create resources in the specified namespace from the file (not template)
func createResourceFromFile(oc *exutil.CLI, ns, file string) {
	err := oc.WithoutNamespace().Run("create").Args("-f", file, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForPodWithLabelReady(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(15*time.Second, 10*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label, "-ojsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}").Output()
		e2e.Logf("the Ready status of pod is %v", status)
		if err != nil || status == "" {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			e2e.Logf("the pod Ready status not met; wanted True but got %v, retrying...", status)
			return false, nil
		}
		return true, nil
	})
}

func getSvcIPv4(oc *exutil.CLI, namespace string, svcName string) string {
	svcIPv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The service %s IPv4 in namespace %s is %q", svcName, namespace, svcIPv4)
	return svcIPv4
}

func getSvcIPv6(oc *exutil.CLI, namespace string, svcName string) string {
	svcIPv6, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The service %s IPv6 in namespace %s is %q", svcName, namespace, svcIPv6)
	return svcIPv6
}

func getSvcIPdualstack(oc *exutil.CLI, namespace string, svcName string) (string, string) {
	svcIPv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The service %s IPv4 in namespace %s is %q", svcName, namespace, svcIPv4)
	svcIPv6, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[1]}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The service %s IPv6 in namespace %s is %q", svcName, namespace, svcIPv6)
	return svcIPv4, svcIPv6
}

// check if a configmap is created in specific namespace [usage: checkConfigMap(oc, namesapce, configmapName)]
func checkConfigMap(oc *exutil.CLI, ns, configmapName string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		searchOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "-n", ns).Output()
		if err != nil {
			e2e.Logf("failed to get configmap: %v", err)
			return false, nil
		}
		if o.Expect(searchOutput).To(o.ContainSubstring(configmapName)) {
			e2e.Logf("configmap %v found", configmapName)
			return true, nil
		}
		return false, nil
	})
}

func sshRunCmd(host string, user string, cmd string) error {
	privateKey := os.Getenv("SSH_CLOUD_PRIV_KEY")
	if privateKey == "" {
		privateKey = "../internal/config/keys/openshift-qe.pem"
	}
	sshClient := exutil.SshClient{User: user, Host: host, Port: 22, PrivateKey: privateKey}
	return sshClient.Run(cmd)
}

// For Admin to patch a resource in the specified namespace
func patchResourceAsAdmin(oc *exutil.CLI, resource, patch string) {
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(resource, "-p", patch, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Testing will exit when network operator is in abnormal state during 60 seconding of checking operator.
func checkNetworkOperatorDEGRADEDState(oc *exutil.CLI) {
	errCheck := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "network").Output()
		if err != nil {
			e2e.Logf("Fail to get clusteroperator network, error:%s. Trying again", err)
			return false, nil
		}
		matched, _ := regexp.MatchString("True.*False.*False", output)
		e2e.Logf("Network operator state is:%s", output)
		o.Expect(matched).To(o.BeTrue())
		return false, nil
	})
	o.Expect(errCheck.Error()).To(o.ContainSubstring("timed out waiting for the condition"))
}

func getNodeIPv4(oc *exutil.CLI, namespace string, nodeName string) string {
	nodeipv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", oc.Namespace(), "node", nodeName, "-o=jsonpath={.status.addresses[?(@.type==\"InternalIP\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		e2e.Logf("Cannot get node default interface ipv4 address, errors: %v", err)
	}
	e2e.Logf("The IPv4 of node's default interface is %q", nodeipv4)
	return nodeipv4
}

//Return IPv6 and IPv4 in vars respectively for Dual Stack and IPv4/IPv6 in 2nd var for single stack Clusters, and var1 will be nil in those cases
func getNodeIP(oc *exutil.CLI, nodeName string) (string, string) {
	ipStack := checkIPStackType(oc)
	if (ipStack == "ipv6single") || (ipStack == "ipv4single") {
		e2e.Logf("Its a Single Stack Cluster, either IPv4 or IPv6")
		InternalIP, err := oc.AsAdmin().Run("get").Args("node", nodeName, "-o=jsonpath={.status.addresses[?(@.type==\"InternalIP\")].address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The node's Internal IP is %q", InternalIP)
		return "", InternalIP
	}
	e2e.Logf("Its a Dual Stack Cluster")
	InternalIP1, err := oc.AsAdmin().Run("get").Args("node", nodeName, "-o=jsonpath={.status.addresses[?(@.type==\"InternalIP\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The node's 1st Internal IP is %q", InternalIP1)
	InternalIP2, err := oc.AsAdmin().Run("get").Args("node", nodeName, "-o=jsonpath={.status.addresses[?(@.type==\"InternalIP\")].address}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The node's 2nd Internal IP is %q", InternalIP2)
	if netutils.IsIPv6String(InternalIP1) {
		return InternalIP1, InternalIP2
	}
	return InternalIP2, InternalIP1
}

func getLeaderInfo(oc *exutil.CLI, namespace string, cmName string, networkType string) string {
	output1, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", cmName, "-n", namespace, "-o=jsonpath={.metadata.annotations.control-plane\\.alpha\\.kubernetes\\.io/leader}").OutputToFile("oc_describe_nodes.txt")
	o.Expect(err1).NotTo(o.HaveOccurred())
	output2, err2 := exec.Command("bash", "-c", "cat "+output1+" |  jq -r .holderIdentity").Output()
	o.Expect(err2).NotTo(o.HaveOccurred())
	leaderNodeName := strings.Trim(strings.TrimSpace(string(output2)), "\"")
	e2e.Logf("The leader node name is %s", leaderNodeName)
	if networkType == "ovnkubernetes" {
		_, leaderNodeIP := getNodeIP(oc, leaderNodeName)
		e2e.Logf("The leader node's IP is: %v", leaderNodeIP)
		return leaderNodeIP
	}
	ocGetPods, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-sdn", "pod", "-l app=sdn", "-o=wide").OutputToFile("ocgetpods.txt")
	o.Expect(err3).NotTo(o.HaveOccurred())
	rawGrepOutput, err3 := exec.Command("bash", "-c", "cat "+ocGetPods+" | grep "+leaderNodeName+" | awk '{print $1}'").Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	leaderPodName := strings.TrimSpace(string(rawGrepOutput))
	e2e.Logf("The leader Pod's name: %v", leaderPodName)
	return leaderPodName
}

func checkSDNMetrics(oc *exutil.CLI, url string, metrics string) {
	var metricsOutput []byte
	var metricsLog []byte
	olmToken, err := exutil.GetSAToken(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(olmToken).NotTo(o.BeEmpty())
	//olmToken, err := exutil.GetSAToken(oc)
	//olmToken, err :=oc.AsAdmin().WithoutNamespace().Run("create").Args("token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	//olmToken, err := getSAToken(oc, "prometheusk8s", "openshift-monitoring")
	//olmToken, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	//o.Expect(err).NotTo(o.HaveOccurred())
	metricsErr := wait.Poll(5*time.Second, 10*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), fmt.Sprintf("%s", url)).OutputToFile("metrics.txt")
		if err != nil {
			e2e.Logf("Can't get metrics and try again, the error is:%s", err)
			return false, nil
		}
		metricsLog, _ = exec.Command("bash", "-c", "cat "+output+" ").Output()
		metricsString := string(metricsLog)
		if strings.Contains(metricsString, "ovnkube_master_pod") {
			metricsOutput, _ = exec.Command("bash", "-c", "cat "+output+" | grep "+metrics+" | awk 'NR==1{print $2}'").Output()
		} else {
			metricsOutput, _ = exec.Command("bash", "-c", "cat "+output+" | grep "+metrics+" | awk 'NR==3{print $2}'").Output()
		}
		metricsValue := strings.TrimSpace(string(metricsOutput))
		if metricsValue != "" {
			e2e.Logf("The output of the metrics for %s is : %v", metrics, metricsValue)
		} else {
			e2e.Logf("Can't get metrics for %s:", metrics)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(metricsErr, fmt.Sprintf("Fail to get metric and the error is:%s", metricsErr))
}

func getEgressCIDRs(oc *exutil.CLI, node string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hostsubnet", node, "-o=jsonpath={.egressCIDRs}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("egressCIDR for hostsubnet node %v is: %v", node, output)
	return output
}

// get egressIP from a node
// When they are multiple egressIPs on the node, egressIp list is in format of ["10.0.247.116","10.0.156.51"]
// as an example from the output of command "oc get hostsubnet <node> -o=jsonpath={.egressIPs}"
// convert the iplist into an array of ip addresses
func getEgressIPonSDNHost(oc *exutil.CLI, node string, expectedNum int) ([]string, error) {
	var ip = []string{}
	iplist, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hostsubnet", node, "-o=jsonpath={.egressIPs}").Output()
	if iplist != "" {
		ip = strings.Split(iplist[2:len(iplist)-2], "\",\"")
	}
	if iplist == "" || len(ip) < expectedNum || err != nil {
		err = wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
			iplist, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("hostsubnet", node, "-o=jsonpath={.egressIPs}").Output()
			if iplist != "" {
				ip = strings.Split(iplist[2:len(iplist)-2], "\",\"")
			}
			if len(ip) < expectedNum || err != nil {
				e2e.Logf("only got %d egressIP, or have err:%v, and try next round", len(ip), err)
				return false, nil
			}
			if iplist != "" && len(ip) == expectedNum {
				e2e.Logf("Found egressIP list for node %v is: %v", node, iplist)
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("Only got %d egressIP, or have err:%v", len(ip), err)
		return ip, err
	}
	return ip, nil
}

func getPodName(oc *exutil.CLI, namespace string, label string) []string {
	var podName []string
	podNameAll, err := oc.AsAdmin().Run("get").Args("-n", namespace, "pod", "-l", label, "-ojsonpath={.items..metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podName = strings.Split(podNameAll, " ")
	e2e.Logf("The pod(s) are  %v ", podName)
	return podName
}

// starting from first node, compare its subnet with subnet of subsequent nodes in the list
// until two nodes with same subnet found, otherwise, return false to indicate that no two nodes with same subnet found
func findTwoNodesWithSameSubnet(oc *exutil.CLI, nodeList *v1.NodeList) (bool, [2]string) {
	var nodes [2]string
	for i := 0; i < (len(nodeList.Items) - 1); i++ {
		for j := i + 1; j < len(nodeList.Items); j++ {
			firstSub := getIfaddrFromNode(nodeList.Items[i].Name, oc)
			secondSub := getIfaddrFromNode(nodeList.Items[j].Name, oc)
			if firstSub == secondSub {
				e2e.Logf("Found nodes with same subnet.")
				nodes[0] = nodeList.Items[i].Name
				nodes[1] = nodeList.Items[j].Name
				return true, nodes
			}
		}
	}
	return false, nodes
}

func getSDNMetrics(oc *exutil.CLI, podName string) string {
	var metricsLog string
	metricsErr := wait.Poll(5*time.Second, 10*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-sdn", fmt.Sprintf("%s", podName), "--", "curl", "localhost:29100/metrics").OutputToFile("metrics.txt")
		if err != nil {
			e2e.Logf("Can't get metrics and try again, the error is:%s", err)
			return false, nil
		}
		metricsLog = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(metricsErr, fmt.Sprintf("Fail to get metric and the error is:%s", metricsErr))
	return metricsLog
}

func getOVNMetrics(oc *exutil.CLI, url string) string {
	var metricsLog string
	olmToken, err := exutil.GetSAToken(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(olmToken).NotTo(o.BeEmpty())
	//olmToken, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	//olmToken, err := getSAToken(oc, "prometheusk8s", "openshift-monitoring")
	//o.Expect(err).NotTo(o.HaveOccurred())
	metricsErr := wait.Poll(5*time.Second, 10*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", olmToken), fmt.Sprintf("%s", url)).OutputToFile("metrics.txt")
		if err != nil {
			e2e.Logf("Can't get metrics and try again, the error is:%s", err)
			return false, nil
		}
		metricsLog = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(metricsErr, fmt.Sprintf("Fail to get metric and the error is:%s", metricsErr))
	return metricsLog
}

func checkIPsec(oc *exutil.CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.ovnKubernetesConfig.ipsecConfig}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return output
}

func getAssignedEIPInEIPObject(oc *exutil.CLI, egressIPObject string) []map[string]string {
	var egressIPs string
	egressipErr := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
		egressIPStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("egressip", egressIPObject, "-ojsonpath={.status.items}").Output()
		if err != nil {
			e2e.Logf("Wait to get EgressIP object applied,try next round. %v", err)
			return false, nil
		}
		if egressIPStatus == "" {
			e2e.Logf("Wait to get EgressIP object applied,try next round. %v", err)
			return false, nil
		}
		egressIPs = egressIPStatus
		e2e.Logf("egressIPStatus: %v", egressIPs)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("Failed to apply egressIPs:%s", egressipErr))

	var egressIPJsonMap []map[string]string
	json.Unmarshal([]byte(egressIPs), &egressIPJsonMap)
	return egressIPJsonMap
}

func rebootNode(oc *exutil.CLI, nodeName string) {
	e2e.Logf("\nRebooting node %s....", nodeName)
	_, err1 := exutil.DebugNodeWithChroot(oc, nodeName, "shutdown", "-r", "+1")
	o.Expect(err1).NotTo(o.HaveOccurred())
}

func checkNodeStatus(oc *exutil.CLI, nodeName string, expectedStatus string) {
	var expectedStatus1 string
	if expectedStatus == "Ready" {
		expectedStatus1 = "True"
	} else if expectedStatus == "NotReady" {
		expectedStatus1 = "Unknown"
	} else {
		err1 := fmt.Errorf("TBD supported node status")
		o.Expect(err1).NotTo(o.HaveOccurred())
	}
	err := wait.Poll(10*time.Second, 6*time.Minute, func() (bool, error) {
		statusOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-ojsonpath={.status.conditions[-1].status}").Output()
		if err != nil {
			e2e.Logf("\nGet node status with error : %v", err)
			return false, nil
		}
		e2e.Logf("Node %s kubelet status is %s", nodeName, statusOutput)
		if statusOutput != expectedStatus1 {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Node %s is not in expected status %s", nodeName, expectedStatus))
}

func updateEgressIPObject(oc *exutil.CLI, egressIPObjectName string, egressIP string) {
	patchResourceAsAdmin(oc, "egressip/"+egressIPObjectName, "{\"spec\":{\"egressIPs\":[\""+egressIP+"\"]}}")
	egressipErr := wait.Poll(10*time.Second, 100*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("egressip", egressIPObjectName, "-o=jsonpath={.status.items[*]}").Output()
		if err != nil {
			e2e.Logf("Wait to get EgressIP object applied,try next round. %v", err)
			return false, nil
		}
		if !strings.Contains(output, egressIP) {
			e2e.Logf("Wait for new IP applied,try next round.")
			e2e.Logf(output)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(egressipErr, fmt.Sprintf("Failed to apply new egressIPs:%s", egressipErr))
}

func getTwoNodesSameSubnet(oc *exutil.CLI, nodeList *v1.NodeList) (bool, []string) {
	var egressNodes []string
	if len(nodeList.Items) < 2 {
		e2e.Logf("Not enough nodes available for the test, skip the case!!")
		return false, nil
	}
	switch exutil.CheckPlatform(oc) {
	case "aws":
		e2e.Logf("find the two nodes that have same subnet")
		check, nodes := findTwoNodesWithSameSubnet(oc, nodeList)
		if check {
			egressNodes = nodes[:2]
		} else {
			e2e.Logf("No more than 2 worker nodes in same subnet, skip the test!!!")
			return false, nil
		}
	case "gcp":
		e2e.Logf("since GCP worker nodes all have same subnet, just pick first two nodes as egress nodes")
		egressNodes = append(egressNodes, nodeList.Items[0].Name)
		egressNodes = append(egressNodes, nodeList.Items[1].Name)
	default:
		e2e.Logf("Not supported platform yet!")
		return false, nil
	}

	return true, egressNodes
}

/*getSvcIP returns IPv6 and IPv4 in vars in order on dual stack respectively and main Svc IP in case of single stack (v4 or v6) in 1st var, and nil in 2nd var.
LoadBalancer svc will return Ingress VIP in var1, v4 or v6 and NodePort svc will return Ingress SvcIP in var1 and NodePort in var2*/
func getSvcIP(oc *exutil.CLI, namespace string, svcName string) (string, string) {
	ipStack := checkIPStackType(oc)
	svctype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ipFamilyType, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.ipFamilyPolicy}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if (svctype == "ClusterIP") || (svctype == "NodePort") {
		if (ipStack == "ipv6single") || (ipStack == "ipv4single") {
			svcIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if svctype == "ClusterIP" {
				e2e.Logf("The service %s IP in namespace %s is %q", svcName, namespace, svcIP)
				return svcIP, ""
			}
			nodePort, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.ports[*].nodePort}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The NodePort service %s IP and NodePort in namespace %s is %s %s", svcName, namespace, svcIP, nodePort)
			return svcIP, nodePort

		} else if (ipStack == "dualstack" && ipFamilyType == "PreferDualStack") || (ipStack == "dualstack" && ipFamilyType == "RequireDualStack") {
			ipFamilyPrecedence, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.ipFamilies[0]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			//if IPv4 is listed first in ipFamilies then clustrIPs allocation will take order as Ipv4 first and then Ipv6 else reverse
			svcIPv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The service %s IP in namespace %s is %q", svcName, namespace, svcIPv4)
			svcIPv6, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[1]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The service %s IP in namespace %s is %q", svcName, namespace, svcIPv6)
			/*As stated Nodeport type svc will return node port value in 2nd var. We don't care about what svc address is coming in 1st var as we evetually going to get
			node IPs later and use that in curl operation to node_ip:nodeport*/
			if ipFamilyPrecedence == "IPv4" {
				e2e.Logf("The ipFamilyPrecedence is Ipv4, Ipv6")
				switch svctype {
				case "NodePort":
					nodePort, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.ports[*].nodePort}").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("The Dual Stack NodePort service %s IP and NodePort in namespace %s is %s %s", svcName, namespace, svcIPv4, nodePort)
					return svcIPv4, nodePort
				default:
					return svcIPv6, svcIPv4
				}
			} else {
				e2e.Logf("The ipFamilyPrecedence is Ipv6, Ipv4")
				switch svctype {
				case "NodePort":
					nodePort, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.ports[*].nodePort}").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("The Dual Stack NodePort service %s IP and NodePort in namespace %s is %s %s", svcName, namespace, svcIPv6, nodePort)
					return svcIPv6, nodePort
				default:
					svcIPv4, svcIPv6 = svcIPv6, svcIPv4
					return svcIPv6, svcIPv4
				}
			}
		} else {
			//Its a Dual Stack Cluster with SingleStack ipFamilyPolicy
			svcIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The service %s IP in namespace %s is %q", svcName, namespace, svcIP)
			return svcIP, ""
		}
	} else {
		//Loadbalancer will be supported for single stack Ipv4 here for mostly GCP,Azure. We can take further enhancements wrt Metal platforms in Metallb utils later
		e2e.Logf("The serviceType is LoadBalancer")
		err := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			svcIP, er := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}").Output()
			o.Expect(er).NotTo(o.HaveOccurred())
			if svcIP == "" {
				e2e.Logf("Waiting for lb service IP assignment. Trying again...")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to assign lb svc IP to %v", svcName))
		lbSvcIP, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", namespace, svcName, "-o=jsonpath={.status.loadBalancer.ingress[0].ip}").Output()
		e2e.Logf("The %s lb service Ingress VIP in namespace %s is %q", svcName, namespace, lbSvcIP)
		return lbSvcIP, ""
	}
}

//getPodIP returns IPv6 and IPv4 in vars in order on dual stack respectively and main IP in case of single stack (v4 or v6) in 1st var, and nil in 2nd var
func getPodIP(oc *exutil.CLI, namespace string, podName string) (string, string) {
	ipStack := checkIPStackType(oc)
	if (ipStack == "ipv6single") || (ipStack == "ipv4single") {
		podIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IP in namespace %s is %q", podName, namespace, podIP)
		return podIP, ""
	} else if ipStack == "dualstack" {
		podIPv6, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[1].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IPv6 in namespace %s is %q", podName, namespace, podIPv6)
		podIPv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IPv4 in namespace %s is %q", podName, namespace, podIPv4)
		return podIPv6, podIPv4
	}
	return "", ""
}

//CurlPod2PodPass checks connectivity across pods regardless of network addressing type on cluster
func CurlPod2PodPass(oc *exutil.CLI, namespaceSrc string, podNameSrc string, namespaceDst string, podNameDst string) {
	podIP1, podIP2 := getPodIP(oc, namespaceDst, podNameDst)
	if podIP2 != "" {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP1, "8080"))
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP2, "8080"))
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP1, "8080"))
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//CurlPod2PodFail ensures no connectivity from a pod to pod regardless of network addressing type on cluster
func CurlPod2PodFail(oc *exutil.CLI, namespaceSrc string, podNameSrc string, namespaceDst string, podNameDst string) {
	podIP1, podIP2 := getPodIP(oc, namespaceDst, podNameDst)
	if podIP2 != "" {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP1, "8080"))
		o.Expect(err).To(o.HaveOccurred())
		_, err = e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP2, "8080"))
		o.Expect(err).To(o.HaveOccurred())
	} else {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(podIP1, "8080"))
		o.Expect(err).To(o.HaveOccurred())
	}
}

//CurlNode2PodPass checks node to pod connectivity regardless of network addressing type on cluster
func CurlNode2PodPass(oc *exutil.CLI, nodeName string, namespace string, podName string) {
	//getPodIP returns IPv6 and IPv4 in order on dual stack in PodIP1 and PodIP2 respectively and main IP in case of single stack (v4 or v6) in PodIP1, and nil in PodIP2
	podIP1, podIP2 := getPodIP(oc, namespace, podName)
	if podIP2 != "" {
		podv6URL := net.JoinHostPort(podIP1, "8080")
		podv4URL := net.JoinHostPort(podIP2, "8080")
		_, err := exutil.DebugNode(oc, nodeName, "curl", podv4URL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = exutil.DebugNode(oc, nodeName, "curl", podv6URL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		podURL := net.JoinHostPort(podIP1, "8080")
		_, err := exutil.DebugNode(oc, nodeName, "curl", podURL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//CurlNode2SvcPass checks node to svc connectivity regardless of network addressing type on cluster
func CurlNode2SvcPass(oc *exutil.CLI, nodeName string, namespace string, svcName string) {
	svcIP1, svcIP2 := getSvcIP(oc, namespace, svcName)
	if svcIP2 != "" {
		svc6URL := net.JoinHostPort(svcIP1, "27017")
		svc4URL := net.JoinHostPort(svcIP2, "27017")
		_, err := exutil.DebugNode(oc, nodeName, "curl", svc4URL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = exutil.DebugNode(oc, nodeName, "curl", svc6URL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		svcURL := net.JoinHostPort(svcIP1, "27017")
		_, err := exutil.DebugNode(oc, nodeName, "curl", svcURL, "-s", "--connect-timeout", "5")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//CurlNode2SvcFail checks node to svc connectivity regardless of network addressing type on cluster
func CurlNode2SvcFail(oc *exutil.CLI, nodeName string, namespace string, svcName string) {
	svcIP1, svcIP2 := getSvcIP(oc, namespace, svcName)
	if svcIP2 != "" {
		svc6URL := net.JoinHostPort(svcIP1, "27017")
		svc4URL := net.JoinHostPort(svcIP2, "27017")
		output, _ := exutil.DebugNode(oc, nodeName, "curl", svc4URL, "--connect-timeout", "5")
		o.Expect(output).To(o.Or(o.ContainSubstring("28"), o.ContainSubstring("Failed")))
		output, _ = exutil.DebugNode(oc, nodeName, "curl", svc6URL, "--connect-timeout", "5")
		o.Expect(output).To(o.Or(o.ContainSubstring("28"), o.ContainSubstring("Failed")))
	} else {
		svcURL := net.JoinHostPort(svcIP1, "27017")
		output, _ := exutil.DebugNode(oc, nodeName, "curl", svcURL, "--connect-timeout", "5")
		o.Expect(output).To(o.Or(o.ContainSubstring("28"), o.ContainSubstring("Failed")))
	}
}

//CurlPod2SvcPass checks pod to svc connectivity regardless of network addressing type on cluster
func CurlPod2SvcPass(oc *exutil.CLI, namespaceSrc string, namespaceSvc string, podNameSrc string, svcName string) {
	svcIP1, svcIP2 := getSvcIP(oc, namespaceSvc, svcName)
	if svcIP2 != "" {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP1, "27017"))
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP2, "27017"))
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP1, "27017"))
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//CurlPod2SvcFail ensures no connectivity from a pod to svc regardless of network addressing type on cluster
func CurlPod2SvcFail(oc *exutil.CLI, namespaceSrc string, namespaceSvc string, podNameSrc string, svcName string) {
	svcIP1, svcIP2 := getSvcIP(oc, namespaceSvc, svcName)
	if svcIP2 != "" {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP1, "27017"))
		o.Expect(err).To(o.HaveOccurred())
		_, err = e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP2, "27017"))
		o.Expect(err).To(o.HaveOccurred())
	} else {
		_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl --connect-timeout 5 -s "+net.JoinHostPort(svcIP1, "27017"))
		o.Expect(err).To(o.HaveOccurred())
	}
}

func checkProxy(oc *exutil.CLI) bool {
	httpProxy, err := doAction(oc, "get", true, true, "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	httpsProxy, err := doAction(oc, "get", true, true, "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	if httpProxy != "" || httpsProxy != "" {
		return true
	}
	return false
}

// SDNHostwEgressIP find out which egress node has the egressIP
func SDNHostwEgressIP(oc *exutil.CLI, node []string, egressip string) string {
	var ip []string
	var foundHost string
	for i := 0; i < len(node); i++ {
		iplist, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hostsubnet", node[i], "-o=jsonpath={.egressIPs}").Output()
		if iplist != "" {
			ip = strings.Split(iplist[2:len(iplist)-2], "\",\"")
		}
		if iplist == "" || err != nil {
			err = wait.Poll(30*time.Second, 3*time.Minute, func() (bool, error) {
				iplist, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("hostsubnet", node[i], "-o=jsonpath={.egressIPs}").Output()
				if iplist != "" {
					e2e.Logf("Found egressIP list for node %v is: %v", node, iplist)
					ip = strings.Split(iplist[2:len(iplist)-2], "\",\"")
					return true, nil
				}
				if err != nil {
					e2e.Logf("only got %d egressIP, or have err:%v, and try next round", len(ip), err)
					return false, nil
				}
				return false, nil
			})
		}
		if isValueInList(egressip, ip) {
			foundHost = node[i]
			break
		}
	}
	return foundHost
}

func isValueInList(value string, list []string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// getPodMultiNetwork is designed to get both v4 and v6 addresses from pod's secondary interface(net1) which is not in the cluster's SDN or OVN network
// currently the v4 address of pod's secondary interface is always displyed before v6 address no matter the order configred in the net-attach-def YAML file
func getPodMultiNetwork(oc *exutil.CLI, namespace string, podName string) (string, string) {
	cmd1 := "ip a sho net1 | awk 'NR==3{print $2}' |grep -Po '((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])'"
	cmd2 := "ip a sho net1 | awk 'NR==5{print $2}' |grep -Po '([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4}'"
	podIPv4, err := e2e.RunHostCmd(namespace, podName, cmd1)
	o.Expect(err).NotTo(o.HaveOccurred())
	podIPv6, err1 := e2e.RunHostCmd(namespace, podName, cmd2)
	o.Expect(err1).NotTo(o.HaveOccurred())
	return podIPv4, podIPv6
}

//Pinging pod's secondary interfaces should pass
func curlPod2PodMultiNetworkPass(oc *exutil.CLI, namespaceSrc string, podNameSrc string, podIPv4 string, podIPv6 string) {
	msg, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl  "+podIPv4+":8080  --connect-timeout 5")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(strings.Contains(msg, "Hello OpenShift!")).To(o.BeTrue())
	//MultiNetworkPolicy not support ipv6 yet, disabel ipv6 curl right now
	//msg1, err1 := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl -g -6 [" +podIPv6+ "]:8080  --connect-timeout 5")
	//o.Expect(err1).NotTo(o.HaveOccurred())
	//o.Expect(strings.Contains(msg1, "Hello OpenShift!")).To(o.BeTrue())
}

//Pinging pod's secondary interfaces should fail
func curlPod2PodMultiNetworkFail(oc *exutil.CLI, namespaceSrc string, podNameSrc string, podIPv4 string, podIPv6 string) {
	_, err := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl  "+podIPv4+":8080  --connect-timeout 5")
	o.Expect(err).To(o.HaveOccurred())
	//MultiNetworkPolicy not support ipv6 yet, disabel ipv6 curl right now
	//_, err1 := e2e.RunHostCmd(namespaceSrc, podNameSrc, "curl -g -6 [" +podIPv6+ "]:8080  --connect-timeout 5")
	//o.Expect(err1).To(o.HaveOccurred())
}

//This function will bring 2 namespaces, 5 pods and 2 NADs for all multus multinetworkpolicy cases
func prepareMultinetworkTest(oc *exutil.CLI, ns1 string, ns2 string, patchInfo string) {
	nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(nodeList.Items) < 2 {
		g.Skip("This case requires 2 nodes, but the cluster has less than two nodes")
	}

	buildPruningBaseDir := exutil.FixturePath("testdata", "networking/multinetworkpolicy")
	netAttachDefFile1 := filepath.Join(buildPruningBaseDir, "MultiNetworkPolicy-NAD1.yaml")
	netAttachDefFile2 := filepath.Join(buildPruningBaseDir, "MultiNetworkPolicy-NAD2.yaml")
	pingPodTemplate := filepath.Join(buildPruningBaseDir, "MultiNetworkPolicy-pod-template.yaml")
	patchSResource := "networks.operator.openshift.io/cluster"

	g.By("Enable MacvlanNetworkpolicy in the cluster")
	patchResourceAsAdmin(oc, patchSResource, patchInfo)

	g.By("Create first namespace")
	nserr1 := oc.Run("new-project").Args(ns1).Execute()
	o.Expect(nserr1).NotTo(o.HaveOccurred())
	_, proerr1 := oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns1, "user="+ns1).Output()
	o.Expect(proerr1).NotTo(o.HaveOccurred())

	g.By("Create MultiNetworkPolicy-NAD1 in ns1")
	err1 := oc.AsAdmin().Run("create").Args("-f", netAttachDefFile1, "-n", ns1).Execute()
	o.Expect(err1).NotTo(o.HaveOccurred())
	output, err2 := oc.Run("get").Args("net-attach-def", "-n", ns1).Output()
	o.Expect(err2).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("macvlan-nad1"))

	g.By("Create 1st pod in ns1")
	pod1ns1 := testPodMultinetwork{
		name:      "blue-pod-1",
		namespace: ns1,
		nodename:  nodeList.Items[0].Name,
		nadname:   "macvlan-nad1",
		labelname: "blue-openshift",
		template:  pingPodTemplate,
	}
	pod1ns1.createTestPodMultinetwork(oc)
	waitPodReady(oc, pod1ns1.namespace, pod1ns1.name)

	g.By("Create second pod in ns1")
	pod2ns1 := testPodMultinetwork{
		name:      "blue-pod-2",
		namespace: ns1,
		nodename:  nodeList.Items[1].Name,
		nadname:   "macvlan-nad1",
		labelname: "blue-openshift",
		template:  pingPodTemplate,
	}
	pod2ns1.createTestPodMultinetwork(oc)
	waitPodReady(oc, pod2ns1.namespace, pod2ns1.name)

	g.By("Create third pod in ns1")
	pod3ns1 := testPodMultinetwork{
		name:      "red-pod-1",
		namespace: ns1,
		nodename:  nodeList.Items[0].Name,
		nadname:   "macvlan-nad1",
		labelname: "red-openshift",
		template:  pingPodTemplate,
	}
	pod3ns1.createTestPodMultinetwork(oc)
	waitPodReady(oc, pod3ns1.namespace, pod3ns1.name)

	g.By("Create second namespace")
	nserr2 := oc.Run("new-project").Args(ns2).Execute()
	o.Expect(nserr2).NotTo(o.HaveOccurred())
	_, proerr2 := oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", ns2, "user="+ns2).Output()
	o.Expect(proerr2).NotTo(o.HaveOccurred())

	g.By("Create MultiNetworkPolicy-NAD2 in ns2")
	err4 := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", netAttachDefFile2, "-n", ns2).Execute()
	o.Expect(err4).NotTo(o.HaveOccurred())
	output, err5 := oc.Run("get").Args("net-attach-def", "-n", ns2).Output()
	o.Expect(err5).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("macvlan-nad2"))

	g.By("Create 1st pod in ns2")
	pod1ns2 := testPodMultinetwork{
		name:      "blue-pod-3",
		namespace: ns2,
		nodename:  nodeList.Items[0].Name,
		nadname:   "macvlan-nad2",
		labelname: "blue-openshift",
		template:  pingPodTemplate,
	}
	pod1ns2.createTestPodMultinetwork(oc)
	waitPodReady(oc, pod1ns2.namespace, pod1ns2.name)

	g.By("Create second pod in ns2")
	pod2ns2 := testPodMultinetwork{
		name:      "red-pod-2",
		namespace: ns2,
		nodename:  nodeList.Items[0].Name,
		nadname:   "macvlan-nad2",
		labelname: "red-openshift",
		template:  pingPodTemplate,
	}
	pod2ns2.createTestPodMultinetwork(oc)
	waitPodReady(oc, pod2ns2.namespace, pod2ns2.name)
}

// check if an ip address is added to node's NIC, or removed from node's NIC
func checkPrimaryNIC(oc *exutil.CLI, nodeName string, ip string, flag bool) {
	checkErr := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		output, err := exutil.DebugNodeWithChroot(oc, nodeName, "bash", "-c", "/usr/sbin/ip -4 -brief address show")
		if err != nil {
			e2e.Logf("Cannot get primary NIC interface, errors: %v, try again", err)
			return false, nil
		}
		if flag && !strings.Contains(output, ip) {
			e2e.Logf("egressIP has not been added to node's NIC correctly, try again")
			return false, nil
		}
		if !flag && strings.Contains(output, ip) {
			e2e.Logf("egressIP has not been removed from node's NIC correctly, try again")
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(checkErr, fmt.Sprintf("Failed to get NIC on the host:%s", checkErr))
}

func checkEgressIPonSDNHost(oc *exutil.CLI, node string, expectedEgressIP []string) {
	checkErr := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		ip, err := getEgressIPonSDNHost(oc, node, len(expectedEgressIP))
		if err != nil {
			e2e.Logf("\n got the error: %v\n, try again", err)
			return false, nil
		}
		if !unorderedEqual(ip, expectedEgressIP) {
			e2e.Logf("\n got egressIP as %v while expected egressIP is %v, try again", ip, expectedEgressIP)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(checkErr, fmt.Sprintf("Failed to get egressIP on the host:%s", checkErr))
}

func unorderedEqual(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for _, value := range first {
		if !contains(second, value) {
			return false
		}
	}
	return true
}
