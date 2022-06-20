package gpu

import (
	"os"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("gpu-operator-test", exutil.KubeConfigPath())
		gpuDir               = exutil.FixturePath("testdata", "psap", "gpu")
		iaasPlatform         string
		gpuMachinesetName    = "openshift-psap-qe-gpu"
		gpuClusterPolicyName = "gpu-cluster-policy"
	)

	g.BeforeEach(func() {
		// get IaaS platform
		iaasPlatform = exutil.CheckPlatform(oc)

		// Ensure NFD operator is installed
		// Test requires NFD to be installed and an NodeFeatureDiscovery operand instance to be runnning
		g.By("Deploy NFD Operator and create NFD operand instance on Openshift Container Platform")
		isNodeLabeled := exutil.IsNodeLabeledByNFD(oc)
		//If the node has been labeled, the NFD operator and instnace
		if isNodeLabeled {
			e2e.Logf("NFD installation and node label found! Continuing with test ...")
		} else {
			e2e.Logf("NFD is not deployed, deploying NFD operator and operand instance")
			exutil.InstallNFD(oc, "openshift-nfd")
			//Check if the NFD Operator installed in namespace openshift-nfd
			exutil.WaitOprResourceReady(oc, "deployment", "nfd-controller-manager", "openshift-nfd", true, false)
			//create NFD instance in openshift-nfd
			exutil.CreateNFDInstance(oc, "openshift-nfd")
		}
	})

	// author: wabouham@redhat.com
	g.It("Longduration-NonPreRelease-Author:wabouham-Medium-48452-Deploy NVIDIA GPU Operator with DTK without cluster-wide entitlement via yaml files[Slow]", func() {

		// currently test is only supported on AWS
		if iaasPlatform != "aws" {
			g.Skip("IAAS platform: " + iaasPlatform + " is not automated yet and is only supported on AWS - skipping test ...")
		}

		// Code here to check for GPU instance and create a new machineset and substitute name and instance type to g4dn.xlarge
		g.By("Check if we have an existing \"g4dn\" GPU enabled worker node, if not reate a new machineset of instance type \"g4dn.xlarge\" on OCP")
		checkGPU, err := checkIfWorkerNodesHaveGPUInstances(oc, "g4dn")
		o.Expect(err).NotTo(o.HaveOccurred())

		// For clean up GPU machineset in case of error during test case execution or after testcase completes execution
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args(exutil.MapiMachineset, gpuMachinesetName, "-n", "openshift-machine-api", "--ignore-not-found").Execute()

		if !checkGPU {
			e2e.Logf("No worker node detected with GPU instance, creating a g4dn.xlarge machineset ...")
			createMachinesetbyInstanceType(oc, gpuMachinesetName, "g4dn.xlarge")
			// Verify new node was created and is running
			exutil.WaitForMachinesRunning(oc, 1, gpuMachinesetName)

			e2e.Logf("Newly created GPU machineset name: %v", gpuMachinesetName)
			// Check that the NFD labels are created
			ocDescribeNodes, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(ocDescribeNodes).To(o.ContainSubstring("feature.node.kubernetes.io/pci-10de.present=true"))

		} else {
			e2e.Logf("At least one worker node detected with GPU instance, continuing with test ...")
		}

		g.By("Get the subscription channel and csv version names")
		gpuOperatorDefaultChannelOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests/gpu-operator-certified", "-n", "openshift-marketplace", "-o", "jsonpath='{.status.defaultChannel}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		gpuOperatorDefaultChannelOutputSpace := strings.Trim(gpuOperatorDefaultChannelOutput, "'")
		gpuOperatorDefaultChannel := strings.Trim(gpuOperatorDefaultChannelOutputSpace, " ")
		e2e.Logf("GPU Operator default channel is:  %v", gpuOperatorDefaultChannel)

		// Get the GPU Operator CSV name
		gpuOperatorCsvNameStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", "gpu-operator-certified", "-n", "openshift-marketplace", "-ojsonpath={.status..currentCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// oc get packagemanifest gpu-operator-certified -n openshift-marketplace -o jsonpath='{.status..currentCSV}' -- output returns currentCSV string two occurences:
		// gpu-operator-certified.v1.10.1 gpu-operator-certified.v1.10.1
		gpuOperatorCsvNameArray := strings.Split(gpuOperatorCsvNameStr, " ")
		gpuOperatorCsvName := gpuOperatorCsvNameArray[0]
		e2e.Logf("GPU Operator CSV name is:  %v", gpuOperatorCsvName)

		subTemplate := filepath.Join(gpuDir, "gpu-operator-subscription.yaml")
		sub := subResource{
			name:        "gpu-operator-certified",
			namespace:   gpuOperatorNamespace,
			channel:     gpuOperatorDefaultChannel,
			template:    subTemplate,
			startingCSV: gpuOperatorCsvName,
		}

		// Using defer to cleanup after testcase execution or in event of a testcase failure
		// defer statements are exuted in "Last in - First out" order, with last one exeucted first
		// so REVERSE of normal order of resource deletion
		defer exutil.CleanupOperatorResourceByYaml(oc, "", gpuOperatorNamespaceFile)
		defer exutil.CleanupOperatorResourceByYaml(oc, "", gpuOperatorGroupFile)

		// sub.delete(oc) will also delete the installed CSV
		defer sub.delete(oc)

		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterPolicy", gpuClusterPolicyName).Execute()
		defer exutil.CleanupOperatorResourceByYaml(oc, gpuOperatorNamespace, gpuBurnWorkloadFile)

		g.By("Get cluster version")
		clusterVersion, _, err := exutil.GetClusterVersion(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Cluster Version: %v", clusterVersion)

		g.By("Run 'oc get node'")
		ocGetNodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Output()
		// after error checking, we log the output in the console
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Output: %v", ocGetNodes)

		// check for labeled GPU worker nodes
		g.By("Run 'oc describe node | grep feature | grep 10de'")
		ocCommandWithPipeCmdsOutput := runOcCommandWithPipeCmd(oc, "describe", "node", " | grep feature | grep 10de")
		e2e.Logf("Running 'oc describe node | grep feature | grep 10de' output: %v", ocCommandWithPipeCmdsOutput)
		o.Expect(ocCommandWithPipeCmdsOutput).To(o.ContainSubstring("feature.node.kubernetes.io/pci-10de.present=true"))

		g.By("Run 'oc get packagemanifests/gpu-operator-certified -n nvidia-gpu-operator'")
		ocGetPackagemanifestOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests/gpu-operator-certified", "-n", "openshift-marketplace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc get packagemanifests/gpu-operator-certified -n nvidia-gpu-operator Output: %v", ocGetPackagemanifestOutput)

		// Check if GPU Operator ClusterPolicy is installed and ready
		clusterPolicyReady := checkIfGPUOperatorClusterPolicyIsReady(oc, gpuOperatorNamespace)
		e2e.Logf("clusterPolicyReady: %v", clusterPolicyReady)

		if clusterPolicyReady {
			e2e.Logf("clusterPolicyReady is true, cleaning up, undeploying GPU operator resources first, and re-deploying GPU operator")

			ocDeleteClusterPolicyOutput, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterPolicy", gpuClusterPolicyName).Output()
			// after error checking, we log the output in the console
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("ocGetPodsNvidiaGpuOperator output: \n%v", ocDeleteClusterPolicyOutput)

			exutil.CleanupOperatorResourceByYaml(oc, "", gpuOperatorNamespaceFile)

		} else {
			e2e.Logf("clusterPolicyReady is false, need to deploy GPU operator")
		}

		// run oc apply -f <filename.yaml>.  Create the nvidia-gpu-operator namespace
		g.By("Create namespace nvidia-gpu-operator from gpu-operator-namespace.yaml")
		exutil.ApplyOperatorResourceByYaml(oc, "", gpuOperatorNamespaceFile)

		g.By("Create GPU Operator OperatorGroup from yaml file")
		exutil.ApplyOperatorResourceByYaml(oc, "", gpuOperatorGroupFile)

		g.By("Create GPU Operator Subscription from yaml file")
		sub.createIfNotExist(oc)

		// The only deployment is the gpu-operator
		// gpu-operator   1/1     1            1           7m56s
		g.By("Wait for gpu-operator deployment to be ready")
		exutil.WaitOprResourceReady(oc, "deployment", "gpu-operator", gpuOperatorNamespace, true, false)

		baseDir, err := os.MkdirTemp("/tmp/", "tmp_48452")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(baseDir)
		extractedClusterPolicyFileName := filepath.Join(baseDir, "clusterPolicy-48452-after-jq.json")
		createClusterPolicyJSONFromCSV(oc, gpuOperatorNamespace, gpuOperatorCsvName, extractedClusterPolicyFileName)

		g.By("Create GPU Operator ClusterPolicy from extracted json file from csv")
		exutil.ApplyOperatorResourceByYaml(oc, "", extractedClusterPolicyFileName)

		g.By("Run 'oc get clusterPolicy'")
		ocGetClusterPolicyOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterPolicy").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("ocGetClusterPolicyOutput: \n%v", ocGetClusterPolicyOutput)

		// Damemonsets are:
		// ----------------
		// gpu-feature-discovery                           1         1         1       1            1           nvidia.com/gpu.deploy.gpu-feature-discovery=true                                                                      7m50s
		// nvidia-container-toolkit-daemonset              1         1         1       1            1           nvidia.com/gpu.deploy.container-toolkit=true                                                                          7m51s
		// nvidia-dcgm                                     1         1         1       1            1           nvidia.com/gpu.deploy.dcgm=true                                                                                       7m51s
		// nvidia-dcgm-exporter                            1         1         1       1            1           nvidia.com/gpu.deploy.dcgm-exporter=true                                                                              7m50s
		// nvidia-device-plugin-daemonset                  1         1         1       1            1           nvidia.com/gpu.deploy.device-plugin=true                                                                              7m51s
		// nvidia-driver-daemonset-410.84.202203290245-0   1         1         1       1            1           feature.node.kubernetes.io/system-os_release.OSTREE_VERSION=410.84.202203290245-0,nvidia.com/gpu.deploy.driver=true   7m51s
		// nvidia-mig-manager                              0         0         0       0            0           nvidia.com/gpu.deploy.mig-manager=true                                                                                7m50s
		// nvidia-node-status-exporter                     1         1         1       1            1           nvidia.com/gpu.deploy.node-status-exporter=true                                                                       7m51s
		// nvidia-operator-validator

		g.By("Wait for the daemonsets in the GPU operator namespace to be ready")
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-container-toolkit-daemonset", gpuOperatorNamespace, true, false)
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-dcgm", gpuOperatorNamespace, true, false)
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-dcgm-exporter", gpuOperatorNamespace, true, false)
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-device-plugin-daemonset", gpuOperatorNamespace, true, false)
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-node-status-exporter", gpuOperatorNamespace, true, false)
		exutil.WaitOprResourceReady(oc, "daemonset", "nvidia-operator-validator", gpuOperatorNamespace, true, false)

		ocGetPodsNvidiaGpuOperator, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "nvidia-gpu-operator").Output()
		// after error checking, we log the output in the console
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("ocGetPodsNvidiaGpuOperator output: \n%v", ocGetPodsNvidiaGpuOperator)

		g.By("Run oc describe node | grep gpu command before running the gpu-burn workload")
		// Check if the GPU device plugins are showing up in oc describe node and nvidia.com/gpu: 0
		ocDescribeNodeGpuInstance := runOcCommandWithPipeCmd(oc, "describe", "node", " | grep gpu")
		e2e.Logf("Before running gpu-burn workload, output of oc describe node grepping for gpu:: \n%v", ocDescribeNodeGpuInstance)
		o.Expect(ocDescribeNodeGpuInstance).To(o.ContainSubstring("nvidia"))
		o.Expect(ocDescribeNodeGpuInstance).To(o.ContainSubstring("nvidia.com/gpu              0           0"))

		// Deploy the gpu-burn workload gpu-burn-resource.yaml
		g.By("Deploy the gpu-burn workload gpu-burn-resource.yaml file")
		exutil.ApplyOperatorResourceByYaml(oc, gpuOperatorNamespace, gpuBurnWorkloadFile)
		exutil.WaitOprResourceReady(oc, "daemonset", "gpu-burn-daemonset", gpuOperatorNamespace, true, false)
		assertGPUBurnApp(oc, gpuOperatorNamespace, "gpu-burn-daemonset")

		// Check if the GPU device plugins are showing up in oc describe node and nvidia.com/gpu: 1
		g.By("Run oc describe node | grep gpu command after running the gpu-burn workload")
		ocDescribeNodeGpuInstance1 := runOcCommandWithPipeCmd(oc, "describe", "node", " | grep gpu")
		e2e.Logf("After running gpu-burn workload, output of oc describe node grepping for gpu: \n%v", ocDescribeNodeGpuInstance1)
		o.Expect(ocDescribeNodeGpuInstance1).To(o.ContainSubstring("nvidia.com/gpu              1           1"))

	})
})
