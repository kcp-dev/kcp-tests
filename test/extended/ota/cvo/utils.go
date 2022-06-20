package cvo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type JSONp struct {
	Oper string      `json:"op"`
	Path string      `json:"path"`
	Valu interface{} `json:"value,omitempty"`
}

// GetDeploymentsYaml dumps out deployment in yaml format in specific namespace
func GetDeploymentsYaml(oc *exutil.CLI, deployment_name string, namespace string) (string, error) {
	e2e.Logf("Dumping deployments %s from namespace %s", deployment_name, namespace)
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", deployment_name, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping deployments: %v", err)
		return "", err
	}
	e2e.Logf(out)
	return out, err
}

// PodExec executes a single command or a bash script in the running pod. It returns the
// command output and error if the command finished with non-zero status code or the
// command took longer than 3 minutes to run.
func PodExec(oc *exutil.CLI, script string, namespace string, podName string) (string, error) {
	var out string
	waitErr := wait.PollImmediate(1*time.Second, 3*time.Minute, func() (bool, error) {
		var err error
		out, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podName, "--", "/bin/bash", "-c", script).Output()
		return true, err
	})
	return out, waitErr
}

// GetSAToken get a token assigned to prometheus-k8s from openshift-monitoring namespace
func getSAToken(oc *exutil.CLI) (string, error) {
	e2e.Logf("Getting a token assgined to prometheus-k8s from openshift-monitoring namespace...")
	token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
	return token, err
}

// WaitForAlert check if an alert appears
// Return value: bool: indicate if the alert is found
// Return value: map: annotation map which contains reason and message information
// Retrun value: error: any error
func waitForAlert(oc *exutil.CLI, alertString string, interval time.Duration, timeout time.Duration, state string) (bool, map[string]string, error) {
	if len(state) > 0 {
		if state != "pending" && state != "firing" {
			return false, nil, fmt.Errorf("state %s is not supported", state)
		}
	}
	e2e.Logf("Waiting for alert %s pending or firing...", alertString)
	url, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "prometheus-k8s", "-n", "openshift-monitoring", "-o=jsonpath={.spec.host}").Output()
	if err != nil || len(url) == 0 {
		return false, nil, fmt.Errorf("error getting the hostname of route prometheus-k8s %v", err)
	}
	token, err := getSAToken(oc)
	if err != nil || len(token) == 0 {
		return false, nil, fmt.Errorf("error getting SA token %v", err)
	}

	alertCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\")'", token, url, alertString)
	alertAnnoCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").annotations'", token, url, alertString)
	alertStateCMD := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer %s\" https://%s/api/v1/alerts | jq -r '.data.alerts[] | select (.labels.alertname == \"%s\").state'", token, url, alertString)

	// Poll returns timed out waiting for the condition when timeout is reached
	count := 0
	if pollErr := wait.Poll(interval*time.Second, timeout*time.Second, func() (bool, error) {
		count += 1
		metrics, err := exec.Command("bash", "-c", alertCMD).Output()
		if err != nil {
			e2e.Logf("Error retrieving prometheus alert metrics: %v, retry %d...", err, count)
			return false, nil
		}
		if len(string(metrics)) == 0 {
			e2e.Logf("Prometheus alert metrics nil, retry %d...", count)
			return false, nil
		}

		if len(state) > 0 {
			alertState, err := exec.Command("bash", "-c", alertStateCMD).Output()
			if err != nil {
				return false, fmt.Errorf("error getting alert state")
			}
			if state == "pending" && string(alertState) != "pending" {
				return false, fmt.Errorf("alert state is not expected, expected pending but actual is %s", string(alertState))
			}
			if state == "firing" {
				if int(interval)*count < int(timeout) {
					if string(alertState) == "pending" {
						e2e.Logf("Prometheus alert state is pending, waiting for firing, retry %d...", count)
						return false, nil
					} else {
						return false, fmt.Errorf("alert state is not expected, expected pending in the waiting time window but actual is %s", string(alertState))
					}
				} else if string(alertState) == "firing" {
					return true, nil
				} else {
					return false, fmt.Errorf("alert state is not expected, expected firing when the waiting time is reached but actual is %s", string(alertState))
				}
			}
			return true, nil
		}
		return true, nil
	}); pollErr != nil {
		return false, nil, pollErr
	}
	e2e.Logf("Alert %s found", alertString)
	annotation, err := exec.Command("bash", "-c", alertAnnoCMD).Output()
	if err != nil || len(string(annotation)) == 0 {
		return true, nil, fmt.Errorf("error getting annotation for alert %s", alertString)
	}
	var annoMap map[string]string
	if err := json.Unmarshal(annotation, &annoMap); err != nil {
		return true, nil, fmt.Errorf("error converting annotation to map for alert %s", alertString)
	}

	return true, annoMap, nil
}

