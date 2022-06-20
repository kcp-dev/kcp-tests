package gpu

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	machinesetNamespace         = "openshift-machine-api"
	gpuOperatorNamespace        = "nvidia-gpu-operator"
	gpuOperatorNamespaceFile    = exutil.FixturePath("testdata", "psap", "gpu", "gpu-operator-namespace.yaml")
	gpuOperatorVersion          = "v1.10.1"
	gpuOperatorGroupFile        = exutil.FixturePath("testdata", "psap", "gpu", "gpu-operator-group.yaml")
	gpuOperatorSubscriptionFile = exutil.FixturePath("testdata", "psap", "gpu", "gpu-operator-subscription.yaml")
	gpuBurnWorkloadFile         = exutil.FixturePath("testdata", "psap", "gpu", "gpu-burn-resource.yaml")
)

// Run oc create -f <filename_yaml_file>.yaml, throws an error if creation fails
func runOcCreateYAML(oc *exutil.CLI, filename string) error {
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filename).Execute()
}

func checkIfGPUOperatorClusterPolicyIsReady(oc *exutil.CLI, namespace string) bool {
	// oc get clusterPolicy -o jsonpath='{.items[*].status.state}'
	// returns: ready
	// oc get clusterPolicy
	// error: the server doesn't have a resource type "clusterPolicy"
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterPolicy").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found") || strings.Contains(output, "doesn't have a resource type") || err != nil {
		e2e.Logf("No clusterPolicy was found on this cluster")
		return false
	}

	clusterPolicyState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterPolicy", "-o", "jsonpath='{.items[*].status.state}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	// clusterPolicy has single quotes around it: 'ready' from output, need to remove them
	clusterPolicyWithoutSingleQuotes := strings.ReplaceAll(clusterPolicyState, "'", "")
	e2e.Logf("clusterPolicyState: %v", clusterPolicyWithoutSingleQuotes)

	return strings.Compare(clusterPolicyWithoutSingleQuotes, "ready") == 0
}

func runOcCommandWithPipeCmd(oc *exutil.CLI, ocCommand string, ocArgs string, pipeCmdString string) string {
	// Run the base command with arguments and capture the output in a file
	ocCommandOutputFile, err := oc.AsAdmin().WithoutNamespace().Run(ocCommand).Args(ocArgs).OutputToFile("ocCommandOutputFile.txt")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Execute a basic bash command, piping the contents of the file into another command and again capturing the output
	// Checking here if we have the "10de" label detected on GPU instance
	rawOutput, err := exec.Command("bash", "-c", "cat "+ocCommandOutputFile+" "+pipeCmdString).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	// we need to format this output before logging it
	stringifyOutput := strings.TrimSpace(string(rawOutput))

	return stringifyOutput
}

func checkIfWorkerNodesHaveGPUInstances(oc *exutil.CLI, instanceTypePrefix string) (bool, error) {
	// GPU enabled worker node instances will have the label from NFD:
	//    feature.node.kubernetes.io/pci-10de.present=true
	// and also have labels:
	// oc describe node -l feature.node.kubernetes.io/pci-10de.present=true | grep "instance-type=g4dn.xlarge"
	//   beta.kubernetes.io/instance-type=g4dn.xlarge
	//   node.kubernetes.io/instance-type=g4dn.xlarge
	// instance-type=g4dn.<size>
	// Run the base 'oc describe node` command and capture the output
	ocDescribeNodes, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node", "-l feature.node.kubernetes.io/pci-10de.present=true").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	// example lable for g4dn instance type prefix:  "node.kubernetes.io/instance-type="g4dn"
	gpuInstanceLabel := "node.kubernetes.io/instance-type=" + instanceTypePrefix

	instanceTypeMatched := strings.Contains(ocDescribeNodes, gpuInstanceLabel)

	if !instanceTypeMatched {
		e2e.Logf("No worker nodes with GPU instances were detected")
		return false, nil
	}

	e2e.Logf("At least one worker node contains a GPU with instanceType of prefix %v :", instanceTypePrefix)
	return true, nil
}

