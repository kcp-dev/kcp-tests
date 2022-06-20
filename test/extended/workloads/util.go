package workloads

import (
	"encoding/json"
	"fmt"
	o "github.com/onsi/gomega"
	"io/ioutil"
	"regexp"

	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type podNodeSelector struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	nodeKey    string
	nodeValue  string
	template   string
}

type podSinglePts struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	template   string
}

type podSinglePtsNodeSelector struct {
	name       string
	namespace  string
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	nodeKey    string
	nodeValue  string
	template   string
}

type deploySinglePts struct {
	dName      string
	namespace  string
	replicaNum int
	labelKey   string
	labelValue string
	ptsKeyName string
	ptsPolicy  string
	skewNum    int
	template   string
}

type deployNodeSelector struct {
	dName      string
	namespace  string
	replicaNum int
	labelKey   string
	labelValue string
	nodeKey    string
	nodeValue  string
	template   string
}

type podAffinityRequiredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	affinityMethod string
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podAffinityPreferredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	affinityMethod string
	weigthNum      int
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podNodeAffinityRequiredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	ptsKey2Name    string
	ptsPolicy2     string
	skewNum2       int
	affinityMethod string
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podSingleNodeAffinityRequiredPts struct {
	name           string
	namespace      string
	labelKey       string
	labelValue     string
	ptsKeyName     string
	ptsPolicy      string
	skewNum        int
	affinityMethod string
	keyName        string
	valueName      string
	operatorName   string
	template       string
}

type podTolerate struct {
	namespace      string
	keyName        string
	operatorPolicy string
	valueName      string
	effectPolicy   string
	tolerateTime   int
	template       string
}

// ControlplaneInfo ...
type ControlplaneInfo struct {
	HolderIdentity       string `json:"holderIdentity"`
	LeaseDurationSeconds int    `json:"leaseDurationSeconds"`
	AcquireTime          string `json:"acquireTime"`
	RenewTime            string `json:"renewTime"`
	LeaderTransitions    int    `json:"leaderTransitions"`
}

type serviceInfo struct {
	serviceIP   string
	namespace   string
	servicePort string
	serviceURL  string
	serviceName string
}

type registry struct {
	dockerImage string
	namespace   string
}

type podMirror struct {
	name            string
	namespace       string
	cliImageID      string
	imagePullSecret string
	imageSource     string
	imageTo         string
	imageToRelease  string
	template        string
}

type debugPodUsingDefinition struct {
	name       string
	namespace  string
	cliImageID string
	template   string
}

func (pod *podNodeSelector) createPodNodeSelector(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"NODEKEY="+pod.nodeKey, "NODEVALUE="+pod.nodeValue, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podSinglePts) createPodSinglePts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podSinglePtsNodeSelector) createPodSinglePtsNodeSelector(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"NODEKEY="+pod.nodeKey, "NODEVALUE="+pod.nodeValue)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (deploy *deploySinglePts) createDeploySinglePts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deploy.template, "-p", "DNAME="+deploy.dName, "NAMESPACE="+deploy.namespace,
			"REPLICASNUM="+strconv.Itoa(deploy.replicaNum), "LABELKEY="+deploy.labelKey, "LABELVALUE="+deploy.labelValue, "PTSKEYNAME="+deploy.ptsKeyName,
			"PTSPOLICY="+deploy.ptsPolicy, "SKEWNUM="+strconv.Itoa(deploy.skewNum))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deploy %s with %s is not created successfully", deploy.dName, deploy.labelKey))
}

func (pod *podAffinityRequiredPts) createPodAffinityRequiredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podAffinityPreferredPts) createPodAffinityPreferredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace,
			"LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum),
			"AFFINITYMETHOD="+pod.affinityMethod, "WEIGHTNUM="+strconv.Itoa(pod.weigthNum), "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podSinglePts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podNodeSelector) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podSinglePtsNodeSelector) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podAffinityPreferredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "workload-config.json")
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

