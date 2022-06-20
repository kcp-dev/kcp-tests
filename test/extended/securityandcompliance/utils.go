package securityandcompliance

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

type fileintegrity struct {
	name              string
	namespace         string
	configname        string
	configkey         string
	graceperiod       int
	debug             bool
	nodeselectorkey   string
	nodeselectorvalue string
	template          string
}

type podModify struct {
	name      string
	namespace string
	nodeName  string
	args      string
	template  string
}

func (fi1 *fileintegrity) checkFileintegrityStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", fi1.namespace, "-l app=aide-example-fileintegrity",
			"-o=jsonpath={.items[*].status.containerStatuses[*].state}").Output()
		e2e.Logf("the result of checkFileintegrityStatus:%v", output)
		if strings.Contains(output, expected) && (!(strings.Contains(strings.ToLower(output), "error"))) && (!(strings.Contains(strings.ToLower(output), "crashLoopbackOff"))) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("the state of pod with app=aide-example-fileintegrity is not expected %s", expected))
}

func (fi1 *fileintegrity) getConfigmapFromFileintegritynodestatus(oc *exutil.CLI, nodeName string) string {
	var cmName string
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		cmName, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].resultConfigMapName}").Output()
		e2e.Logf("the result of cmName:%v", cmName)
		if strings.Contains(cmName, "failed") {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resultConfigMapName %s is not failed", fi1.name+"-"+nodeName))
	if strings.Compare(cmName, "") == 0 {
		e2e.Failf("Failed to get configmap name!")
	}
	return cmName
}

func (fi1 *fileintegrity) getDataFromConfigmap(oc *exutil.CLI, cmName string, expected string) {
	e2e.Logf("the result of cmName:%v", cmName)
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		aideResult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap/"+cmName, "-n", fi1.namespace, "-o=jsonpath={.data}").Output()
		if strings.Contains(aideResult, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %s does not include %s", cmName, expected))
}

func getOneWorkerNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l node-role.kubernetes.io/worker=",
		"-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of nodename:%v", nodeName)
	return nodeName
}

func getOneMasterNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l node-role.kubernetes.io/master=",
		"-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of nodename:%v", nodeName)
	return nodeName
}

func (fi1 *fileintegrity) getOneFioPodName(oc *exutil.CLI) string {
	fioPodName, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=",
		"-n", fi1.namespace, "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	e2e.Logf("the result of fioPodName:%v", fioPodName)
	if strings.Compare(fioPodName, "") != 0 {
		return fioPodName
	}
	return fioPodName
}

func (fi1 *fileintegrity) checkKeywordNotExistInLog(oc *exutil.CLI, podName string, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		logs, err1 := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podName, "-n", fi1.namespace).Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of logs:%v", logs)
		if strings.Compare(logs, "") != 0 && !strings.Contains(logs, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s includes %s", podName, expected))
}

func (fi1 *fileintegrity) checkKeywordExistInLog(oc *exutil.CLI, podName string, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		logs, err1 := oc.AsAdmin().WithoutNamespace().Run("logs").Args("pod/"+podName, "-n", fi1.namespace).Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of logs:%v", logs)
		if strings.Contains(logs, expected) {
			e2e.Logf("The pod '%s' logs include '%s'", podName, expected)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s does not include %s", podName, expected))
}

func (fi1 *fileintegrity) checkArgsInPod(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		fioPodArgs, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=",
			"-n", fi1.namespace, "-o=jsonpath={.items[0].spec.containers[].args}").Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("the result of fioPodArgs: %v", fioPodArgs)
		if strings.Contains(fioPodArgs, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("args of does not include %s", expected))
}

func (pod *podModify) doActionsOnNode(oc *exutil.CLI, expected string, dr describerResrouce) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"NODENAME="+pod.nodeName, "PARAC="+pod.args)
		o.Expect(err1).NotTo(o.HaveOccurred())
		podModifyresult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.status.phase}").Output()
		e2e.Logf("the result of pod %s: %v", pod.name, podModifyresult)
		if strings.Contains(podModifyresult, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("phase of pod is not expected %s", expected))
}

