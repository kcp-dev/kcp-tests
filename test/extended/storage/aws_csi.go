package storage

import (
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("storage-aws-csi", exutil.KubeConfigPath())
		storageTeamBaseDir   string
		storageClassTemplate string
		pvcTemplate          string
		podTemplate          string
	)
	// aws-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "aws") {
			g.Skip("Skip for non-supported cloud provider: *" + cloudProvider + "* !!!")
		}
		storageTeamBaseDir = exutil.FixturePath("testdata", "storage")
		storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
		pvcTemplate = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
		podTemplate = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
	})

	// author: pewang@redhat.com
	// Author:pewang-[AWS-EBS-CSI] [VOLUME-TYPES] support scenarios testsuit
	awsEBSvolTypeTestSuit := map[string]string{
		"24484": "io1",      // High-24484-[AWS-EBS-CSI] [Dynamic PV] io1 type ebs volumes should store data and allow exec of file
		"24546": "sc1",      // High-24546-[AWS-EBS-CSI] [Dynamic PV] sc1 type ebs volumes should store data and allow exec of file
		"24572": "st1",      // High-24572-[AWS-EBS-CSI] [Dynamic PV] st1 type ebs volumes should store data and allow exec of file
		"50272": "io2",      // High-50272-[AWS-EBS-CSI] [Dynamic PV] io2 type ebs volumes should store data and allow exec of file
		"50273": "gp2",      // High-50273-[AWS-EBS-CSI] [Dynamic PV] gp2 type ebs volumes should store data and allow exec of file
		"50274": "gp3",      // High-50274-[AWS-EBS-CSI] [Dynamic PV] gp3 type ebs volumes should store data and allow exec of file
		"50275": "standard", // High-50275-[AWS-EBS-CSI] [Dynamic PV] standard type ebs volumes should store data and allow exec of file
	}
	caseIds := []string{"24484", "24546", "24572", "50272", "50273", "50274", "50275"}
	for i := 0; i < len(caseIds); i++ {
		volumeType := awsEBSvolTypeTestSuit[caseIds[i]]
		// author: pewang@redhat.com
		g.It("Author:pewang-High-"+caseIds[i]+"-[AWS-EBS-CSI] [VOLUME-TYPES] dynamic "+volumeType+" type ebs volume should store data and allow exec of files", func() {
			// Set the resource objects definition for the scenario
			var (
				storageClass = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("ebs.csi.aws.com"))
				pvc          = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
					setPersistentVolumeClaimCapacity(getValidRandomCapacityByCsiVolType("ebs.csi.aws.com", volumeType)))
				pod = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
			)

			g.By("# Create new project for the scenario")
			oc.SetupProject()

			g.By("# Create \"" + volumeType + "\" type aws-ebs-csi storageclass")
			storageClass.createWithExtraParameters(oc, gererateCsiScExtraParametersByVolType(oc, "ebs.csi.aws.com", volumeType))
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not

			g.By("# Create a pvc with the aws-ebs-csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("# Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			waitPodReady(oc, pod.namespace, pod.name)

			g.By("# Check the pvc bound pv's type as expected on the aws backend")
			getCredentialFromCluster(oc)
			volumeID := pvc.getVolumeID(oc)
			o.Expect(getAwsVolumeTypeByVolumeID(volumeID)).To(o.Equal(volumeType))

			if volumeType == "io1" || volumeType == "io2" {
				volCapacityInt64, err := strconv.ParseInt(strings.TrimSuffix(pvc.capacity, "Gi"), 10, 64)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("# Check the pvc bound pv's info on the aws backend, iops = iopsPerGB * volumeCapacity")
				o.Expect(getAwsVolumeIopsByVolumeID(volumeID)).To(o.Equal(int64(volCapacityInt64 * 50)))
			}

			g.By("# Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("# Check the pod volume have the exec right")
			pod.checkMountedVolumeHaveExecRight(oc)
		})
	}
})
