package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	qeCatalogSource     string = "qe-app-registry"
	redhatCatalogSource string = "redhat-operators"
	sourceNameSpace     string = "openshift-marketplace"
)

// Define the localStorageOperator struct
type localStorageOperator struct {
	namespace      string
	channel        string
	source         string
	deployTemplate string
	currentCSV     string
}

// function option mode to change the default values of lso attributes
type lsoOption func(*localStorageOperator)

// Replace the default value of lso namespace
func setLsoNamespace(namespace string) lsoOption {
	return func(lso *localStorageOperator) {
		lso.namespace = namespace
	}
}

// Replace the default value of lso channel
func setLsoChannel(channel string) lsoOption {
	return func(lso *localStorageOperator) {
		lso.channel = channel
	}
}

// Replace the default value of lso source
func setLsoSource(source string) lsoOption {
	return func(lso *localStorageOperator) {
		lso.source = source
	}
}

// Replace the default value of lso deployTemplate
func setLsoTemplate(deployTemplate string) lsoOption {
	return func(lso *localStorageOperator) {
		lso.deployTemplate = deployTemplate
	}
}

//  Create a new customized lso object
func newLso(opts ...lsoOption) localStorageOperator {
	defaultLso := localStorageOperator{
		// namespace:      "local-storage-" + getRandomString(),
		namespace:      "",
		channel:        "4.11",
		source:         "qe-app-registry",
		deployTemplate: "/lso/lso-deploy-template.yaml",
	}
	for _, o := range opts {
		o(&defaultLso)
	}
	return defaultLso
}

