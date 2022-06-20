package storage

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()
	var (
		oc                               = exutil.NewCLI("storage-general-intree", exutil.KubeConfigPath())
		cloudProviderSupportProvisioners []string
		expandedCapacity                 string
	)

	//Resize expand capacity
	expandedCapacity = "4Gi"

	// csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		generalIntreeSupportCheck(cloudProvider)
		cloudProviderSupportProvisioners = getIntreeSupportProvisionersByCloudProvider(oc)

		//Identify the cluster version
		clusterVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "version", "-o=jsonpath={.status.desired.version}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The cluster version for platform %s is %s", cloudProvider, clusterVersion)
	})

	// author: ropatil@redhat.com
	// [CSI-Migration] PVCs created with in-tree storageclass,mountOptions are processed by CSI Driver after CSI migration is enabled
	g.It("NonPreRelease-PstChkUpgrade-Author:ropatil-Medium-49496-Upgrade CSIMigration] PVCs created with in-tree storageclass,mountOptions are processed by CSI Driver after CSI migration is enabled", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"kubernetes.io/aws-ebs", "kubernetes.io/azure-disk"}
		// Set the resource template for the scenario
		var (
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, cloudProviderSupportProvisioners)
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}

		g.By("Using the existing project")
		namespace := "migration-upgrade-49496"
		scName := "mysc-49496"
		pvcName := "mypvc-49496"
		depName := "mydep-49496"

		// Check the project exists after upgrade
		_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("project", namespace).Output()
		if err != nil {
			e2e.Failf("There is no project existing with ns %s", namespace)
		}

		// Delete the storageclass
		defer deleteSpecifiedResource(oc.AsAdmin(), "sc", scName, "")
		// Delete the project"
		defer deleteProjectAsAdmin(oc, namespace)
		// Delete the dep,pvc
		defer deleteSpecifiedResource(oc.AsAdmin(), "pvc", pvcName, namespace)
		defer deleteSpecifiedResource(oc.AsAdmin(), "deployment", depName, namespace)

		for _, provisioner := range supportProvisioners {
			g.By("****** CSIMigration post for " + cloudProvider + " platform with provisioner: \"" + provisioner + "\" test phase start" + "******")

			postCheckCommonTestSteps(oc, scName, pvcName, depName, namespace, expandedCapacity)

			g.By("****** CSIMigration post for " + cloudProvider + " platform with provisioner: \"" + provisioner + "\" test phase finished" + "******")

		}
	})

	// author: ropatil@redhat.com
	// [CSI-Migration] PVCs created with in-tree storageclass, block volume are processed by CSI Driver after CSI migration is enabled
	g.It("NonPreRelease-PstChkUpgrade-Author:ropatil-Medium-49678-Upgrade CSIMigration] PVCs created with in-tree storageclass, block volume are processed by CSI Driver after CSI migration is enabled", func() {
		// Define the test scenario support provisioners
		scenarioSupportProvisioners := []string{"kubernetes.io/aws-ebs", "kubernetes.io/azure-disk"}
		// Set the resource template for the scenario
		var (
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, cloudProviderSupportProvisioners)
		)
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}

		g.By("Using the existing project")
		namespace := "migration-upgrade-49678"
		scName := "mysc-49678"
		pvcName := "mypvc-49678"
		depName := "mydep-49678"

		_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("project", namespace).Output()
		if err != nil {
			e2e.Failf("There is no project existing with ns %s", namespace)
		}

		// Delete the storageclass
		defer deleteSpecifiedResource(oc.AsAdmin(), "sc", scName, "")
		// Delete the project"
		defer deleteProjectAsAdmin(oc, namespace)
		// Delete the dep,pvc
		defer deleteSpecifiedResource(oc.AsAdmin(), "pvc", pvcName, namespace)
		defer deleteSpecifiedResource(oc.AsAdmin(), "deployment", depName, namespace)

		for _, provisioner := range supportProvisioners {
			g.By("****** CSIMigration post for " + cloudProvider + " platform with provisioner: \"" + provisioner + "\" test phase start" + "******")

			postCheckCommonTestSteps(oc, scName, pvcName, depName, namespace, expandedCapacity)

			g.By("****** CSIMigration post for " + cloudProvider + " platform with provisioner: \"" + provisioner + "\" test phase finished" + "******")

		}
	})
})

