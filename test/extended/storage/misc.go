package storage

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc                               = exutil.NewCLI("mapi-operator", exutil.KubeConfigPath())
		cloudProviderSupportProvisioners []string
	)
	// ultraSSD Azure cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		generalCsiSupportCheck(cloudProvider)
		cloudProviderSupportProvisioners = getSupportProvisionersByCloudProvider(oc)
	})
	// author: miyadav@redhat.com
	g.It("Longduration-NonPreRelease-Author:miyadav-High-49809-Enable the capability to use UltraSSD disks in Azure worker VMs provisioned by machine-api", func() {
		scenarioSupportProvisioners := []string{"disk.csi.azure.com"}
		var (
			testMachineset         = exutil.MachineSetwithLabelDescription{"machineset-48909", 1, "ultrassd", "Enabled"}
			storageTeamBaseDir     = exutil.FixturePath("testdata", "storage")
			storageClassTemplate   = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			pvcTemplate            = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate            = filepath.Join(storageTeamBaseDir, "pod-cloudcase-template.yaml")
			storageClassParameters = map[string]string{
				"skuname":     "UltraSSD_LRS",
				"kind":        "managed",
				"cachingMode": "None",
			}
			storageClass        = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("disk.csi.azure.com"))
			pvc                 = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name), setPersistentVolumeClaimCapacity("8Gi"))
			pod                 = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
			supportProvisioners = sliceIntersect(scenarioSupportProvisioners, cloudProviderSupportProvisioners)
		)
		extraParameters := map[string]interface{}{
			"parameters":           storageClassParameters,
			"allowVolumeExpansion": true,
		}
		if len(supportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}
		g.By("Create storageclass for Azure with skuname")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc)

		g.By("Create PVC")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create machineset")
		testMachineset.CreateMachineSet(oc)
		defer testMachineset.DeleteMachineSet(oc)

		g.By("Create pod with selector label ultrassd")
		pod.create(oc)
		defer pod.delete(oc)
		pod.waitReady(oc)

		g.By("Check the pv.spec.csi.volumeAttributes.skuname")
		pvName := pvc.getVolumeName(oc)
		skunamePv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes.skuname}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(skunamePv).To(o.Equal("UltraSSD_LRS"))

	})

})