//Check if operator's condition is expected until timeout or return ture or an error happened.
func waitForCondition(interval time.Duration, timeout time.Duration, expectedCondition string, parameters string) error {
	err := wait.Poll(interval*time.Second, timeout*time.Second, func() (bool, error) {
		output, err := exec.Command("bash", "-c", parameters).Output()
		if err != nil {
			e2e.Logf("Checking condition error:%v", err)
			return false, err
		}
		condition := strings.Replace(string(output), "\n", "", -1)
		if strings.Compare(condition, expectedCondition) != 0 {
			e2e.Logf("Current condition is: %v.Waiting for condition to be enabled...", condition)
			return false, nil
		}
		e2e.Logf("Current condition is: %v", condition)
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

//Get detail alert info by selector
func getAlert(alertSelector string) map[string]interface{} {
	var alertInfo map[string]interface{}
	command := fmt.Sprintf("curl -s -k -H \"Authorization: Bearer $(oc -n openshift-monitoring sa get-token prometheus-k8s)\"  https://$(oc get route prometheus-k8s -n openshift-monitoring --no-headers|awk '{print $2}')/api/v1/alerts | jq -r '.data.alerts[]|select(%s)'", alertSelector)
	output, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		e2e.Logf("Getting alert error:%v", err)
		return nil
	}
	if len(output) == 0 {
		e2e.Logf("No alert found for %v", alertSelector)
		return nil
	}
	err = json.Unmarshal(output, &alertInfo)
	if err != nil {
		e2e.Logf("Unmarshal alert error:%v", err)
		return nil
	}
	e2e.Logf("Alert found: %v", alertInfo)
	return alertInfo
}

//Get detail alert info by alertname
func getAlertByName(alertName string) map[string]interface{} {
	selector := fmt.Sprintf(".labels.alertname == \"%s\"", alertName)
	return getAlert(selector)
}

// createBucket creates a new bucket in the gcs
// projectID := "my-project-id"
// bucketName := "bucket-name"
// return value: error: any error
func CreateBucket(client *storage.Client, projectID, bucketName string) error {
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := client.Bucket(bucketName).Create(ctx, projectID, nil); err != nil {
		return err
	}
	return nil
}

// uploadFile uploads a gcs object
// bucket := "bucket-name"
// object := "object-name"
// return value: error: any error
func UploadFile(client *storage.Client, bucket, object, file string) error {
	// Open local file
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("os.Open: %v", err)
	}
	defer f.Close()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	// Upload an object with storage.Writer.
	wc := client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}
	return nil
}

// makePublic makes a gcs object public
// bucket := "bucket-name"
// object := "object-name"
// return value: error: any error
func MakePublic(client *storage.Client, bucket, object string) error {
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	acl := client.Bucket(bucket).Object(object).ACL()
	if err := acl.Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return err
	}
	return nil
}

// Delete deletes the gcs object
// return value: error: any error
func DeleteObject(client *storage.Client, bucket, object string) error {
	if object == "" {
		return nil
	}

	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	o := client.Bucket(bucket).Object(object)
	if err := o.Delete(ctx); err != nil {
		return err
	}
	e2e.Logf("Object: %v deleted", object)
	return nil
}

// DeleteBucket deletes gcs bucket
// return value: error: any error
func DeleteBucket(client *storage.Client, bucketName string) error {
	if bucketName == "" {
		return nil
	}

	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := client.Bucket(bucketName).Delete(ctx); err != nil {
		return err
	}
	e2e.Logf("Bucket: %v deleted", bucketName)
	return nil
}

// GenerateReleaseVersion generates a fake release version based on source release version
func GenerateReleaseVersion(oc *exutil.CLI) string {
	sourceVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion/version", "-o=jsonpath={.status.desired.version}").Output()
	if err != nil {
		return ""
	}
	splits := strings.Split(sourceVersion, ".")
	if len(splits) > 1 {
		if sourceMinorNum, err := strconv.Atoi(splits[1]); err == nil {
			targeMinorNum := sourceMinorNum + 1
			splits[1] = strconv.Itoa(targeMinorNum)
			return strings.Join(splits, ".")
		}
	}
	return ""
}

// GenerateReleasePayload generates a fake release payload based on source release payload by default
func GenerateReleasePayload(oc *exutil.CLI) string {
	var targetDigest string
	sourcePayload, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion/version", "-o=jsonpath={.status.desired.image}").Output()
	if err != nil {
		return ""
	}
	data := make([]byte, 10)
	if _, err := rand.Read(data); err == nil {
		sh256Bytes := sha256.Sum256(data)
		targetDigest = hex.EncodeToString(sh256Bytes[:])
	} else {
		return ""
	}

	splits := strings.Split(sourcePayload, ":")
	if len(splits) > 1 {
		splits[1] = targetDigest
		return strings.Join(splits, ":")
	}
	return ""
}