func postCheckCommonTestSteps(oc *exutil.CLI, scName string, pvcName string, depName string, namespace string, expandedCapacity string) {

	g.By("# Get the info volName, podsList")
	volName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.spec.volumeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s in namespace %s Bound pv is %q", pvcName, namespace, volName)
	podsList, _ := getPodsListByLabel(oc.AsAdmin(), namespace, "app")
	e2e.Logf("The podsList is %v", podsList)

	g.By("# Get the pod located node name")
	nodeName := getNodeNameByPod(oc.AsAdmin(), namespace, podsList[0])

	g.By("# Scale down the replicas number to 0")
	err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", depName, "--replicas=0", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("# Wait for the deployment scale down completed and check nodes has no mounted volume")
	waitUntilPodsAreGoneByLabel(oc.AsAdmin(), namespace, "app")
	// Offline resize need the volume is detached from the node and when resize completely then comsume the volume
	checkVolumeNotMountOnNode(oc.AsAdmin(), volName, nodeName)

	g.By("# Apply the patch to Resize the pvc volume")
	o.Expect(applyVolumeResizePatch(oc, pvcName, namespace, expandedCapacity)).To(o.ContainSubstring("patched"))

	g.By("# Check the pvc resizing status type")
	volumeMode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pvc", pvcName, "-n", namespace, "-o=jsonpath={.spec.volumeMode}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if volumeMode == "Filesystem" {
		getPersistentVolumeClaimStatusMatch(oc.AsAdmin(), namespace, pvcName, "FileSystemResizePending")
	} else {
		getPersistentVolumeClaimStatusType(oc.AsAdmin(), namespace, pvcName)
	}

	g.By("# Scale up the replicas number to 1")
	err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", depName, "--replicas=1", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("# Check the pod status in Running state")
	checkPodStatusByLabel(oc.AsAdmin(), namespace, "app", "Running")

	g.By("# Wait for pv volume to get resized")
	waitPVVolSizeToGetResized(oc.AsAdmin(), namespace, pvcName, expandedCapacity)

	g.By("# Get the podsList and nodeName for the pod in the namespace")
	podsList, _ = getPodsListByLabel(oc.AsAdmin(), namespace, "app")
	e2e.Logf("podsList=%s in namespace=%s ", podsList[0], namespace)
	nodeName = getNodeNameByPod(oc.AsAdmin(), namespace, podsList[0])
	e2e.Logf("The nodename in namespace %s for pod %s is %s", namespace, depName, nodeName)

	if volumeMode == "Filesystem" {
		g.By("# Check the volume mounted contains the mount option by exec mount cmd in the node ")
		checkVolumeMountCmdContain(oc.AsAdmin(), volName, nodeName, "debug")
		checkVolumeMountCmdContain(oc.AsAdmin(), volName, nodeName, "discard")
	}

	g.By("# Check if pv have migration annotation parameters after migration")
	annotationValues := getPvAnnotationValues(oc.AsAdmin(), namespace, pvcName)
	o.Expect(strings.Contains(annotationValues, "pv.kubernetes.io/migrated-to")).Should(o.BeTrue())

	g.By("# Check the pod has original data")
	if volumeMode == "Filesystem" {
		output, err := execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "cat /mnt/storage/testfile_*")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("storage test"))

		// After volume expand write data of the new capacity should succeed
		msg, err := execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "fallocate -l 3G /mnt/storage/"+getRandomString()+" ||true")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.ContainSubstring("No space left on device"))
		// Continue write data of the new capacity should fail of "No space left on device"
		msg, err = execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "fallocate -l 2G /mnt/storage/"+getRandomString()+" ||true")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("No space left on device"))
	} else {
		_, err := execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "/bin/dd if=/dev/dblock of=/tmp/testfile bs=512 count=1")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "cat /tmp/testfile | grep 'test data' ")).To(o.ContainSubstring("matches"))

		e2e.Logf("Writing the data as Block level")
		_, err = execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "/bin/dd  if=/dev/null of=/dev/dblock bs=512 count=1")
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = execCommandInSpecificPod(oc.AsAdmin(), namespace, podsList[0], "echo 'test data' > /dev/dblock ")
		o.Expect(err).NotTo(o.HaveOccurred())

	}

	g.By("# Delete the  Resources: deployment, pvc from namespace and check pv does not exist")
	deleteSpecifiedResource(oc.AsAdmin(), "deployment", depName, namespace)
	deleteSpecifiedResource(oc.AsAdmin(), "pvc", pvcName, namespace)
	checkResourcesNotExist(oc.AsAdmin(), "pv", volName, "")
}
