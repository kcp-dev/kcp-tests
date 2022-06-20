package storage

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Define PersistVolume struct
type persistentVolume struct {
	name          string
	accessmode    string
	capacity      string
	driver        string
	volumeHandle  string
	reclaimPolicy string
	scname        string
	template      string
	volumeMode    string
	volumeKind    string
	nfsServerIP   string
}

// function option mode to change the default values of PersistentVolume Object attributes, e.g. name, namespace, accessmode, capacity, volumemode etc.
type persistentVolumeOption func(*persistentVolume)

// Replace the default value of PersistentVolume name attribute
func setPersistentVolumeName(name string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.name = name
	}
}

// Replace the default value of PersistentVolume template attribute
func setPersistentVolumeTemplate(template string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.template = template
	}
}

// Replace the default value of PersistentVolume accessmode attribute
func setPersistentVolumeAccessMode(accessmode string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.accessmode = accessmode
	}
}

// Replace the default value of PersistentVolume capacity attribute
func setPersistentVolumeCapacity(capacity string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.capacity = capacity
	}
}

// Replace the default value of PersistentVolume capacity attribute
func setPersistentVolumeDriver(driver string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.driver = driver
	}
}

// Replace the default value of PersistentVolume volumeHandle attribute
func setPersistentVolumeHandle(volumeHandle string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.volumeHandle = volumeHandle
	}
}

// Replace the default value of PersistentVolume reclaimPolicy attribute
func setPersistentVolumeReclaimPolicy(reclaimPolicy string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.reclaimPolicy = reclaimPolicy
	}
}

// Replace the default value of PersistentVolume scname attribute
func setPersistentVolumeStorageClassName(scname string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.scname = scname
	}
}

// Replace the default value of PersistentVolume volumeMode attribute
func setPersistentVolumeMode(volumeMode string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.volumeMode = volumeMode
	}
}

// Replace the default value of PersistentVolume nfsServerIP attribute
func setPersistentVolumeNfsServerIP(nfsServerIP string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.nfsServerIP = nfsServerIP
	}
}

// Replace the default value of PersistentVolume volumeKind attribute
func setPersistentVolumeKind(volumeKind string) persistentVolumeOption {
	return func(this *persistentVolume) {
		this.volumeKind = volumeKind
	}
}

//  Create a new customized PersistentVolume object
func newPersistentVolume(opts ...persistentVolumeOption) persistentVolume {
	var defaultVolSize string
	switch cloudProvider {
	// AlibabaCloud minimum volume size is 20Gi
	case "alibabacloud":
		defaultVolSize = strconv.FormatInt(getRandomNum(20, 30), 10) + "Gi"
	// IBMCloud minimum volume size is 10Gi
	case "ibmcloud":
		defaultVolSize = strconv.FormatInt(getRandomNum(10, 20), 10) + "Gi"
	// Other Clouds(AWS GCE Azure OSP vSphere) minimum volume size is 1Gi
	default:
		defaultVolSize = strconv.FormatInt(getRandomNum(1, 10), 10) + "Gi"
	}
	defaultPersistentVolume := persistentVolume{
		name:          "manual-pv-" + getRandomString(),
		template:      "csi-pv-template.yaml",
		accessmode:    "ReadWriteOnce",
		capacity:      defaultVolSize,
		driver:        "csi.vsphere.vmware.com",
		volumeHandle:  "",
		reclaimPolicy: "Delete",
		scname:        "slow",
		volumeMode:    "Filesystem",
		volumeKind:    "csi",
	}

	for _, o := range opts {
		o(&defaultPersistentVolume)
	}

	return defaultPersistentVolume
}

// Create new PersistentVolume with customized attributes
func (pv *persistentVolume) create(oc *exutil.CLI) {
	var pvExtraParameters map[string]interface{}
	switch pv.volumeKind {
	// nfs kind PersistentVolume
	case "nfs":
		nfsParameters := map[string]string{
			"path":   "/",
			"server": pv.nfsServerIP,
		}
		pvExtraParameters = map[string]interface{}{
			"jsonPath": `items.0.spec.`,
			"nfs":      nfsParameters,
		}
	// csi kind PersistentVolume
	default:
		csiParameter := map[string]string{
			"driver":       pv.driver,
			"volumeHandle": pv.volumeHandle,
		}
		pvExtraParameters = map[string]interface{}{
			"jsonPath": `items.0.spec.`,
			"csi":      csiParameter,
		}
	}
	pv.createWithExtraParameters(oc, pvExtraParameters)
}