func (fi1 *fileintegrity) createFIOWithoutConfig(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"GRACEPERIOD="+strconv.Itoa(fi1.graceperiod), "DEBUG="+strconv.FormatBool(fi1.debug), "NODESELECTORKEY="+fi1.nodeselectorkey, "NODESELECTORVALUE="+fi1.nodeselectorvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (fi1 *fileintegrity) createFIOWithoutKeyword(oc *exutil.CLI, itName string, dr describerResrouce, keyword string) {
	err := applyResourceFromTemplateWithoutKeyword(oc, keyword, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"CONFNAME="+fi1.configname, "CONFKEY="+fi1.configkey, "DEBUG="+strconv.FormatBool(fi1.debug), "NODESELECTORKEY="+fi1.nodeselectorkey, "NODESELECTORVALUE="+fi1.nodeselectorvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (fi1 *fileintegrity) createFIOWithConfig(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", fi1.template, "-p", "NAME="+fi1.name, "NAMESPACE="+fi1.namespace,
		"GRACEPERIOD="+strconv.Itoa(fi1.graceperiod), "DEBUG="+strconv.FormatBool(fi1.debug), "CONFNAME="+fi1.configname, "CONFKEY="+fi1.configkey,
		"NODESELECTORKEY="+fi1.nodeselectorkey, "NODESELECTORVALUE="+fi1.nodeselectorvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "fileintegrity", fi1.name, requireNS, fi1.namespace))
}

func (sub *subscriptionDescription) checkPodFioStatus(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", sub.namespace, "-l", "name=file-integrity-operator",
			"-o=jsonpath={.items[*].status.containerStatuses[*].state}").Output()
		e2e.Logf("the result of checkPodFioStatus:%v", output)
		if strings.Contains(strings.ToLower(output), expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("state of pod with name=file-integrity-operator is not expected %s", expected))
}

func (fi1 *fileintegrity) createConfigmapFromFile(oc *exutil.CLI, itName string, dr describerResrouce, cmName string, aideKey string, aideFile string, expected string) (bool, error) {
	output, _ := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", cmName, "-n", fi1.namespace, "--from-file="+aideKey+"="+aideFile).Output()
	dr.getIr(itName).add(newResource(oc, "configmap", cmName, requireNS, fi1.namespace))
	e2e.Logf("the result of checkPodFioStatus:%v", output)
	if strings.Contains(strings.ToLower(output), expected) {
		return true, nil
	}
	return false, nil
}

func (fi1 *fileintegrity) checkConfigmapCreated(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", fi1.configname, "-n", fi1.namespace).Output()
		e2e.Logf("the result of checkConfigmapCreated:%v", output)
		if strings.Contains(output, fi1.configname) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %s is not created", fi1.configname))
}

func (fi1 *fileintegrity) checkFileintegritynodestatus(oc *exutil.CLI, nodeName string, expected string) {
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		fileintegrityName := fi1.name + "-" + nodeName
		fileintegritynodestatusOut, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", fileintegrityName, "-n", fi1.namespace).Output()
		e2e.Logf("the result of fileintegritynodestatusOut:%v", fileintegritynodestatusOut)
		if !strings.Contains(fileintegritynodestatusOut, fileintegrityName) || err1 != nil {
			return false, nil
		}
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fileintegrityName,
			"-o=jsonpath={.lastResult.condition}").Output()
		e2e.Logf("the result of checkFileintegritynodestatus:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fileintegritynodestatuses %s is not expected %s", fi1.name+"-"+nodeName, expected))
}