func assertGPUBurnApp(oc *exutil.CLI, namespace string, gpuDsPodname string) {
	// get the gpu-burn daemonset pod name
	gpuPodsOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-oname", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	//Filter pod name base on deployment/daemonset name
	regexpoprname, _ := regexp.Compile(".*" + gpuDsPodname + ".*")
	isMatch := regexpoprname.MatchString(gpuPodsOutput)
	gpuPodname := regexpoprname.FindAllString(gpuPodsOutput, -1)
	gpuBurnPodName := gpuPodname[0]
	e2e.Logf("gpuPodname is : %v", gpuBurnPodName)

	// Wait 10 sec in each iteration before condition function () returns true or errors or times out after 12 mins
	// Here the body under waitr.Poll(...) is execuded over and over until we timeout or func() returns true or an error.
	ocLogsGpuBurnOutput := ""
	err1 := wait.Poll(10*time.Second, 12*time.Minute, func() (bool, error) {
		// GetPod logs from gpu-burn daemonset.  Analyse later, look for "Gflop" and "errors: 0" in pod log
		var err error
		ocLogsGpuBurnOutput, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args(gpuBurnPodName, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		isMatch2 := strings.Contains(ocLogsGpuBurnOutput, "Gflop")
		isMatch3 := strings.Contains(ocLogsGpuBurnOutput, "errors: 0")
		/*  gpu-burn-daemonset pod log:  example of last lines after execution completes:
				96.0%  proc'd: 61198 (3484 Gflop/s)   errors: 0   temps: 75 C
			            Summary at:   Thu May 12 02:00:27 UTC 2022

		        100.0%  proc'd: 63679 (3518 Gflop/s)   errors: 0   temps: 74 C
		        Killing processes.. done

		        Tested 1 GPUs:
			            GPU 0: OK
		*/
		isMatch4 := strings.Contains(ocLogsGpuBurnOutput, "Tested 1 GPUs")
		isMatch5 := strings.Contains(ocLogsGpuBurnOutput, "GPU 0: OK")
		isMatch6 := strings.Contains(ocLogsGpuBurnOutput, "100.0%  proc'd:")

		if isMatch && isMatch2 && isMatch3 && isMatch4 && isMatch5 && isMatch6 && err == nil {
			e2e.Logf("gpu-burn workload execution completed successfully on the GPU instance")
			// this stops the polling
			return true, nil
		} else if isMatch && isMatch2 && isMatch3 && err == nil {
			e2e.Logf("gpu-burn workload still running on the GPU instance")
			// return false to loop again
			return false, nil
		} else {
			e2e.Logf("gpu-burn workload did NOT run successfully on the GPU instance")
			return false, nil
		}

	})

	// output the final pod log once
	e2e.Logf("ocLogsGpuBurnOutput: \n%v", ocLogsGpuBurnOutput)

	exutil.AssertWaitPollNoErr(err1, "gpu-burn workload ran abnormally")
}

type subResource struct {
	name         string
	namespace    string
	channel      string
	startingCSV  string
	installedCSV string
	template     string
}

func (sub *subResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		applyResource(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "CHANNEL="+sub.channel, "CSV_VERSION="+sub.startingCSV, "GPU_NAMESPACE="+sub.namespace)
		err = wait.Poll(5*time.Second, 240*time.Second, func() (bool, error) {
			state, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", state, err)
				return false, nil
			}
			if strings.Compare(state, "AtLatestKnown") == 0 || strings.Compare(state, "UpgradeAvailable") == 0 {
				return true, nil
			}
			e2e.Logf("sub %s state is %s, not AtLatestKnown or UpgradeAvailable", sub.name, state)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown or UpgradeAvailable", sub.name))

		installedCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(installedCSV).NotTo(o.BeEmpty())
		sub.installedCSV = installedCSV
	} else {
		e2e.Logf(fmt.Sprintf("Already exist sub in project: %s", sub.namespace))
	}
}

func applyResource(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile("templateSubstituted.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func (sub *subResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sub", sub.name, "-n", sub.namespace).Output()
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("csv", sub.installedCSV, "-n", sub.namespace).Output()
}

func createClusterPolicyJSONFromCSV(oc *exutil.CLI, namespace string, csvName string, policyFileName string) {
	// retruns a string obejct with angle brackets "[ { clusterPolicy.json} ]"
	ocCommandOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", namespace, "-ojsonpath={.metadata.annotations.alm-examples}").OutputToFile("cluster-policy-output-jq-file.txt")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Execute a basic bash command, piping the contents of the file into jq cmd
	// to remove the angle bracket around the clusterPolicy json body and retain proper formatting
	rawJqOutput, err := exec.Command("bash", "-c", "cat "+ocCommandOutput+" | jq .[0]").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	// we need to format this output before logging it
	stringifyJqOutput := strings.TrimSpace(string(rawJqOutput))
	e2e.Logf("CLusterPolicy output file after piping into jq: \n%v", stringifyJqOutput)

	// rawJqOutput is of type []byte, a byte array
	err = ioutil.WriteFile(policyFileName, rawJqOutput, 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

}

func createMachinesetbyInstanceType(oc *exutil.CLI, machinesetName string, instanceType string) {
	// Get existing machinesets in cluster
	ocGetMachineset, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(exutil.MapiMachineset, "-n", "openshift-machine-api", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Existing machinesets:\n%v", ocGetMachineset)

	// Get name of first machineset in existing machineset list
	firstMachinesetName := exutil.GetFirstLinuxMachineSets(oc)
	e2e.Logf("Got %v from machineset list", firstMachinesetName)

	machinesetYamlOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(exutil.MapiMachineset, firstMachinesetName, "-n", "openshift-machine-api", "-oyaml").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	//Create machinset by specifying a machineset name
	regMachineSet := regexp.MustCompile(firstMachinesetName)
	newMachinesetYaml := regMachineSet.ReplaceAllString(machinesetYamlOutput, machinesetName)

	//Change instanceType to g4dn.xlarge
	regInstanceType := regexp.MustCompile(`instanceType:.*`)
	newInstanceType := "instanceType: " + instanceType
	newMachinesetYaml = regInstanceType.ReplaceAllString(newMachinesetYaml, newInstanceType)

	//Make sure the replicas is 1
	regReplicas := regexp.MustCompile(`replicas:.*`)
	replicasNum := "replicas: 1"
	newMachinesetYaml = regReplicas.ReplaceAllString(newMachinesetYaml, replicasNum)

	machinesetNewB := []byte(newMachinesetYaml)
	err = ioutil.WriteFile(machinesetName+"-new.yaml", machinesetNewB, 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(machinesetName + "-new.yaml")

	exutil.ApplyOperatorResourceByYaml(oc, "openshift-machine-api", machinesetName+"-new.yaml")
}