// updateGraph updates the cincy.json
// return value: string: graph json filename
// return value: string: target version
// return value: string: target payload
// return value: error: any error
func updateGraph(oc *exutil.CLI, graphName string) (string, string, string, error) {
	graphDataDir := exutil.FixturePath("testdata", "ota/cvo")
	graphTemplate := filepath.Join(graphDataDir, graphName)

	e2e.Logf("Graph Template: %v", graphTemplate)

	// Assume the cluster is not being upgraded, then desired version will be the current version
	sourceVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion/version", "-o=jsonpath={.status.desired.version}").Output()
	if err != nil {
		return "", "", "", err
	}
	sourcePayload, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion/version", "-o=jsonpath={.status.desired.image}").Output()
	if err != nil {
		return "", "", "", err
	}

	targetVersion := GenerateReleaseVersion(oc)
	if targetVersion == "" {
		return "", "", "", fmt.Errorf("error get target version")
	}
	targetPayload := GenerateReleasePayload(oc)
	if targetPayload == "" {
		return "", "", "", fmt.Errorf("error get target payload")
	}

	// Give the new graph a unique name
	// graphFile := fmt.Sprintf("cincy-%d", time.Now().Unix())

	sedCmd := fmt.Sprintf("sed -i -e 's|sourceversion|%s|g; s|sourcepayload|%s|g; s|targetversion|%s|g; s|targetpayload|%s|g' %s", sourceVersion, sourcePayload, targetVersion, targetPayload, graphTemplate)
	//fmt.Println(sedCmd)
	if err := exec.Command("bash", "-c", sedCmd).Run(); err == nil {
		return graphTemplate, targetVersion, targetPayload, nil
	} else {
		e2e.Logf("Error on sed command: %v", err.Error())
		return "", "", "", err
	}
}

// buildGraph creates a gcs bucket, upload the graph file and make it public for CVO to use
// projectID := "projectID"
// return value: string: the public url of the object
// return value: string: the bucket name
// return value: string: the object name
// return value: string: the target version
// return value: string: the target payload
// return value: error: any error
func buildGraph(client *storage.Client, oc *exutil.CLI, projectID string, graphName string) (string, string, string, string, string, error) {
	graphFile, targetVersion, targetPayload, err := updateGraph(oc, graphName)
	if err != nil {
		return "", "", "", "", "", err
	}
	e2e.Logf("Graph file: %v updated", graphFile)

	// Give the bucket a unique name
	bucket := fmt.Sprintf("ocp-ota-%d", time.Now().Unix())
	if err := CreateBucket(client, projectID, bucket); err != nil {
		return "", "", "", "", "", err
	}
	e2e.Logf("Bucket: %v created", bucket)

	// Give the object a unique name
	object := fmt.Sprintf("graph-%d", time.Now().Unix())
	if err := UploadFile(client, bucket, object, graphFile); err != nil {
		return "", bucket, "", "", "", err
	}
	e2e.Logf("Object: %v uploaded", object)

	// Make the object public
	if err := MakePublic(client, bucket, object); err != nil {
		return "", bucket, object, "", "", err
	}
	e2e.Logf("Object: %v public", object)

	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, object), bucket, object, targetVersion, targetPayload, nil
}

//restoreCVSpec restores upstream and channel of clusterversion
//if no need to restore, pass "nochange" to the argument(s)
func restoreCVSpec(upstream string, channel string, oc *exutil.CLI) {
	if channel != "nochange" {
		oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "channel", "--allow-explicit-channel", channel).Execute()
		exec.Command("bash", "-c", "sleep 5").Output()
		currChannel, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.channel}").Output()
		if currChannel != channel {
			e2e.Logf("Error on channel recovery, expected %s, but got %s", channel, currChannel)
		}
	}

	if upstream != "nochange" {
		if upstream == "" {
			oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=json", "-p", "[{\"op\":\"remove\", \"path\":\"/spec/upstream\"}]").Execute()
		} else {
			oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterversion/version", "--type=merge", "--patch", fmt.Sprintf("{\"spec\":{\"upstream\":\"%s\"}}", upstream)).Execute()
		}
		currUpstream, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[].spec.upstream}").Output()
		if currUpstream != upstream {
			e2e.Logf("Error on upstream recovery, expected %s, but got %s", upstream, currUpstream)
		}
	}
}