//  Create a new PersistentVolume with extra parameters
func (pv *persistentVolume) createWithExtraParameters(oc *exutil.CLI, extraParameters map[string]interface{}) {
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", pv.template, "-p", "NAME="+pv.name, "ACCESSMODE="+pv.accessmode,
		"CAPACITY="+pv.capacity, "RECLAIMPOLICY="+pv.reclaimPolicy, "SCNAME="+pv.scname, "VOLUMEMODE="+pv.volumeMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the PersistentVolume use kubeadmin
func (pv *persistentVolume) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("pv", pv.name).Execute()
}

// Use the bounded persistent volume claim name get the persistent volume name
func getPersistentVolumeNameByPersistentVolumeClaim(oc *exutil.CLI, namespace string, pvcName string) string {
	pvName, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.spec.volumeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s in namespace %s Bound pv is %q", pvcName, namespace, pvName)
	return pvName
}

// Get the persistent volume status
func getPersistentVolumeStatus(oc *exutil.CLI, pvName string) (string, error) {
	pvStatus, err := oc.AsAdmin().Run("get").Args("pv", pvName, "-o=jsonpath={.status.phase}").Output()
	e2e.Logf("The PV  %s status is %q", pvName, pvStatus)
	return pvStatus, err
}

// Use persistent volume name get the volumeID
func getVolumeIDByPersistentVolumeName(oc *exutil.CLI, pvName string) string {
	volumeID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV %s volumeID is %q", pvName, volumeID)
	return volumeID
}

// Use persistent volume claim name get the volumeID
func getVolumeIDByPersistentVolumeClaimName(oc *exutil.CLI, namespace string, pvcName string) string {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	return getVolumeIDByPersistentVolumeName(oc, pvName)
}

// Use persistent volume name to get the volumeSize
func getPvCapacityByPvcName(oc *exutil.CLI, pvcName string, namespace string) (string, error) {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	volumeSize, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.capacity.storage}").Output()
	e2e.Logf("The PV %s volumesize is %s", pvName, volumeSize)
	return volumeSize, err
}

// Get specified storageClass related pv names
func getPvNamesOfSpecifiedSc(oc *exutil.CLI, scName string) ([]string, error) {
	pvNamesStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", "-o=jsonpath={.items[?(@.spec.storageClassName==\""+scName+"\")].metadata.name}").Output()
	e2e.Logf("The storageClass \"%s\" PVs are %s", scName, pvNamesStr)
	return strings.Split(pvNamesStr, " "), err
}

// Check persistent volume has the Attributes
func checkVolumeCsiContainAttributes(oc *exutil.CLI, pvName string, content string) bool {
	volumeAttributes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeAttributes}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Volume Attributes are %s", volumeAttributes)
	return strings.Contains(volumeAttributes, content)
}

// Get persistent volume annotation value
func getPvAnnotationValues(oc *exutil.CLI, namespace string, pvcName string) string {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	annotationsValue, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.metadata.annotations}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The annotationsValues are %s", annotationsValue)
	return annotationsValue
}

