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
		oc = exutil.NewCLI("storage-gcp-csi", exutil.KubeConfigPath())

		storageTeamBaseDir   string
		storageClassTemplate string
		pvcTemplate          string
		deploymentTemplate   string
		podTemplate          string
	)
	// gcp-csi test suite cloud provider support check

	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "gcp") {
			g.Skip("Skip for non-supported cloud provider!!!")
		}
		storageTeamBaseDir = exutil.FixturePath("testdata", "storage")
		storageClassTemplate = filepath.Join(storageTeamBaseDir, "storageclass-template.yaml")
		pvcTemplate = filepath.Join(storageTeamBaseDir, "pvc-template.yaml")
		deploymentTemplate = filepath.Join(storageTeamBaseDir, "dep-template.yaml")
		podTemplate = filepath.Join(storageTeamBaseDir, "pod-template.yaml")
	})

	// author: chaoyang@redhat.com
	// [GKE-PD-CSI] [Dynamic Regional PV]Check provisioned region pv and drain node function
	g.It("Author:chaoyang-Critical-37490-[GKE-PD-CSI] Check provisioned region pv and drain node function [Disruptive]", func() {
		var (
			storageClass = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))

			pvc = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
				setPersistentVolumeClaimCapacity("200Gi"))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			storageClassParameters = map[string]string{

				"type":             "pd-standard",
				"replication-type": "regional-pd"}
			extraParameters = map[string]interface{}{
				"parameters":           storageClassParameters,
				"allowVolumeExpansion": true,
			}
		)

		g.By("0. Create new project for the scenario")
		oc.SetupProject() //create new project
		pvc.namespace = oc.Namespace()
		dep.namespace = pvc.namespace

		g.By("1. Create gcp-pd-csi storageclass")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc)

		g.By("2. Create a pvc with the gcp-pd-csi storageclass")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("3. Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("4. Wait for the deployment ready")
		dep.waitReady(oc)
		podLabel := "app=" + dep.applabel

		g.By("5. Drain the pod to other nodes")
		nodeName0 := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
		nodeZone0, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+nodeName0, "-o=jsonpath={.metadata.labels.topology\\.gke\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		errDrain := oc.AsAdmin().WithoutNamespace().Run("adm").Args("drain", "nodes/"+nodeName0, "--pod-selector", podLabel).Execute()
		o.Expect(errDrain).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", "nodes/"+nodeName0).Execute()

		g.By("6. Wait for the pod ready again")
		dep.waitReady(oc)
		nodeName1 := getNodeNameByPod(oc, dep.namespace, dep.getPodList(oc)[0])
		nodeZone1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+nodeName1, "-o=jsonpath={.metadata.labels.topology\\.gke\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7. Check node0 and node1 are in different zones")
		o.Expect(nodeZone0).ShouldNot(o.Equal(nodeZone1))

	})

	// author: chaoyang@redhat.com
	// [GKE-PD-CSI] [Dynamic Regional PV]Provision region pv with allowedTopologies
	g.It("Author:chaoyang-High-37514-[GKE-PD-CSI] Check provisioned region pv with allowedTopologies", func() {
		zones := getZonesFromWorker(oc)
		if len(zones) < 2 {
			g.Skip("Have less than 2 zones - skipping test ... ")
		}
		var (
			storageClass = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))

			pvc = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name),
				setPersistentVolumeClaimCapacity("200Gi"))

			dep                    = newDeployment(setDeploymentTemplate(deploymentTemplate), setDeploymentPVCName(pvc.name))
			storageClassParameters = map[string]string{
				"type":             "pd-standard",
				"replication-type": "regional-pd"}
			matchLabelExpressions = []map[string]interface{}{
				{"key": "topology.gke.io/zone",
					"values": zones[:2],
				},
			}
			allowedTopologies = []map[string]interface{}{
				{"matchLabelExpressions": matchLabelExpressions},
			}
			extraParameters = map[string]interface{}{
				"parameters":        storageClassParameters,
				"allowedTopologies": allowedTopologies,
			}
		)

		g.By("Create default namespace")
		oc.SetupProject() //create new project

		g.By("Create storage class with allowedTopologies")
		storageClass.createWithExtraParameters(oc, extraParameters)
		defer storageClass.deleteAsAdmin(oc)

		g.By("Create pvc")
		pvc.create(oc)
		defer pvc.deleteAsAdmin(oc)

		g.By("Create deployment with the created pvc and wait for the pod ready")
		dep.create(oc)
		defer dep.deleteAsAdmin(oc)

		g.By("Wait for the deployment ready")
		dep.waitReady(oc)

		g.By("Check pv nodeAffinity is two items")
		pvName := pvc.getVolumeName(oc)
		outPut, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.nodeAffinity.required.nodeSelectorTerms}").Output()
		o.Expect(outPut).To(o.ContainSubstring(zones[0]))
		o.Expect(outPut).To(o.ContainSubstring(zones[1]))

	})
	// author: chaoyang@redhat.com
	// Author:chaoyang-[GCP-PD-CSI] [VOLUME-TYPES] support scenarios testsuit
	// https://cloud.google.com/compute/docs/disks
	gcpPDvolTypeTestSuit := map[string]string{
		"51150": "pd-ssd",      // High-51150-[GCP-PD-CSI] [Dynamic PV] pd-ssd type volumes should store data and allow exec of file
		"51151": "pd-standard", // High-51151-[GCP-PD-CSI] [Dynamic PV] pd-standard type volumes should store data and allow exec of file
		"51152": "pd-balanced", // High-51152-[GCP-PD-CSI] [Dynamic PV] pd-balanced type volumes should store data and allow exec of file
		"51153": "pd-extreme",  // High-51153-[GCP-PD-CSI] [Dynamic PV] pd-extreme type volumes should store data and allow exec of file
	}
	caseIds := []string{"51150", "51151", "51152", "51153"}
	for i := 0; i < len(caseIds); i++ {
		volumeType := gcpPDvolTypeTestSuit[caseIds[i]]
		g.It("Author:chaoyang-High-"+caseIds[i]+"-[GCP-PD-CSI] [VOLUME-TYPES] dynamic "+volumeType+" type pd volume should store data and allow exec of files", func() {
			var (
				storageClassParameters = map[string]string{
					"replication-type": "none",
					"type":             volumeType,
				}
				extraParameters = map[string]interface{}{
					"parameters": storageClassParameters,
				}
				storageClass = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))
				pvc          = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
				pod          = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(pvc.name))
			)
			g.By("# Create new project for the scenario")
			oc.SetupProject()

			g.By("# Create \"" + volumeType + "\" type gcp-pd-csi storageclass")
			storageClass.createWithExtraParameters(oc, extraParameters)
			defer storageClass.deleteAsAdmin(oc) // ensure the storageclass is deleted whether the case exist normally or not

			g.By("# Create a pvc with the gcp-pd-csi storageclass")
			pvc.create(oc)
			defer pvc.deleteAsAdmin(oc)

			g.By("# Create pod with the created pvc and wait for the pod ready")
			pod.create(oc)
			defer pod.deleteAsAdmin(oc)
			waitPodReady(oc, pod.namespace, pod.name)

			g.By("# Check the pod volume can be read and write")
			pod.checkMountedVolumeCouldRW(oc)

			g.By("# Check the pod volume have the exec right")
			pod.checkMountedVolumeHaveExecRight(oc)
		})

		// author: chaoyang@redhat.com

		g.It("Author:chaoyang-Critical-51995-[GCE-PD-CSI][snapshot]Provision image disk snapshot and restore successfully", func() {
			var (
				volumeSnapshotClassTemplate = filepath.Join(storageTeamBaseDir, "volumesnapshotclass-template.yaml")
				volumesnapshotTemplate      = filepath.Join(storageTeamBaseDir, "volumesnapshot-template.yaml")

				storageClass                  = newStorageClass(setStorageClassTemplate(storageClassTemplate), setStorageClassProvisioner("pd.csi.storage.gke.io"))
				oripvc                        = newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimStorageClassName(storageClass.name))
				oripod                        = newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(oripvc.name))
				volumesnapshotClass           = newVolumeSnapshotClass(setVolumeSnapshotClassTemplate(volumeSnapshotClassTemplate))
				volumeSnapshotClassParameters = map[string]string{
					"image-family":  "openshiftqe-test",
					"snapshot-type": "images",
				}
				vscExtraParameters = map[string]interface{}{
					"parameters":     volumeSnapshotClassParameters,
					"driver":         "pd.csi.storage.gke.io",
					"deletionPolicy": "Delete",
				}
			)
			g.By("# Create new project for the scenario")
			oc.SetupProject()
			storageClass.create(oc)
			defer storageClass.deleteAsAdmin(oc)

			g.By("# Create a pvc with the storageclass")
			oripvc.create(oc)
			defer oripvc.deleteAsAdmin(oc)

			g.By("# Create pod with the created pvc and wait for the pod ready")
			oripod.create(oc)
			defer oripod.deleteAsAdmin(oc)
			waitPodReady(oc, oripod.namespace, oripod.name)

			g.By("# Write file to volume")
			oripod.checkMountedVolumeCouldRW(oc)
			oripod.execCommand(oc, "sync")

			g.By("# Create new volumesnapshotclass with parameter snapshot-type as image")
			volumesnapshotClass.createWithExtraParameters(oc, vscExtraParameters)
			defer volumesnapshotClass.deleteAsAdmin(oc)

			g.By("# Create volumesnapshot with new volumesnapshotclass")
			volumesnapshot := newVolumeSnapshot(setVolumeSnapshotTemplate(volumesnapshotTemplate), setVolumeSnapshotSourcepvcname(oripvc.name), setVolumeSnapshotVscname(volumesnapshotClass.name))
			volumesnapshot.create(oc)
			defer volumesnapshot.delete(oc) //in case of delete volumesnapshot in the steps is failed

			g.By("# Check volumesnapshotcontent type is disk image")
			volumesnapshot.waitReadyToUse(oc)
			vscontent := getVSContentByVSname(oc, volumesnapshot.namespace, volumesnapshot.name)
			//  for example, one snapshotHandle is projects/openshift-qe/global/images/snapshot-2e7b8095-198d-48f2-acdc-96b050a9a07a
			vsContentSnapShotHandle, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("volumesnapshotcontent", vscontent, "-o=jsonpath={.status.snapshotHandle}").Output()
			o.Expect(vsContentSnapShotHandle).To(o.ContainSubstring("images"))

			g.By("# Restore disk image ")
			restorepvc := newPersistentVolumeClaim(setPersistentVolumeClaimTemplate(pvcTemplate), setPersistentVolumeClaimDataSourceName(volumesnapshot.name), setPersistentVolumeClaimStorageClassName(storageClass.name))
			restorepvc.capacity = oripvc.capacity
			restorepvc.createWithSnapshotDataSource(oc)
			defer restorepvc.deleteAsAdmin(oc)

			restorepod := newPod(setPodTemplate(podTemplate), setPodPersistentVolumeClaim(restorepvc.name))
			restorepod.create(oc)
			defer restorepod.deleteAsAdmin(oc)
			restorepod.waitReady(oc)

			g.By("Check the file exist in restored volume")
			restorepod.checkMountedVolumeDataExist(oc, true)
			restorepod.checkMountedVolumeCouldWriteData(oc, true)
			restorepod.checkMountedVolumeHaveExecRight(oc)

		})

	}

})
