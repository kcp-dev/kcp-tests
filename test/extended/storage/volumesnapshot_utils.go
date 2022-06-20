package storage

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"github.com/tidwall/sjson"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type volumeSnapshot struct {
	name          string
	namespace     string
	vscname       string
	template      string
	sourcepvcname string
}

// function option mode to change the default values of VolumeSnapshot parameters, e.g. name, namespace, volumesnapshotclassname, source.pvcname etc.
type volumeSnapshotOption func(*volumeSnapshot)

// Replace the default value of VolumeSnapshot name parameter
func setVolumeSnapshotName(name string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.name = name
	}
}

// Replace the default value of VolumeSnapshot template parameter
func setVolumeSnapshotTemplate(template string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.template = template
	}
}

// Replace the default value of VolumeSnapshot namespace parameter
func setVolumeSnapshotNamespace(namespace string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.namespace = namespace
	}
}

// Replace the default value of VolumeSnapshot vsc parameter
func setVolumeSnapshotVscname(vscname string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.vscname = vscname
	}
}

// Replace the default value of VolumeSnapshot source.pvc parameter
func setVolumeSnapshotSourcepvcname(sourcepvcname string) volumeSnapshotOption {
	return func(this *volumeSnapshot) {
		this.sourcepvcname = sourcepvcname
	}
}

//  Create a new customized VolumeSnapshot object
func newVolumeSnapshot(opts ...volumeSnapshotOption) volumeSnapshot {
	defaultVolumeSnapshot := volumeSnapshot{
		name:          "my-snapshot-" + getRandomString(),
		template:      "volumesnapshot-template.yaml",
		namespace:     "",
		vscname:       "volumesnapshotclass",
		sourcepvcname: "my-pvc",
	}

	for _, o := range opts {
		o(&defaultVolumeSnapshot)
	}

	return defaultVolumeSnapshot
}