func (fi1 *fileintegrity) checkOnlyOneDaemonset(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 45*time.Second, func() (bool, error) {
		daemonsetPodNumber, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("daemonset", "-n", fi1.namespace, "-o=jsonpath={.items[].status.numberReady}").Output()
		e2e.Logf("the result of daemonsetPodNumber:%v", daemonsetPodNumber)
		podNameString, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l file-integrity.openshift.io/pod=", "-n", fi1.namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
		e2e.Logf("the result of podNameString:%v", podNameString)
		intDaemonsetPodNumber, _ := strconv.Atoi(daemonsetPodNumber)
		intPodNumber := len(strings.Fields(podNameString))
		e2e.Logf("the result of intPodNumber:%v", intPodNumber)
		if intPodNumber == intDaemonsetPodNumber {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "daemonset number is not expted ")
}

func (fi1 *fileintegrity) removeFileintegrity(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("delete").Args("fileintegrity", fi1.name, "-n", fi1.namespace).Output()
		e2e.Logf("the result of removeFileintegrity:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}

		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("delete fileintegrity  %s is not expected %s", fi1.name, expected))
}

func (fi1 *fileintegrity) reinitFileintegrity(oc *exutil.CLI, expected string) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("fileintegrity", fi1.name, "-n", fi1.namespace, "file-integrity.openshift.io/re-init=").Output()
		e2e.Logf("the result of reinitFileintegrity:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("annotate fileintegrity  %s is not expected %s", fi1.name, expected))
}

func (fi1 *fileintegrity) getDetailedDataFromFileintegritynodestatus(oc *exutil.CLI, nodeName string) (int, int, int) {
	var intFilesAdded, intFilesChanged, intFilesRemoved int
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		filesAdded, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].filesAdded}").Output()
		e2e.Logf("the result of filesAdded in Fileintegritynodestatus:%v", filesAdded)
		filesChanged, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].filesChanged}").Output()
		e2e.Logf("the result of filesChanged in Fileintegritynodestatus:%v", filesChanged)
		filesRemoved, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegritynodestatuses", "-n", fi1.namespace, fi1.name+"-"+nodeName,
			"-o=jsonpath={.results[-1].filesRemoved}").Output()
		e2e.Logf("the result of filesRemoved in Fileintegritynodestatus:%v", filesRemoved)
		if filesAdded == "" && filesChanged == "" && filesRemoved == "" {
			return false, nil
		}
		if filesAdded == "" {
			intFilesAdded = 0
		} else {
			intFilesAdded, _ = strconv.Atoi(filesAdded)
		}
		if filesChanged == "" {
			intFilesChanged = 0
		} else {
			intFilesChanged, _ = strconv.Atoi(filesChanged)
		}
		if filesRemoved == "" {
			intFilesRemoved = 0
		} else {
			intFilesRemoved, _ = strconv.Atoi(filesRemoved)
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("file of fileintegritynodestatuses  %s is not added, changed or removed", fi1.name+"-"+nodeName))
	return intFilesAdded, intFilesChanged, intFilesRemoved
}

func (fi1 *fileintegrity) getDetailedDataFromConfigmap(oc *exutil.CLI, cmName string) (int, int, int) {
	var intFilesAdded, intFilesChanged, intFilesRemoved int
	e2e.Logf("the result of cmName:%v", cmName)
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		annotations, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", cmName, "-n", fi1.namespace,
			"-o=jsonpath={.metadata.annotations}").Output()
		e2e.Logf("the result of annotations in configmap:%v", annotations)
		filesAdded, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", cmName, "-n", fi1.namespace,
			"-o=jsonpath={.metadata.annotations.file-integrity\\.openshift\\.io/files-added}").Output()
		e2e.Logf("the result of filesAdded in configmap:%v", filesAdded)
		filesChanged, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", cmName, "-n", fi1.namespace,
			"-o=jsonpath={.metadata.annotations.file-integrity\\.openshift\\.io/files-changed}").Output()
		e2e.Logf("the result of filesChanged in configmap:%v", filesChanged)
		filesRemoved, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", cmName, "-n", fi1.namespace,
			"-o=jsonpath={.metadata.annotations.file-integrity\\.openshift\\.io/files-removed}").Output()
		e2e.Logf("the result of filesRemoved in configmap:%v", filesRemoved)
		if (filesAdded == "" && filesChanged == "" && filesRemoved == "") || (filesAdded == "0" && filesChanged == "0" && filesRemoved == "0") {
			return false, nil
		}
		if filesAdded == "" {
			intFilesAdded = 0
		} else {
			intFilesAdded, _ = strconv.Atoi(filesAdded)
		}
		if filesChanged == "" {
			intFilesChanged = 0
		} else {
			intFilesChanged, _ = strconv.Atoi(filesChanged)
		}
		if filesRemoved == "" {
			intFilesRemoved = 0
		} else {
			intFilesRemoved, _ = strconv.Atoi(filesRemoved)
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("file of cm  %s is not added, changed or removed", cmName))
	return intFilesAdded, intFilesChanged, intFilesRemoved
}

