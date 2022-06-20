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

	var (
		oc                   = exutil.NewCLI("storage-azure-file-csi", exutil.KubeConfigPath())
		storageTeamBaseDir   string
		storageClassTemplate string
		pvcTemplate          string
		podTemplate          string
		deploymentTemplate   string
	)

	// azure-file-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "azure") {
			g.Skip("Skip for non-supported cloud provider: *" + cloudProvider + "* !!!")
		}
		if checkFips(oc) {
			g.Skip("Azure-file CSI Driver don't support FIPS enabled env, skip!!!")
		}
		storageTeamBaseDir = exutil.FixturePath("testdata", "storage")
		storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
		pvcTemplate = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
		podTemplate = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
		deploymentTemplate = filepath.Join(storageTeamBaseDir, "dep-template.yaml")

	})

	// author: wduan@redhat.com
	// OCP-50377-[Azure-File-CSI-Driver] support using resource group in storageclass
	g.It("Author:wduan-High-50377-[Azure-File-CSI-Driver] support using resource group in storageclass", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		g.By("Get resource group from new created Azure-file volume")
		scI := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvcI := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(scI.name), setPersistentVolumeClaimNamespace(oc.Namespace()))
		defer pvcI.deleteAsAdmin(oc)
		defer scI.deleteAsAdmin(oc)
		rg, _, _ := getAzureFileVolumeHandle(oc, scI, pvcI)

		// Set the resource definition for the scenario
		storageClassParameters := map[string]string{
			"resourceGroup": rg,
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("Check the deployment's pod mounted volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("Check the deployment's pod mounted volume have the exec right")
		dep.checkPodMountedVolumeHaveExecRight(oc)
	})

	// author: wduan@redhat.com
	// OCP-50360 - [Azure-File-CSI-Driver] support using storageAccount in storageclass
	g.It("Author:wduan-High-50360-[Azure-File-CSI-Driver] support using storageAccount in storageclass", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		g.By("Get storageAccount from new created Azure-file volume")
		scI := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvcI := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(scI.name), setPersistentVolumeClaimNamespace(oc.Namespace()))
		defer pvcI.deleteAsAdmin(oc)
		defer scI.deleteAsAdmin(oc)
		_, sa, _ := getAzureFileVolumeHandle(oc, scI, pvcI)

		// Set the resource definition for the scenario
		storageClassParameters := map[string]string{
			"storageAccount": sa,
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("Check the deployment's pod mounted volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("Check the deployment's pod mounted volume have the exec right")
		dep.checkPodMountedVolumeHaveExecRight(oc)
	})

	// author: wduan@redhat.com
	// OCP-50471 - [Azure-File-CSI-Driver] support using sharename in storageclass
	g.It("Author:wduan-High-50471-[Azure-File-CSI-Driver] support using sharename in storageclass", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Define the supported skuname
		g.By("Get resourcegroup, storageAccount,sharename from new created Azure-file volume")
		scI := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvcI := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(scI.name), setPersistentVolumeClaimNamespace(oc.Namespace()))
		defer pvcI.deleteAsAdmin(oc)
		defer scI.deleteAsAdmin(oc)
		rg, sa, share := getAzureFileVolumeHandle(oc, scI, pvcI)

		// Set the resource definition for the scenario
		storageClassParameters := map[string]string{
			"resourceGroup":  rg,
			"storageAccount": sa,
			"shareName":      share,
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		// Only suport creating pvc with same size as existing share
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name), setPersistentVolumeClaimCapacity(pvcI.capacity))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("Check the deployment's pod mounted volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("Check the deployment's pod mounted volume have the exec right")
		dep.checkPodMountedVolumeHaveExecRight(oc)
	})

	// author: rdeore@redhat.com
	// Author:rdeore-[Azure-File-CSI-Driver] [SKU-NAMES] support different skuName in storageclass
	var azureSkuNamesCaseIDMap = map[string]string{
		"50392": "Standard_LRS",    // High-50392-[Azure-File-CSI-Driver] [Standard_LRS] support different skuName in storageclass
		"50590": "Standard_GRS",    // High-50590-[Azure-File-CSI-Driver] [Standard_GRS] support different skuName in storageclass
		"50591": "Standard_RAGRS",  // High-50591-[Azure-File-CSI-Driver] [Standard_RAGRS] support different skuName in storageclass
		"50592": "Standard_RAGZRS", // High-50592-[Azure-File-CSI-Driver] [Standard_RAGZRS] support different skuName in storageclass
		"50593": "Premium_LRS",     // High-50593-[Azure-File-CSI-Driver] [Premium_LRS] support different skuName in storageclass
		"50594": "Standard_ZRS",    // High-50594-[Azure-File-CSI-Driver] [Standard_ZRS] support different skuName in storageclass
		"50595": "Premium_ZRS",     // High-50595-[Azure-File-CSI-Driver] [Premium_ZRS] support different skuName in storageclass
	}
	caseIds := []string{"50392", "50590", "50591", "50592", "50593", "50594", "50595"}
	for i := 0; i < len(caseIds); i++ {
		skuName := azureSkuNamesCaseIDMap[caseIds[i]]

		g.It("Author:rdeore-High-"+caseIds[i]+"-[Azure-File-CSI-Driver] [SKU-NAMES] support different skuName in storageclass with "+skuName, func() {
			region := getClusterRegion(oc)
			supportRegions := []string{"westus2", "westeurope", "northeurope", "francecentral"}
			if strings.Contains(skuName, "ZRS") && !contains(supportRegions, region) {
				g.Skip("Current region doesn't support zone-redundant storage")
			}

			// Set the resource template for the scenario
			var (
				podTemplate = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
				provisioner = "file.csi.azure.com"
			)

			// Set up a specified project share for all the phases
			g.By("#. Create new project for the scenario")
			oc.SetupProject()

			// Set the resource definition for the scenario
			storageClassParameters := map[string]string{
				"skuname": skuName,
			}
			extraParameters := map[string]interface{}{
				"parameters": storageClassParameters,
			}

			storageClass := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner(provisioner), setStorageClassVolumeBindingMode("Immediate"))
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

			g.By("#. Create csi storageclass with skuname: " + skuName)
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc)

			g.By("#. Create a pvc with the csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("#. Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("#. Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("#. Check the pod volume have the exec right")
			pod.checkMountedVolumeHaveExecRight(oc)

			g.By("#. Check the pv.spec.csi.volumeAttributes.skuname")
			pvName := pvc.getVolumeName(oc)
			skunamePv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.skuname}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The skuname in PV is: %v.", skunamePv)
			o.Expect(skunamePv).To(o.Equal(skuName))
		})
	}

	// author: rdeore@redhat.com
	// OCP-50634 -[Azure-File-CSI-Driver] fail to provision Block volumeMode
	g.It("Author:rdeore-High-50634-[Azure-File-CSI-Driver] fail to provision Block volumeMode", func() {
		// Set the resource template for the scenario
		var (
			provisioner = "file.csi.azure.com"
		)

		// Set up a specified project share for all the phases
		g.By("#. Create new project for the scenario")
		oc.SetupProject() //create new project

		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner(provisioner), setStorageClassVolumeBindingMode("Immediate"))

		g.By("#. Create a storageclass")
		sc.create(oc)
		defer sc.deleteAsAdmin(oc)

		g.By("#. Create a pvc with the csi storageclass")
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name), setPersistentVolumeClaimVolumemode("Block"))
		e2e.Logf("%s", pvc.scname)
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("#. Check pvc: " + pvc.name + " is in pending status")
		pvc.waitPvcStatusToTimer(oc, "Pending")

		g.By("#. Check pvc provisioning failure information is clear")
		info, err := pvc.getDescription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(info).To(o.ContainSubstring("driver does not support block volume"))
	})

	// author: wduan@redhat.com
	// OCP-50732 - [Azure-File-CSI-Driver] specify shareNamePrefix in storageclass
	g.It("Author:wduan-Medium-50732-[Azure-File-CSI-Driver] specify shareNamePrefix in storageclass", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource definition for the scenario
		prefix := getRandomString()
		storageClassParameters := map[string]string{
			"shareNamePrefix": prefix,
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}

		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name))
		pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create pod")
		pod.create(oc)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("Check the pod mounted volume can be read and write")
		pod.checkMountedVolumeCouldRW(oc)

		g.By("Check the pod mounted volume have the exec right")
		pod.checkMountedVolumeHaveExecRight(oc)

		g.By("Check pv has the prefix in the pv.spec.csi.volumeHandle")
		pvName := pvc.getVolumeName(oc)
		volumeHandle, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The Azure-File volumeHandle is: %v.", volumeHandle)
		o.Expect(volumeHandle).To(o.ContainSubstring(prefix))
	})

	// author: wduan@redhat.com
	// OCP-50919 - [Azure-File-CSI-Driver] support smb file share protocol
	g.It("Author:wduan-High-50919-[Azure-File-CSI-Driver] support smb file share protocol", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource definition for the scenario
		storageClassParameters := map[string]string{
			"protocol": "smb",
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("Check the deployment's pod mounted volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("Check the deployment's pod mounted volume have the exec right")
		dep.checkPodMountedVolumeHaveExecRight(oc)

		g.By("Check pv has protocol parameter in the pv.spec.csi.volumeAttributes.protocol")
		pvName := pvc.getVolumeName(oc)
		protocol, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.protocol}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The protocol is: %v.", protocol)
		o.Expect(protocol).To(o.ContainSubstring("smb"))
	})

	// author: wduan@redhat.com
	// OCP-50918 - [Azure-File-CSI-Driver] support nfs file share protocol
	g.It("Author:wduan-High-50918-[Azure-File-CSI-Driver] support nfs file share protocol", func() {
		// Set up a specified project share for all the phases
		g.By("Create new project for the scenario")
		oc.SetupProject() //create new project

		// Set the resource definition for the scenario
		storageClassParameters := map[string]string{
			"protocol": "nfs",
		}
		extraParameters := map[string]interface{}{
			"parameters": storageClassParameters,
		}
		sc := newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("file.csi.azure.com"), setStorageClassVolumeBindingMode("Immediate"))
		pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(sc.name))
		dep := newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))

		g.By("Create storageclass")
		sc.createWithExtraParameters(oc, extraParameters)
		defer sc.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("Check the deployment's pod mounted volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("Check the deployment's pod mounted volume have the exec right")
		dep.checkPodMountedVolumeHaveExecRight(oc)

		g.By("Check pv has protocol parameter in the pv.spec.csi.volumeAttributes.protocol")
		pvName := pvc.getVolumeName(oc)
		protocol, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.protocol}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The protocol is: %v.", protocol)
		o.Expect(protocol).To(o.ContainSubstring("nfs"))
	})
})

// Get resourceGroup/account/share name by creating a new azure-file volume
func getAzureFileVolumeHandle(oc *exutil.CLI, sc storageClass, pvc persistentVolumeClaim) (resourceGroup string, account string, share string) {
	sc.create(oc)
	pvc.create(oc)
	pvc.waitStatusAsExpected(oc, "Bound")
	pvName := pvc.getVolumeName(oc)
	volumeHandle, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The Azure-File volumeHandle is: %v.", volumeHandle)
	items := strings.Split(volumeHandle, "#")
	debugLogf("resource-group-name is \"%s\", account-name is \"%s\", share-name is \"%s\"", items[0], items[1], items[2])
	return items[0], items[1], items[2]
}