// Install openshift local storage operator
func (lso *localStorageOperator) install(oc *exutil.CLI) {
	if lso.namespace == "" {
		lso.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplateAsAdmin(oc, "--ignore-unknown-parameters=true", "-f", lso.deployTemplate, "-p", "NAMESPACE="+lso.namespace, "CHANNEL="+lso.channel,
		"SOURCE="+lso.source)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Get openshift local storage operator currentCSV
func (lso *localStorageOperator) getCurrentCSV(oc *exutil.CLI) string {
	var (
		currentCSV string
		errinfo    error
	)
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		currentCSV, errinfo = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", lso.namespace, "Subscription", "-o=jsonpath={.items[?(@.metadata.name==\"local-storage-operator\")].status.currentCSV}").Output()
		if errinfo != nil {
			e2e.Logf("Get local storage operator currentCSV failed :%v, wait for next round get.", errinfo)
			return false, errinfo
		}
		if currentCSV != "" {
			e2e.Logf("The openshift local storage operator currentCSV is: \"%s\"", currentCSV)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		describeSubscription, _ := oc.AsAdmin().WithoutNamespace().Run("describe").Args("-n", lso.namespace, "Subscription/local-storage-operator").Output()
		e2e.Logf("The openshift local storage operator Subscription detail info is:\n \"%s\"", describeSubscription)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Get local storage operator currentCSV in ns/%s timeout", lso.namespace))
	lso.currentCSV = currentCSV
	return currentCSV
}

// Check the cluster CatalogSource, use qeCatalogSource first
// If qeCatalogSource not exist check the redhatCatalogSource
// If both qeCatalogSource and redhatCatalogSource not exist skip the test
func (lso *localStorageOperator) checkClusterCatalogSource(oc *exutil.CLI) error {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", sourceNameSpace, "catalogsource/"+qeCatalogSource).Output()
	if err != nil {
		if strings.Contains(output, "not found") {
			output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", sourceNameSpace, "catalogsource/"+redhatCatalogSource).Output()
			if err != nil {
				if strings.Contains(output, "not found") {
					g.Skip("Skip for both qeCatalogSource and redhatCatalogSource don't exist !!!")
					return nil
				}
				e2e.Logf("Get redhatCatalogSource failed of: \"%v\"", err)
				return err
			}
			lso.source = redhatCatalogSource
			lso.channel = "stable"
			e2e.Logf("Since qeCatalogSource doesn't exist, use offical: \"%s:%s\" instead", lso.source, lso.channel)
			return nil
		}
		e2e.Logf("Get qeCatalogSource failed of: \"%v\"", err)
		return err
	}
	e2e.Logf("qeCatalogSource exist, use qe catalogsource: \"%s:%s\" start test", lso.source, lso.channel)
	return nil
}

// Check openshift local storage operator install succeed
func (lso *localStorageOperator) checkInstallSucceed(oc *exutil.CLI) (bool, error) {
	lsoCSVinfo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", lso.namespace, "csv/"+lso.currentCSV, "-o", "json").Output()
	if err != nil {
		e2e.Logf("Check openshift local storage operator install phase failed : \"%v\".", err)
		return false, nil
	}
	if gjson.Get(lsoCSVinfo, `status.phase`).String() == "Succeeded" && gjson.Get(lsoCSVinfo, `status.reason`).String() == "InstallSucceeded" {
		e2e.Logf("openshift local storage operator:\"%s\" In channel:\"%s\" install succeed in ns/%s", lso.currentCSV, lso.channel, lso.namespace)
		return true, nil
	}
	return false, nil
}

// Waiting for openshift local storage operator install succeed
func (lso *localStorageOperator) waitInstallSucceed(oc *exutil.CLI) {
	lso.getCurrentCSV(oc)
	err := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		return lso.checkInstallSucceed(oc)
	})
	if err != nil {
		e2e.Logf("LSO *%s* install failed caused by:\n%s", lso.currentCSV, getOcDescribeInfo(oc, lso.namespace, "csv", lso.currentCSV))
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting for local storage operator:\"%s\" install succeed in ns/%s timeout", lso.currentCSV, lso.namespace))
}

// Uninstall specified openshift local storage operator
func (lso *localStorageOperator) uninstall(oc *exutil.CLI) error {
	var (
		err           error
		errs          []error
		resourceTypes = []string{"localvolume", "localvolumeset", "localvolumediscovery", "deployment", "ds", "pod", "pvc"}
	)
	for _, resouceType := range resourceTypes {
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", lso.namespace, resouceType, "--all", "--ignore-not-found").Execute()
		if err != nil {
			e2e.Logf("Clean \"%s\" resources failed of %v", resouceType, err)
			errs = append(errs, err)
		}
	}
	o.Expect(errs).Should(o.HaveLen(0))
	e2e.Logf("LSO uninstall Succeed")
	return nil
}

// Get the diskmaker-manager log content
func (lso *localStorageOperator) getDiskManagerLoginfo(oc *exutil.CLI, extraParameters ...string) (string, error) {
	cmdArgs := []string{"-n", lso.namespace, "-l", "app=diskmaker-manager", "-c", "diskmaker-manager"}
	cmdArgs = append(cmdArgs, extraParameters...)
	return oc.AsAdmin().WithoutNamespace().Run("logs").Args(cmdArgs...).Output()
}

// Check diskmaker-manager log contains specified content
func (lso *localStorageOperator) checkDiskManagerLogContains(oc *exutil.CLI, expectedContent string, checkFlag bool) {
	logContent, err := lso.getDiskManagerLoginfo(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	if os.Getenv("STORAGE_LOG_LEVEL") == "DEBUG" {
		path := filepath.Join(e2e.TestContext.OutputDir, "lso-diskmaker-manager-log-"+getRandomString()+".log")
		ioutil.WriteFile(path, []byte(logContent), 0644)
		debugLogf("The diskmaker-manager log is %s", path)
	}
	o.Expect(strings.Contains(logContent, expectedContent)).Should(o.Equal(checkFlag))
}

// Define LocalVolume CR
type localVolume struct {
	name       string
	namespace  string
	deviceID   string
	fsType     string
	scname     string
	volumeMode string
	template   string
}

// function option mode to change the default values of localVolume attributes
type localVolumeOption func(*localVolume)

// Replace the default value of localVolume name
func setLvName(name string) localVolumeOption {
	return func(lv *localVolume) {
		lv.name = name
	}
}

// Replace the default value of localVolume namespace
func setLvNamespace(namespace string) localVolumeOption {
	return func(lv *localVolume) {
		lv.namespace = namespace
	}
}

// Replace the default value of localVolume deviceID
func setLvDeviceID(deviceID string) localVolumeOption {
	return func(lv *localVolume) {
		lv.deviceID = deviceID
	}
}

// Replace the default value of localVolume scname
func setLvScname(scname string) localVolumeOption {
	return func(lv *localVolume) {
		lv.scname = scname
	}
}

// Replace the default value of localVolume volumeMode
func setLvVolumeMode(volumeMode string) localVolumeOption {
	return func(lv *localVolume) {
		lv.volumeMode = volumeMode
	}
}

// Replace the default value of localVolume fsType
func setLvFstype(fsType string) localVolumeOption {
	return func(lv *localVolume) {
		lv.fsType = fsType
	}
}

// Replace the default value of localVolume template
func setLvTemplate(template string) localVolumeOption {
	return func(lv *localVolume) {
		lv.template = template
	}
}

//  Create a new customized localVolume object
func newLocalVolume(opts ...localVolumeOption) localVolume {
	defaultLocalVolume := localVolume{
		name:       "lv-" + getRandomString(),
		namespace:  "",
		deviceID:   "",
		fsType:     "ext4",
		scname:     "lvsc-" + getRandomString(),
		volumeMode: "Filesystem",
		template:   "/lso/localvolume-template.yaml",
	}
	for _, o := range opts {
		o(&defaultLocalVolume)
	}
	return defaultLocalVolume
}

// Create localVolume CR
func (lv *localVolume) create(oc *exutil.CLI) {
	var deletePaths = make([]string, 0, 5)
	if lv.volumeMode == "Block" {
		deletePaths = []string{`items.0.spec.storageClassDevices.0.fsType`}
	}
	err := applyResourceFromTemplateDeleteParametersAsAdmin(oc, deletePaths, "--ignore-unknown-parameters=true", "-f", lv.template, "-p", "NAME="+lv.name, "NAMESPACE="+lv.namespace, "DEVICEID="+lv.deviceID,
		"FSTYPE="+lv.fsType, "SCNAME="+lv.scname, "VOLUMEMODE="+lv.volumeMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Create localVolume CR with extra parameters
func (lv *localVolume) createWithExtraParameters(oc *exutil.CLI, extraParameters map[string]interface{}) {
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", lv.template, "-p", "NAME="+lv.name, "NAMESPACE="+lv.namespace, "DEVICEID="+lv.deviceID,
		"FSTYPE="+lv.fsType, "SCNAME="+lv.scname, "VOLUMEMODE="+lv.volumeMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete localVolume CR
func (lv *localVolume) deleteAsAdmin(oc *exutil.CLI) {
	lvPvs, _ := getPvNamesOfSpecifiedSc(oc, lv.scname)
	// Temp avoid known issue of PV couldn't become avaiable may cause delete LocalVolume CR stucked will remove the extra step later
	// Delete the localvolume CR provisioned PVs before delete the CR accually it should do after delete the CR but just for temp avoid the known issue
	// https://bugzilla.redhat.com/show_bug.cgi?id=2091873
	for _, pv := range lvPvs {
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("pv/" + pv).Execute()
	}
	// Delete the localvolume CR
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("localvolume/"+lv.name, "-n", lv.namespace).Execute()
	for _, pv := range lvPvs {
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("pv/" + pv).Execute()
	}
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sc/" + lv.scname).Execute()
	command := "rm -rf /mnt/local-storage/" + lv.scname
	workers := getWorkersList(oc)
	for _, worker := range workers {
		execCommandInSpecificNode(oc, worker, command)
	}
}

// Define LocalVolumeSet CR
type localVolumeSet struct {
	name           string
	namespace      string
	fsType         string
	maxDeviceCount int64
	scname         string
	volumeMode     string
	template       string
}

// function option mode to change the default values of localVolumeSet attributes
type localVolumeSetOption func(*localVolumeSet)

// Replace the default value of localVolumeSet name
func setLvsName(name string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.name = name
	}
}

// Replace the default value of localVolumeSet namespace
func setLvsNamespace(namespace string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.namespace = namespace
	}
}

// Replace the default value of localVolumeSet storageclass name
func setLvsScname(scname string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.scname = scname
	}
}

// Replace the default value of localVolumeSet fsType
func setLvsFstype(fsType string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.fsType = fsType
	}
}

// Replace the default value of localVolumeSet maxDeviceCount
func setLvsMaxDeviceCount(maxDeviceCount int64) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.maxDeviceCount = maxDeviceCount
	}
}

