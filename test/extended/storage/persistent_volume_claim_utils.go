package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type persistentVolumeClaim struct {
	name           string
	namespace      string
	scname         string
	template       string
	volumemode     string
	accessmode     string
	capacity       string
	dataSourceName string
}

// function option mode to change the default values of PersistentVolumeClaim parameters, e.g. name, namespace, accessmode, capacity, volumemode etc.
type persistentVolumeClaimOption func(*persistentVolumeClaim)

// Replace the default value of PersistentVolumeClaim name parameter
func setPersistentVolumeClaimName(name string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.name = name
	}
}

// Replace the default value of PersistentVolumeClaim template parameter
func setPersistentVolumeClaimTemplate(template string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.template = template
	}
}

// Replace the default value of PersistentVolumeClaim namespace parameter
func setPersistentVolumeClaimNamespace(namespace string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.namespace = namespace
	}
}

// Replace the default value of PersistentVolumeClaim accessmode parameter
func setPersistentVolumeClaimAccessmode(accessmode string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.accessmode = accessmode
	}
}

// Replace the default value of PersistentVolumeClaim scname parameter
func setPersistentVolumeClaimStorageClassName(scname string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.scname = scname
	}
}

// Replace the default value of PersistentVolumeClaim capacity parameter
func setPersistentVolumeClaimCapacity(capacity string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.capacity = capacity
	}
}

// Replace the default value of PersistentVolumeClaim volumemode parameter
func setPersistentVolumeClaimVolumemode(volumemode string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.volumemode = volumemode
	}
}

// Replace the default value of PersistentVolumeClaim DataSource Name
func setPersistentVolumeClaimDataSourceName(name string) persistentVolumeClaimOption {
	return func(this *persistentVolumeClaim) {
		this.dataSourceName = name
	}
}

//  Create a new customized PersistentVolumeClaim object
func newPersistentVolumeClaim(opts ...persistentVolumeClaimOption) persistentVolumeClaim {
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
	defaultPersistentVolumeClaim := persistentVolumeClaim{
		name:       "my-pvc-" + getRandomString(),
		template:   "pvc-template.yaml",
		namespace:  "",
		capacity:   defaultVolSize,
		volumemode: "Filesystem",
		scname:     "gp2-csi",
		accessmode: "ReadWriteOnce",
	}

	for _, o := range opts {
		o(&defaultPersistentVolumeClaim)
	}

	return defaultPersistentVolumeClaim
}

