package storage

import (
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var (
		oc                          = exutil.NewCLI("storage-alibaba-csi", exutil.KubeConfigPath())
		cloudProviderSupportVolumes []string
	)

	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "alibabacloud") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
		cloudProviderSupportVolumes = getSupportVolumesByCloudProvider()
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: file system [ext4/ext3/xfs]
	g.It("Author:ropatil-Medium-47918-[Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: file system [ext4/ext3/xfs]", func() {
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project
		//Define the test scenario support fsTypes
		fsTypes := []string{"ext4", "ext3", "xfs"}
		for _, fsType := range fsTypes {
			// Set the resource template and definition for the scenario
			var (
				storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
				storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
				pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
				deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
				storageClass           = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
				pvc                    = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
				dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
				storageClassParameters = map[string]string{
					"csi.storage.k8s.io/fstype": fsType,
					"diskTags":                  "team:storage,user:Alitest",
				}
				extraParameters = map[string]interface{}{
					"parameters":           storageClassParameters,
					"allowVolumeExpansion": true,
				}
			)

			g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\" for fsType: \"" + fsType + "\" test phase start" + "******")

			g.By("Create csi storageclass")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create deployment with the created pvc and wait for the pod ready")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("Check volume have the diskTags attribute")
			volName := pvc.getVolumeName(oc)
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, "team:storage,user:Alitest")).To(o.BeTrue())

			g.By("Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("******" + cloudProvider + " csi driver: \"" + storageClass.provisioner + "\" for fsType: \"" + fsType + "\" test phase finished" + "******")
		}
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: Block
	g.It("Author:ropatil-Medium-47919-[Alibaba-CSI-Driver] [Dynamic PV] should have diskTags attribute for volume mode: Block", func() {

		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource template and definition for the scenario
		for _, volumetype := range cloudProviderSupportVolumes {
			var (
				storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
				storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
				pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
				deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
				storageClass           = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
				pvc                    = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name), setPersistentVolumeClaimVolumemode("Block"))
				dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name), setDeploymentVolumeType("volumeDevices"), setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
				storageClassParameters = map[string]string{
					"diskTags": "team:storage,user:Alitest",
					"type":     volumetype,
				}
				extraParameters = map[string]interface{}{
					"parameters":           storageClassParameters,
					"allowVolumeExpansion": true,
				}
			)

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + " volumetype " + volumetype + " for Block volume mode test phase start" + "******")

			g.By("Create csi storageclass")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create deployment with the created pvc and wait for the pod ready")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("Check volume have the diskTags,volume type attributes")
			volName := pvc.getVolumeName(oc)
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, "team:storage,user:Alitest")).To(o.BeTrue())
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, volumetype)).To(o.BeTrue())

			g.By("Check the deployment's pod mounted volume can be read and write")
			dep.writeDataBlockType(oc)

			g.By("Check the deployment's pod mounted volume have the exec right")
			dep.checkDataBlockType(oc)

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + ", volume type " + volumetype + " for Block volume mode test phase finished" + "******")
		}
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] [Filesystem default] support mountOptions, mkfsOptions
	g.It("Author:ropatil-High-47999-[CSI Driver] [Dynamic PV] [Filesystem default] support mountOptions, mkfsOptions", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource template and definition for the scenario
		for _, volumetype := range cloudProviderSupportVolumes {
			var (
				storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
				storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
				pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
				deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
				storageClass           = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
				pvc                    = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
				dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
				mountOption            = []string{"nodiratime", "barrier=0"}
				storageClassParameters = map[string]string{
					"mkfsOptions": "-q -L yunpan -J size=2048 -T largefile",
					"type":        volumetype,
				}
				extraParameters = map[string]interface{}{
					"allowVolumeExpansion": true,
					"mountOptions":         mountOption,
					"parameters":           storageClassParameters,
				}
			)

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + " volume type: " + volumetype + " for Filesystem default mode test phase start" + "******")

			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create deployment with the created pvc")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("Check the volume mounted contains the mount option by exec mount cmd in the node")
			volName := pvc.getVolumeName(oc)
			nodeName := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
			checkVolumeMountCmdContain(oc, volName, nodeName, "nodiratime")
			checkVolumeMountCmdContain(oc, volName, nodeName, "nobarrier")

			g.By("Check the volume has attributes mkfsOptions, volumetype")
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, "-q -L yunpan -J size=2048 -T largefile")).To(o.BeTrue())
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, volumetype)).To(o.BeTrue())

			g.By("Scale down the replicas number to 0")
			dep.scaleReplicas(oc, "0")

			g.By("Wait for the deployment scale down completed and check nodes has no mounted volume")
			dep.waitReady(oc)
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("Scale up the deployment replicas number to 1")
			dep.scaleReplicas(oc, "1")

			g.By("Wait for the deployment scale up completed")
			dep.waitReady(oc)

			g.By("After scaled check the deployment's pod mounted volume contents and exec right")
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "cat /mnt/storage/testfile*")).To(o.ContainSubstring("storage test"))
			o.Expect(execCommandInSpecificPod(oc, dep.namespace, dep.getPodList(oc)[0], "/mnt/storage/hello")).To(o.ContainSubstring("Hello OpenShift Storage"))

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + " volume type: " + volumetype + " for Filesystem default mode test phase finished" + "******")
		}
	})

	// author: ropatil@redhat.com
	// [Alibaba-CSI-Driver] [Dynamic PV] with resource group id and allow volumes to store data
	g.It("Author:ropatil-Medium-49498-[Alibaba-CSI-Driver] [Dynamic PV] with resource group id and allow volumes to store data", func() {
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		g.By("Get the resource group id for the cluster")
		rgid := getResourceGroupID(oc)

		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		for _, volumetype := range cloudProviderSupportVolumes {
			// Set the resource template and definition for the scenario
			var (
				storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
				storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
				pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
				deploymentTemplate     = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
				storageClass           = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("diskplugin.csi.alibabacloud.com"))
				pvc                    = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
				dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
				storageClassParameters = map[string]string{
					"resourceGroupId": rgid,
					"type":            volumetype,
				}
				extraParameters = map[string]interface{}{
					"parameters":           storageClassParameters,
					"allowVolumeExpansion": true,
				}
			)

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + " volume type: " + volumetype + " resourceGroupId: " + rgid + " test phase start" + "******")

			g.By("Create csi storageclass")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.

			g.By("Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("Create deployment with the created pvc")
			dep.create(oc)
			defer dep.deleteAsAdmin(oc)

			g.By("Wait for the deployment ready")
			dep.waitReady(oc)

			g.By("Check the volume mounted on the pod located node")
			volName := pvc.getVolumeName(oc)
			nodeName := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
			checkVolumeMountOnNode(oc, volName, nodeName)

			g.By("Check volume have the resourcegroup id, volumetype attribute")
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, rgid)).To(o.BeTrue())
			o.Expect(checkVolumeCsiContainAttributes(oc, volName, volumetype)).To(o.BeTrue())

			g.By("Check the deployment's pod mounted volume can be read and write")
			dep.checkPodMountedVolumeCouldRW(oc)

			g.By("Check the deployment's pod mounted volume have the exec right")
			dep.checkPodMountedVolumeHaveExecRight(oc)

			g.By("Delete the deployment and pvc")
			dep.delete(oc)
			pvc.delete(oc)

			g.By("#Check the volume got deleted and not mounted on node")
			waitForPersistentVolumeStatusAsExpected(oc, volName, "deleted")
			checkVolumeNotMountOnNode(oc, volName, nodeName)

			g.By("******" + cloudProvider + " csi driver: " + storageClass.provisioner + " volume type: " + volumetype + " resourceGroupId: " + rgid + " test phase finished" + "******")
		}
	})
})
