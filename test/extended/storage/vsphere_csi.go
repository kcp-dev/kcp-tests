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

	var oc = exutil.NewCLI("storage-vsphere-csi", exutil.KubeConfigPath())

	// vsphere-csi test suite cloud provider support check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "vsphere") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
	})

	// author: wduan@redhat.com
	g.It("Author:wduan-High-44257-[vSphere CSI Driver Operator] Create StorageClass along with a vSphere Storage Policy", func() {
		var (
			storageTeamBaseDir = exutil.FixturePath("testdata", "storage")
			pvcTemplate        = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
			podTemplate        = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
			pvc                = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName("thin-csi"))
			pod                = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
		)

		// The storageclass/thin-csi should contain the .parameters.StoragePolicyName, and its value should be like "openshift-storage-policy-*"
		g.By("1. Check StoragePolicyName exist in storageclass/thin-csi")
		spn, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("storageclass/thin-csi", "-o=jsonpath={.parameters.StoragePolicyName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(spn).To(o.ContainSubstring("openshift-storage-policy"))

		// Basic check the provisioning with the storageclass/thin-csi
		g.By("2. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		pod.namespace = pvc.namespace

		g.By("3. Create a pvc with the thin-csi storageclass")
		pvc.create(oc)
		defer pvc.delete(oc)

		g.By("4. Create pod with the created pvc and wait for the pod ready")
		pod.create(oc)
		defer pod.delete(oc)
		waitPodReady(oc, pod.namespace, pod.name)

		g.By("5. Check the pvc status to Bound")
		o.Expect(getPersistentVolumeClaimStatus(oc, pvc.namespace, pvc.name)).To(o.Equal("Bound"))
	})

	// author: pewang@redhat.com
	// webhook Validating admission controller helps prevent user from creating or updating StorageClass using "csi.vsphere.vmware.com" as provisioner with these parameters.
	// 1. csimigration
	// 2. datastore-migrationparam
	// 3. diskformat-migrationparam
	// 4. hostfailurestotolerate-migrationparam
	// 5. forceprovisioning-migrationparam
	// 6. cachereservation-migrationparam
	// 7. diskstripes-migrationparam
	// 8. objectspacereservation-migrationparam
	// 9. iopslimit-migrationparam
	// This Validating admission controller also helps prevent user from creating or updating StorageClass using "kubernetes.io/vsphere-volume" as provisioner with AllowVolumeExpansion to true.
	// Reference: https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/release-2.4/docs/book/features/vsphere_csi_migration.md
	// https://issues.redhat.com/browse/STOR-562
	g.It("Author:pewang-High-47387-[vSphere CSI Driver Webhook] should prevent user from creating or updating StorageClass with unsupported parameters", func() {
		// Set the resource definition for the scenario
		var (
			storageTeamBaseDir    = exutil.FixturePath("testdata", "storage")
			storageClassTemplate  = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
			unsupportedParameters = []string{"csimigration", "datastore-migrationparam", "diskformat-migrationparam", "hostfailurestotolerate-migrationparam",
				"forceprovisioning-migrationparam", "cachereservation-migrationparam", "diskstripes-migrationparam", "objectspacereservation-migrationparam",
				"iopslimit-migrationparam"}
			webhookDeployment  = newDeployment(setDeploymentName("vmware-vsphere-csi-driver-webhook"), setDeploymentNamespace("openshift-cluster-csi-drivers"), setDeploymentApplabel("vmware-vsphere-csi-driver-webhook"))
			csiStorageClass    = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("csi.vsphere.vmware.com"))
			intreeStorageClass = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("kubernetes.io/vsphere-volume"))
		)

		g.By("# Check the CSI Driver Webhook deployment is ready")
		webhookDeployment.waitReady(oc.AsAdmin())

		g.By("# Using 'csi.vsphere.vmware.com' as provisioner create storageclass with unsupported parameters")
		for _, unsupportParameter := range unsupportedParameters {
			storageClassParameters := map[string]string{
				unsupportParameter: "true",
			}
			extraParameters := map[string]interface{}{

				"parameters": storageClassParameters,
			}
			e2e.Logf("Using 'csi.vsphere.vmware.com' as provisioner create storageclass with parameters.%s", unsupportParameter)
			err := csiStorageClass.negative().createWithExtraParameters(oc, extraParameters)
			defer csiStorageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.
			o.Expect(interfaceToString(err)).Should(o.ContainSubstring("admission webhook \\\"validation.csi.vsphere.vmware.com\\\" denied the request: Invalid StorageClass Parameters"))
		}

		g.By("# Using 'kubernetes.io/vsphere-volume' as provisioner create storageclass with allowVolumeExpandsion: true")
		extraParameters := map[string]interface{}{
			"allowVolumeExpansion": true,
		}
		err := intreeStorageClass.negative().createWithExtraParameters(oc, extraParameters)
		defer intreeStorageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not.
		o.Expect(interfaceToString(err)).Should(o.ContainSubstring("admission webhook \\\"validation.csi.vsphere.vmware.com\\\" denied the request: AllowVolumeExpansion can not be set to true on the in-tree vSphere StorageClass"))

		g.By("# Check csi driver webhook pod log record the failed requests")
		logRecord, errinfo := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment.apps/vmware-vsphere-csi-driver-webhook", "-n", "openshift-cluster-csi-drivers").Output()
		o.Expect(errinfo).ShouldNot(o.HaveOccurred())
		o.Expect(logRecord).Should(o.And(
			o.ContainSubstring("validation of StorageClass: \\\""+intreeStorageClass.name+"\\\" Failed"),
			o.ContainSubstring("validation of StorageClass: \\\""+csiStorageClass.name+"\\\" Failed")))
	})
})
