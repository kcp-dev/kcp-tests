package util

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	machineAPINamespace = "openshift-machine-api"
	//MapiMachineset means the fullname of mapi machineset
	MapiMachineset = "machinesets.machine.openshift.io"
	//MapiMachine means the fullname of mapi machine
	MapiMachine = "machines.machine.openshift.io"
	//MapiMHC means the fullname of mapi machinehealthcheck
	MapiMHC = "machinehealthchecks.machine.openshift.io"
)

// MachineSetDescription define fields to create machineset
type MachineSetDescription struct {
	Name     string
	Replicas int
}

// CreateMachineSet create a new machineset
func (ms *MachineSetDescription) CreateMachineSet(oc *CLI) {
	e2e.Logf("Creating a new MachineSets ...")
	machinesetName := GetRandomMachineSetName(oc)
	machineSetJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", machineAPINamespace, "-o=json").OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())

	bytes, _ := ioutil.ReadFile(machineSetJSON)
	value1, _ := sjson.Set(string(bytes), "metadata.name", ms.Name)
	value2, _ := sjson.Set(value1, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	value3, _ := sjson.Set(value2, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	value4, _ := sjson.Set(value3, "spec.replicas", ms.Replicas)
	// Adding taints to machineset so that pods without toleration can not schedule to the nodes we provision
	value5, _ := sjson.Set(value4, "spec.template.spec.taints.0", map[string]interface{}{"effect": "NoSchedule", "key": "mapi", "value": "mapi_test"})
	err = ioutil.WriteFile(machineSetJSON, []byte(value5), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	if err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", machineSetJSON).Execute(); err != nil {
		ms.DeleteMachineSet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		WaitForMachinesRunning(oc, ms.Replicas, ms.Name)
	}
}

// DeleteMachineSet delete a machineset
func (ms *MachineSetDescription) DeleteMachineSet(oc *CLI) error {
	e2e.Logf("Deleting a MachineSets ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(MapiMachineset, ms.Name, "-n", machineAPINamespace).Execute()
}

// ListAllMachineNames list all machines
func ListAllMachineNames(oc *CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, "-o=jsonpath={.items[*].metadata.name}", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// ListWorkerMachineSetNames list all worker machineSets
func ListWorkerMachineSetNames(oc *CLI) []string {
	e2e.Logf("Listing all MachineSets ...")
	machineSetNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, "-o=jsonpath={.items[*].metadata.name}", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineSetNames, " ")
}

// ListWorkerMachineNames list all worker machines
func ListWorkerMachineNames(oc *CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machine-type=worker", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// GetMachineNamesFromMachineSet get all Machines in a Machineset
func GetMachineNamesFromMachineSet(oc *CLI, machineSetName string) []string {
	e2e.Logf("Getting all Machines in a Machineset ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// GetNodeNameFromMachine get node name for a machine
func GetNodeNameFromMachine(oc *CLI, machineName string) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-o=jsonpath={.status.nodeRef.name}", "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodeName
}

// GetRandomMachineSetName get a random MachineSet name
func GetRandomMachineSetName(oc *CLI) string {
	e2e.Logf("Getting a random MachineSet ...")
	return ListWorkerMachineSetNames(oc)[0]
}

// ScaleMachineSet scale a MachineSet by replicas
func ScaleMachineSet(oc *CLI, machineSetName string, replicas int) {
	e2e.Logf("Scaling MachineSets ...")
	_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), MapiMachineset, machineSetName, "-n", machineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// WaitForMachinesRunning check if all the machines are Running in a MachineSet
func WaitForMachinesRunning(oc *CLI, machineNumber int, machineSetName string) {
	e2e.Logf("Waiting for the machines Running ...")
	pollErr := wait.Poll(60*time.Second, 720*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machineSetName, "-o=jsonpath={.status.readyReplicas}", "-n", machineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(msg)
		if machinesRunning != machineNumber {
			e2e.Logf("Expected %v  machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
			return false, nil
		}
		e2e.Logf("Expected %v  machines are Running", machineNumber)
		return true, nil
	})
	if pollErr != nil {
		e2e.Failf("Expected %v  machines are not Running after waiting up to 12 minutes ...", machineNumber)
	}
	e2e.Logf("All machines are Running ...")
}

// WaitForMachineFailed check if all the machines are Failed in a MachineSet
func WaitForMachineFailed(oc *CLI, machineSetName string) {
	e2e.Logf("Wait for machine to go into Failed phase")
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=jsonpath={.items[0].status.phase}").Output()
		if output != "Failed" {
			e2e.Logf("machine is not in Failed phase and waiting up to 3 seconds ...")
			return false, nil
		}
		e2e.Logf("machine is in Failed phase")
		return true, nil
	})
	AssertWaitPollNoErr(err, "Check machine phase failed")
}

// WaitForMachineProvisioned check if all the machines are Provisioned in a MachineSet
func WaitForMachineProvisioned(oc *CLI, machineSetName string) {
	e2e.Logf("Wait for machine to go into Provisioned phase")
	err := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=jsonpath={.items[0].status.phase}").Output()
		if output != "Provisioned" {
			e2e.Logf("machine is not in Provisioned phase and waiting up to 60 seconds ...")
			return false, nil
		}
		e2e.Logf("machine is in Provisioned phase")
		return true, nil
	})
	AssertWaitPollNoErr(err, "Check machine phase failed")
}

//CheckPlatform check the cluster's platform
func CheckPlatform(oc *CLI) string {
	output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	return strings.ToLower(output)
}

// SkipConditionally check the total number of Running machines, if greater than zero, we think machines are managed by machine api operator.
func SkipConditionally(oc *CLI) {
	msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "--no-headers", "-n", machineAPINamespace).Output()
	machinesRunning := strings.Count(msg, "Running")
	if machinesRunning == 0 {
		g.Skip("Expect at least one Running machine. Found none!!!")
	}
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

// GetAwsVolumeInfoAttachedToInstanceID get detail info of the volume attached to the instance id
func GetAwsVolumeInfoAttachedToInstanceID(instanceID string) (string, error) {
	mySession := session.Must(session.NewSession())
	svc := ec2.New(mySession, aws.NewConfig())
	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("attachment.instance-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
	}
	volumeInfo, err := svc.DescribeVolumes(input)
	newValue, _ := json.Marshal(volumeInfo)
	return string(newValue), err
}

// GetAwsCredentialFromCluster get aws credential from cluster
func GetAwsCredentialFromCluster(oc *CLI) (string, string) {
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	// Disconnected and STS type test clusters
	newValue, _ := json.Marshal(err)
	if strings.Contains(string(newValue), "not found") {
		credential, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/ebs-cloud-credentials", "-n", "openshift-cluster-csi-drivers", "-o", "json").Output()
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	clusterRegion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	os.Setenv("AWS_REGION", clusterRegion)
	var c2sConfigPrefix, stsConfigPrefix string
	// C2S type test clusters
	if gjson.Get(credential, `data.credentials`).Exists() && gjson.Get(credential, `data.role`).Exists() {
		c2sConfigPrefix = "/tmp/storage-c2sconfig-" + getRandomString() + "-"
		e2e.Logf("C2S config prefix is: %s", c2sConfigPrefix)
		extraCA, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap/kube-cloud-config", "-n", "openshift-cluster-csi-drivers", "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ioutil.WriteFile(c2sConfigPrefix+"ca.pem", []byte(gjson.Get(extraCA, `data.ca-bundle\.pem`).String()), 0644)).NotTo(o.HaveOccurred())
		os.Setenv("AWS_CA_BUNDLE", c2sConfigPrefix+"ca.pem")
	}
	// STS type test clusters
	if gjson.Get(credential, `data.credentials`).Exists() && !gjson.Get(credential, `data.aws_access_key_id`).Exists() {
		stsConfigPrefix = "/tmp/storage-stsconfig-" + getRandomString() + "-"
		e2e.Logf("STS config prefix is: %s", stsConfigPrefix)
		stsConfigBase64 := gjson.Get(credential, `data.credentials`).String()
		stsConfig, err := base64.StdEncoding.DecodeString(stsConfigBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		var tokenPath, roleArn string
		dataList := strings.Split(string(stsConfig), ` `)
		for _, subStr := range dataList {
			if strings.Contains(subStr, `/token`) {
				tokenPath = subStr
			}
			if strings.Contains(subStr, `arn:`) {
				roleArn = strings.Split(string(subStr), "\n")[0]
			}
		}
		cfgStr := strings.Replace(string(stsConfig), tokenPath, stsConfigPrefix+"token", -1)
		tempToken, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-cluster-csi-drivers", "deployment/aws-ebs-csi-driver-controller", "-c", "csi-driver", "--", "cat", tokenPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ioutil.WriteFile(stsConfigPrefix+"config", []byte(cfgStr), 0644)).NotTo(o.HaveOccurred())
		o.Expect(ioutil.WriteFile(stsConfigPrefix+"token", []byte(tempToken), 0644)).NotTo(o.HaveOccurred())
		os.Setenv("AWS_ROLE_ARN", roleArn)
		os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", stsConfigPrefix+"token")
		os.Setenv("AWS_CONFIG_FILE", stsConfigPrefix+"config")
		os.Setenv("AWS_PROFILE", "storageAutotest"+getRandomString())
	} else {
		accessKeyIDBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).String(), gjson.Get(credential, `data.aws_secret_access_key`).String()
		accessKeyID, err := base64.StdEncoding.DecodeString(accessKeyIDBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		secureKey, err := base64.StdEncoding.DecodeString(secureKeyBase64)
		o.Expect(err).NotTo(o.HaveOccurred())
		os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyID))
		os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
	}
	return c2sConfigPrefix, stsConfigPrefix
}

// DeleteAwsCredentialTmpFile delete aws credential tmp file
func DeleteAwsCredentialTmpFile(c2sConfigPrefix string, stsConfigPrefix string) {
	if c2sConfigPrefix != "" {
		e2e.Logf("remove C2S config tmp file")
		os.Remove(c2sConfigPrefix + "ca.pem")
	}
	if stsConfigPrefix != "" {
		e2e.Logf("remove STS config tmp file")
		os.Remove(stsConfigPrefix + "config")
		os.Remove(stsConfigPrefix + "token")
	}
}
