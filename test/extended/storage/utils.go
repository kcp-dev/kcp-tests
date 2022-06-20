package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Define the global cloudProvider
var cloudProvider string

//  Kubeadmin user use oc client apply yaml template
func applyResourceFromTemplateAsAdmin(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("as admin fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	jsonOutput, _ := ioutil.ReadFile(configFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

//  Common user use oc client apply yaml template
func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.Run("process").Args(parameters...).OutputToFile(getRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	jsonOutput, _ := ioutil.ReadFile(configFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	return oc.WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

//  Get a random string of 8 byte
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
// Define vaild devMap for aws instance ebs volume device "/dev/sd[f-p]"
var devMaps = map[string]bool{"f": false, "g": false, "h": false, "i": false, "j": false,
	"k": false, "l": false, "m": false, "n": false, "o": false, "p": false}

// Get a valid device for EFS volume attach
func getVaildDeviceForEbsVol() string {
	var validStr string
	for k, v := range devMaps {
		if !v {
			devMaps[k] = true
			validStr = k
			break
		}
	}
	e2e.Logf("validDevice: \"/dev/sd%s\", devMaps: \"%+v\"", validStr, devMaps)
	return "/dev/sd" + validStr
}

//  Get the cloud provider type of the test environment
func getCloudProvider(oc *exutil.CLI) string {
	var (
		errMsg error
		output string
	)
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		output, errMsg = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		if errMsg != nil {
			e2e.Logf("Get cloudProvider *failed with* :\"%v\",wait 5 seconds retry.", errMsg)
			return false, errMsg
		}
		e2e.Logf("The test cluster cloudProvider is :\"%s\".", strings.ToLower(output))
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "Waiting for get cloudProvider timeout")
	return strings.ToLower(output)
}

//  Get the cluster infrastructureName(ClusterID)
func getClusterID(oc *exutil.CLI) (string, error) {
	clusterID, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
	if err != nil || clusterID == "" {
		e2e.Logf("Get infrastructureName(ClusterID) failed with \"%v\", Or infrastructureName(ClusterID) is null:\"%s\"", err, clusterID)
	} else {
		debugLogf("The infrastructureName(ClusterID) is:\"%s\"", clusterID)
	}
	return clusterID, err
}

//  Get the cluster version channel x.x (e.g. 4.11)
func getClusterVersionChannel(oc *exutil.CLI) string {
	// clusterbot env don't have ".spec.channel", So change to use desire version
	clusterVersion, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("clusterversion", "-o=jsonpath={.items[?(@.kind==\"ClusterVersion\")].status.desired.version}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	tempSlice := strings.Split(clusterVersion, ".")
	clusterVersion = tempSlice[0] + "." + tempSlice[1]
	e2e.Logf("The Cluster version is belong to channel: \"%s\"", clusterVersion)
	return clusterVersion
}

//  Strings contain sub string check
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// Strings slice contains duplicate string check
func containsDuplicate(strings []string) bool {
	elemMap := make(map[string]bool)
	for _, value := range strings {
		if _, ok := elemMap[value]; ok {
			return true
		}
		elemMap[value] = true
	}
	return false
}

// Convert interface type to string
func interfaceToString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}

	switch value.(type) {
	case float64:
		ft := value.(float64)
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := value.(float32)
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		it := value.(int)
		key = strconv.Itoa(it)
	case uint:
		it := value.(uint)
		key = strconv.Itoa(int(it))
	case int8:
		it := value.(int8)
		key = strconv.Itoa(int(it))
	case uint8:
		it := value.(uint8)
		key = strconv.Itoa(int(it))
	case int16:
		it := value.(int16)
		key = strconv.Itoa(int(it))
	case uint16:
		it := value.(uint16)
		key = strconv.Itoa(int(it))
	case int32:
		it := value.(int32)
		key = strconv.Itoa(int(it))
	case uint32:
		it := value.(uint32)
		key = strconv.Itoa(int(it))
	case int64:
		it := value.(int64)
		key = strconv.FormatInt(it, 10)
	case uint64:
		it := value.(uint64)
		key = strconv.FormatUint(it, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}

	return key
}

// Json add extra parameters to jsonfile
func jsonAddExtraParametersToFile(jsonInput string, extraParameters map[string]interface{}) (string, error) {
	var (
		jsonPath string
		err      error
	)
	if interfaceToString(extraParameters["jsonPath"]) == "" {
		jsonPath = `items.0.`
	} else {
		jsonPath = interfaceToString(extraParameters["jsonPath"])
	}
	for extraParametersKey, extraParametersValue := range extraParameters {
		if extraParametersKey != "jsonPath" {
			jsonInput, err = sjson.Set(jsonInput, jsonPath+extraParametersKey, extraParametersValue)
			debugLogf("Process jsonPath: \"%s\" Value: \"%s\"", jsonPath+extraParametersKey, extraParametersValue)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
	if cloudProvider == "ibmcloud" && !gjson.Get(jsonInput, `items.0.parameters.profile`).Bool() && strings.EqualFold(gjson.Get(jsonInput, `items.0.kind`).String(), "storageclass") {
		jsonInput, err = sjson.Set(jsonInput, jsonPath+"parameters.profile", "10iops-tier")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	path := filepath.Join(e2e.TestContext.OutputDir, "storageConfig"+"-"+getRandomString()+".json")
	return path, ioutil.WriteFile(path, pretty.Pretty([]byte(jsonInput)), 0644)
}

// Json delete paths to jsonfile
func jsonDeletePathsToFile(jsonInput string, deletePaths []string) (string, error) {
	var err error
	if len(deletePaths) != 0 {
		for _, path := range deletePaths {
			jsonInput, err = sjson.Delete(jsonInput, path)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
	path := filepath.Join(e2e.TestContext.OutputDir, "storageConfig"+"-"+getRandomString()+".json")
	return path, ioutil.WriteFile(path, pretty.Pretty([]byte(jsonInput)), 0644)
}

//  Kubeadmin user use oc client apply yaml template delete parameters
func applyResourceFromTemplateDeleteParametersAsAdmin(oc *exutil.CLI, deletePaths []string, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile, _ = jsonDeletePathsToFile(output, deletePaths)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("as admin fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	jsonOutput, _ := ioutil.ReadFile(configFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

//  Kubeadmin user use oc client apply yaml template with extra parameters
func applyResourceFromTemplateWithExtraParametersAsAdmin(oc *exutil.CLI, extraParameters map[string]interface{}, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile, _ = jsonAddExtraParametersToFile(output, extraParameters)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("as admin fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	jsonOutput, _ := ioutil.ReadFile(configFile)
	debugLogf("The file content is: \n%s", jsonOutput)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

// None dupulicate element slice intersect
func sliceIntersect(slice1, slice2 []string) []string {
	m := make(map[string]int)
	sliceResult := make([]string, 0)
	for _, value1 := range slice1 {
		m[value1]++
	}

	for _, value2 := range slice2 {
		appearTimes := m[value2]
		if appearTimes == 1 {
			sliceResult = append(sliceResult, value2)
		}
	}
	return sliceResult
}

// Convert String Slice to Map: map[string]struct{}
func convertStrSliceToMap(strSlice []string) map[string]struct{} {
	set := make(map[string]struct{}, len(strSlice))
	for _, v := range strSlice {
		set[v] = struct{}{}
	}
	return set
}

// Judge whether the map contains specified key
func isInMap(inputMap map[string]struct{}, inputString string) bool {
	_, ok := inputMap[inputString]
	return ok
}

// Judge whether the String Slice contains specified element, return bool
func strSliceContains(sl []string, element string) bool {
	return isInMap(convertStrSliceToMap(sl), element)
}

// Common csi cloud provider support check
func generalCsiSupportCheck(cloudProvider string) {
	generalCsiSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-csi-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	supportPlatformsBool := gjson.GetBytes(generalCsiSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+")|@flatten").Exists()
	e2e.Logf("%s * %v * %v", cloudProvider, gjson.GetBytes(generalCsiSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten"), supportPlatformsBool)
	if !supportPlatformsBool {
		g.Skip("Skip for non-supported cloud provider: " + cloudProvider + "!!!")
	}
}

// Common Intree cloud provider support check
func generalIntreeSupportCheck(cloudProvider string) {
	generalIntreeSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-intree-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	supportPlatformsBool := gjson.GetBytes(generalIntreeSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+")|@flatten").Exists()
	e2e.Logf("%s * %v * %v", cloudProvider, gjson.GetBytes(generalIntreeSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten"), supportPlatformsBool)
	if !supportPlatformsBool {
		g.Skip("Skip for non-supported cloud provider: " + cloudProvider + "!!!")
	}
}

// Get common csi provisioners by cloudplatform
func getSupportProvisionersByCloudProvider(oc *exutil.CLI) []string {
	csiCommonSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-csi-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	supportProvisioners := []string{}
	supportProvisionersResult := gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten").Array()
	e2e.Logf("%s support provisioners are : %v", cloudProvider, supportProvisionersResult)
	for i := 0; i < len(supportProvisionersResult); i++ {
		supportProvisioners = append(supportProvisioners, gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten."+strconv.Itoa(i)).String())
	}
	if cloudProvider == "aws" && !checkCSIDriverInstalled(oc, []string{"efs.csi.aws.com"}) {
		supportProvisioners = deleteElement(supportProvisioners, "efs.csi.aws.com")
		e2e.Logf("***%s \"AWS-EFS CSI Driver\" not installed, updating support provisioners to: %v***", cloudProvider, supportProvisioners)
	}
	if cloudProvider == "azure" && checkFips(oc) {
		supportProvisioners = deleteElement(supportProvisioners, "file.csi.azure.com")
		e2e.Logf("***%s \"Azure-file CSI Driver\" don't support FIPS enabled env, updating support provisioners to: %v***", cloudProvider, supportProvisioners)
	}
	return supportProvisioners
}

// Get common csi volumetypes by cloudplatform
func getSupportVolumesByCloudProvider() []string {
	csiCommonSupportVolumeMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-csi-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	supportVolumes := []string{}
	supportVolumesResult := gjson.GetBytes(csiCommonSupportVolumeMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").volumetypes|@flatten").Array()
	e2e.Logf("%s support volumes are : %v", cloudProvider, supportVolumesResult)
	for i := 0; i < len(supportVolumesResult); i++ {
		supportVolumes = append(supportVolumes, gjson.GetBytes(csiCommonSupportVolumeMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").volumetypes|@flatten."+strconv.Itoa(i)).String())
	}
	return supportVolumes
}

// Get common Intree provisioners by cloudplatform
func getIntreeSupportProvisionersByCloudProvider(oc *exutil.CLI) []string {
	csiCommonSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-intree-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	supportProvisioners := []string{}
	supportProvisionersResult := gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten").Array()
	e2e.Logf("%s support provisioners are : %v", cloudProvider, supportProvisionersResult)
	for i := 0; i < len(supportProvisionersResult); i++ {
		supportProvisioners = append(supportProvisioners, gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#.name|@flatten."+strconv.Itoa(i)).String())
	}
	return supportProvisioners
}

// Get pre-defined storageclass by cloudplatform and provisioner
func getPresetStorageClassNameByProvisioner(cloudProvider string, provisioner string) string {
	csiCommonSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-csi-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	return gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#(name="+provisioner+").preset_scname").String()
}

// Get pre-defined storageclass by cloudplatform and provisioner
func getIntreePresetStorageClassNameByProvisioner(cloudProvider string, provisioner string) string {
	intreeCommonSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-intree-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	return gjson.GetBytes(intreeCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#(name="+provisioner+").preset_scname").String()
}

// Get pre-defined volumesnapshotclass by cloudplatform and provisioner
func getPresetVolumesnapshotClassNameByProvisioner(cloudProvider string, provisioner string) string {
	csiCommonSupportMatrix, err := ioutil.ReadFile(filepath.Join(exutil.FixturePath("testdata", "storage"), "general-csi-support-provisioners.json"))
	o.Expect(err).NotTo(o.HaveOccurred())
	return gjson.GetBytes(csiCommonSupportMatrix, "support_Matrix.platforms.#(name="+cloudProvider+").provisioners.#(name="+provisioner+").preset_vscname").String()
}

// Get the now timestamp mil second
func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

// Log output the storage debug info
func debugLogf(format string, args ...interface{}) {
	if logLevel := os.Getenv("STORAGE_LOG_LEVEL"); logLevel == "DEBUG" {
		e2e.Logf(fmt.Sprintf(nowStamp()+": *STORAGE_DEBUG*:\n"+format, args...))
	}
}

func getZonesFromWorker(oc *exutil.CLI) []string {
	var workerZones []string
	workerNodes, err := exutil.GetClusterNodesBy(oc, "worker")
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, workerNode := range workerNodes {
		zone, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes/"+workerNode, "-o=jsonpath={.metadata.labels.failure-domain\\.beta\\.kubernetes\\.io\\/zone}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !contains(workerZones, zone) {
			workerZones = append(workerZones, zone)
		}
	}

	return workerZones
}

// Common oc CLI
//  Get the oc describe info, set namespace as "" for cluster-wide resource
func getOcDescribeInfo(oc *exutil.CLI, namespace string, resourceKind string, resourceName string) string {
	var ocDescribeInfo string
	var err error
	if namespace != "" {
		ocDescribeInfo, err = oc.WithoutNamespace().Run("describe").Args("-n", namespace, resourceKind, resourceName).Output()
	} else {
		ocDescribeInfo, err = oc.WithoutNamespace().Run("describe").Args(resourceKind, resourceName).Output()
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	return ocDescribeInfo
}

// Get a random number of int64 type [m,n], n > m
func getRandomNum(m int64, n int64) int64 {
	rand.Seed(time.Now().UnixNano())
	return rand.Int63n(n-m+1) + m
}

// Restore the credential of vSphere CSI driver
func restoreVsphereCSIcredential(oc *exutil.CLI, pwdKey string, originPwd string) error {
	e2e.Logf("****** Restore the credential of vSphere CSI driver and make sure the CSO recover healthy ******")
	output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret/vmware-vsphere-cloud-credentials", "-n", "openshift-cluster-csi-drivers", `-p={"data":{"`+pwdKey+`":"`+originPwd+`"}}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("patched"))
	driverController.waitReady(oc.AsAdmin())
	// Make sure the Cluster Storage Operator recover healthy
	waitCSOhealthy(oc.AsAdmin())
	return nil
}

// Delete string list's specified string element
func deleteElement(list []string, element string) []string {
	result := make([]string, 0)
	for _, v := range list {
		if v != element {
			result = append(result, v)
		}
	}
	return result
}

// Get Cluster Storage Operator specified status value
func getCSOspecifiedStatusValue(oc *exutil.CLI, specifiedStatus string) (string, error) {
	status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/storage", "-o=jsonpath={.status.conditions[?(.type=='"+specifiedStatus+"')].status}").Output()
	debugLogf("CSO \"%s\" status value is \"%s\"", specifiedStatus, status)
	return status, err
}

// Wait for Cluster Storage Operator specified status value as expected
func waitCSOspecifiedStatusValueAsExpected(oc *exutil.CLI, specifiedStatus string, expectedValue string) {
	pollErr := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		realValue, err := getCSOspecifiedStatusValue(oc, specifiedStatus)
		if err != nil {
			e2e.Logf("Get CSO \"%s\" status value failed of: \"%v\"", err)
			return false, err
		}
		if realValue == expectedValue {
			e2e.Logf("CSO \"%s\" status value become expected \"%s\"", specifiedStatus, expectedValue)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(pollErr, fmt.Sprintf("Waiting for CSO \"%s\" status value become expected \"%s\" timeout", specifiedStatus, expectedValue))
}

// Check Cluster Storage Operator healthy
func checkCSOhealthy(oc *exutil.CLI) (bool, error) {
	// CSO healthyStatus:[degradedStatus:False, progressingStatus:False, avaiableStatus:True, upgradeableStatus:True]
	var healthyStatus = []string{"False", "False", "True", "True"}
	csoStatusJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/storage", "-o", "json").Output()
	degradedStatus := gjson.Get(csoStatusJSON, `status.conditions.#(type=Degraded).status`).String()
	progressingStatus := gjson.Get(csoStatusJSON, `status.conditions.#(type=Progressing).status`).String()
	avaiableStatus := gjson.Get(csoStatusJSON, `status.conditions.#(type=Available).status`).String()
	upgradeableStatus := gjson.Get(csoStatusJSON, `status.conditions.#(type=Upgradeable).status`).String()
	e2e.Logf("CSO degradedStatus:%s, progressingStatus:%v, avaiableStatus:%v, upgradeableStatus:%v", degradedStatus, progressingStatus, avaiableStatus, upgradeableStatus)
	return reflect.DeepEqual([]string{degradedStatus, progressingStatus, avaiableStatus, upgradeableStatus}, healthyStatus), err
}

// Wait for Cluster Storage Operator become healthy
func waitCSOhealthy(oc *exutil.CLI) {
	pollErr := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
		healthyBool, err := checkCSOhealthy(oc)
		if err != nil {
			e2e.Logf("Get CSO status failed of: \"%v\"", err)
			return false, err
		}
		if healthyBool {
			e2e.Logf("CSO status become healthy")
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(pollErr, "Waiting for CSO become healthy timeout")
}

// Check CSI driver successfully installed or no
func checkCSIDriverInstalled(oc *exutil.CLI, supportProvisioners []string) bool {
	var provisioner string
	for _, provisioner = range supportProvisioners {
		csiDriver, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clustercsidrivers", provisioner).Output()
		if err != nil || strings.Contains(csiDriver, "not found") {
			e2e.Logf("Error to get CSI driver:%v", err)
			return false
		}
	}
	e2e.Logf("CSI driver got successfully installed for provisioner '%s'", provisioner)
	return true
}

//Get the Resource Group id value
func getResourceGroupID(oc *exutil.CLI) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", "cluster-config-v1", "-n", "kube-system", "-o=jsonpath={.data.install-config}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	jsonOutput, err := yaml.YAMLToJSON([]byte(output))
	o.Expect(err).NotTo(o.HaveOccurred())
	rgid := gjson.Get(string(jsonOutput), `platform.`+cloudProvider+`.resourceGroupID`).String()
	o.Expect(rgid).NotTo(o.BeEmpty())
	return rgid
}

// Check if FIPS is enabled
// Azure-file doesn't work on FIPS enabled cluster
func checkFips(oc *exutil.CLI) bool {
	masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	fipsInfo, err := execCommandInSpecificNode(oc, masterNode, "fips-mode-setup --check")
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(fipsInfo, "FIPS mode is disabled.") {
		e2e.Logf("FIPS is not enabled.")
		return false
	}
	e2e.Logf("FIPS is enabled.")
	return true
}

// Convert strings slice to integer slice
func stringSliceToIntSlice(strSlice []string) ([]int, []error) {
	var (
		intSlice = make([]int, 0, len(strSlice))
		errSlice = make([]error, 0, len(strSlice))
	)
	for _, strElement := range strSlice {
		intElement, err := strconv.Atoi(strElement)
		if err != nil {
			errSlice = append(errSlice, err)
		}
		intSlice = append(intSlice, intElement)
	}
	return intSlice, errSlice
}

// Compare cluster versions
// versionA, versionB should be the same length
// E.g. [{versionA: "4.10.1", versionB: "4.10.12"}, {versionA: "4.10", versionB: "4.11}]
// IF versionA above versionB return "bool:true"
// ELSE return "bool:false" (Contains versionA = versionB)
func versionIsAbove(versionA, versionB string) bool {
	var (
		subVersionStringA, subVersionStringB = make([]string, 0, 5), make([]string, 0, 5)
		subVersionIntA, subVersionIntB       = make([]int, 0, 5), make([]int, 0, 5)
		errList                              = make([]error, 0, 5)
	)
	subVersionStringA = strings.Split(versionA, ".")
	subVersionIntA, errList = stringSliceToIntSlice(subVersionStringA)
	o.Expect(errList).Should(o.HaveLen(0))
	subVersionStringB = strings.Split(versionB, ".")
	subVersionIntB, errList = stringSliceToIntSlice(subVersionStringB)
	o.Expect(errList).Should(o.HaveLen(0))
	o.Expect(len(subVersionIntA)).Should(o.Equal(len(subVersionIntB)))
	var minusRes int
	for i := 0; i < len(subVersionIntA); i++ {
		minusRes = subVersionIntA[i] - subVersionIntB[i]
		if minusRes > 0 {
			e2e.Logf("Version:\"%s\" is above Version:\"%s\"", versionA, versionB)
			return true
		}
		if minusRes == 0 {
			continue
		}
		e2e.Logf("Version:\"%s\" is below Version:\"%s\"", versionA, versionB)
		return false
	}
	e2e.Logf("Version:\"%s\" is the same with Version:\"%s\"", versionA, versionB)
	return false
}

// Patch a specified resource
// E.g. oc patch -n <namespace> <resourceKind> <resourceName> -p <JSONPatch> --type=<patchType>
// type parameter that you can set to one of these values:
// Parameter value	Merge type
// 1. json	JSON Patch, RFC 6902
// 2. merge	JSON Merge Patch, RFC 7386
// 3. strategic	Strategic merge patch
func patchResourceAsAdmin(oc *exutil.CLI, namespace, resourceKindAndName, JSONPatch, patchType string) {
	if namespace == "" {
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("patch").Args(resourceKindAndName, "-p", JSONPatch, "--type="+patchType).Output()).To(o.ContainSubstring("patched"))
	} else {
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("patch").Args("-n", namespace, resourceKindAndName, "-p", JSONPatch, "--type="+patchType).Output()).To(o.ContainSubstring("patched"))
	}
}

//  Get the oc client version major.minor x.x (e.g. 4.11)
func getClientVersion(oc *exutil.CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("version").Args("-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	clientVersion := gjson.Get(output, `clientVersion.gitVersion`).String()
	o.Expect(clientVersion).NotTo(o.BeEmpty())
	tempSlice := strings.Split(clientVersion, ".")
	clientVersion = tempSlice[0] + "." + tempSlice[1]
	e2e.Logf("The oc client version is : \"%s\"", clientVersion)
	return clientVersion
}