func applyResourceFromTemplate48681(oc *exutil.CLI, parameters ...string) (string, error) {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "workload-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return configFile, oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func describePod(oc *exutil.CLI, namespace string, podName string) string {
	podDescribe, err := oc.WithoutNamespace().Run("describe").Args("pod", "-n", namespace, podName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podDescribe)
	return podDescribe
}

func getPodStatus(oc *exutil.CLI, namespace string, podName string) string {
	podStatus, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod  %s status is %q", podName, podStatus)
	return podStatus
}

func getPodNodeListByLabel(oc *exutil.CLI, namespace string, labelKey string) []string {
	output, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", labelKey, "-o=jsonpath={.items[*].spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNameList := strings.Fields(output)
	return nodeNameList
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

func (pod *podNodeAffinityRequiredPts) createpodNodeAffinityRequiredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum), "PTSKEY2NAME="+pod.ptsKey2Name, "PTSPOLICY2="+pod.ptsPolicy2, "SKEWNUM2="+strconv.Itoa(pod.skewNum2), "AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podNodeAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podSingleNodeAffinityRequiredPts) createpodSingleNodeAffinityRequiredPts(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "LABELKEY="+pod.labelKey, "LABELVALUE="+pod.labelValue, "PTSKEYNAME="+pod.ptsKeyName, "PTSPOLICY="+pod.ptsPolicy, "SKEWNUM="+strconv.Itoa(pod.skewNum), "AFFINITYMETHOD="+pod.affinityMethod, "KEYNAME="+pod.keyName, "VALUENAME="+pod.valueName, "OPERATORNAME="+pod.operatorName)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.labelKey))
}

func (pod *podSingleNodeAffinityRequiredPts) getPodNodeName(oc *exutil.CLI) string {
	nodeName, err := oc.WithoutNamespace().Run("get").Args("pod", "-n", pod.namespace, pod.name, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", pod.name, nodeName)
	return nodeName
}

func (pod *podTolerate) createPodTolerate(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAMESPACE="+pod.namespace, "KEYNAME="+pod.keyName,
			"OPERATORPOLICY="+pod.operatorPolicy, "VALUENAME="+pod.valueName, "EFFECTPOLICY="+pod.effectPolicy, "TOLERATETIME="+strconv.Itoa(pod.tolerateTime))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s is not created successfully", pod.keyName))
}

func getPodNodeName(oc *exutil.CLI, namespace string, podName string) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The pod %s lands on node %q", podName, nodeName)
	return nodeName
}

func createLdapService(oc *exutil.CLI, namespace string, podName string, initGroup string) {
	err := oc.Run("run").Args(podName, "--image", "quay.io/openshifttest/ldap:openldap-2441-centos7", "-n", namespace).Execute()
	if err != nil {
		oc.Run("delete").Args("pod/ldapserver", "-n", namespace).Execute()
		e2e.Failf("failed to run the ldap pod")
	}
	err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		podStatus, _ := oc.AsAdmin().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.status.phase}").Output()
		if strings.Compare(podStatus, "Running") != 0 {
			e2e.Logf("the podstatus is :%v, and try next round", podStatus)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		oc.Run("delete").Args("pod/ldapserver", "-n", namespace).Execute()
		e2e.Failf("ldap pod run failed")
	}
	err = oc.Run("cp").Args("-n", namespace, initGroup, podName+":/tmp/").Execute()
	if err != nil {
		oc.Run("delete").Args("pod/ldapserver", "-n", oc.Namespace()).Execute()
		e2e.Failf("failed to copy the init group to ldap server")
	}
	err = oc.Run("exec").Args(podName, "-n", namespace, "--", "ldapadd", "-x", "-h", "127.0.0.1", "-p", "389", "-D", "cn=Manager,dc=example,dc=com", "-w", "admin", "-f", "/tmp/init.ldif").Execute()
	if err != nil {
		oc.Run("delete").Args("pod/ldapserver", "-n", namespace).Execute()
		e2e.Failf("failed to config the ldap server ")
	}

}

