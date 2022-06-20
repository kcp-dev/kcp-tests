package mco

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	b64 "encoding/base64"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// MachineConfig struct is used to handle MachineConfig resources in OCP
type MachineConfig struct {
	name           string
	template       string
	pool           string
	parameters     []string
	skipWaitForMcp bool
}

// MachineConfigPool struct is used to handle MachineConfigPool resources in OCP
type MachineConfigPool struct {
	template string
	*Resource
}

// PodDisruptionBudget struct is used to handle PodDisruptionBudget resources in OCP
type PodDisruptionBudget struct {
	name      string
	namespace string
	template  string
}

// KubeletConfig struct is used to handle KubeletConfig resources in OCP
type KubeletConfig struct {
	*Resource
	template string
}

// ContainerRuntimeConfig struct is used to handle ContainerRuntimeConfig resources in OCP
type ContainerRuntimeConfig struct {
	*Resource
	template string
}

// ImageContentSourcePolicy struct is used to handle ImageContentSourcePolicy resources in OCP
type ImageContentSourcePolicy struct {
	name     string
	template string
}

// TextToVerify is a helper struct to verify configurations using the `createMcAndVerifyMCValue` function
type TextToVerify struct {
	textToVerifyForMC   string
	textToVerifyForNode string
	needBash            bool
	needChroot          bool
}

// NewMachineConfigPool create a NewMachineConfigPool struct
func NewMachineConfigPool(oc *exutil.CLI, name string) *MachineConfigPool {
	return &MachineConfigPool{Resource: NewResource(oc, "mcp", name)}
}

func (mc *MachineConfig) create(oc *exutil.CLI) {
	mc.name = mc.name + "-" + exutil.GetRandomString()
	params := []string{"--ignore-unknown-parameters=true", "-f", mc.template, "-p", "NAME=" + mc.name, "POOL=" + mc.pool}
	params = append(params, mc.parameters...)
	exutil.CreateClusterResourceFromTemplate(oc, params...)

	pollerr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc/"+mc.name, "-o", "jsonpath='{.metadata.name}'").Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, mc.name) {
			e2e.Logf("mc %s is created successfully", mc.name)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(pollerr, fmt.Sprintf("create machine config %v failed", mc.name))

	if !mc.skipWaitForMcp {
		mcp := NewMachineConfigPool(oc.AsAdmin(), mc.pool)
		mcp.waitForComplete()
	}

}

func (mc *MachineConfig) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := NewMachineConfigPool(oc.AsAdmin(), mc.pool)
	mcp.waitForComplete()
}

// NewKubeletConfig create a NewKubeletConfig struct
func NewKubeletConfig(oc *exutil.CLI, name string, template string) *KubeletConfig {
	return &KubeletConfig{Resource: NewResource(oc, "KubeletConfig", name), template: template}
}

func (kc *KubeletConfig) create() {
	exutil.CreateClusterResourceFromTemplate(kc.oc, "--ignore-unknown-parameters=true", "-f", kc.template, "-p", "NAME="+kc.name)
}

func (kc KubeletConfig) waitUntilSuccess(timeout string) {
	e2e.Logf("wait for %s to report success", kc.name)
	o.Eventually(func() map[string]interface{} {
		successCond := JSON(kc.GetConditionByType("Success"))
		if successCond.Exists() {
			return successCond.ToMap()
		}
		return nil
	},
		timeout).Should(o.SatisfyAll(o.HaveKeyWithValue("status", "True"),
		o.HaveKeyWithValue("message", "Success")))
}

func (kc KubeletConfig) waitUntilFailure(expectedMsg, timeout string) {

	e2e.Logf("wait for %s to report failure", kc.name)
	o.Eventually(func() map[string]interface{} {
		failureCond := JSON(kc.GetConditionByType("Failure"))
		if failureCond.Exists() {
			return failureCond.ToMap()
		}
		return nil
	},
		timeout).Should(o.SatisfyAll(o.HaveKeyWithValue("status", "False"), o.HaveKeyWithValue("message", o.ContainSubstring(expectedMsg))))
}

