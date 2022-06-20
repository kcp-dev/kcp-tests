package networking

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"net"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Get AWS credential from cluster
func getAwsCredentialFromCluster(oc *exutil.CLI) {
	if exutil.CheckPlatform(oc) != "aws" {
		g.Skip("it is not aws platform and can not get credential, and then skip it.")
	}
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	// Skip for sts and c2s clusters.
	if err != nil {
		g.Skip("Did not get credential to update security rule, skip the testing.")

	}
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIDBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).String(), gjson.Get(credential, `data.aws_secret_access_key`).String()
	accessKeyID, err1 := base64.StdEncoding.DecodeString(accessKeyIDBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	clusterRegion, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyID))
	os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
	os.Setenv("AWS_REGION", clusterRegion)

}

// Get AWS int svc instance ID
func getAwsIntSvcInstanceID(a *exutil.AwsClient, oc *exutil.CLI) (string, error) {
	clusterPrefixName := exutil.GetClusterPrefixName(oc)
	instanceName := clusterPrefixName + "-int-svc"
	instanceID, err := a.GetAwsInstanceID(instanceName)
	if err != nil {
		e2e.Logf("Get bastion instance id failed with error %v .", err)
		return "", err
	}
	return instanceID, nil
}

// Get int svc instance private ip and public ip
func getAwsIntSvcIPs(a *exutil.AwsClient, oc *exutil.CLI) map[string]string {
	instanceID, err := getAwsIntSvcInstanceID(a, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	ips, err := a.GetAwsIntIPs(instanceID)
	o.Expect(err).NotTo(o.HaveOccurred())
	return ips
}

//Update int svc instance ingress rule to allow destination port
func updateAwsIntSvcSecurityRule(a *exutil.AwsClient, oc *exutil.CLI, dstPort int64) {
	instanceID, err := getAwsIntSvcInstanceID(a, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = a.UpdateAwsIntSecurityRule(instanceID, dstPort)
	o.Expect(err).NotTo(o.HaveOccurred())

}

func installIPEchoServiceOnAWS(a *exutil.AwsClient, oc *exutil.CLI) (string, error) {
	user := os.Getenv("SSH_CLOUD_PRIV_AWS_USER")
	if user == "" {
		user = "core"
	}

	sshkey := os.Getenv("SSH_CLOUD_PRIV_KEY")
	if sshkey == "" {
		sshkey = "../internal/config/keys/openshift-qe.pem"
	}
	command := "sudo netstat -ntlp | grep 9095 || sudo podman run --name ipecho -d -p 9095:80 quay.io/openshifttest/ip-echo:multiarch"
	e2e.Logf("Run command", command)

	ips := getAwsIntSvcIPs(a, oc)
	publicIP, ok := ips["publicIP"]
	if !ok {
		return "", fmt.Errorf("no public IP found for Int Svc instance")
	}
	privateIP, ok := ips["privateIP"]
	if !ok {
		return "", fmt.Errorf("no private IP found for Int Svc instance")
	}

	sshClient := exutil.SshClient{User: user, Host: publicIP, Port: 22, PrivateKey: sshkey}
	err := sshClient.Run(command)
	if err != nil {
		e2e.Logf("Failed to run %v: %v", command, err)
		return "", err
	}

	updateAwsIntSvcSecurityRule(a, oc, 9095)

	ipEchoURL := net.JoinHostPort(privateIP, "9095")
	return ipEchoURL, nil
}

func getIfaddrFromNode(nodeName string, oc *exutil.CLI) string {
	egressIpconfig, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.annotations.cloud\\.network\\.openshift\\.io/egress-ipconfig}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The egressipconfig is %v", egressIpconfig)
	ifaddr := strings.Split(egressIpconfig, "\"")[9]
	e2e.Logf("The subnet of node %s is %v .", nodeName, ifaddr)
	return ifaddr
}

func findUnUsedIPsOnNode(oc *exutil.CLI, nodeName, cidr string, number int) []string {
	ipRange, _ := Hosts(cidr)
	var ipUnused = []string{}
	//shuffle the ips slice
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ipRange), func(i, j int) { ipRange[i], ipRange[j] = ipRange[j], ipRange[i] })
	networkType := checkNetworkType(oc)
	var msg string
	var err error
	for _, ip := range ipRange {
		if len(ipUnused) < number {
			pingCmd := "ping -c4 -t1 " + ip
			if strings.Contains(networkType, "ovn") {
				msg, err = execCommandInOVNPodOnNode(oc, nodeName, pingCmd)
			}
			if strings.Contains(networkType, "sdn") {
				msg, err = execCommandInSDNPodOnNode(oc, nodeName, pingCmd)
			}
			if err != nil && (strings.Contains(msg, "Destination Host Unreachable") || strings.Contains(msg, "100% packet loss")) {
				e2e.Logf("%s is not used!\n", ip)
				ipUnused = append(ipUnused, ip)
			} else if err != nil {
				break
			}
		} else {
			break
		}

	}
	return ipUnused
}