// Create new VolumeSnapshot with customized parameters
func (vs *volumeSnapshot) create(oc *exutil.CLI) {
	if vs.namespace == "" {
		vs.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", vs.template, "-p", "VSNAME="+vs.name, "VSNAMESPACE="+vs.namespace, "SOURCEPVCNAME="+vs.sourcepvcname, "VSCNAME="+vs.vscname)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the VolumeSnapshot
func (vs *volumeSnapshot) delete(oc *exutil.CLI) {
	oc.WithoutNamespace().Run("delete").Args("volumesnapshot", vs.name, "-n", vs.namespace).Execute()
}

//  Check whether the VolumeSnapshot becomes ready_to_use status
func (vs *volumeSnapshot) checkVsStatusReadyToUse(oc *exutil.CLI) (bool, error) {
	vsStatus, err := oc.WithoutNamespace().Run("get").Args("volumesnapshot", "-n", vs.namespace, vs.name, "-o=jsonpath={.status.readyToUse}").Output()
	e2e.Logf("The volumesnapshot %s ready_to_use status in namespace %s is %s", vs.name, vs.namespace, vsStatus)
	return strings.EqualFold("true", vsStatus), err
}

// Waiting the volumesnapshot to ready_to_use
func (vs *volumeSnapshot) waitReadyToUse(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		status, err1 := vs.checkVsStatusReadyToUse(oc)
		if err1 != nil {
			e2e.Logf("The err:%v, wait for volumesnapshot %v to become ready_to_use.", err1, vs.name)
			return status, err1
		}
		if !status {
			e2e.Logf("Waiting the volumesnapshot %v in namespace %v to be ready_to_use.", vs.name, vs.namespace)
			return status, nil
		}
		e2e.Logf("The volumesnapshot %v in namespace %v is ready_to_use.", vs.name, vs.namespace)
		return status, nil
	})

	if err != nil {
		vsDescribe := getOcDescribeInfo(oc, vs.namespace, "volumesnapshot", vs.name)
		e2e.Logf("oc describe volumesnapshot %s:\n%s", vs.name, vsDescribe)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Volumeshapshot %s is not ready_to_use", vs.name))
}

// Create static volumeSnapshot with specified volumeSnapshotContent
func createVolumeSnapshotWithSnapshotHandle(oc *exutil.CLI, originVolumeSnapshotExportJSON string, newVolumeSnapshotName string, volumeSnapshotContentName string, volumesnapshotNamespace string) {
	var (
		err            error
		outputJSONFile string
	)
	jsonPathList := []string{`spec.source.persistentVolumeClaimName`, `status`, `metadata`}

	for _, jsonPath := range jsonPathList {
		originVolumeSnapshotExportJSON, err = sjson.Delete(originVolumeSnapshotExportJSON, jsonPath)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	volumesnapshotName := map[string]interface{}{
		"jsonPath":  `metadata.`,
		"name":      newVolumeSnapshotName,
		"namespace": volumesnapshotNamespace,
	}

	volumeSnapshotContent := map[string]interface{}{
		"jsonPath":                  `spec.source.`,
		"volumeSnapshotContentName": volumeSnapshotContentName,
	}

	for _, extraParameter := range []map[string]interface{}{volumesnapshotName, volumeSnapshotContent} {
		outputJSONFile, err = jsonAddExtraParametersToFile(originVolumeSnapshotExportJSON, extraParameter)
		o.Expect(err).NotTo(o.HaveOccurred())
		tempJSONByte, _ := ioutil.ReadFile(outputJSONFile)
		originVolumeSnapshotExportJSON = string(tempJSONByte)
	}

	e2e.Logf("The new volumesnapshot jsonfile of resource is %s", outputJSONFile)
	jsonOutput, _ := ioutil.ReadFile(outputJSONFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", outputJSONFile).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The new volumeSnapshot:\"%s\" created", newVolumeSnapshotName)

}

// Get the volumeSnapshot's bound VolumeSnapshotContentName
func getVSContentByVSname(oc *exutil.CLI, namespace string, vsName string) string {
	vscontentName, err := oc.WithoutNamespace().Run("get").Args("volumesnapshot", "-n", namespace, vsName, "-o=jsonpath={.status.boundVolumeSnapshotContentName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return vscontentName
}

type volumeSnapshotClass struct {
	name           string
	driver         string
	template       string
	deletionPolicy string
}

// function option mode to change the default values of VolumeSnapshotClass parameters, e.g. name, driver, deletionPolicy  etc.
type volumeSnapshotClassOption func(*volumeSnapshotClass)

// Replace the default value of VolumeSnapshotClass name parameter
func setVolumeSnapshotClassName(name string) volumeSnapshotClassOption {
	return func(this *volumeSnapshotClass) {
		this.name = name
	}
}

// Replace the default value of VolumeSnapshotClass template parameter
func setVolumeSnapshotClassTemplate(template string) volumeSnapshotClassOption {
	return func(this *volumeSnapshotClass) {
		this.template = template
	}
}

// Replace the default value of VolumeSnapshotClass driver parameter
func setVolumeSnapshotClassDriver(driver string) volumeSnapshotClassOption {
	return func(this *volumeSnapshotClass) {
		this.driver = driver
	}
}

// Replace the default value of VolumeSnapshotClass deletionPolicy parameter
func setVolumeSnapshotDeletionpolicy(deletionPolicy string) volumeSnapshotClassOption {
	return func(this *volumeSnapshotClass) {
		this.deletionPolicy = deletionPolicy
	}
}

// Create a new customized VolumeSnapshotClass object
func newVolumeSnapshotClass(opts ...volumeSnapshotClassOption) volumeSnapshotClass {
	defaultVolumeSnapshotClass := volumeSnapshotClass{
		name:           "my-snapshotclass-" + getRandomString(),
		template:       "volumesnapshotclass-template.yaml",
		driver:         "ebs.csi.aws.com",
		deletionPolicy: "Retain",
	}
	for _, o := range opts {
		o(&defaultVolumeSnapshotClass)
	}

	return defaultVolumeSnapshotClass
}

// Create new VolumeSnapshotClass with customized parameters
func (vsc *volumeSnapshotClass) create(oc *exutil.CLI) {
	err := applyResourceFromTemplateAsAdmin(oc, "--ignore-unknown-parameters=true", "-f", vsc.template, "-p", "VSCNAME="+vsc.name, "DRIVER="+vsc.driver, "DELETIONPOLICY="+vsc.deletionPolicy)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Create a new customized VolumeSnapshotClass with extra parameters
func (vsc *volumeSnapshotClass) createWithExtraParameters(oc *exutil.CLI, extraParameters map[string]interface{}) {
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", vsc.template, "-p", "VSCNAME="+vsc.name, "DRIVER="+vsc.driver, "DELETIONPOLICY="+vsc.deletionPolicy)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//  Delete the VolumeSnapshotClass
func (vsc *volumeSnapshotClass) deleteAsAdmin(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("volumesnapshotclass", vsc.name).Execute()
}

type volumeSnapshotContent struct {
	vscontentname  string
	deletionPolicy string
	driver         string
	snHandle       string
	vsclassname    string
	vsname         string
	vsnamespace    string
	template       string
}

// function option mode to change the default values of VolumeSnapshotContent parameters, e.g. name, driver, deletionPolicy  etc.
type volumeSnapshotContentOption func(*volumeSnapshotContent)

// Replace the default value of VolumeSnapshotContent name
func setVolumeSnapshotContentName(vscontentName string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.vscontentname = vscontentName
	}
}

// Replace the default value of VolumeSnapshotContent driver
func setVolumeSnapshotContentDriver(driver string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.driver = driver
	}
}

// Replace the default value of VolumeSnapshotContent deletionPolicy
func setVolumeSnapshotContentDeletionPolicy(deletionPolicy string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.deletionPolicy = deletionPolicy
	}
}

// Replace the default value of VolumeSnapshotContent Handle
func setVolumeSnapshotContentSnapshotHandle(snapshotHandle string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.snHandle = snapshotHandle
	}
}

// Replace the default value of VolumeSnapshotContent ref volumeSnapshotClass name
func setVolumeSnapshotContentVolumeSnapshotClass(volumeSnapshotClass string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.vsclassname = volumeSnapshotClass
	}
}