func (pdb *PodDisruptionBudget) create(oc *exutil.CLI) {
	e2e.Logf("Creating pod disruption budget: %s", pdb.name)
	exutil.CreateNsResourceFromTemplate(oc, pdb.namespace, "--ignore-unknown-parameters=true", "-f", pdb.template, "-p", "NAME="+pdb.name)
}

func (pdb *PodDisruptionBudget) delete(oc *exutil.CLI) {
	e2e.Logf("Deleting pod disruption budget: %s", pdb.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("pdb", pdb.name, "-n", pdb.namespace, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (icsp *ImageContentSourcePolicy) create(oc *exutil.CLI) {
	exutil.CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", icsp.template, "-p", "NAME="+icsp.name)
	mcp := NewMachineConfigPool(oc.AsAdmin(), "worker")
	mcp.waitForComplete()
	mcp.name = "master"
	mcp.waitForComplete()
}

func (icsp *ImageContentSourcePolicy) delete(oc *exutil.CLI) {
	e2e.Logf("deleting icsp config: %s", icsp.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagecontentsourcepolicy", icsp.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp := NewMachineConfigPool(oc.AsAdmin(), "worker")
	mcp.waitForComplete()
	mcp.name = "master"
	mcp.waitForComplete()
}

// NewContainerRuntimeConfig creates a ContainerRuntimeConfig struct
func NewContainerRuntimeConfig(oc *exutil.CLI, name string, template string) *ContainerRuntimeConfig {
	return &ContainerRuntimeConfig{Resource: NewResource(oc, "ContainerRuntimeConfig", name), template: template}
}

func (cr *ContainerRuntimeConfig) create() {
	exutil.CreateClusterResourceFromTemplate(cr.oc, "--ignore-unknown-parameters=true", "-f", cr.template, "-p", "NAME="+cr.name)
}

func (cr ContainerRuntimeConfig) waitUntilSuccess(timeout string) {
	e2e.Logf("wait for %s to report success", cr.name)
	o.Eventually(func() map[string]interface{} {
		successCond := JSON(cr.GetConditionByType("Success"))
		if successCond.Exists() {
			return successCond.ToMap()
		}
		return nil
	},
		timeout).Should(o.SatisfyAll(o.HaveKeyWithValue("status", "True"),
		o.HaveKeyWithValue("message", "Success")))
}

func (cr ContainerRuntimeConfig) waitUntilFailure(expectedMsg string, timeout string) {
	e2e.Logf("wait for %s to report failure", cr.name)
	o.Eventually(func() map[string]interface{} {
		failureCond := JSON(cr.GetConditionByType("Failure"))
		if failureCond.Exists() {
			return failureCond.ToMap()
		}
		return nil
	},
		timeout).Should(o.SatisfyAll(o.HaveKeyWithValue("status", "False"), o.HaveKeyWithValue("message", o.ContainSubstring(expectedMsg))))
}

func (mcp *MachineConfigPool) create() {
	exutil.CreateClusterResourceFromTemplate(mcp.oc, "--ignore-unknown-parameters=true", "-f", mcp.template, "-p", "NAME="+mcp.name)
	mcp.waitForComplete()
}

func (mcp *MachineConfigPool) delete() {
	e2e.Logf("deleting custom mcp: %s", mcp.name)
	err := mcp.oc.AsAdmin().WithoutNamespace().Run("delete").Args("mcp", mcp.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (mcp *MachineConfigPool) pause(enable bool) {
	e2e.Logf("patch mcp %v, change spec.paused to %v", mcp.name, enable)
	err := mcp.Patch("merge", `{"spec":{"paused": `+strconv.FormatBool(enable)+`}}`)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// SetMaxUnavailable sets the value for maxUnavailable
func (mcp *MachineConfigPool) SetMaxUnavailable(maxUnavailable int) {
	e2e.Logf("patch mcp %v, change spec.maxUnavailable to %d", mcp.name, maxUnavailable)
	err := mcp.Patch("merge", fmt.Sprintf(`{"spec":{"maxUnavailable": %d}}`, maxUnavailable))
	o.Expect(err).NotTo(o.HaveOccurred())
}

// RemoveMaxUnavailable removes spec.maxUnavailable attribute from the pool config
func (mcp *MachineConfigPool) RemoveMaxUnavailable() {
	e2e.Logf("patch mcp %v, removing spec.maxUnavailable")
	err := mcp.Patch("json", `[{ "op": "remove", "path": "/spec/maxUnavailable" }]`)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (mcp *MachineConfigPool) getConfigNameOfSpec() (string, error) {
	output, err := mcp.Get(`{.spec.configuration.name}`)
	e2e.Logf("spec.configuration.name of mcp/%v is %v", mcp.name, output)
	return output, err
}

func (mcp *MachineConfigPool) getConfigNameOfStatus() (string, error) {
	output, err := mcp.Get(`{.status.configuration.name}`)
	e2e.Logf("status.configuration.name of mcp/%v is %v", mcp.name, output)
	return output, err
}

func (mcp *MachineConfigPool) getMachineCount() (int, error) {
	machineCountStr, ocErr := mcp.Get(`{.status.machineCount}`)
	if ocErr != nil {
		e2e.Logf("Error getting machineCount: %s", ocErr)
		return -1, ocErr
	}
	machineCount, convErr := strconv.Atoi(machineCountStr)

	if convErr != nil {
		e2e.Logf("Error converting machineCount to integer: %s", ocErr)
		return -1, convErr
	}

	return machineCount, nil
}

func (mcp *MachineConfigPool) getDegradedMachineCount() (int, error) {
	dmachineCountStr, ocErr := mcp.Get(`{.status.degradedMachineCount}`)
	if ocErr != nil {
		e2e.Logf("Error getting degradedmachineCount: %s", ocErr)
		return -1, ocErr
	}
	dmachineCount, convErr := strconv.Atoi(dmachineCountStr)

	if convErr != nil {
		e2e.Logf("Error converting degradedmachineCount to integer: %s", ocErr)
		return -1, convErr
	}

	return dmachineCount, nil
}

func (mcp *MachineConfigPool) pollMachineCount() func() string {
	return mcp.Poll(`{.status.machineCount}`)
}

func (mcp *MachineConfigPool) pollReadyMachineCount() func() string {
	return mcp.Poll(`{.status.readyMachineCount}`)
}

func (mcp *MachineConfigPool) pollDegradedMachineCount() func() string {
	return mcp.Poll(`{.status.degradedMachineCount}`)
}

// GetDegradedStatus returns the value of the 'Degraded' condition in the MCP
func (mcp *MachineConfigPool) GetDegradedStatus() (string, error) {
	return mcp.Get(`{.status.conditions[?(@.type=="Degraded")].status}`)
}

func (mcp *MachineConfigPool) pollDegradedStatus() func() string {
	return mcp.Poll(`{.status.conditions[?(@.type=="Degraded")].status}`)
}

func (mcp *MachineConfigPool) pollUpdatedStatus() func() string {
	return mcp.Poll(`{.status.conditions[?(@.type=="Updated")].status}`)
}

func (mcp *MachineConfigPool) estimateWaitTimeInMinutes() int {
	var totalNodes int

	o.Eventually(func() int {
		var err error
		totalNodes, err = mcp.getMachineCount()
		if err != nil {
			return -1
		}
		return totalNodes
	},
		"5m").Should(o.BeNumerically(">=", 0), fmt.Sprintf("machineCount field has no value in MCP %s", mcp.name))

	return totalNodes * 10

}

// GetNodes returns a list with the nodes that belong to the machine config pool
func (mcp *MachineConfigPool) GetNodes() ([]Node, error) {
	labels := JSON(mcp.GetOrFail(`{.spec.nodeSelector.matchLabels}`))
	o.Expect(labels.Exists()).Should(o.BeTrue(), fmt.Sprintf("The pool %s has no machLabels value defined", mcp.GetName()))
	nodeList := NewNodeList(mcp.oc)
	for k, v := range labels.ToMap() {
		requiredLabel := fmt.Sprintf("%s=%s", k, v.(string))
		nodeList.ByLabel(requiredLabel)
	}
	return nodeList.GetAll()
}

// GetSortedNodes returns a list with the nodes that belong to the machine config pool in the same order used to update them
//  when a configuration is applied
func (mcp *MachineConfigPool) GetSortedNodes() ([]Node, error) {

	poolNodes, err := mcp.GetNodes()
	if err != nil {
		return nil, err
	}

	return sortNodeList(poolNodes), nil

}

// GetSortedUpdatedNodes returns the list of the UpdatedNodes sorted by the time when they started to be updated.
//	If maxUnavailable>0, then the function will fail if more that maxUpdatingNodes are being updated at the same time
func (mcp *MachineConfigPool) GetSortedUpdatedNodes(maxUnavailable int) []Node {
	timeToWait := time.Duration(mcp.estimateWaitTimeInMinutes()) * time.Minute
	e2e.Logf("Waiting %s in pool %s for all nodes to start updating.", timeToWait, mcp.name)

	poolNodes, errget := mcp.GetNodes()
	o.Expect(errget).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot get nodes in pool %s", mcp.GetName()))

	pendingNodes := poolNodes
	updatedNodes := []Node{}
	err := wait.Poll(20*time.Second, timeToWait, func() (bool, error) {
		// If there are degraded machines, stop polling, directly fail
		degradedstdout, degradederr := mcp.getDegradedMachineCount()
		if degradederr != nil {
			e2e.Logf("the err:%v, and try next round", degradederr)
			return false, nil
		}

		if degradedstdout != 0 {
			exutil.AssertWaitPollNoErr(fmt.Errorf("Degraded machines"), fmt.Sprintf("mcp %s has degraded %d machines", mcp.name, degradedstdout))
		}

		// Check that there aren't more thatn maxUpdatingNodes updating at the same time
		if maxUnavailable > 0 {
			totalUpdating := 0
			for _, node := range poolNodes {
				if node.IsUpdating() {
					totalUpdating++
				}
			}
			if totalUpdating > maxUnavailable {
				exutil.AssertWaitPollNoErr(fmt.Errorf("maxUnavailable Not Honored"), fmt.Sprintf("Pool %s, error: %d nodes were updating at the same time. Only %d nodes should be updating at the same time.", mcp.GetName(), totalUpdating, maxUnavailable))
			}
		}

		remainingNodes := []Node{}
		for _, node := range pendingNodes {
			if node.IsUpdating() {
				e2e.Logf("Node %s is UPDATING", node.GetName())
				updatedNodes = append(updatedNodes, node)
			} else {
				remainingNodes = append(remainingNodes, node)
			}
		}

		if len(remainingNodes) == 0 {
			e2e.Logf("All nodes have started to be updated on mcp %s", mcp.name)
			return true, nil

		}
		e2e.Logf(" %d remaining nodes", len(remainingNodes))
		pendingNodes = remainingNodes
		return false, nil
	})

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Could not get the list of updated nodes on mcp %s", mcp.name))
	return updatedNodes
}

// WaitForNotDegradedStatus waits until MCP is not degraded, if the condition times out the returned error is != nil
func (mcp MachineConfigPool) WaitForNotDegradedStatus() error {
	timeToWait := time.Duration(mcp.estimateWaitTimeInMinutes()) * time.Minute
	e2e.Logf("Waiting %s for MCP %s status to be not degraded.", timeToWait, mcp.name)

	err := wait.Poll(1*time.Minute, timeToWait, func() (bool, error) {
		stdout, err := mcp.GetDegradedStatus()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, "False") {
			e2e.Logf("MCP Updated status is True %s", mcp.name)
			return true, nil
		}
		return false, nil
	})

	return err
}

func (mcp *MachineConfigPool) waitForComplete() {
	timeToWait := time.Duration(mcp.estimateWaitTimeInMinutes()) * time.Minute
	e2e.Logf("Waiting %s for MCP %s to be completed.", timeToWait, mcp.name)

	err := wait.Poll(1*time.Minute, timeToWait, func() (bool, error) {
		// If there are degraded machines, stop polling, directly fail
		degradedstdout, degradederr := mcp.getDegradedMachineCount()
		if degradederr != nil {
			e2e.Logf("the err:%v, and try next round", degradederr)
			return false, nil
		}

		if degradedstdout != 0 {
			exutil.AssertWaitPollNoErr(fmt.Errorf("Degraded machines"), fmt.Sprintf("mcp %s has degraded %d machines", mcp.name, degradedstdout))
		}

		stdout, err := mcp.Get(`{.status.conditions[?(@.type=="Updated")].status}`)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, "True") {
			// i.e. mcp updated=true, mc is applied successfully
			e2e.Logf("mc operation is completed on mcp %s", mcp.name)
			return true, nil
		}
		return false, nil
	})

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("mc operation is not completed on mcp %s", mcp.name))
}

func getTimeDifferenceInMinute(oldTimestamp string, newTimestamp string) float64 {
	oldTimeValues := strings.Split(oldTimestamp, ":")
	oldTimeHour, _ := strconv.Atoi(oldTimeValues[0])
	oldTimeMinute, _ := strconv.Atoi(oldTimeValues[1])
	oldTimeSecond, _ := strconv.Atoi(strings.Split(oldTimeValues[2], ".")[0])
	oldTimeNanoSecond, _ := strconv.Atoi(strings.Split(oldTimeValues[2], ".")[1])
	newTimeValues := strings.Split(newTimestamp, ":")
	newTimeHour, _ := strconv.Atoi(newTimeValues[0])
	newTimeMinute, _ := strconv.Atoi(newTimeValues[1])
	newTimeSecond, _ := strconv.Atoi(strings.Split(newTimeValues[2], ".")[0])
	newTimeNanoSecond, _ := strconv.Atoi(strings.Split(newTimeValues[2], ".")[1])
	y, m, d := time.Now().Date()
	oldTime := time.Date(y, m, d, oldTimeHour, oldTimeMinute, oldTimeSecond, oldTimeNanoSecond, time.UTC)
	newTime := time.Date(y, m, d, newTimeHour, newTimeMinute, newTimeSecond, newTimeNanoSecond, time.UTC)
	return newTime.Sub(oldTime).Minutes()
}

func filterTimestampFromLogs(logs string, numberOfTimestamp int) []string {
	return regexp.MustCompile("(?m)[0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}.[0-9]{1,6}").FindAllString(logs, numberOfTimestamp)
}

func getMachineConfigDetails(oc *exutil.CLI, mcName string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", mcName, "-o", "yaml").Output()
}

// func getKubeletConfigDetails(oc *exutil.CLI, kcName string) (string, error) {
// 	return oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", kcName, "-o", "yaml").Output()
// }

func getPullSecret(oc *exutil.CLI) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/pull-secret", "-n", "openshift-config", `--template={{index .data ".dockerconfigjson" | base64decode}}`).OutputToFile("auth.dockerconfigjson")
}

func setDataForPullSecret(oc *exutil.CLI, configFile string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("set").Args("data", "secret/pull-secret", "-n", "openshift-config", "--from-file=.dockerconfigjson="+configFile).Output()
}

func getCommitID(oc *exutil.CLI, component string, clusterVersion string) (string, error) {
	secretFile, secretErr := getPullSecret(oc)
	if secretErr != nil {
		return "", secretErr
	}
	outFilePath, ocErr := oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "info", "--registry-config="+secretFile, "--commits", clusterVersion, "--insecure=true").OutputToFile("commitIdLogs.txt")
	if ocErr != nil {
		return "", ocErr
	}
	commitID, cmdErr := exec.Command("bash", "-c", "cat "+outFilePath+" | grep "+component+" | awk '{print $3}'").Output()
	return strings.TrimSuffix(string(commitID), "\n"), cmdErr
}

func getGoVersion(component string, commitID string) (float64, error) {
	curlOutput, curlErr := exec.Command("bash", "-c", "curl -Lks https://raw.githubusercontent.com/openshift/"+component+"/"+commitID+"/go.mod | egrep '^go'").Output()
	if curlErr != nil {
		return 0, curlErr
	}
	goVersion := string(curlOutput)[3:]
	return strconv.ParseFloat(strings.TrimSuffix(goVersion, "\n"), 64)
}

func getStatusCondition(oc *exutil.CLI, resource string, ctype string) (map[string]interface{}, error) {
	jsonstr, ocerr := oc.AsAdmin().WithoutNamespace().Run("get").Args(resource, "-o", "jsonpath='{.status.conditions[?(@.type==\""+ctype+"\")]}'").Output()
	if ocerr != nil {
		return nil, ocerr
	}
	e2e.Logf("condition info of %v-%v : %v", resource, ctype, jsonstr)
	jsonstr = strings.Trim(jsonstr, "'")
	jsonbytes := []byte(jsonstr)
	var datamap map[string]interface{}
	jsonerr := json.Unmarshal(jsonbytes, &datamap)
	if jsonerr != nil {
		return nil, jsonerr
	}
	e2e.Logf("umarshalled json: %v", datamap)
	return datamap, jsonerr
}

func containsMultipleStrings(sourceString string, expectedStrings []string) bool {
	o.Expect(sourceString).NotTo(o.BeEmpty())
	o.Expect(expectedStrings).NotTo(o.BeEmpty())

	var count int
	for _, element := range expectedStrings {
		if strings.Contains(sourceString, element) {
			count++
		}
	}
	return len(expectedStrings) == count
}

func generateTemplateAbsolutePath(fileName string) string {
	mcoBaseDir := exutil.FixturePath("testdata", "mco")
	return filepath.Join(mcoBaseDir, fileName)
}

func getSATokenFromContainer(oc *exutil.CLI, podName string, podNamespace string, container string) string {
	podOut, err := exutil.RemoteShContainer(oc, podNamespace, podName, container, "cat", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	o.Expect(err).NotTo(o.HaveOccurred())

	return podOut
}

func getHostFromRoute(oc *exutil.CLI, routeName string, routeNamespace string) string {
	stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", routeName, "-n", routeNamespace, "-o", "jsonpath='{.spec.host}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return stdout
}

func generateTmpFile(oc *exutil.CLI, fileName string) string {
	return filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-"+fileName)
}

func getPrometheusQueryResults(oc *exutil.CLI, query string) string {

	token := getSATokenFromContainer(oc, "prometheus-k8s-0", "openshift-monitoring", "prometheus")

	routeHost := getHostFromRoute(oc, "prometheus-k8s", "openshift-monitoring")
	url := fmt.Sprintf("https://%s/api/v1/query?query=%s", routeHost, query)
	headers := fmt.Sprintf("Authorization: Bearer %s", token)

	curlCmd := fmt.Sprintf("curl -ks -H '%s' %s", headers, url)
	e2e.Logf("curl cmd:\n %s", curlCmd)

	curlOutput, cmdErr := exec.Command("bash", "-c", curlCmd).Output()
	e2e.Logf("curl output:\n%s", curlOutput)
	o.Expect(cmdErr).NotTo(o.HaveOccurred())

	return string(curlOutput)
}

func gZipData(data []byte) (compressedData []byte, err error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	defer func() {
		_ = gz.Close()
	}()

	_, err = gz.Write(data)
	if err != nil {
		return nil, err
	}

	if err = gz.Flush(); err != nil {
		return nil, err
	}

	if err = gz.Close(); err != nil {
		return nil, err
	}

	compressedData = b.Bytes()

	return compressedData, nil
}

func jsonEncode(s string) string {
	e, err := json.Marshal(s)
	if err != nil {
		e2e.Failf("Error json encoding the string: %s", s)
	}
	return string(e)
}

func getURLEncodedFileConfig(destinationPath string, content string, mode string) string {
	encodedContent := url.PathEscape(content)

	return getFileConfig(destinationPath, "data:,"+encodedContent, mode)
}

func getBase64EncodedFileConfig(destinationPath string, content string, mode string) string {
	encodedContent := b64.StdEncoding.EncodeToString([]byte(content))

	return getFileConfig(destinationPath, "data:text/plain;charset=utf-8;base64,"+encodedContent, mode)
}

func getFileConfig(destinationPath string, source string, mode string) string {
	decimalMode := mode
	// if octal number we convert it to decimal. Json templates do not accept numbers with a leading zero (octal).
	// if we don't do this conversion the 'oc process' command will not be able to render the template because {"mode": 0666}
	//   is not a valid json. Numbers in json cannot start with a leading 0
	if mode != "" && mode[0] == '0' {
		// parse the octal string and conver to integer
		iMode, err := strconv.ParseInt(mode, 8, 64)
		// get a string with the decimal numeric representation of the mode
		decimalMode = fmt.Sprintf("%d", os.FileMode(iMode))
		if err != nil {
			e2e.Failf("Filer permissions %s cannot be converted to integer", mode)
		}
	}

	var fileConfig string
	if mode == "" {
		fileConfig = fmt.Sprintf(`{"contents": {"source": "%s"}, "path": "%s"}`, source, destinationPath)
	} else {
		fileConfig = fmt.Sprintf(`{"contents": {"source": "%s"}, "path": "%s", "mode": %s}`, source, destinationPath, decimalMode)
	}

	return fileConfig
}

func getGzipFileJSONConfig(destinationPath string, fileContent string) string {
	compressedContent, err := gZipData([]byte(fileContent))
	o.Expect(err).NotTo(o.HaveOccurred())
	encodedContent := b64.StdEncoding.EncodeToString(compressedContent)
	fileConfig := fmt.Sprintf(`{"contents": {"compression": "gzip", "source": "data:;base64,%s"}, "path": "%s"}`, encodedContent, destinationPath)
	return fileConfig
}

func getMaskServiceConfig(name string, mask bool) string {
	return fmt.Sprintf(`{"name": "%s", "mask": %t}`, name, mask)
}

func getDropinFileConfig(unitName string, enabled bool, fileName string, fileContent string) string {
	// Escape not valid characters in json from the file content
	escapedContent := jsonEncode(fileContent)
	return fmt.Sprintf(`{"name": "%s", "enabled": %t, "dropins": [{"name": "%s", "contents": %s}]}`, unitName, enabled, fileName, escapedContent)
}

func getSingleUnitConfig(unitName string, unitEnabled bool, unitContents string) string {
	// Escape not valid characters in json from the file content
	escapedContent := jsonEncode(unitContents)
	return fmt.Sprintf(`{"name": "%s", "enabled": %t, "contents": %s}`, unitName, unitEnabled, escapedContent)
}

// AddToAllMachineSets adds a delta to all MachineSets replicas and wait for the MachineSets to be ready
func AddToAllMachineSets(oc *exutil.CLI, delta int) error {
	allMs, err := NewMachineSetList(oc, "openshift-machine-api").GetAll()
	o.Expect(err).NotTo(o.HaveOccurred())

	var addErr error = nil
	modifiedMSs := []MachineSet{}
	for _, ms := range allMs {
		addErr = ms.AddToScale(delta)
		if addErr == nil {
			modifiedMSs = append(modifiedMSs, ms)
		} else {
			break
		}
	}

	if addErr != nil {
		e2e.Logf("Error reconfiguring MachineSets. Restoring original replicas.")
		for _, ms := range modifiedMSs {
			_ = ms.AddToScale(-1 * delta)
		}

		return addErr
	}

	var waitErr error = nil
	for _, ms := range allMs {
		waitErr = wait.PollImmediate(30*time.Second, 8*time.Minute, func() (bool, error) { return ms.PollIsReady()(), nil })
		if waitErr != nil {
			e2e.Logf("MachineSet %s is not ready. Restoring original replicas.", ms.GetName())
			for _, ms := range modifiedMSs {
				_ = ms.AddToScale(-1 * delta)
			}
			break
		}
	}

	return waitErr
}

func sortNodeList(nodes []Node) []Node {
	sort.Slice(nodes, func(l, r int) bool {
		lMetadata := JSON(nodes[l].GetOrFail("{.metadata}"))
		rMetadata := JSON(nodes[r].GetOrFail("{.metadata}"))

		lLabels := &JSONData{nil}
		if lMetadata.Get("labels").Exists() {
			lLabels = lMetadata.Get("labels")
		}
		rLabels := &JSONData{nil}
		if rMetadata.Get("labels").Exists() {
			rLabels = rMetadata.Get("labels")
		}

		lZone := lLabels.Get("topology.kubernetes.io/zone")
		rZone := rLabels.Get("topology.kubernetes.io/zone")
		// if both nodes have zone label, sort by zone, push nodes without label to end of list
		if lZone.Exists() && rZone.Exists() {
			if lZone.ToString() != rZone.ToString() {
				return lZone.ToString() < rZone.ToString()
			}
		} else if rZone.Exists() {
			return false
		} else if lZone.Exists() {
			return true
		}

		// if nodes are in the same zone or they have no labels sortby creationTime oldest to newest
		dateLayout := "2006-01-02T15:04:05Z"
		lDate, err := time.Parse(dateLayout, lMetadata.Get("creationTimestamp").ToString())
		if err != nil {
			e2e.Failf("Cannot parse creationTimestamp %s in node %s", lMetadata.Get("creationTimestamp").ToString(), nodes[l].GetName())

		}
		rDate, err := time.Parse(dateLayout, rMetadata.Get("creationTimestamp").ToString())
		if err != nil {
			e2e.Failf("Cannot parse creationTimestamp %s in node %s", rMetadata.Get("creationTimestamp").ToString(), nodes[r].GetName())

		}
		return lDate.Before(rDate)

	})
	return nodes
}

func getMachineConfigControllerPod(oc *exutil.CLI) (string, error) {

	pods, podsErr := exutil.GetAllPods(oc.AsAdmin(), "openshift-machine-config-operator")
	if podsErr != nil {
		return "", podsErr
	}

	var ctrlerPod string
	if len(pods) > 0 {
		for _, pod := range pods {
			if strings.HasPrefix(pod, "machine-config-controller") {
				ctrlerPod = pod
				e2e.Logf("machine config controller pod name is %s", ctrlerPod)
				break
			}
		}
	} else {
		podsErr = errors.New("mco pod list is empty") // if get pod command returns empty list w/o error
	}

	return ctrlerPod, podsErr

}

func getAlertsByName(oc *exutil.CLI, alertName string) ([]JSONData, error) {

	mon, monErr := exutil.NewPrometheusMonitor(oc.AsAdmin())
	if monErr != nil {
		return nil, monErr
	}

	allAerts, allAlertErr := mon.GetAlerts()

	if allAlertErr != nil {
		return nil, allAlertErr
	}

	e2e.Logf("get all alerts: %s\n", allAerts)

	jsonObj := JSON(allAerts)
	filteredAlerts, filteredAlertErr := jsonObj.GetJSONPath(fmt.Sprintf(`{.data.alerts[?(@.labels.alertname=="%s")]}`, alertName))

	if filteredAlertErr != nil {
		return nil, filteredAlertErr
	}

	for _, alert := range filteredAlerts {
		e2e.Logf("filtered alert %s\n", alert.String())
	}

	return filteredAlerts, nil
}

// WrapWithBracketsIfIpv6 wraps the ip with brackets if it is an IPV6 address.
// In order to use IPV6 addresses with curl commands we need to wrap them between brackets.
func WrapWithBracketsIfIpv6(ip string) (string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("The string %s is not a valid IP", ip)
	}

	// If it is an IPV6 address, wrap it
	if parsedIP.To4() == nil {
		return "[" + ip + "]", nil
	}

	return ip, nil
}