// Wait for PV capacity expand successfully
func waitPVVolSizeToGetResized(oc *exutil.CLI, namespace string, pvcName string, expandedCapactiy string) {
	pvName := getPersistentVolumeNameByPersistentVolumeClaim(oc, namespace, pvcName)
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		capacity, err := getPvCapacityByPvcName(oc, pvcName, namespace)
		if err != nil {
			e2e.Logf("Err occurred: \"%v\", get PV: \"%s\" capacity failed.", err, pvName)
			return false, err
		}
		if capacity == expandedCapactiy {
			e2e.Logf("The PV: \"%s\" capacity expand to \"%s\"", pvName, capacity)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Wait for the PV :%s expand successfully timeout.", pvName))
}

// Wait specified Persist Volume status becomes to expected status
func waitForPersistentVolumeStatusAsExpected(oc *exutil.CLI, pvName string, expectedStatus string) {
	var (
		status string
		err    error
	)
	if expectedStatus == "deleted" {
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			status, err = getPersistentVolumeStatus(oc, pvName)
			if err != nil && strings.Contains(interfaceToString(err), "not found") {
				e2e.Logf("The persist volume '%s' becomes to expected status: '%s' ", pvName, expectedStatus)
				return true, nil
			}
			e2e.Logf("The persist volume '%s' is not deleted yet", pvName)
			return false, nil
		})
	} else {
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			status, err = getPersistentVolumeStatus(oc, pvName)
			if err != nil {
				// Adapt for LSO test
				// When pvc deleted the related pv status become [Released -> Deleted -> Avaiable]
				// The default storageclass reclaimpolicy is delete but after deleted the LSO will generate a same name pv
				if strings.Contains(interfaceToString(err), "not found") {
					e2e.Logf("Get persist volume '%s' status failed of *not fonud*, try another round", pvName)
					return false, nil
				}
				e2e.Logf("Get persist volume '%v' status failed of: %v.", pvName, err)
				return false, err
			}
			if status == expectedStatus {
				e2e.Logf("The persist volume '%s' becomes to expected status: '%s' ", pvName, expectedStatus)
				return true, nil
			}
			return false, nil
		})
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The persist volume '%s' didn't become to expected status'%s' ", pvName, expectedStatus))
}

// Use the retain persist volume create a new persist volume object
func createNewPersistVolumeWithRetainVolume(oc *exutil.CLI, originPvExportJSON string, storageClassName string, newPvName string) {
	var (
		err            error
		outputJSONFile string
	)
	jsonPathList := []string{`spec.claimRef`, `spec.storageClassName`, `status`, `metadata`}
	// vSphere: Do not specify the key storage.kubernetes.io/csiProvisionerIdentity in csi.volumeAttributes in PV specification. This key indicates dynamically provisioned PVs.
	// Note: https://docs.vmware.com/en/VMware-vSphere-Container-Storage-Plug-in/2.0/vmware-vsphere-csp-getting-started/GUID-D736C518-E641-4AA9-8BBD-973891AEB554.html
	if cloudProvider == "vsphere" {
		jsonPathList = append(jsonPathList, `spec.csi.volumeAttributes.storage\.kubernetes\.io\/csiProvisionerIdentity`)
	}
	for _, jsonPath := range jsonPathList {
		originPvExportJSON, err = sjson.Delete(originPvExportJSON, jsonPath)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	pvNameParameter := map[string]interface{}{
		"jsonPath": `metadata.`,
		"name":     newPvName,
	}
	retainPolicyParameter := map[string]interface{}{
		"jsonPath":                      `spec.`,
		"storageClassName":              storageClassName,
		"persistentVolumeReclaimPolicy": "Delete", // Seems invalid when the volumeID ever maked retain
	}
	for _, extraParameter := range []map[string]interface{}{pvNameParameter, retainPolicyParameter} {
		outputJSONFile, err = jsonAddExtraParametersToFile(originPvExportJSON, extraParameter)
		o.Expect(err).NotTo(o.HaveOccurred())
		tempJSONByte, _ := ioutil.ReadFile(outputJSONFile)
		originPvExportJSON = string(tempJSONByte)
	}
	e2e.Logf("The new PV jsonfile of resource is %s", outputJSONFile)
	jsonOutput, _ := ioutil.ReadFile(outputJSONFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", outputJSONFile).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The new persist volume:\"%s\" created", newPvName)
}

// Check if persistent volume has the nodeAffinity
func checkPvNodeAffinityContains(oc *exutil.CLI, pvName string, content string) bool {
	nodeAffinity, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.nodeAffinity}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("PV \"%s\" nodeAffinity: %s", pvName, nodeAffinity)
	return strings.Contains(nodeAffinity, content)
}