// Replace the default value of VolumeSnapshotContent ref volumeSnapshotClass
func setVolumeSnapshotContentRefVolumeSnapshotName(volumeSnapshotName string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.vsname = volumeSnapshotName
	}
}

// Replace the default value of VolumeSnapshotContent ref volumeSnapshot namespace
func setVolumeSnapshotContentRefVolumeSnapshotNS(volumeSnapshotNS string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.vsnamespace = volumeSnapshotNS
	}
}

// Replace the default value of VolumeSnapshotContent template
func setVolumeSnapshotContentTemplate(volumeSnapshotContentTemplate string) volumeSnapshotContentOption {
	return func(this *volumeSnapshotContent) {
		this.template = volumeSnapshotContentTemplate
	}
}

//  Create a new customized VolumeSnapshotContent object
func newVolumeSnapshotContent(opts ...volumeSnapshotContentOption) volumeSnapshotContent {
	defaultVolumeSnapshotContent := volumeSnapshotContent{
		vscontentname:  "my-volumesnapshotcontent-" + getRandomString(),
		deletionPolicy: "Delete",
		driver:         "ebs.csi.aws.com",
		snHandle:       "snap-0e4bf1485cde980a5",
		vsclassname:    "csi-aws-vsc",
		vsname:         "my-volumesnapshot" + getRandomString(),
		vsnamespace:    "",
		template:       "volumesnapshotcontent-template.yaml",
	}

	for _, o := range opts {
		o(&defaultVolumeSnapshotContent)
	}

	return defaultVolumeSnapshotContent
}

// Create new VolumeSnapshotContent with customized parameters
func (vsc *volumeSnapshotContent) create(oc *exutil.CLI) {
	if vsc.vsnamespace == "" {
		vsc.vsnamespace = oc.Namespace()
	}
	err := applyResourceFromTemplateAsAdmin(oc, "--ignore-unknown-parameters=true", "-f", vsc.template, "-p", "VSCONTENTNAME="+vsc.vscontentname, "VSNAMESPACE="+vsc.vsnamespace, "DELETIONPOLICY="+vsc.deletionPolicy, "DRIVER="+vsc.driver, "SNHANDLE="+vsc.snHandle, "VSCLASSNAME="+vsc.vsclassname, "VSNAME="+vsc.vsname)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Get specified volumesnapshotcontent's deletionPolicy
func getVSContentDeletionPolicy(oc *exutil.CLI, vscontentName string) string {
	vscontentDeletionPolicy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("volumesnapshotcontent", vscontentName, "-o=jsonpath={.spec.deletionPolicy}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return vscontentDeletionPolicy
}