func getSyncGroup(oc *exutil.CLI, syncConfig string) string {
	var groupFile string
	err := wait.Poll(5*time.Second, 200*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("adm").Args("groups", "sync", "--sync-config="+syncConfig).OutputToFile(getRandomString() + "workload-group.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		groupFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "adm groups sync fails")
	if strings.Compare(groupFile, "") == 0 {
		e2e.Failf("Failed to get group infomation!")
	}
	return groupFile
}

func getLeaderKCM(oc *exutil.CLI) string {
	var leaderKCM string
	e2e.Logf("Get the control-plane from configmap")
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap/kube-controller-manager", "-n", "kube-system", "-o=jsonpath={.metadata.annotations.control-plane\\.alpha\\.kubernetes\\.io/leader}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Print the output: %v ", output)
	contronplanInfo := &ControlplaneInfo{}
	e2e.Logf("convert to json file ")
	if err = json.Unmarshal([]byte(output), &contronplanInfo); err != nil {
		e2e.Failf("unable to decode with error: %v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	leaderIP := strings.Split(contronplanInfo.HolderIdentity, "_")[0]

	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	masterList := strings.Fields(out)
	for _, masterNode := range masterList {
		if matched, _ := regexp.MatchString(leaderIP, masterNode); matched {
			e2e.Logf("Find the leader of KCM :%s\n", masterNode)
			leaderKCM = masterNode
			break
		}
	}
	return leaderKCM
}

func removeDuplicateElement(elements []string) []string {
	result := make([]string, 0, len(elements))
	temp := map[string]struct{}{}
	for _, item := range elements {
		if _, ok := temp[item]; !ok { //if can't find the item，ok=false，!ok is true，then append item。
			temp[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func (registry *registry) createregistry(oc *exutil.CLI) serviceInfo {
	err := oc.AsAdmin().Run("new-app").Args("--image", registry.dockerImage, "-n", registry.namespace).Execute()
	if err != nil {
		e2e.Failf("Failed to create the registry server")
	}
	err = oc.AsAdmin().Run("set").Args("probe", "deploy/registry", "--readiness", "--liveness", "--get-url="+"http://:5000/v2", "-n", registry.namespace).Execute()
	if err != nil {
		e2e.Failf("Failed to config the registry")
	}
	err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err = oc.AsAdmin().Run("get").Args("pod", "-l", "deployment=registry", "-n", registry.namespace).Execute()
		if err != nil {
			e2e.Logf("The err:%v, and try next round", err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "pod of deployment=registry is not got")

	e2e.Logf("Get the service info of the registry")
	regSvcIP, err := oc.AsAdmin().Run("get").Args("svc", "registry", "-n", registry.namespace, "-o=jsonpath={.spec.clusterIP}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	regSvcPort, err := oc.AsAdmin().Run("get").Args("svc", "registry", "-n", registry.namespace, "-o=jsonpath={.spec.ports[0].port}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	regSvcURL := regSvcIP + ":" + regSvcPort
	regName := "registry"
	svc := serviceInfo{
		serviceIP:   regSvcIP,
		namespace:   registry.namespace,
		servicePort: regSvcPort,
		serviceURL:  regSvcURL,
		serviceName: regName,
	}
	return svc

}

func (registry *registry) deleteregistry(oc *exutil.CLI) {
	_ = oc.Run("delete").Args("svc", "registry", "-n", registry.namespace).Execute()
	_ = oc.Run("delete").Args("deploy", "registry", "-n", registry.namespace).Execute()
	_ = oc.Run("delete").Args("is", "registry", "-n", registry.namespace).Execute()
}

func (pod *podMirror) createPodMirror(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "CLIIMAGEID="+pod.cliImageID, "IMAGEPULLSECRET="+pod.imagePullSecret, "IMAGESOURCE="+pod.imageSource, "IMAGETO="+pod.imageTo, "IMAGETORELEASE="+pod.imageToRelease)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.cliImageID))
}

func createPullSecret(oc *exutil.CLI, namespace string) {
	err := oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--to=/tmp", "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("create").Args("secret", "generic", "my-secret", "--from-file="+"/tmp/.dockerconfigjson", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getCliImage(oc *exutil.CLI) string {
	cliImage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("imagestreams", "cli", "-n", "openshift", "-o=jsonpath={.spec.tags[0].from.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return cliImage
}

func getScanNodesLabels(oc *exutil.CLI, nodeList []string, expected string) []string {
	var machedLabelsNodeNames []string
	for _, nodeName := range nodeList {
		nodeLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString(expected, nodeLabels); matched {
			machedLabelsNodeNames = append(machedLabelsNodeNames, nodeName)
		}
	}
	return machedLabelsNodeNames
}

func checkMustgatherPodNode(oc *exutil.CLI) {
	var nodeNameList []string
	e2e.Logf("Get the node list of the must-gather pods running on")
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-l", "app=must-gather", "-A", "-o=jsonpath={.items[*].spec.nodeName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeNameList = strings.Fields(output)
		if nodeNameList == nil {
			e2e.Logf("Can't find must-gather pod now, and try next round")
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("must-gather pod is not created successfully"))
	e2e.Logf("must-gather scheduled on: %v", nodeNameList)

	e2e.Logf("make sure all the nodes in nodeNameList are not windows node")
	expectedNodeLabels := getScanNodesLabels(oc, nodeNameList, "windows")
	if expectedNodeLabels == nil {
		e2e.Logf("must-gather scheduled as expected, no windows node found in the cluster")
	} else {
		e2e.Failf("Scheduled the must-gather pod to windows node: %v", expectedNodeLabels)
	}
}

func (pod *debugPodUsingDefinition) createDebugPodUsingDefinition(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		outputFile, err1 := applyResourceFromTemplate48681(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "CLIIMAGEID="+pod.cliImageID)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		e2e.Logf("Waiting for pod running")
		err := wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
			phase, err := oc.AsAdmin().Run("get").Args("pods", pod.name, "--template", "{{.status.phase}}", "-n", pod.namespace).Output()
			if err != nil {
				return false, nil
			}
			if phase != "Running" {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod has not been started successfully"))

		debugPod, err := oc.Run("debug").Args("-f", outputFile).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if match, _ := regexp.MatchString("Starting pod/pod48681-debug", debugPod); !match {
			e2e.Failf("Image debug container is being started instead of debug pod using the pod definition yaml file")
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pod %s with %s is not created successfully", pod.name, pod.cliImageID))
}

func createDeployment(oc *exutil.CLI, namespace string, deployname string) {
	err := oc.Run("create").Args("-n", namespace, "deployment", deployname, "--image=quay.io/openshifttest/hello-openshift@sha256:1e70b596c05f46425c39add70bf749177d78c1e98b2893df4e5ae3883c2ffb5e", "--replicas=20").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func triggerSucceedDeployment(oc *exutil.CLI, namespace string, deployname string, num int, expectedPods int) {
	for i := 0; i < num; i++ {
		err := oc.Run("set").Args("-n", namespace, "env", "deployment", deployname, "paramtest=test"+strconv.Itoa(i)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, currentRsName := getCurrentRs(oc, namespace, "app="+deployname)
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			availablePodNum, errGet := oc.Run("get").Args("-n", namespace, "rs", currentRsName, "-o=jsonpath='{.status.availableReplicas}'").Output()
			if errGet != nil {
				e2e.Logf("Err Occurred: %v", errGet)
				return false, errGet
			}
			availableNum, _ := strconv.Atoi(strings.ReplaceAll(availablePodNum, "'", ""))
			if availableNum != expectedPods {
				e2e.Logf("new triggered apps not deploy successfully, wait more times")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("failed to deploy %v", deployname))

	}
}
func triggerFailedDeployment(oc *exutil.CLI, namespace string, deployname string) {
	patchYaml := `[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "quay.io/openshifttest/hello-openshift:nonexist"}]`
	err := oc.Run("patch").Args("-n", namespace, "deployment", deployname, "--type=json", "-p", patchYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getShouldPruneRSFromPrune(oc *exutil.CLI, pruneRsNumCMD string, pruneRsCMD string, prunedNum int) []string {
	e2e.Logf("Get pruned rs name by dry-run")
	e2e.Logf("pruneRsNumCMD %v:", pruneRsNumCMD)
	err := wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		pruneRsNum, err := exec.Command("bash", "-c", pruneRsNumCMD).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pruneNum, err := strconv.Atoi(strings.ReplaceAll(string(pruneRsNum), "\n", ""))
		o.Expect(err).NotTo(o.HaveOccurred())
		if pruneNum != prunedNum {
			e2e.Logf("pruneNum is not equal %v: ", prunedNum)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Check pruned RS failed"))

	e2e.Logf("pruneRsCMD %v:", pruneRsCMD)
	pruneRsName, err := exec.Command("bash", "-c", pruneRsCMD).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	pruneRsList := strings.Fields(strings.ReplaceAll(string(pruneRsName), "\n", " "))
	sort.Strings(pruneRsList)
	e2e.Logf("pruneRsList %v:", pruneRsList)
	return pruneRsList
}

func getCompeletedRsInfo(oc *exutil.CLI, namespace string, deployname string) (completedRsList []string, completedRsNum int) {
	out, err := oc.Run("get").Args("-n", namespace, "rs", "--sort-by={.metadata.creationTimestamp}", "-o=jsonpath='{.items[?(@.spec.replicas == 0)].metadata.name}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("string out %v:", out)
	totalCompletedRsList := strings.Fields(strings.ReplaceAll(out, "'", ""))
	totalCompletedRsListNum := len(totalCompletedRsList)
	return totalCompletedRsList, totalCompletedRsListNum
}

func getShouldPruneRSFromCreateTime(totalCompletedRsList []string, totalCompletedRsListNum int, keepNum int) []string {
	rsList := totalCompletedRsList[0:(totalCompletedRsListNum - keepNum)]
	sort.Strings(rsList)
	e2e.Logf("rsList %v:", rsList)
	return rsList

}

func comparePrunedRS(rsList []string, pruneRsList []string) bool {
	e2e.Logf("Check pruned rs whether right")
	if !reflect.DeepEqual(rsList, pruneRsList) {
		return false
	}
	return true
}

func checkRunningRsList(oc *exutil.CLI, namespace string, deployname string) []string {
	e2e.Logf("Get all the running RSs")
	out, err := oc.Run("get").Args("-n", namespace, "rs", "-o=jsonpath='{.items[?(@.spec.replicas > 0)].metadata.name}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	runningRsList := strings.Fields(strings.ReplaceAll(out, "'", ""))
	sort.Strings(runningRsList)
	e2e.Logf("runningRsList %v:", runningRsList)
	return runningRsList
}

func pruneCompletedRs(oc *exutil.CLI, parameters ...string) {
	e2e.Logf("Delete all the completed RSs")
	err := oc.AsAdmin().WithoutNamespace().Run("adm").Args(parameters...).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRemainingRs(oc *exutil.CLI, namespace string, deployname string) []string {
	e2e.Logf("Get all the remaining RSs")
	remainRs, err := oc.WithoutNamespace().Run("get").Args("rs", "-l", "app="+deployname, "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	remainRsList := strings.Fields(string(remainRs))
	sort.Strings(remainRsList)
	e2e.Logf("remainRsList %v:", remainRsList)
	return remainRsList
}

func getCurrentRs(oc *exutil.CLI, projectName string, labels string) (string, string) {
	var podTHash, rsName string
	currentGeneration, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "-n", projectName, "-l", labels, "-o=jsonpath={.items[*].status.observedGeneration}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("rs", "-n", projectName, "-l", labels, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	rsNameList := strings.Fields(output)
	for _, rsname := range rsNameList {
		version, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("rs", rsname, "-n", projectName, "-o=jsonpath={.metadata.annotations.deployment\\.kubernetes\\.io/revision}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString(currentGeneration, version); matched {
			podTHash, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("rs", rsname, "-n", projectName, "-o=jsonpath={.spec.selector.matchLabels.pod-template-hash}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("%s is the current rs", rsname)
			rsName = rsname
			break
		}
	}
	return podTHash, rsName
}

func copyFile(source string, dest string) {
	bytesRead, err := ioutil.ReadFile(source)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = ioutil.WriteFile(dest, bytesRead, 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func locatePodmanCred(oc *exutil.CLI, dst string) error {
	err := oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--to="+dst, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	key := "XDG_RUNTIME_DIR"
	currentRuntime, ex := os.LookupEnv(key)
	if !ex {
		err = os.MkdirAll("/tmp/configocmirror/containers", 0700)
		o.Expect(err).NotTo(o.HaveOccurred())
		os.Setenv(key, "/tmp/configocmirror")
		copyFile(dst+"/"+".dockerconfigjson", "/tmp/configocmirror/containers/auth.json")
		return nil
	}
	_, err = os.Stat(currentRuntime + "containers/auth.json")
	if os.IsNotExist(err) {
		err1 := os.MkdirAll(currentRuntime+"containers", 0700)
		o.Expect(err1).NotTo(o.HaveOccurred())
		copyFile(dst+"/"+".dockerconfigjson", "/tmp/configocmirror/containers/auth.json")
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func checkPodStatus(oc *exutil.CLI, podLabel string, namespace string, expected string) {
	err := wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", podLabel, "-o=jsonpath={.items[*].status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of pod:%v", output)
		if strings.Contains(output, expected) && (!(strings.Contains(strings.ToLower(output), "error"))) && (!(strings.Contains(strings.ToLower(output), "crashLoopbackOff"))) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("the state of pod with %s is not expected %s", podLabel, expected))
}

func locateDockerCred(oc *exutil.CLI, dst string) (string, string, error) {
	err := oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--to="+dst, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	homePath := os.Getenv("HOME")
	dockerCreFile := homePath + "/.docker/config.json"
	_, err = os.Stat(homePath + "/.docker/config.json")
	if os.IsNotExist(err) {
		err1 := os.MkdirAll(homePath+"/.docker", 0700)
		o.Expect(err1).NotTo(o.HaveOccurred())
		copyFile(dst+"/"+".dockerconfigjson", homePath+"/.docker/config.json")
		return dockerCreFile, homePath, nil
	}
	if err != nil {
		return "", "", err
	}
	copyFile(homePath+"/.docker/config.json", homePath+"/.docker/config.json.back")
	copyFile(dst+"/"+".dockerconfigjson", homePath+"/.docker/config.json")
	return dockerCreFile, homePath, nil

}

func waitCoBecomes(oc *exutil.CLI, coName string, waitTime int, expectedStatus map[string]string) error {
	return wait.Poll(5*time.Second, time.Duration(waitTime)*time.Second, func() (bool, error) {
		gottenStatus := getCoStatus(oc, coName, expectedStatus)
		eq := reflect.DeepEqual(expectedStatus, gottenStatus)
		if eq {
			eq := reflect.DeepEqual(expectedStatus, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
			if eq {
				// For True False False, we want to wait some bit more time and double check, to ensure it is stably healthy
				time.Sleep(100 * time.Second)
				gottenStatus := getCoStatus(oc, coName, expectedStatus)
				eq := reflect.DeepEqual(expectedStatus, gottenStatus)
				if eq {
					e2e.Logf("Given operator %s becomes available/non-progressing/non-degraded", coName)
					return true, nil
				}
			} else {
				e2e.Logf("Given operator %s becomes %s", coName, gottenStatus)
				return true, nil
			}
		}
		return false, nil
	})
}

func getCoStatus(oc *exutil.CLI, coName string, statusToCompare map[string]string) map[string]string {
	newStatusToCompare := make(map[string]string)
	for key := range statusToCompare {
		args := fmt.Sprintf(`-o=jsonpath={.status.conditions[?(.type == '%s')].status}`, key)
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", args, coName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		newStatusToCompare[key] = status
	}
	return newStatusToCompare
}