func execCommandInOVNPodOnNode(oc *exutil.CLI, nodeName, command string) (string, error) {
	ovnPodName, err := exutil.GetPodName(oc, "openshift-ovn-kubernetes", "app=ovnkube-node", nodeName)
	o.Expect(err).NotTo(o.HaveOccurred())
	msg, err := exutil.RemoteShPodWithBash(oc, "openshift-ovn-kubernetes", ovnPodName, command)
	if err != nil {
		e2e.Logf("Execute ovn command failed with  err:%v .", err)
		return msg, err
	}
	return msg, nil
}

func execCommandInSDNPodOnNode(oc *exutil.CLI, nodeName, command string) (string, error) {
	sdnPodName, err := exutil.GetPodName(oc, "openshift-sdn", "app=sdn", nodeName)
	o.Expect(err).NotTo(o.HaveOccurred())
	msg, err := exutil.RemoteShPodWithBash(oc, "openshift-sdn", sdnPodName, command)
	if err != nil {
		e2e.Logf("Execute sdn command failed with  err:%v .", err)
		return msg, err
	}
	return msg, nil
}

func getgcloudClient(oc *exutil.CLI) *exutil.Gcloud {
	if exutil.CheckPlatform(oc) != "gcp" {
		g.Skip("it is not gcp platform!")
	}
	projectID, err := exutil.GetGcpProjectId(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	if projectID != "openshift-qe" {
		g.Skip("openshift-qe project is needed to execute this test case!")
	}
	gcloud := exutil.Gcloud{ProjectID: projectID}
	return gcloud.Login()
}

func getIntSvcExternalIPFromGcp(oc *exutil.CLI, infraID string) (string, error) {
	externalIP, err := getgcloudClient(oc).GetIntSvcExternalIP(infraID)
	e2e.Logf("Additional VM external ip: %s", externalIP)
	return externalIP, err
}

func installIPEchoServiceOnGCP(oc *exutil.CLI, infraID string, host string) (string, error) {
	e2e.Logf("Infra id: %s, install ipecho service on host %s", infraID, host)

	// Run ip-echo service on the additional VM
	serviceName := "ip-echo"
	internalIP, err := getgcloudClient(oc).GetIntSvcInternalIP(infraID)
	o.Expect(err).NotTo(o.HaveOccurred())
	port := "9095"
	runIPEcho := fmt.Sprintf("sudo netstat -ntlp | grep %s || sudo podman run --name %s -d -p %s:80 quay.io/openshifttest/ip-echo:multiarch", port, serviceName, port)
	user := os.Getenv("SSH_CLOUD_PRIV_GCP_USER")
	if user == "" {
		user = "cloud-user"
	}
	//o.Expect(sshRunCmd(host, user, runIPEcho)).NotTo(o.HaveOccurred())
	err = sshRunCmd(host, user, runIPEcho)
	if err != nil {
		e2e.Logf("Failed to run %v: %v", runIPEcho, err)
		return "", err
	}

	// Update firewall rules to expose ip-echo service
	ruleName := fmt.Sprintf("%s-int-svc-ingress-allow", infraID)
	ports, err := getgcloudClient(oc).GetFirewallAllowPorts(ruleName)
	if err != nil {
		e2e.Logf("Failed to update firewall rules for port %v: %v", ports, err)
		return "", err
	}
	//o.Expect(err).NotTo(o.HaveOccurred())
	if !strings.Contains(ports, "tcp:"+port) {
		addIPEchoPort := fmt.Sprintf("%s,tcp:%s", ports, port)
		o.Expect(getgcloudClient(oc).UpdateFirewallAllowPorts(ruleName, addIPEchoPort)).NotTo(o.HaveOccurred())
		e2e.Logf("Allow Ports: %s", addIPEchoPort)
	}
	ipEchoURL := net.JoinHostPort(internalIP, port)
	return ipEchoURL, nil
}

func uninstallIPEchoServiceOnGCP(oc *exutil.CLI) {
	infraID, err := exutil.GetInfraId(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	host, err := getIntSvcExternalIPFromGcp(oc, infraID)
	o.Expect(err).NotTo(o.HaveOccurred())
	//Remove ip-echo service
	user := os.Getenv("SSH_CLOUD_PRIV_GCP_USER")
	if user == "" {
		user = "cloud-user"
	}
	o.Expect(sshRunCmd(host, user, "sudo podman rm ip-echo -f")).NotTo(o.HaveOccurred())
	//Update firewall rules
	ruleName := fmt.Sprintf("%s-int-svc-ingress-allow", infraID)
	ports, err := getgcloudClient(oc).GetFirewallAllowPorts(ruleName)
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(ports, "tcp:9095") {
		updatedPorts := strings.Replace(ports, ",tcp:9095", "", -1)
		o.Expect(getgcloudClient(oc).UpdateFirewallAllowPorts(ruleName, updatedPorts)).NotTo(o.HaveOccurred())
	}
}

func getZoneOfInstanceFromGcp(oc *exutil.CLI, infraID string, workerName string) (string, error) {
	zone, err := getgcloudClient(oc).GetZone(infraID, workerName)
	e2e.Logf("zone for instance %v is: %s", workerName, zone)
	return zone, err
}

func startInstanceOnGcp(oc *exutil.CLI, nodeName string, zone string) error {
	err := getgcloudClient(oc).StartInstance(nodeName, zone)
	return err
}

func stopInstanceOnGcp(oc *exutil.CLI, nodeName string, zone string) error {
	err := getgcloudClient(oc).StopInstance(nodeName, zone)
	return err
}

//start one AWS instance
func startInstanceOnAWS(a *exutil.AwsClient, hostname string) {
	instanceID, err := a.GetAwsInstanceIDFromHostname(hostname)
	o.Expect(err).NotTo(o.HaveOccurred())
	stateErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		state, err := a.GetAwsInstanceState(instanceID)
		if err != nil {
			e2e.Logf("%v", err)
			return false, nil
		}
		if state == "running" {
			e2e.Logf("The instance  is running")
			return true, nil
		}
		if state == "stopped" {
			err = a.StartInstance(instanceID)
			o.Expect(err).NotTo(o.HaveOccurred())
			return true, nil
		}
		e2e.Logf("The instance  is in %v,not in a state from which it can be started.", state)
		return false, nil

	})
	exutil.AssertWaitPollNoErr(stateErr, fmt.Sprintf("The instance  is not in a state from which it can be started."))
}

func stopInstanceOnAWS(a *exutil.AwsClient, hostname string) {
	instanceID, err := a.GetAwsInstanceIDFromHostname(hostname)
	o.Expect(err).NotTo(o.HaveOccurred())
	stateErr := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		state, err := a.GetAwsInstanceState(instanceID)
		if err != nil {
			e2e.Logf("%v", err)
			return false, nil
		}
		if state == "stopped" {
			e2e.Logf("The instance  is already stopped.")
			return true, nil
		}
		if state == "running" {
			err = a.StopInstance(instanceID)
			o.Expect(err).NotTo(o.HaveOccurred())
			return true, nil
		}
		e2e.Logf("The instance is in %v,not in a state from which it can be stopped.", state)
		return false, nil

	})
	exutil.AssertWaitPollNoErr(stateErr, fmt.Sprintf("The instance  is not in a state from which it can be stopped."))
}

func findIP(input string) []string {
	numBlock := "(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])"
	regexPattern := numBlock + "\\." + numBlock + "\\." + numBlock + "\\." + numBlock

	regEx := regexp.MustCompile(regexPattern)
	return regEx.FindAllString(input, -1)
}

func unique(s []string) []string {
	inResult := make(map[string]bool)
	var result []string
	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}