// Replace the default value of localVolumeSet volumeMode
func setLvsVolumeMode(volumeMode string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.volumeMode = volumeMode
	}
}

// Replace the default value of localVolumeSet template
func setLvsTemplate(template string) localVolumeSetOption {
	return func(lvs *localVolumeSet) {
		lvs.template = template
	}
}

//  Create a new customized localVolumeSet object
func newLocalVolumeSet(opts ...localVolumeSetOption) localVolumeSet {
	defaultLocalVolumeSet := localVolumeSet{
		name:           "lvs-" + getRandomString(),
		namespace:      "",
		fsType:         "ext4",
		maxDeviceCount: 10,
		scname:         "lvs-sc-" + getRandomString(),
		volumeMode:     "Filesystem",
		template:       "/lso/localvolumeset-template.yaml",
	}
	for _, o := range opts {
		o(&defaultLocalVolumeSet)
	}
	return defaultLocalVolumeSet
}

// Create localVolumeSet CR
func (lvs *localVolumeSet) create(oc *exutil.CLI) {
	var deletePaths = make([]string, 0, 5)
	if lvs.volumeMode == "Block" {
		deletePaths = []string{`items.0.spec.storageClassDevices.0.fsType`}
	}
	err := applyResourceFromTemplateDeleteParametersAsAdmin(oc, deletePaths, "--ignore-unknown-parameters=true", "-f", lvs.template, "-p", "NAME="+lvs.name, "NAMESPACE="+lvs.namespace,
		"FSTYPE="+lvs.fsType, "MAXDEVICECOUNT="+strconv.FormatInt(lvs.maxDeviceCount, 10), "SCNAME="+lvs.scname, "VOLUMEMODE="+lvs.volumeMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Create localVolumeSet CR with extra parameters
func (lvs *localVolumeSet) createWithExtraParameters(oc *exutil.CLI, extraParameters map[string]interface{}) {
	err := applyResourceFromTemplateWithExtraParametersAsAdmin(oc, extraParameters, "--ignore-unknown-parameters=true", "-f", lvs.template, "-p", "NAME="+lvs.name, "NAMESPACE="+lvs.namespace,
		"FSTYPE="+lvs.fsType, "MAXDEVICECOUNT="+strconv.FormatInt(lvs.maxDeviceCount, 10), "SCNAME="+lvs.scname, "VOLUMEMODE="+lvs.volumeMode)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete localVolumeSet CR
func (lvs *localVolumeSet) deleteAsAdmin(oc *exutil.CLI) {
	lvsPvs, _ := getPvNamesOfSpecifiedSc(oc, lvs.scname)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("localvolumeSet/"+lvs.name, "-n", lvs.namespace).Execute()
	for _, pv := range lvsPvs {
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("pv/" + pv).Execute()
	}
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sc/" + lvs.scname).Execute()
	command := "rm -rf /mnt/local-storage/" + lvs.scname
	workers := getWorkersList(oc)
	for _, worker := range workers {
		execCommandInSpecificNode(oc, worker, command)
	}
}

// Get the localVolumeSet CR totalProvisionedDeviceCount
func (lvs *localVolumeSet) getTotalProvisionedDeviceCount(oc *exutil.CLI) (int64, error) {
	var (
		output                 string
		provisionedDeviceCount int64
		err                    error
	)
	output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", lvs.namespace, "localvolumeSet/"+lvs.name, "-o=jsonpath={.status.totalProvisionedDeviceCount}").Output()
	if err == nil {
		provisionedDeviceCount, err = strconv.ParseInt(output, 10, 64)
		if err != nil {
			e2e.Logf("The localVolumeSet CR totalProvisionedDeviceCount is: \"%d\"", provisionedDeviceCount)
		}
	}
	return provisionedDeviceCount, err
}

// Waiting for the localVolumeSet CR have already provisioned Device
func (lvs *localVolumeSet) waitDeviceProvisioned(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		provisionedDeviceCount, errinfo := lvs.getTotalProvisionedDeviceCount(oc)
		if errinfo != nil {
			e2e.Logf("Get LVS provisionedDeviceCount failed :%v, wait for next round get.", errinfo)
			return false, errinfo
		}
		if provisionedDeviceCount > 0 {
			e2e.Logf("The localVolumeSet \"%s\" have already provisioned Device [provisionedDeviceCount: %d]", lvs.name, provisionedDeviceCount)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("describe").Args("-n", lvs.namespace, "localvolumeSet/"+lvs.name).Output()
		e2e.Logf("***$ oc describe localVolumeSet/%s\n***%s", lvs.name, output)
		output, _ = oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", lvs.namespace, "-l", "app=diskmaker-manager", "-c", "diskmaker-manager", "--since=2m").Output()
		e2e.Logf("***$ oc logs -l app=diskmaker-manager -c diskmaker-manager --since=2m\n***%s", output)
		e2e.Logf("**************************************************************************")
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting for the localVolumeSet \"%s\" have already provisioned Device timeout", lvs.name))
}
