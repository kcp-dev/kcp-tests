package storage

import (
	//"path/filepath"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/gjson"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("vsphere-problem-detector-operator", exutil.KubeConfigPath())
		mo *monitor
	)

	// vsphere-problem-detector test suite infrastructure check
	g.BeforeEach(func() {
		cloudProvider = getCloudProvider(oc)
		if !strings.Contains(cloudProvider, "vsphere") {
			g.Skip("Skip for non-supported infrastructure!!!")
		}
		mo = newMonitor(oc.AsAdmin())
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-44254-[vsphere-problem-detector] should check the node hardware version and report in metric for alerter raising by CSO", func() {

		g.By("# Check HW version from vsphere-problem-detector-operator log")
		vpdPodlog, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/vsphere-problem-detector-operator", "-n", "openshift-cluster-storage-operator", "--limit-bytes", "50000").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(vpdPodlog).NotTo(o.BeEmpty())
		o.Expect(vpdPodlog).To(o.ContainSubstring("has HW version vmx"))

		g.By("# Get the node hardware versioni")
		re := regexp.MustCompile(`HW version vmx-([0-9][0-9])`)
		matchRes := re.FindStringSubmatch(vpdPodlog)
		hwVersion := matchRes[1]
		e2e.Logf("The node hardware version is %v", hwVersion)

		g.By("# Check HW version from metrics")
		token := getSAToken(oc)
		url := "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=vsphere_node_hw_version_total"
		metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("prometheus-k8s-0", "-c", "prometheus", "-n", "openshift-monitoring", "-i", "--", "curl", "-k", "-H", fmt.Sprintf("Authorization: Bearer %v", token), url).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metrics).NotTo(o.BeEmpty())
		o.Expect(metrics).To(o.ContainSubstring("\"hw_version\":\"vmx-" + hwVersion))

		g.By("# Check alert for if there is unsupported HW version")
		if hwVersion == "13" || hwVersion == "14" {
			e2e.Logf("Checking the CSIWithOldVSphereHWVersion alert")
			checkAlertRaised(oc, "CSIWithOldVSphereHWVersion")
		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-Medium-44664-[vsphere-problem-detector] The vSphere cluster is marked as unupgradable if vcenter, esxi versions or HW versions are unsupported", func() {
		g.By("# Get log from vsphere-problem-detector-operator")
		podlog, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/vsphere-problem-detector-operator", "-n", "openshift-cluster-storage-operator").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		mes := map[string]string{
			"HW version":      "Marking cluster un-upgradeable because one or more VMs are on hardware version",
			"esxi version":    "Marking cluster un-upgradeable because host .* is on esxi version",
			"vCenter version": "Marking cluster un-upgradeable because connected vcenter is on",
		}
		for kind, expectedMes := range mes {
			g.By("# Check upgradeable status and reason is expected from clusterversion")
			e2e.Logf("%s: Check upgradeable status and reason is expected from clusterversion if %s not support", kind, kind)
			matched, _ := regexp.MatchString(expectedMes, podlog)
			if matched {
				reason, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].status.conditions[?(.type=='Upgradeable')].reason}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(reason).To(o.Equal("VSphereProblemDetectorController_VSphereOlderVersionDetected"))
				status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].status.conditions[?(.type=='Upgradeable')].status}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(status).To(o.Equal("False"))
				e2e.Logf("The cluster is marked as unupgradeable due to %s", kind)
			} else {
				e2e.Logf("The %s is supported", kind)
			}

		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-45514-[vsphere-problem-detector] should report metric about vpshere env", func() {
		// Add 'vsphere_rwx_volumes_total' metric from ocp 4.10
		g.By("Check metric: vsphere_vcenter_info, vsphere_esxi_version_total, vsphere_node_hw_version_total, vsphere_datastore_total, vsphere_rwx_volumes_total")
		checkStorageMetricsContent(oc, "vsphere_vcenter_info", "api_version")
		checkStorageMetricsContent(oc, "vsphere_esxi_version_total", "api_version")
		checkStorageMetricsContent(oc, "vsphere_node_hw_version_total", "hw_version")
		checkStorageMetricsContent(oc, "vsphere_datastore_total", "instance")
		checkStorageMetricsContent(oc, "vsphere_rwx_volumes_total", "value")
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-37728-[vsphere-problem-detector] should report vsphere_cluster_check_total metric correctly", func() {
		g.By("Check metric vsphere_cluster_check_total should contain CheckDefaultDatastore, CheckFolderPermissions, CheckTaskPermissions, CheckStorageClasses, ClusterInfo check.")
		metric := getStorageMetrics(oc, "vsphere_cluster_check_total")
		clusterCheckList := []string{"CheckDefaultDatastore", "CheckFolderPermissions", "CheckTaskPermissions", "CheckStorageClasses", "ClusterInfo"}
		for i := range clusterCheckList {
			o.Expect(metric).To(o.ContainSubstring(clusterCheckList[i]))
		}
	})

	// author:wduan@redhat.com
	g.It("Author:wduan-High-37729-[vsphere-problem-detector] should report vsphere_node_check_total metric correctly", func() {
		g.By("Check metric vsphere_node_check_total should contain CheckNodeDiskUUID, CheckNodePerf, CheckNodeProviderID, CollectNodeESXiVersion, CollectNodeHWVersion.")
		metric := getStorageMetrics(oc, "vsphere_node_check_total")
		nodeCheckList := []string{"CheckNodeDiskUUID", "CheckNodePerf", "CheckNodeProviderID", "CollectNodeESXiVersion", "CollectNodeHWVersion"}
		for i := range nodeCheckList {
			o.Expect(metric).To(o.ContainSubstring(nodeCheckList[i]))
		}
	})

	// author:pewang@redhat.com
	// Since it'll restart deployment/vsphere-problem-detector-operator maybe conflict with the other vsphere-problem-detector cases,so set it as [Serial]
	g.It("NonPreRelease-Author:pewang-High-48763-[vsphere-problem-detector] should report 'vsphere_rwx_volumes_total' metric correctly [Serial]", func() {
		g.By("# Get the value of 'vsphere_rwx_volumes_total' metric real init value")
		// Restart vsphere-problem-detector-operator and get the init value of 'vsphere_rwx_volumes_total' metric
		detectorOperator.restart(oc.AsAdmin())
		newInstanceName := detectorOperator.getPodList(oc.AsAdmin())[0]
		// When the metric update by restart the instance the metric's pod's `data.result.0.metric.pod` name will change to the newInstanceName
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.metric.pod`, newInstanceName)
		initCount, err := mo.getSpecifiedMetricValue("vsphere_rwx_volumes_total", `data.result.0.value.1`)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create two manual fileshare persist volumes(vSphere CNS File Volume) and one manual general volume")
		// The backend service count the total number of 'fileshare persist volumes' by only count the pvs which volumeHandle prefix with `file:`
		// https://github.com/openshift/vsphere-problem-detector/pull/64/files
		// So I create 2 pvs volumeHandle prefix with `file:` with different accessModes
		// and 1 general pv with accessMode:"ReadWriteOnce" to check the count logic's accurateness
		storageTeamBaseDir := exutil.FixturePath("testdata", "storage")
		pvTemplate := filepath.Join(storageTeamBaseDir, "csi-pv-template.yaml")
		rwxPersistVolume := newPersistentVolume(setPersistentVolumeAccessMode("ReadWriteMany"), setPersistentVolumeHandle("file:a7d6fcdd-1cbd-4e73-a54f-a3c7"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		rwxPersistVolume.create(oc)
		defer rwxPersistVolume.deleteAsAdmin(oc)
		rwoPersistVolume := newPersistentVolume(setPersistentVolumeAccessMode("ReadWriteOnce"), setPersistentVolumeHandle("file:a7d6fcdd-1cbd-4e73-a54f-a3c7"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		rwoPersistVolume.create(oc)
		defer rwoPersistVolume.deleteAsAdmin(oc)
		generalPersistVolume := newPersistentVolume(setPersistentVolumeHandle("a7d6fcdd-1cbd-4e73-a54f-a3c7qawkdl"+getRandomString()), setPersistentVolumeTemplate(pvTemplate))
		generalPersistVolume.create(oc)
		defer generalPersistVolume.deleteAsAdmin(oc)

		g.By("# Check the metric update correctly")
		// Since the vsphere-problem-detector update the metric every hour restart the deployment to trigger the update right now
		detectorOperator.restart(oc.AsAdmin())
		// Wait for 'vsphere_rwx_volumes_total' metric value update correctly
		initCountInt, err := strconv.Atoi(initCount)
		o.Expect(err).NotTo(o.HaveOccurred())
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.value.1`, interfaceToString(initCountInt+2))

		g.By("# Delete one RWX pv and wait for it deleted successfully")
		rwxPersistVolume.deleteAsAdmin(oc)
		waitForPersistentVolumeStatusAsExpected(oc, rwxPersistVolume.name, "deleted")

		g.By("# Check the metric update correctly again")
		detectorOperator.restart(oc.AsAdmin())
		mo.waitSpecifiedMetricValueAsExpected("vsphere_rwx_volumes_total", `data.result.0.value.1`, interfaceToString(initCountInt+1))
	})

	// author:pewang@redhat.com
	// Since it'll make the vSphere CSI driver credential invaild during the execution,so mark it Disruptive
	g.It("Author:pewang-High-48875-[vmware-vsphere-csi-driver-operator] should report 'vsphere_csi_driver_error' metric when couldn't connect to vCenter [Disruptive]", func() {
		g.By("# Get the origin credential of vSphere CSI driver")
		// Make sure the CSO is healthy
		waitCSOhealthy(oc)
		originCredential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/vmware-vsphere-cloud-credentials", "-n", "openshift-cluster-csi-drivers", "-o", "json").Output()
		if strings.Contains(interfaceToString(err), "not found") {
			g.Skip("Unsupport profile or test cluster is abnormal")
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Get the credential pwd key name and key value")
		var pwdKey string
		dataList := strings.Split(gjson.Get(originCredential, `data`).String(), `"`)
		for _, subStr := range dataList {
			if strings.Contains(subStr, "password") {
				pwdKey = subStr
				break
			}
		}
		debugLogf("The credential pwd key name is: \"%s\"", pwdKey)
		originPwd := gjson.Get(originCredential, `data.*password`).String()

		g.By("# Replace the origin credential of vSphere CSI driver to wrong")
		invaildPwd := base64.StdEncoding.EncodeToString([]byte(getRandomString()))
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret/vmware-vsphere-cloud-credentials", "-n", "openshift-cluster-csi-drivers", `-p={"data":{"`+pwdKey+`":"`+invaildPwd+`"}}`).Output()
		// Restore the credential of vSphere CSI driver and make sure the CSO recover healthy by defer
		defer restoreVsphereCSIcredential(oc, pwdKey, originPwd)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))
		debugLogf("Replace the credential of vSphere CSI driver pwd to invaildPwd: \"%s\" succeed", invaildPwd)

		g.By("# Wait for the 'vsphere_csi_driver_error' metric report with correct content")
		mo.waitSpecifiedMetricValueAsExpected("vsphere_csi_driver_error", `data.result.0.metric.failure_reason`, "vsphere_connection_failed")

		g.By("# Check the cluster could still upgrade and cluster storage operator not avaiable")
		// Don't block upgrades if we can't connect to vcenter
		// https://bugzilla.redhat.com/show_bug.cgi?id=2040880
		waitCSOspecifiedStatusValueAsExpected(oc, "Upgradeable", "True")
		waitCSOspecifiedStatusValueAsExpected(oc, "Available", "False")
	})
})
