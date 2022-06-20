package sro

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ogResource struct {
	name      string
	namespace string
	template  string
}

func (og *ogResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("og", og.name, "-n", og.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No operatorgroup in project: %s, create one: %s", og.namespace, og.name))
		applyResource(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist operatorgroup in project: %s", og.namespace))
	}
}

func (og *ogResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", og.name, "-n", og.namespace).Output()
}

type subResource struct {
	name         string
	namespace    string
	channel      string
	installedCSV string
	template     string
	source       string
}

func (sub *subResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		applyResource(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.name, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel, "SOURCE="+sub.source)
		err = wait.Poll(5*time.Second, 240*time.Second, func() (bool, error) {
			state, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", state, err)
				return false, nil
			}
			if strings.Compare(state, "AtLatestKnown") == 0 || strings.Compare(state, "UpgradeAvailable") == 0 {
				return true, nil
			}
			e2e.Logf("sub %s state is %s, not AtLatestKnown or UpgradeAvailable", sub.name, state)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown or UpgradeAvailable", sub.name))

		installedCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(installedCSV).NotTo(o.BeEmpty())
		sub.installedCSV = installedCSV
	} else {
		e2e.Logf(fmt.Sprintf("Already exist sub in project: %s", sub.namespace))
	}
}

func (sub *subResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("sub", sub.name, "-n", sub.namespace).Output()
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("csv", sub.installedCSV, "-n", sub.namespace).Output()
}

type nsResource struct {
	name     string
	template string
}

func (ns *nsResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", ns.name).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("create one: %s", ns.name))
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ns.template, "-p", "NAME="+ns.name).OutputToFile("sro-ns.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
	} else {
		e2e.Logf(fmt.Sprintf("Already exist ns: %s", ns.name))
	}
}

func (ns *nsResource) delete(oc *exutil.CLI) {
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", ns.name).Output()
}

func applyResource(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile("sro.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

type oprResource struct {
	kind      string
	name      string
	namespace string
}

//When will return false if Operator is not installed, and true otherwise
func (opr *oprResource) checkOperatorPOD(oc *exutil.CLI) bool {
	e2e.Logf("Checking if " + opr.name + " pod is succesfully running...")
	podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, "-n", opr.namespace, "--no-headers").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	var (
		activepod int
		totalpod  int
	)
	if strings.Contains(podList, "NotFound") || strings.Contains(podList, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No pod is running in project: %s", opr.namespace))
		return false
	} else {
		runningpod := strings.Count(podList, "Running")
		runningjob := strings.Count(podList, "Completed")
		activepod = runningjob + runningpod
		totalpod = strings.Count(podList, "\n") + 1
	}

	if reflect.DeepEqual(activepod, totalpod) {
		e2e.Logf(fmt.Sprintf("The active pod is :%d  The total pod is:%d", activepod, totalpod))
		e2e.Logf(opr.name + " pod is suceessfully running :(")
		return true
	} else {
		e2e.Logf(opr.name + " pod abnormal, please check!")
		return false
	}
}

func (opr *oprResource) applyResourceByYaml(oc *exutil.CLI, yamlfile string) {
	if len(opr.namespace) == 0 {
		//Create cluster-wide resource
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile).Execute()
	} else {
		//Create namespace-wide resource
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile, "-n", opr.namespace).Execute()
	}
}

func (opr *oprResource) CleanupResource(oc *exutil.CLI) {
	if len(opr.namespace) == 0 {
		//Delete cluster-wide resource
		oc.AsAdmin().WithoutNamespace().Run("delete").Args(opr.kind, opr.name).Execute()
	} else {
		//Delete namespace-wide resource
		oc.AsAdmin().WithoutNamespace().Run("delete").Args(opr.kind, opr.name, "-n", opr.namespace).Execute()
	}
}

func (opr *oprResource) waitLongDurationDaemonsetReady(oc *exutil.CLI, timeDurationSec int) {

	waitErr := wait.Poll(20*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
		var (
			kindnames  string
			err        error
			isCreated  bool
			desirednum string
			readynum   string
		)

		//Check if deployment/daemonset/statefulset is created.

		kindnames, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, "-n", opr.namespace, "-oname").Output()
		e2e.Logf("daemonset name is:" + kindnames)
		if len(kindnames) == 0 || err != nil {
			isCreated = false
		} else {
			//daemonset/statefulset has been created, but not running, need to compare .status.desiredNumberScheduled and .status.numberReady}
			//if the two value is equal, set output="has successfully progressed"
			isCreated = true
			desirednum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindnames, "-n", opr.namespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
			readynum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindnames, "-n", opr.namespace, "-o=jsonpath={.status.numberReady}").Output()
		}

		e2e.Logf("desirednum is: " + desirednum + " readynum is: " + readynum)
		//daemonset/deloyment has been created, but not running, need to compare desirednum and readynum
		//if isCreate is true and the two value is equal, the pod is ready
		if isCreated && len(kindnames) != 0 && desirednum == readynum {
			e2e.Logf("The %v is successfully progressed and running normally", kindnames)
			return true, nil
		} else {
			e2e.Logf("The %v is not ready or running normally", kindnames)
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", opr.name))
}

