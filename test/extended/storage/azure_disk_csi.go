package storage

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("storage-azure-csi", exutil.KubeConfigPath())

	// azure-disk-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "azure") {
			g.Skip("Skip for non-supported cloud provider: *" + cloudProvider + "* !!!")
		}
	})

	// author: wduan@redhat.com
	// OCP-47001 - [Azure-Disk-CSI-Driver] support different skuName in storageclass with Premium_LRS, StandardSSD_LRS, Standard_LRS
	g.It("Author:wduan-High-47001-[Azure-Disk-CSI-Driver] support different skuName in storageclass with Premium_LRS, StandardSSD_LRS, Standard_LRS", func() {
		// Set the resource template for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate          = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
		)

		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		skunames := []string{"Premium_LRS", "StandardSSD_LRS", "Standard_LRS"}

		for _, skuname := range skunames {
			g.By("******" + " The skuname: " + skuname + " test phase start " + "******")

			// Set the resource definition for the scenario
			storageClassParameters := map[string]string{
				"skuname": skuname,
			}
			extraParameters := map[string]interface{}{
				"parameters": storageClassParameters,
			}

			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

			g.By("Create csi storageclass with skuname")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("Check the pv.spec.csi.volumeAttributes.skuname")
			pvName := pvc.getVolumeName(oc)
			skunamePv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.skuname}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The skuname in PV is: %v.", skunamePv)
			o.Expect(skunamePv).To(o.Equal(skuname))
		}
	})

	// author: wduan@redhat.com
	// OCP-49625 - [Azure-Disk-CSI-Driver] support different skuName in storageclass with Premium_ZRS, StandardSSD_ZRS
	g.It("Author:wduan-High-49625-[Azure-Disk-CSI-Driver] support different skuName in storageclass with Premium_ZRS, StandardSSD_ZRS", func() {
		region := getClusterRegion(oc)
		supportRegions := []string{"westus2", "westeurope", "northeurope", "francecentral"}
		if !contains(supportRegions, region) {
			g.Skip("Current region doesn't support zone-redundant storage")
		}

		// Set the resource template for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate          = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
		)

		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		skunames := []string{"Premium_ZRS", "StandardSSD_ZRS"}

		for _, skuname := range skunames {
			g.By("******" + " The skuname: " + skuname + " test phase start " + "******")

			// Set the resource definition for the scenario
			storageClassParameters := map[string]string{
				"skuname": skuname,
			}
			extraParameters := map[string]interface{}{
				"parameters": storageClassParameters,
			}

			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

			g.By("Create csi storageclass with skuname")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("Check the pv.spec.csi.volumeAttributes.skuname")
			pvName := pvc.getVolumeName(oc)
			skunamePv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.skuname}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The skuname in PV is: %v.", skunamePv)
			o.Expect(skunamePv).To(o.Equal(skuname))

			g.By("Delete pod")
			nodeName := getNodeNameByPod(oc, pod.namespace, pod.name)
			nodeList := []string{nodeName}
			volName := pvc.getVolumeName(oc)
			pod.deleteAsAdmin(oc)
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("Create new pod and schedule to another node")
			schedulableLinuxWorkers := getSchedulableLinuxWorkers(getAllNodesInfo(oc))
			if len(schedulableLinuxWorkers) >= 2 {
				podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
				podNew.createWithNodeAffinity(oc, "kubernetes.io/hostname", "NotIn", nodeList)
				defer podNew.deleteAsAdmin(oc)
				podNew.waitReady(oc)
				output, err := podNew.execCommand(oc, "cat "+podNew.mountPath+"/testfile")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("storage test"))
			} else {
				e2e.Logf("There are not enough schedulable workers, not testing the re-schedule scenario.")
			}
		}
	})

	// author: wduan@redhat.com
	// OCP-49366 - [Azure-Disk-CSI-Driver] support shared disk to mount to different nodes
	// https://github.com/kubernetes-sigs/azuredisk-csi-driver/tree/master/deploy/example/sharedisk
	g.It("Author:wduan-High-49366-[Azure-Disk-CSI-Driver] support shared disks to mount to different nodes", func() {
		schedulableLinuxWorkers := getSchedulableLinuxWorkers(getAllNodesInfo(oc))
		if len(schedulableLinuxWorkers) < 2 || checkNodeZoned(oc) {
			g.Skip("No enough schedulable node or the cluster is not zoned")
		}

		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource template for the scenario
		var (
			storageTeamBaseDir   = exutil.FixturePath("testdata", "storage")
			storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate          = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			deploymentTemplate   = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
		)

		storageClassParameters := map[string]string{
			"skuName":     "Premium_LRS",
			"maxShares":   "2",
			"cachingMode": "None",
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimCapacity("256Gi"), setPersistentVolumeClaimAccessmode("ReadWriteMany"), setPersistentVolumeClaimStorageClassName(storageClass.name), setPersistentVolumeClaimVolumemode("Block"))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentReplicasNo("2"), setDeploymentPVCName(pvc.name), setDeploymentVolumeType("volumeDevices"), setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))

		g.By("Create csi storageclass with maxShares")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

		g.By("Create a pvc with the csi storageclass")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment with the created pvc and wait for the pods ready")
		dep.createWithTopologySpreadConstraints(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("Wait for the deployment ready")
		dep.waitReady(oc)

		g.By("Verify two pods are scheduled to different nodes")
		podList := dep.getPodList(oc)
		nodeName0 := getNodeNameByPod(oc, dep.namespace, podList[0])
		e2e.Logf("Pod: \"%s\" is running on the node: \"%s\"", podList[0], nodeName0)
		nodeName1 := getNodeNameByPod(oc, dep.namespace, podList[1])
		e2e.Logf("Pod: \"%s\" is running on the node: \"%s\"", podList[1], nodeName1)
		o.Expect(nodeName0).ShouldNot(o.Equal(nodeName1))

		g.By("Check data shared between the pods")
		_, err := execCommandInSpecificPod(oc, dep.namespace, podList[0], "/bin/dd  if=/dev/null of="+dep.mpath+" bs=512 count=1 conv=fsync")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = execCommandInSpecificPod(oc, dep.namespace, podList[0], "echo 'test data' > "+dep.mpath+";sync")
		o.Expect(err).NotTo(o.HaveOccurred())
		// Data writen to raw block is cached, restart the pod to test data shared between the pods
		dep.scaleReplicas(oc, "0")
		dep.waitReady(oc)
		dep.scaleReplicas(oc, "2")
		dep.waitReady(oc)
		podList = dep.getPodList(oc)
		_, err = execCommandInSpecificPod(oc, dep.namespace, podList[1], "/bin/dd if="+dep.mpath+" of=/tmp/testfile bs=512 count=1 conv=fsync")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execCommandInSpecificPod(oc, dep.namespace, podList[1], "cat /tmp/testfile | grep 'test data' ")).To(o.ContainSubstring("matches"))
	})
})