func checkDataDetailsEqual(intFileAddedCM int, intFileChangedCM int, intFileRemovedCM int, intFileAddedFins int, intFileChangedFins int, intFileRemovedFins int) {
	if intFileAddedCM != intFileAddedFins || intFileChangedCM != intFileChangedFins || intFileRemovedCM != intFileRemovedFins {
		e2e.Failf("the data datails in configmap and fileintegrity not equal!")
	}
}

func (fi1 *fileintegrity) checkPodNumerLessThanNodeNumber(oc *exutil.CLI, label string) {
	err := wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
		intNodeNumber := getNodeNumberPerLabel(oc, label)
		daemonsetPodNumber, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("daemonset", "-n", fi1.namespace, "-o=jsonpath={.items[].status.numberReady}").Output()
		e2e.Logf("the result of intNodeNumber:%v", intNodeNumber)
		e2e.Logf("the result of daemonsetPodNumber:%v", daemonsetPodNumber)
		intDaemonsetPodNumber, _ := strconv.Atoi(daemonsetPodNumber)
		if intNodeNumber != intDaemonsetPodNumber+1 {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "daemonset pod number greater than node number")
}

func (fi1 *fileintegrity) checkPodNumerEqualNodeNumber(oc *exutil.CLI, label string) {
	err := wait.Poll(5*time.Second, 100*time.Second, func() (bool, error) {
		intNodeNumber := getNodeNumberPerLabel(oc, label)
		daemonsetPodNumber, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("daemonset", "-n", fi1.namespace, "-o=jsonpath={.items[].status.numberReady}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of intNodeNumber:%v", intNodeNumber)
		e2e.Logf("the result of daemonsetPodNumber:%v", daemonsetPodNumber)
		intDaemonsetPodNumber, _ := strconv.Atoi(daemonsetPodNumber)
		if intNodeNumber != intDaemonsetPodNumber {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "daemonset pod number not equal to node number")
}

func (fi1 *fileintegrity) recreateFileintegrity(oc *exutil.CLI) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("fileintegrity", fi1.name, "-n", fi1.namespace, "-ojson").OutputToFile(getRandomString() + "isc-config.json")
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fileintegrity %s is not got", fi1.name))
	e2e.Logf("the file of resource is %s", configFile)
	fi1.removeFileintegrity(oc, "deleted")
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func setLabelToSpecificNode(oc *exutil.CLI, nodeName string, label string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("node", nodeName, label).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (fi1 *fileintegrity) expectedStringNotExistInConfigmap(oc *exutil.CLI, cmName string, expected string) {
	e2e.Logf("the result of cmName:%v", cmName)
	err := wait.Poll(5*time.Second, 150*time.Second, func() (bool, error) {
		aideResult, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap/"+cmName, "-n", fi1.namespace, "-o=jsonpath={.data}").Output()
		if !strings.Contains(aideResult, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cm %s contains %s", cmName, expected))
}