// Run "oc adm release extract" cmd to extract manifests from current live cluster
func extractManifest(oc *exutil.CLI) (string, error) {
	tempDataDir := filepath.Join("/tmp/", fmt.Sprintf("ota-%s", getRandomString()))
	err := os.Mkdir(tempDataDir, 0755)
	if err != nil {
		e2e.Logf("Fail to create directory: %v", err)
		return tempDataDir, err
	}
	manifestDir := filepath.Join(tempDataDir, "manifest")
	err = oc.AsAdmin().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--confirm", "--to="+tempDataDir).Execute()
	if err != nil {
		e2e.Logf("Fail to extract dockerconfig: %v", err)
		return tempDataDir, err
	}
	err = oc.AsAdmin().Run("adm").Args("release", "extract", "--to", manifestDir, "-a", tempDataDir+"/.dockerconfigjson").Execute()
	if err != nil {
		e2e.Logf("Fail to extract manifests: %v", err)
		return tempDataDir, err
	}
	return tempDataDir, nil
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

// get clusterversion version object values by jsonpath.
// Returns: object_value(string), error
func getCVObyJP(oc *exutil.CLI, jsonpath string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").
		Args("clusterversion", "version",
			"-o", fmt.Sprintf("jsonpath={%s}", jsonpath)).Output()
}

// find argument index in CVO container args in deployment (by arg name).
// Returns: arg_value(string), arg_index(int), error
func getCVOcontArg(oc *exutil.CLI, argQuery string) (string, int, error) {
	depArgs, err := oc.AsAdmin().WithoutNamespace().Run("get").
		Args("-n", "openshift-cluster-version",
			"deployment", "cluster-version-operator",
			"-o", "jsonpath={.spec.template.spec.containers[0].args}").Output()
	if err != nil {
		e2e.Logf("Error getting cvo deployment args: %v", err)
		return "", -1, err
	}

	var result []string
	err = json.Unmarshal([]byte(depArgs), &result)
	if err != nil {
		e2e.Logf("Error Unmarshal cvo deployment args: %v", err)
		return "", -1, err
	}

	for index, arg := range result {
		if strings.Contains(arg, argQuery) {
			e2e.Logf("query '%s' found '%s' at Index: %d", argQuery, arg, index)
			val := strings.Split(arg, "=")
			if len(val) > 1 {
				return val[1], index, nil
			}
			return val[0], index, nil
		}
	}
	return "", -1, fmt.Errorf("error: cvo deployment arg %s not found", argQuery)
}

// patch resource (namespace - use "" if none, resource_name, patch).
// Returns: result(string), error
func ocJsonPatch(oc *exutil.CLI, namespace string, resource string, patch []JSONp) (patchOutput string, err error) {
	p, err := json.Marshal(patch)
	if err != nil {
		e2e.Logf("ocJsonPatch Error - json.Marshal: '%v'", err)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	if namespace != "" {
		patchOutput, err = oc.AsAdmin().WithoutNamespace().Run("patch").
			Args("-n", namespace, resource, "--type=json", "--patch", string(p)).Output()
	} else {
		patchOutput, err = oc.AsAdmin().WithoutNamespace().Run("patch").
			Args(resource, "--type=json", "--patch", string(p)).Output()
	}
	e2e.Logf("patching '%s'\nwith '%s'\nresult '%s'", resource, string(p), patchOutput)
	return
}

// patch CVO container argument (arg_index, arg_value)
// Returns: result(string), error
func patchCVOcontArg(oc *exutil.CLI, index int, value string) (string, error) {
	patch := []JSONp{
		{"replace",
			fmt.Sprintf("/spec/template/spec/containers/0/args/%d", index),
			value},
	}
	return ocJsonPatch(oc,
		"openshift-cluster-version",
		"deployment/cluster-version-operator",
		patch)
}

// Get updates by using "oc adm upgrade ..." command in the given timeout
// Check expStrings in the result of the updates
// Returns: true - found, false - not found
func checkUpdates(oc *exutil.CLI, conditional bool, interval time.Duration, timeout time.Duration, expStrings ...string) bool {
	var (
		cmdOut string
		err    error
	)
	if pollErr := wait.Poll(interval*time.Second, timeout*time.Second, func() (bool, error) {
		if conditional {
			cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade", "--include-not-recommended").Output()
		} else {
			cmdOut, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("upgrade").Output()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, str := range expStrings {
			if !strings.Contains(cmdOut, str) {
				return false, nil
			}
		}
		return true, nil
	}); pollErr != nil {
		e2e.Logf(cmdOut)
		return false
	}
	return true
}

// change the spec.capabilities
// if base==true, change the baselineCapabilitySet, otherwise, change the additionalEnabledCapabilities
func changeCap(oc *exutil.CLI, base bool, cap interface{}) (string, error) {
	var spec string
	if base {
		spec = "/spec/capabilities/baselineCapabilitySet"
	} else {
		spec = "/spec/capabilities/additionalEnabledCapabilities"
	}
	if cap == nil {
		return ocJsonPatch(oc, "", "clusterversion/version", []JSONp{{"remove", spec, nil}})
	} else {
		return ocJsonPatch(oc, "", "clusterversion/version", []JSONp{{"add", spec, cap}})
	}
}
