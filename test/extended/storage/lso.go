package storage

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var (
		oc              = exutil.NewCLI("storage-lso", exutil.KubeConfigPath())
		ac              *ec2.EC2
		allNodes        []node
		testChannel     string
		lsoBaseDir      string
		lsoTemplate     string
		clusterIDTagKey string
		myLso           localStorageOperator
	)

	// LSO test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "aws") {
			g.Skip("Skip for non-supported cloud provider for LSO test: *" + cloudProvider + "* !!!")
		}
		lsoBaseDir = exutil.FixturePath("testdata", "storage")
		lsoTemplate = filepath.Join(lsoBaseDir, "/lso/lso-subscription-template.yaml")
		testChannel = getClusterVersionChannel(oc)
		if versionIsAbove(testChannel, "4.10") {
			testChannel = "stable"
		}
		myLso = newLso(setLsoChannel(testChannel), setLsoTemplate(lsoTemplate))
		o.Expect(myLso.checkClusterCatalogSource(oc)).NotTo(o.HaveOccurred())
		myLso.install(oc)
		myLso.waitInstallSucceed(oc)
		allNodes = getAllNodesInfo(oc)
		// Get the backend credential and init aws ec2 session
		getCredentialFromCluster(oc)
		ac = newAwsClient()
		clusterIDTagKey, _ = getClusterID(oc)
		clusterIDTagKey = "kubernetes.io/cluster/" + clusterIDTagKey
	})

	g.AfterEach(func() {
		myLso.uninstall(oc)
	})

	// author: pewang@redhat.com
	g.It("Author:pewang-Critical-24523-[LSO] [block volume] LocalVolume CR related pv could be used by Pod", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			depTemplate = filepath.Join(lsoBaseDir, "dep-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvVolumeMode("Block"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimVolumemode("Block"),
				setPersistentVolumeClaimStorageClassName(mylv.scname))
			dep = newDeployment(setDeploymentTemplate(depTemplate), setDeploymentPVCName(pvc.name), setDeploymentVolumeType("volumeDevices"),
				setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("# Write file to raw block volume")
		dep.writeDataBlockType(oc)

		g.By("# Scale down the deployment replicas num to zero")
		dep.scaleReplicas(oc, "0")
		dep.waitReady(oc)

		g.By("# Scale up the deployment replicas num to 1 and wait it ready")
		dep.scaleReplicas(oc, "1")
		dep.waitReady(oc)

		g.By("# Check the data still in the raw block volume")
		dep.checkDataBlockType(oc)

		g.By("# Delete deployment and pvc and check the related pv's status")
		pvName := pvc.getVolumeName(oc)
		dep.delete(oc)
		pvc.delete(oc)
		pvc.waitStatusAsExpected(oc, "deleted")
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("# Create new pvc,deployment and check the data in origin volume is cleaned up")
		pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimVolumemode("Block"),
			setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"), setPersistentVolumeClaimStorageClassName(mylv.scname))
		pvcNew.create(oc)
		defer pvcNew.deleteAsAdmin(oc)
		depNew := newDeployment(setDeploymentTemplate(depTemplate), setDeploymentPVCName(pvcNew.name),
			setDeploymentVolumeType("volumeDevices"), setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
		depNew.create(oc)
		defer depNew.deleteAsAdmin(oc)
		depNew.waitReady(oc)
		// Check the data is cleaned up in the volume
		command := []string{"-n", depNew.namespace, "deployment/" + depNew.name, "--", "/bin/dd if=" + dep.mpath + " of=/tmp/testfile bs=512 count=1"}
		output, err := oc.WithoutNamespace().Run("exec").Args(command...).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("no such file or directory"))
	})

	// author: pewang@redhat.com
	g.It("Author:pewang-Critical-24524-[LSO] [Filesystem xfs] LocalVolume CR related pv could be used by Pod", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("xfs"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			pod         = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		pod.create(oc)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("#. Check the volume fsType as expected")
		volName := pvc.getVolumeName(oc)
		checkVolumeMountCmdContain(oc, volName, myWorker.name, "xfs")

		g.By("# Check the pod volume can be read and write and have the exec right")
		pod.checkMountedVolumeCouldRW(oc)
		pod.checkMountedVolumeHaveExecRight(oc)

		g.By("# Delete pod and pvc and check the related pv's status")
		pvName := pvc.getVolumeName(oc)
		pod.delete(oc)
		pvc.delete(oc)
		pvc.waitStatusAsExpected(oc, "deleted")
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("# Create new pvc,pod and check the data in origin volume is cleaned up")
		pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname),
			setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"))
		pvcNew.create(oc)
		defer pvcNew.deleteAsAdmin(oc)
		podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvcNew.name))
		podNew.create(oc)
		defer podNew.deleteAsAdmin(oc)
		podNew.waitReady(oc)
		// Check the data is cleaned up in the volume
		podNew.checkMountedVolumeDataExist(oc, false)
	})

	// author: pewang@redhat.com
	g.It("Author:pewang-Critical-24525-[LSO] [Filesystem ext4] LocalVolume CR related pv could be used by Pod", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("ext4"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			pod         = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		pod.create(oc)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("# Check the volume fsType as expected")
		volName := pvc.getVolumeName(oc)
		checkVolumeMountCmdContain(oc, volName, myWorker.name, "ext4")

		g.By("# Check the pod volume can be read and write and have the exec right")
		pod.checkMountedVolumeCouldRW(oc)
		pod.checkMountedVolumeHaveExecRight(oc)

		g.By("# Delete pod and pvc and check the related pv's status")
		pvName := pvc.getVolumeName(oc)
		pod.delete(oc)
		pvc.delete(oc)
		pvc.waitStatusAsExpected(oc, "deleted")
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("# Create new pvc,pod and check the data in origin volume is cleaned up")
		pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname),
			setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"))
		pvcNew.create(oc)
		defer pvcNew.deleteAsAdmin(oc)
		podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvcNew.name))
		podNew.create(oc)
		defer podNew.deleteAsAdmin(oc)
		podNew.waitReady(oc)
		// Check the data is cleaned up in the volume
		podNew.checkMountedVolumeDataExist(oc, false)
	})

	// author: pewang@redhat.com
	g.It("Author:pewang-Critical-26743-[LSO] [Filesystem ext4] LocalVolume CR with tolerations should work", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("ext4"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			pod         = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myMaster := getOneSchedulableMaster(allNodes)
		myVolume := newEbsVolume(setVolAz(myMaster.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myMaster)

		g.By("# Create a localvolume cr with tolerations use diskPath by id")
		toleration := []map[string]string{
			{
				"key":      "node-role.kubernetes.io/master",
				"operator": "Exists",
			},
		}
		tolerationsParameters := map[string]interface{}{
			"jsonPath":    `items.0.spec.`,
			"tolerations": toleration,
		}
		mylv.deviceID = myVolume.DeviceByID
		mylv.createWithExtraParameters(oc, tolerationsParameters)
		defer mylv.deleteAsAdmin(oc)

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		pod.createWithExtraParameters(oc, tolerationsParameters)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("# Check the pod volume can be read and write and have the exec right")
		pod.checkMountedVolumeCouldRW(oc)
		pod.checkMountedVolumeHaveExecRight(oc)
	})

	// author: pewang@redhat.com
	g.It("NonPreRelease-Author:pewang-Critical-48791-[LSO] [Filesystem ext4] LocalVolume CR related pv should be cleaned up after pvc is deleted and could be reused", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("ext4"))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)

		for i := 1; i <= 10; i++ {
			e2e.Logf("###### The %d loop of test LocalVolume pv cleaned up start ######", i)
			g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
			pvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			pod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
			pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("# Write data to the pod's mount volume")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("# Delete pod and pvc")
			pod.deleteAsAdmin(oc)
			pvc.deleteAsAdmin(oc)
			pvc.waitStatusAsExpected(oc, "deleted")

			g.By("# Create new pvc,pod and check the data in origin volume is cleaned up")
			pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname),
				setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"))
			pvcNew.create(oc)
			defer pvcNew.deleteAsAdmin(oc)
			podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvcNew.name))
			podNew.create(oc)
			defer podNew.deleteAsAdmin(oc)
			podNew.waitReady(oc)
			// Check the data is cleaned up in the volume
			podNew.checkMountedVolumeDataExist(oc, false)

			g.By("# Delete the new pod,pvc")
			podNew.deleteAsAdmin(oc)
			pvcNew.deleteAsAdmin(oc)
			pvcNew.waitStatusAsExpected(oc, "deleted")
			e2e.Logf("###### The %d loop of test LocalVolume pv cleaned up finished ######", i)
		}
	})

	// author: pewang@redhat.com
	// Bug 1915732 - [RFE] Enable volume resizing for local storage PVs
	// https://bugzilla.redhat.com/show_bug.cgi?id=1915732
	// [LSO] [Filesystem types] [Resize] LocalVolume CR related pv could be expanded capacity manually
	lsoFsTypesResizeTestSuit := map[string]string{
		"50951": "ext4", // Author:pewang-High-50951-[LSO] [Filesystem ext4] [Resize] LocalVolume CR related pv could be expanded capacity manually
		"51171": "ext3", // Author:pewang-High-51171-[LSO] [Filesystem ext3] [Resize] LocalVolume CR related pv could be expanded capacity manually
		"51172": "xfs",  // Author:pewang-High-51172-[LSO] [Filesystem xfs]  [Resize] LocalVolume CR related pv could be expanded capacity manually
	}
	caseIds := []string{"50951", "51171", "51172"}
	for i := 0; i < len(caseIds); i++ {
		fsType := lsoFsTypesResizeTestSuit[caseIds[i]]
		g.It("Author:pewang-High-"+caseIds[i]+"-[LSO] [Filesystem "+fsType+"] [Resize] LocalVolume CR related pv could be expanded capacity manually", func() {
			// Set the resource definition for the scenario
			var (
				pvcTemplate       = filepath.Join(lsoBaseDir, "pvc-template.yaml")
				podTemplate       = filepath.Join(lsoBaseDir, "pod-template.yaml")
				lvTemplate        = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
				mylv              = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype(fsType))
				pvc               = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
				pod               = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
				randomExpandInt64 = getRandomNum(5, 10)
			)

			g.By("# Create a new project for the scenario")
			oc.SetupProject() //create new project

			g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
			myWorker := getOneSchedulableWorker(allNodes)
			myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
			defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
			myVolume.createAndReadyToUse(ac)
			// Attach the volume to a schedulable linux worker node
			defer myVolume.detachSucceed(ac)
			myVolume.attachToInstanceSucceed(ac, oc, myWorker)

			g.By("# Create a localvolume cr use diskPath by id with the attached volume")
			mylv.deviceID = myVolume.DeviceByID
			mylv.create(oc)
			defer mylv.deleteAsAdmin(oc)

			g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
			originVolumeCapacity := myVolume.Size
			pvc.capacity = interfaceToString(originVolumeCapacity) + "Gi"
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			pod.waitReady(oc)

			g.By("# Check the pod volume can be read and write and have the exec right")
			pod.checkMountedVolumeCouldRW(oc)
			pod.checkMountedVolumeHaveExecRight(oc)

			g.By("# Expand the volume on backend and waiting for resize complete")
			myVolume.expandSucceed(ac, myVolume.Size+randomExpandInt64)

			g.By("# Patch the LV CR related storageClass allowVolumeExpansion:true")
			scPatchPath := `{"allowVolumeExpansion":true}`
			patchResourceAsAdmin(oc, "", "sc/"+mylv.scname, scPatchPath, "merge")

			g.By("# Patch the pv capacity to expandCapacity")
			pvName := pvc.getVolumeName(oc)
			expandCapacity := strconv.FormatInt(myVolume.ExpandSize, 10) + "Gi"
			pvPatchPath := `{"spec":{"capacity":{"storage":"` + expandCapacity + `"}}}`
			patchResourceAsAdmin(oc, "", "pv/"+pvName, pvPatchPath, "merge")

			g.By("# Patch the pvc capacity to expandCapacity")
			pvc.expand(oc, expandCapacity)
			pvc.waitResizeSuccess(oc, expandCapacity)

			g.By("# Check pod mount volume size updated and the origin data still exist")
			o.Expect(pod.getPodMountFsVolumeSize(oc)).Should(o.Equal(myVolume.ExpandSize))
			pod.checkMountedVolumeDataExist(oc, true)

			g.By("# Write larger than origin capacity and less than new capacity data should succeed")
			msg, err := pod.execCommand(oc, "fallocate -l "+strconv.FormatInt(originVolumeCapacity+getRandomNum(1, 3), 10)+"G "+pod.mountPath+"/"+getRandomString()+" ||true")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).NotTo(o.ContainSubstring("No space left on device"))

			g.By("# Delete pod and pvc and check the related pv's status")
			pod.delete(oc)
			pvc.delete(oc)
			pvc.waitStatusAsExpected(oc, "deleted")
			waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

			g.By("# Create new pvc,pod and check the data in origin volume is cleaned up")
			pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname),
				setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(originVolumeCapacity, myVolume.ExpandSize))+"Gi"))
			pvcNew.create(oc)
			defer pvcNew.deleteAsAdmin(oc)
			podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvcNew.name))
			podNew.create(oc)
			defer podNew.deleteAsAdmin(oc)
			podNew.waitReady(oc)
			// Check the data is cleaned up in the volume
			podNew.checkMountedVolumeDataExist(oc, false)
		})
	}

	// author: pewang@redhat.com
	g.It("Author:pewang-High-32978-Medium-33905-[LSO] [block volume] LocalVolumeSet CR with maxDeviceCount should provision matched device and could be used by Pod [Serial]", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			depTemplate = filepath.Join(lsoBaseDir, "dep-template.yaml")
			lvsTemplate = filepath.Join(lsoBaseDir, "/lso/localvolumeset-template.yaml")
			// Define a localVolumeSet CR with volumeMode:Block  maxDeviceCount:1
			mylvs = newLocalVolumeSet(setLvsNamespace(myLso.namespace), setLvsTemplate(lvsTemplate), setLvsVolumeMode("Block"),
				setLvsMaxDeviceCount(1))
			pvc = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimVolumemode("Block"),
				setPersistentVolumeClaimStorageClassName(mylvs.scname))
			dep = newDeployment(setDeploymentTemplate(depTemplate), setDeploymentPVCName(pvc.name), setDeploymentVolumeType("volumeDevices"),
				setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create 2 aws ebs volumes and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		myVolume1 := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.create(ac)
		defer myVolume1.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume1.create(ac)
		myVolume.waitStateAsExpected(ac, "available")
		myVolume1.waitStateAsExpected(ac, "available")
		// Attach the volumes to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)
		defer myVolume1.detachSucceed(ac)
		myVolume1.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolumeSet cr and wait for device provisioned")
		mylvs.create(oc)
		defer mylvs.deleteAsAdmin(oc)
		mylvs.waitDeviceProvisioned(oc)

		g.By("# Create a pvc use the localVolumeSet storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("# Write file to raw block volume")
		dep.writeDataBlockType(oc)

		g.By("# Scale down the deployment replicas num to zero")
		dep.scaleReplicas(oc, "0")
		dep.waitReady(oc)

		g.By("# Scale up the deployment replicas num to 1 and wait it ready")
		dep.scaleReplicas(oc, "1")
		dep.waitReady(oc)

		g.By("# Check the data still in the raw block volume")
		dep.checkDataBlockType(oc)

		g.By("# Delete deployment and pvc and check the related pv's status")
		pvName := pvc.getVolumeName(oc)
		dep.delete(oc)
		pvc.delete(oc)
		pvc.waitStatusAsExpected(oc, "deleted")
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("# LSO localVolumeSet should only provison 1 volume follow the maxDeviceCount restrict")
		lvPvs, err := getPvNamesOfSpecifiedSc(oc, mylvs.scname)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(lvPvs) == 1).Should(o.BeTrue())

		g.By("# Create new pvc,deployment and check the data in origin volume is cleaned up")
		pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimVolumemode("Block"),
			setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"), setPersistentVolumeClaimStorageClassName(mylvs.scname))
		pvcNew.create(oc)
		defer pvcNew.deleteAsAdmin(oc)
		depNew := newDeployment(setDeploymentTemplate(depTemplate), setDeploymentPVCName(pvcNew.name),
			setDeploymentVolumeType("volumeDevices"), setDeploymentVolumeTypePath("devicePath"), setDeploymentMountpath("/dev/dblock"))
		depNew.create(oc)
		defer depNew.deleteAsAdmin(oc)
		depNew.waitReady(oc)
		// Check the data is cleaned up in the volume
		command := []string{"-n", depNew.namespace, "deployment/" + depNew.name, "--", "/bin/dd if=" + dep.mpath + " of=/tmp/testfile bs=512 count=1"}
		output, err := oc.WithoutNamespace().Run("exec").Args(command...).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("no such file or directory"))
	})

	// author: pewang@redhat.com
	g.It("Author:pewang-Medium-33725-Medium-33726-High-32979-[LSO] [Filesystem ext4] LocalVolumeSet CR with minSize and maxSize should provision matched device and could be used by Pod [Serial]", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvsTemplate = filepath.Join(lsoBaseDir, "/lso/localvolumeset-template.yaml")
			mylvs       = newLocalVolumeSet(setLvsNamespace(myLso.namespace), setLvsTemplate(lvsTemplate), setLvsFstype("ext4"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylvs.scname))
			pod         = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create 3 different capacity aws ebs volume and attach the volume to a schedulable worker node")
		// Create 1 aws ebs volume of random size [5-15Gi] and attach to the schedulable worker node
		// Create 2 aws ebs volumes of random size [1-4Gi] and [16-20Gi] attach to the schedulable worker node
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey), setVolSize(getRandomNum(5, 15)))
		minVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey), setVolSize(getRandomNum(1, 4)))
		maxVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey), setVolSize(getRandomNum(16, 20)))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.create(ac)
		defer minVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		minVolume.create(ac)
		defer maxVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		maxVolume.create(ac)
		myVolume.waitStateAsExpected(ac, "available")
		minVolume.waitStateAsExpected(ac, "available")
		maxVolume.waitStateAsExpected(ac, "available")
		// Attach the volumes to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)
		defer minVolume.detachSucceed(ac)
		minVolume.attachToInstanceSucceed(ac, oc, myWorker)
		defer maxVolume.detachSucceed(ac)
		maxVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolumeSet cr and wait for device provisioned")
		mylvs.create(oc)
		defer mylvs.deleteAsAdmin(oc)
		mylvs.waitDeviceProvisioned(oc)

		g.By("# Create a pvc use the localVolumeSet storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		pod.create(oc)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("# Check the volume fsType as expected")
		pvName := pvc.getVolumeName(oc)
		checkVolumeMountCmdContain(oc, pvName, myWorker.name, "ext4")

		g.By("# Check the pod volume can be read and write and have the exec right")
		pod.checkMountedVolumeCouldRW(oc)
		pod.checkMountedVolumeHaveExecRight(oc)

		g.By("# Check the pv OwnerReference has no node related")
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.metadata.ownerReferences[?(@.kind==\"Node\")].name}").Output()).Should(o.BeEmpty())

		g.By("# Delete pod and pvc and check the related pv's status")
		pod.delete(oc)
		pvc.delete(oc)
		pvc.waitStatusAsExpected(oc, "deleted")
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("# LSO localVolumeSet only provison the matched interval capacity [5-15Gi](defined in lvs cr) volume")
		lvPvs, err := getPvNamesOfSpecifiedSc(oc, mylvs.scname)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(lvPvs) == 1).Should(o.BeTrue())

		g.By("# Create new pvc,pod and check the data in origin volume is cleaned up")
		pvcNew := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylvs.scname),
			setPersistentVolumeClaimCapacity(interfaceToString(getRandomNum(1, myVolume.Size))+"Gi"))
		pvcNew.create(oc)
		defer pvcNew.deleteAsAdmin(oc)
		podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvcNew.name))
		podNew.create(oc)
		defer podNew.deleteAsAdmin(oc)
		podNew.waitReady(oc)
		// Check the data is cleaned up in the volume
		podNew.checkMountedVolumeDataExist(oc, false)
	})

	// author: pewang@redhat.com
	// Customer Scenario for Telco:
	// https://bugzilla.redhat.com/show_bug.cgi?id=2023614
	// https://bugzilla.redhat.com/show_bug.cgi?id=2014083#c18
	// https://access.redhat.com/support/cases/#/case/03078926
	g.It("Author:pewang-Critical-50071-[LSO] LocalVolume CR provisioned volume should be umount when its consumed pod is force deleted", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			podTemplate = filepath.Join(lsoBaseDir, "pod-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("ext4"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			pod         = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		pod.create(oc)
		defer pod.deleteAsAdmin(oc)
		pod.waitReady(oc)

		g.By("# Check the pod volume can be read and write")
		pod.checkMountedVolumeCouldRW(oc)

		g.By("# Force delete pod and check the volume umount form the node")
		pvName := pvc.getVolumeName(oc)
		nodeName := getNodeNameByPod(oc, pod.namespace, pod.name)
		pod.forceDelete(oc)
		checkVolumeNotMountOnNode(oc, pvName, nodeName)

		g.By("# Create new pod and check the data in origin volume is still exist")
		podNew := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		podNew.create(oc)
		defer podNew.deleteAsAdmin(oc)
		podNew.waitReady(oc)
		// Check the origin wrote data is still in the volume
		podNew.checkMountedVolumeDataExist(oc, true)

		g.By("# Force delete the project and check the volume umount from the node and become Available")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", podNew.namespace, "--force", "--grace-period=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Waiting for the volume umount successfully
		checkVolumeNotMountOnNode(oc, pvName, nodeName)
		waitForPersistentVolumeStatusAsExpected(oc, pvName, "Available")

		g.By("Check the diskManager log has no deleter configmap err reported")
		myLso.checkDiskManagerLogContains(oc, "deleter could not get provisioner configmap", false)
	})

	// author: pewang@redhat.com
	// Customer Scenario:
	// https://bugzilla.redhat.com/show_bug.cgi?id=2061447
	g.It("Author:pewang-High-51520-[LSO] LocalVolume CR provisioned volume should have no ownerReferences with Node [Disruptive]", func() {
		// Set the resource definition for the scenario
		var (
			pvcTemplate = filepath.Join(lsoBaseDir, "pvc-template.yaml")
			depTemplate = filepath.Join(lsoBaseDir, "dep-template.yaml")
			lvTemplate  = filepath.Join(lsoBaseDir, "/lso/localvolume-template.yaml")
			mylv        = newLocalVolume(setLvNamespace(myLso.namespace), setLvTemplate(lvTemplate), setLvFstype("ext4"))
			pvc         = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(mylv.scname))
			dep         = newDeployment(setDeploymentTemplate(depTemplate), setDeploymentPVCName(pvc.name))
			pvName      string
		)

		g.By("# Create a new project for the scenario")
		oc.SetupProject() //create new project

		g.By("# Create aws ebs volume and attach the volume to a schedulable worker node")
		myWorker := getOneSchedulableWorker(allNodes)
		myVolume := newEbsVolume(setVolAz(myWorker.avaiableZone), setVolClusterIDTagKey(clusterIDTagKey))
		defer myVolume.delete(ac) // Ensure the volume is deleted even if the case failed on any follow step
		myVolume.createAndReadyToUse(ac)
		// Attach the volume to a schedulable linux worker node
		defer myVolume.detachSucceed(ac)
		myVolume.attachToInstanceSucceed(ac, oc, myWorker)

		g.By("# Create a localvolume cr use diskPath by id with the attached volume")
		mylv.deviceID = myVolume.DeviceByID
		mylv.create(oc)
		defer mylv.deleteAsAdmin(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pv", pvName).Execute()

		g.By("# Create a pvc use the localVolume storageClass and create a pod consume the pvc")
		pvc.capacity = interfaceToString(getRandomNum(1, myVolume.Size)) + "Gi"
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)
		dep.waitReady(oc)

		g.By("# Check the pod volume can be read and write")
		dep.checkPodMountedVolumeCouldRW(oc)

		g.By("# Check the pv OwnerReference has no node related")
		pvName = pvc.getVolumeName(oc)
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.metadata.ownerReferences[?(@.kind==\"Node\")].name}").Output()).Should(o.BeEmpty())

		g.By("# Get the pod locate node's name and cordon the node")
		o.Expect(getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])).Should(o.Equal(myWorker.name))
		// Cordon the node
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("cordon", "node/"+myWorker.name).Execute()).NotTo(o.HaveOccurred())
		// Make sure uncordon the node even if case failed in next several steps
		defer dep.waitReady(oc)
		defer uncordonSpecificNode(oc, myWorker.name)

		g.By("# Delete the node and check the pv's status not become Terminating for 60s")
		deleteSpecifiedResource(oc.AsAdmin(), "node", myWorker.name, "")
		defer waitNodeAvaiable(oc, myWorker.name)
		defer rebootInstanceAndWaitSucceed(ac, myWorker.instanceID)
		// Check the localVolume CR provisioned volume not become "Terminating" after the node object is deleted
		o.Consistently(func() string {
			volState, _ := getPersistentVolumeStatus(oc, pvName)
			return volState
		}, 60*time.Second, 5*time.Second).ShouldNot(o.Equal("Terminating"))
	})
})