// Create new PersistentVolumeClaim with customized parameters
func (pvc *persistentVolumeClaim) create(oc *exutil.CLI) {
	if pvc.namespace == "" {
		pvc.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pvc.template, "-p", "PVCNAME="+pvc.name, "PVCNAMESPACE="+pvc.namespace, "SCNAME="+pvc.scname,
		"ACCESSMODE="+pvc.accessmode, "VOLUMEMODE="+pvc.volumemode, "PVCCAPACITY="+pvc.capacity)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Create a new PersistentVolumeClaim with clone dataSource parameters
func (pvc *persistentVolumeClaim) createWithCloneDataSource(oc *exutil.CLI) {
	if pvc.namespace == "" {
		pvc.namespace = oc.Namespace()
	}
	dataSource := map[string]string{
		"kind": "PersistentVolumeClaim",
		"name": pvc.dataSourceName,
	}
	extraParameters := map[string]interface{}{
		"jsonPath":   `items.0.spec.`,
		"dataSource": dataSource,
	}
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", pvc.template, "-p", "PVCNAME="+pvc.name, "PVCNAMESPACE="+pvc.namespace, "SCNAME="+pvc.scname,
		"ACCESSMODE="+pvc.accessmode, "VOLUMEMODE="+pvc.volumemode, "PVCCAPACITY="+pvc.capacity)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Create a new PersistentVolumeClaim with snapshot dataSource parameters
func (pvc *persistentVolumeClaim) createWithSnapshotDataSource(oc *exutil.CLI) {
	if pvc.namespace == "" {
		pvc.namespace = oc.Namespace()
	}
	dataSource := map[string]string{
		"kind":     "VolumeSnapshot",
		"name":     pvc.dataSourceName,
		"apiGroup": "snapshot.storage.k8s.io",
	}
	extraParameters := map[string]interface{}{
		"jsonPath":   `items.0.spec.`,
		"dataSource": dataSource,
	}
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", pvc.template, "-p", "PVCNAME="+pvc.name, "PVCNAMESPACE="+pvc.namespace, "SCNAME="+pvc.scname,
		"ACCESSMODE="+pvc.accessmode, "VOLUMEMODE="+pvc.volumemode, "PVCCAPACITY="+pvc.capacity)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Create a new PersistentVolumeClaim with specified persist volume
func (pvc *persistentVolumeClaim) createWithSpecifiedPV(oc *exutil.CLI, pvName string) {
	if pvc.namespace == "" {
		pvc.namespace = oc.Namespace()
	}
	extraParameters := map[string]interface{}{
		"jsonPath":   `items.0.spec.`,
		"volumeName": pvName,
	}
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", pvc.template, "-p", "PVCNAME="+pvc.name, "PVCNAMESPACE="+pvc.namespace, "SCNAME="+pvc.scname,
		"ACCESSMODE="+pvc.accessmode, "VOLUMEMODE="+pvc.volumemode, "PVCCAPACITY="+pvc.capacity)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the PersistentVolumeClaim
func (pvc *persistentVolumeClaim) delete(oc *exutil.CLI) {
	err := oc.WithoutNamespace().Run("delete").Args("pvc", pvc.name, "-n", pvc.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the PersistentVolumeClaim use kubeadmin
func (pvc *persistentVolumeClaim) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("pvc", pvc.name, "-n", pvc.namespace).Execute()
}

//  Get the PersistentVolumeClaim status
func (pvc *persistentVolumeClaim) getStatus(oc *exutil.CLI) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", pvc.namespace, pvc.name, "-o=jsonpath={.status.phase}").Output()
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvc.name, pvc.namespace, pvcStatus)
	return pvcStatus, err
}