//trunct pods logs by filter
func (opr *oprResource) assertOprPodLogs(oc *exutil.CLI, filter string) {
	podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", opr.namespace, "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podList).To(o.ContainSubstring(opr.name))

	e2e.Logf("Got pods list as below: \n" + podList)
	//Filter pod name base on deployment name
	regexpoprname, _ := regexp.Compile(".*" + opr.name + ".*")
	podListArry := regexpoprname.FindAllString(podList, -1)

	podListSize := len(podListArry)
	for i := 0; i < podListSize; i++ {
		e2e.Logf("Verify the logs on %v", podListArry[i])

		//Check the log files until finding the keywords by filter
		waitErr := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
			//If have multiple pods of one deployment/daemonset, check each pods logs
			output, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podListArry[i], "-n", opr.namespace).Output()
			if strings.Contains(output, filter) {
				o.Expect(output).To(o.ContainSubstring(filter))
				regexpstr, _ := regexp.Compile(".*" + filter + ".*")
				loglines := regexpstr.FindAllString(output, -1)
				e2e.Logf("The result is: %v", loglines[0])
				return true, nil
			} else {
				e2e.Logf("Can not find the key words in pod logs by: %v", filter)
				return false, nil
			}
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", opr.name))
	}

}

func (opr *oprResource) checkSROControlManagerLabel(oc *exutil.CLI) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, opr.name, "-n", opr.namespace, "-o", "jsonpath='{.spec.template.metadata.labels.control-plane}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return output
}

func (opr *oprResource) createConfigmapFromFile(oc *exutil.CLI, filepath []string) {
	var (
		output string
		err    error
	)
	output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(opr.kind, opr.name, "-n", opr.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		output, err = oc.AsAdmin().WithoutNamespace().Run("create").Args(opr.kind, opr.name, "--from-file="+filepath[0], "--from-file="+filepath[1], "-n", opr.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(opr.name))
		e2e.Logf("The configmap %v created successfully in %v", opr.name, opr.namespace)
	} else {
		e2e.Logf("The configmap %v has been created in %v", opr.name, opr.namespace)
	}
}

func assertSimpleKmodeOnNode(oc *exutil.CLI) {
	nodeList, err := exutil.GetClusterNodesBy(oc, "worker")
	nodeListSize := len(nodeList)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodeListSize).NotTo(o.Equal(0))

	regexpstr, _ := regexp.Compile(`simple.*`)
	waitErr := wait.Poll(15*time.Second, time.Second*300, func() (done bool, err error) {
		e2e.Logf("Check simple-kmod moddule on first worker node")
		firstWokerNode, _ := exutil.GetFirstWorkerNode(oc)
		output, _ := exutil.DebugNodeWithChroot(oc, firstWokerNode, "lsmod")
		match, _ := regexp.MatchString(`simple.*`, output)
		if match {
			//Check all worker nodes and generated full report
			for i := 0; i < nodeListSize; i++ {
				output, err := exutil.DebugNodeWithChroot(oc, nodeList[i], "lsmod")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("simple"))
				e2e.Logf("Verify if simple kmod installed on %v", nodeList[i])
				simpleKmod := regexpstr.FindAllString(output, 2)
				e2e.Logf("The result is: %v", simpleKmod)
			}
			return true, nil
		} else {
			return false, nil
		}
	})
	exutil.AssertWaitPollNoErr(waitErr, "the simple-kmod not found")
}

//Encrypt and Decrypt Pull Secret for multi-build
func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func encryptStr(data []byte, passphrase string) []byte {
	block, _ := aes.NewCipher([]byte(createHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	o.Expect(err).NotTo(o.HaveOccurred())
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext
}

func decryptStr(data []byte, passphrase string) []byte {
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	o.Expect(err).NotTo(o.HaveOccurred())
	gcm, err := cipher.NewGCM(block)
	o.Expect(err).NotTo(o.HaveOccurred())
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	o.Expect(err).NotTo(o.HaveOccurred())
	return plaintext
}

func encryptFile(filename string, data []byte, passphrase string) {
	f, _ := os.Create(filename)
	defer f.Close()
	f.Write(encryptStr(data, passphrase))
}

func decryptFile(filename string, passphrase string) []byte {
	//Auto Close File by os.Readfile
	data, _ := ioutil.ReadFile(filename)
	return decryptStr(data, passphrase)
}

// Base64 Decode
func BASE64DecodeStr(src string) string {
	plaintext, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return ""
	}
	return string(plaintext)
}

//Create docker config for multi-build
type secretResource struct {
	name       string
	namespace  string
	configjson string
	template   string
}

func (secretRes *secretResource) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", secretRes.name, "-n", secretRes.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf(fmt.Sprintf("No secret in project: %s, create one: %s", secretRes.namespace, secretRes.name))
		err = applyResource(oc, "--ignore-unknown-parameters=true", "-f", secretRes.template, "-p", "CONFIGJSON="+secretRes.configjson, "NAMESPACE="+secretRes.namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		e2e.Logf(fmt.Sprintf("Already exist %v in project: %s", secretRes.name, secretRes.namespace))
	}
}