// Get the PersistentVolumeClaim bounded  PersistentVolume's name
func (pvc *persistentVolumeClaim) getVolumeName(oc *exutil.CLI) string {
	pvName, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", pvc.namespace, pvc.name, "-o=jsonpath={.spec.volumeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s in namespace %s Bound pv is %q", pvc.name, pvc.namespace, pvName)
	return pvName
}

// Get the PersistentVolumeClaim bounded  PersistentVolume's volumeID
func (pvc *persistentVolumeClaim) getVolumeID(oc *exutil.CLI) string {
	pvName := pvc.getVolumeName(oc)
	volumeID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pv", pvName, "-o=jsonpath={.spec.csi.volumeHandle}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PV %s volumeID is %q", pvName, volumeID)
	return volumeID
}

//  Get the description of PersistentVolumeClaim
func (pvc *persistentVolumeClaim) getDescription(oc *exutil.CLI) (string, error) {
	output, err := oc.WithoutNamespace().Run("describe").Args("pvc", "-n", pvc.namespace, pvc.name).Output()
	e2e.Logf("****** The PVC  %s in namespace %s detail info: ******\n %s", pvc.name, pvc.namespace, output)
	return output, err
}

// Expand the PersistentVolumeClaim capacity, e.g. expandCapacity string "10Gi"
func (pvc *persistentVolumeClaim) expand(oc *exutil.CLI, expandCapacity string) {
	expandPatchPath := "{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"" + expandCapacity + "\"}}}}"
	patchResourceAsAdmin(oc, pvc.namespace, "pvc/"+pvc.name, expandPatchPath, "merge")
	pvc.capacity = expandCapacity
}

//  Get specified PersistentVolumeClaim status
func getPersistentVolumeClaimStatus(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", namespace, pvcName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvcName, namespace, pvcStatus)
	return pvcStatus, err
}

//  Describe specified PersistentVolumeClaim
func describePersistentVolumeClaim(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	output, err := oc.WithoutNamespace().Run("describe").Args("pvc", "-n", namespace, pvcName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("****** The PVC  %s in namespace %s detail info: ******\n %s", pvcName, namespace, output)
	return output, err
}

//  Get specified PersistentVolumeClaim status type during Resize
func getPersistentVolumeClaimStatusType(oc *exutil.CLI, namespace string, pvcName string) (string, error) {
	pvcStatus, err := oc.WithoutNamespace().Run("get").Args("pvc", pvcName, "-n", namespace, "-o=jsonpath={.status.conditions[0].type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC  %s status in namespace %s is %q", pvcName, namespace, pvcStatus)
	return pvcStatus, err
}

// Apply the patch to Resize volume
func applyVolumeResizePatch(oc *exutil.CLI, pvcName string, namespace string, volumeSize string) (string, error) {
	command1 := "{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"" + volumeSize + "\"}}}}"
	command := []string{"pvc", pvcName, "-n", namespace, "-p", command1, "--type=merge"}
	e2e.Logf("The command is %s", command)
	msg, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(command...).Output()
	if err != nil {
		e2e.Logf("Execute command failed with err:%v .", err)
		return msg, err
	}
	e2e.Logf("The command executed successfully %s", command)
	o.Expect(err).NotTo(o.HaveOccurred())
	return msg, nil
}

// Use persistent volume claim name to get the volumeSize in status.capacity
func getVolSizeFromPvc(oc *exutil.CLI, pvcName string, namespace string) (string, error) {
	volumeSize, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pvc", pvcName, "-n", namespace, "-o=jsonpath={.status.capacity.storage}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The PVC %s volumesize is %s", pvcName, volumeSize)
	return volumeSize, err
}

// Wait for PVC Volume Size to get Resized
func (pvc *persistentVolumeClaim) waitResizeSuccess(oc *exutil.CLI, volResized string) {
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		status, err := getVolSizeFromPvc(oc, pvc.name, pvc.namespace)
		if err != nil {
			e2e.Logf("Err occurred: \"%v\", get PVC: \"%s\" capacity failed.", err, pvc.name)
			return false, err
		}
		if status == volResized {
			e2e.Logf("The PVC capacity updated to : \"%v\"", status)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, did not get Resized.", pvc.name))
}

// Wait for PVC Volume Size to match with Resizing status
func getPersistentVolumeClaimStatusMatch(oc *exutil.CLI, namespace string, pvcName string, expectedValue string) {
	err := wait.Poll(15*time.Second, 120*time.Second, func() (bool, error) {
		status, err := getPersistentVolumeClaimStatusType(oc, namespace, pvcName)
		if err != nil {
			e2e.Logf("the err:%v, to get volume status Type %v .", err, pvcName)
			return false, err
		}
		if status == expectedValue {
			e2e.Logf("The volume size Reached to expected status:%v", status)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The volume:%v, did not reached expected status.", err))
}

// Get pvc list using selector label
func getPvcListWithLabel(oc *exutil.CLI, selectorLabel string) []string {
	pvcList, err := oc.WithoutNamespace().Run("get").Args("pvc", "-n", oc.Namespace(), "-l", selectorLabel, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pvc list is %s", pvcList)
	return strings.Split(pvcList, " ")
}

// Check pvc counts matches with expected number
func checkPvcNumWithLabel(oc *exutil.CLI, selectorLabel string, expectednum string) bool {
	if strconv.Itoa(cap(getPvcListWithLabel(oc, selectorLabel))) == expectednum {
		e2e.Logf("The pvc counts matched to expected replicas number: %s ", expectednum)
		return true
	}
	e2e.Logf("The pvc counts did not matched to expected replicas number: %s", expectednum)
	return false
}

// Wait persistentVolumeClaim status becomes to expected status
func (pvc *persistentVolumeClaim) waitStatusAsExpected(oc *exutil.CLI, expectedStatus string) {
	var (
		status string
		err    error
	)
	if expectedStatus == "deleted" {
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			status, err = pvc.getStatus(oc)
			if err != nil && strings.Contains(interfaceToString(err), "not found") {
				e2e.Logf("The persist volume claim '%s' becomes to expected status: '%s' ", pvc.name, expectedStatus)
				return true, nil
			}
			e2e.Logf("The persist volume claim '%s' is not deleted yet", pvc.name)
			return false, nil
		})
	} else {
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			status, err = pvc.getStatus(oc)
			if err != nil {
				e2e.Logf("Get persist volume claim '%s' status failed of: %v.", pvc.name, err)
				return false, err
			}
			if status == expectedStatus {
				e2e.Logf("The persist volume claim '%s' becomes to expected status: '%s' ", pvc.name, expectedStatus)
				return true, nil
			}
			return false, nil

		})
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The persist volume claim '%s' didn't become to expected status'%s' ", pvc.name, expectedStatus))
}

// Wait persistentVolumeClaim status reach to expected status after 30sec timer
func (pvc *persistentVolumeClaim) waitPvcStatusToTimer(oc *exutil.CLI, expectedStatus string) {
	//Check the status after 30sec of time
	var (
		status string
		err    error
	)
	currentTime := time.Now()
	e2e.Logf("Current time before wait of 30sec: %s", currentTime.String())
	err = wait.Poll(30*time.Second, 60*time.Second, func() (bool, error) {
		currentTime := time.Now()
		e2e.Logf("Current time after wait of 30sec: %s", currentTime.String())
		status, err = pvc.getStatus(oc)
		if err != nil {
			e2e.Logf("Get persist volume claim '%s' status failed of: %v.", pvc.name, err)
			return false, err
		}
		if status == expectedStatus {
			e2e.Logf("The persist volume claim '%s' remained in the expected status '%s'", pvc.name, expectedStatus)
			return true, nil
		}
		describePersistentVolumeClaim(oc, pvc.namespace, pvc.name)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The persist volume claim '%s' changed to status '%s' instead of expected status: '%s' ", pvc.name, status, expectedStatus))
}

// Get valid random capacity by volume type
func getValidRandomCapacityByCsiVolType(csiProvisioner string, volumeType string) string {
	var validRandomCapacityInt64 int64
	switch csiProvisioner {
	// aws-ebs-csi
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html
	// io1, io2, gp2, gp3, sc1, st1,standard
	// Default is gp3 if not set the volumeType in storageClass parameters
	case ebsCsiDriverPrivisioner:
		// General Purpose SSD: 1 GiB - 16 TiB
		ebsGeneralPurposeSSD := []string{"gp2", "gp3"}
		// Provisioned IOPS SSD 4 GiB - 16 TiB
		ebsProvisionedIopsSSD := []string{"io1", "io2"}
		// HDD: {"sc1", "st1" 125 GiB - 16 TiB}, {"standard" 1 GiB - 1 TiB}
		ebsHDD := []string{"sc1", "st1", "standard"}

		if strSliceContains(ebsGeneralPurposeSSD, volumeType) || volumeType == "standard" {
			validRandomCapacityInt64 = getRandomNum(1, 10)
			break
		}
		if strSliceContains(ebsProvisionedIopsSSD, volumeType) {
			validRandomCapacityInt64 = getRandomNum(4, 20)
			break
		}
		if strSliceContains(ebsHDD, volumeType) && volumeType != "standard" {
			validRandomCapacityInt64 = getRandomNum(125, 200)
			break
		}
		validRandomCapacityInt64 = getRandomNum(1, 10)
	// aws-efs-csi
	// https://github.com/kubernetes-sigs/aws-efs-csi-driver
	// Accually for efs-csi volumes the capacity is meaningless
	// efs provides volumes almost unlimited capacity only billed by usage
	case efsCsiDriverPrivisioner:
		validRandomCapacityInt64 = getRandomNum(1, 10)
	default:
		validRandomCapacityInt64 = getRandomNum(1, 10)
	}
	return strconv.FormatInt(validRandomCapacityInt64, 10) + "Gi"
}
