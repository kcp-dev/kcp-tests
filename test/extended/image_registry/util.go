package imageregistry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	withoutNamespace = true
	contain          = false
	ok               = true
)

type prometheusResponse struct {
	Status string                 `json:"status"`
	Error  string                 `json:"error"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

// tbuskey@redhat.com for OCP-22056
type prometheusImageregistryQueryHTTP struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name      string `json:"__name__"`
				Container string `json:"container"`
				Endpoint  string `json:"endpoint"`
				Instance  string `json:"instance"`
				Job       string `json:"job"`
				Namespace string `json:"namespace"`
				Pod       string `json:"pod"`
				Service   string `json:"service"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

func listPodStartingWith(prefix string, oc *exutil.CLI, namespace string) (pod []corev1.Pod) {
	podsToAll := []corev1.Pod{}
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		e2e.Logf("Error listing pods: %v", err)
		return nil
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podsToAll = append(podsToAll, pod)
		}
	}
	return podsToAll
}

func dePodLogs(pods []corev1.Pod, oc *exutil.CLI, matchlogs string) bool {
	for _, pod := range pods {
		depOutput, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(depOutput, matchlogs) {
			return true
		}
	}
	return false
}

func getBearerTokenURLViaPod(ns string, execPodName string, url string, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl --retry 15 --max-time 2 --retry-delay 1 -s -k -H 'Authorization: Bearer %s' %s", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func runQuery(queryURL, ns, execPodName, bearerToken string) (*prometheusResponse, error) {
	contents, err := getBearerTokenURLViaPod(ns, execPodName, queryURL, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("unable to execute query %v", err)
	}
	var result prometheusResponse
	if err := json.Unmarshal([]byte(contents), &result); err != nil {
		return nil, fmt.Errorf("unable to parse query response: %v", err)
	}
	metrics := result.Data.Result
	if result.Status != "success" {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		return nil, fmt.Errorf("incorrect response status: %s with error %s", data, result.Error)
	}
	return &result, nil
}

func metricReportStatus(queryURL, ns, execPodName, bearerToken string, value model.SampleValue) bool {
	result, err := runQuery(queryURL, ns, execPodName, bearerToken)
	o.Expect(err).NotTo(o.HaveOccurred())
	if result.Data.Result[0].Value == value {
		return true
	}
	return false
}

type bcSource struct {
	outname   string
	name      string
	namespace string
	template  string
}

type authRole struct {
	namespace string
	rolename  string
	template  string
}

func (bcsrc *bcSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", bcsrc.template, "-p", "OUTNAME="+bcsrc.outname, "NAME="+bcsrc.name, "NAMESPACE="+bcsrc.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (authrole *authRole) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", authrole.template, "-p", "NAMESPACE="+authrole.namespace, "ROLE_NAME="+authrole.rolename)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
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
	exutil.AssertWaitPollNoErr(err, "Applying resources from template is failed")
	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

//the method is to get something from resource. it is "oc get xxx" actaully
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed to get %v", parameters))
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

//the method is to do something with oc.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func comparePodHostIP(oc *exutil.CLI) (int, int) {
	var hostsIP = []string{}
	var numi, numj int
	podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
	for _, pod := range podList.Items {
		hostsIP = append(hostsIP, pod.Status.HostIP)
	}
	for i := 0; i < len(hostsIP)-1; i++ {
		for j := i + 1; j < len(hostsIP); j++ {
			if hostsIP[i] == hostsIP[j] {
				numi++
			} else {
				numj++
			}
		}
	}
	return numi, numj
}

//Check the latest image pruner pod logs
func imagePruneLog(oc *exutil.CLI, matchLogs, notMatchLogs string) {
	podsOfImagePrune := []corev1.Pod{}
	err := wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
		podsOfImagePrune = listPodStartingWith("image-pruner", oc, "openshift-image-registry")
		if podsOfImagePrune == nil || len(podsOfImagePrune) == 0 {
			e2e.Logf("Can't get pruner pods, go to next round")
			return false, nil
		}
		pod := podsOfImagePrune[len(podsOfImagePrune)-1]
		e2e.Logf("the pod status is %s", pod.Status.Phase)
		if pod.Status.Phase != "ContainerCreating" && pod.Status.Phase != "Pending" {
			depOutput, _ := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-n", pod.Namespace).Output()
			if strings.Contains(depOutput, matchLogs) && !strings.Contains(depOutput, notMatchLogs) {
				return true, nil
			}
		}
		e2e.Logf("The image pruner log doesn't contain %v or contain %v", matchLogs, notMatchLogs)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Can't get the image pruner log or image pruner log doesn't contain %v or contain %v", matchLogs, notMatchLogs))
}

func configureRegistryStorageToEmptyDir(oc *exutil.CLI) {
	var hasstorage string
	emptydirstorage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.status.storage.emptyDir}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if emptydirstorage == "{}" {
		g.By("Image registry is using EmptyDir now")
	} else {
		g.By("Set registry to use EmptyDir storage")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		switch platformtype {
		case "AWS":
			hasstorage = "s3"
		case "OpenStack":
			hasstorage = "swift"
		case "GCP":
			hasstorage = "gcs"
		case "Azure":
			hasstorage = "azure"
		default:
			e2e.Logf("Image Registry is using unknown storage type")
		}
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"`+hasstorage+`":null,"emptyDir":{}}, "replicas":1}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(30*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(podList.Items) == 1 && podList.Items[0].Status.Phase == corev1.PodRunning {
				return true, nil
			}
			e2e.Logf("Continue to next round")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 1")
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("configs.imageregistry/cluster", "--for=condition=Available").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func recoverRegistryStorageConfig(oc *exutil.CLI) {
	g.By("Set image registry storage to default value")
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if platformtype != "VSphere" {
		if platformtype != "None" {
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":null}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Image registry will be auto-recovered to default storage")
		}
	}
}

func recoverRegistryDefaultReplicas(oc *exutil.CLI) {
	g.By("Set image registry to default replicas")
	platforms := map[string]bool{
		"VSphere": true,
		"None":    true,
		"oVirt":   true,
	}
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if !platforms[platformtype] {
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("config.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(30*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 2 {
				e2e.Logf("Continue to next round")
			} else {
				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						e2e.Logf("Continue to next round")
						return false, nil
					}
				}
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 2")
	}
}

func restoreRegistryStorageConfig(oc *exutil.CLI) (string, string) {
	var storagetype, storageinfo string
	g.By("Get image registry storage info")
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	switch platformtype {
	case "AWS":
		storagetype = "s3"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.s3.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "Azure":
		storagetype = "azure"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.azure.container}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "GCP":
		storagetype = "gcs"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.gcs.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "OpenStack":
		storagetype = "swift"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.swift.container}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		//On disconnect & openstack, the registry configure to use persistent volume
		if storageinfo == "" {
			storagetype = "pvc"
			storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.pvc.claim}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	case "AlibabaCloud":
		storagetype = "oss"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.oss.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "IBMCloud":
		storagetype = "ibmocs"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.ibmocs.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	case "None", "VSphere":
		storagetype = "pvc"
		storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.pvc.claim}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if storageinfo == "" {
			storagetype = "emptyDir"
			storageinfo, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage.emptyDir}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	default:
		e2e.Logf("Image Registry is using unknown storage type")
	}
	return storagetype, storageinfo
}

func loginRegistryDefaultRoute(oc *exutil.CLI, defroute string, ns string) {
	var podmanCLI = container.NewPodmanCLI()
	containerCLI := podmanCLI

	g.By("Trust ca for default registry route on your client platform")
	crt, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("secret", "-n", "openshift-ingress", "router-certs-default", "-o", `go-template={{index .data "tls.crt"}}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	sDec, err := base64.StdEncoding.DecodeString(crt)
	if err != nil {
		e2e.Logf("Error decoding string: %s ", err.Error())
	}
	fileName := "/etc/pki/ca-trust/source/anchors/" + defroute + ".crt"
	sDecode := string(sDec)
	cmd := " echo \"" + sDecode + "\"|sudo tee " + fileName + "> /dev/null"
	_, err = exec.Command("bash", "-c", cmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	caCmd := "sudo update-ca-trust enable"
	_, err = exec.Command("bash", "-c", caCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Get admin permission to push image to your project")
	err = oc.Run("create").Args("serviceaccount", "registry", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WithoutNamespace().AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "admin", "-z", "registry", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Login to route")
	token, err := getSAToken(oc, "registry", ns)
	o.Expect(err).NotTo(o.HaveOccurred())
	if output, err := containerCLI.Run("login").Args(defroute, "-u", "registry", "-p", token).Output(); err != nil {
		e2e.Logf(output)
	}
}

func recoverRegistryDefaultPods(oc *exutil.CLI) {
	platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	switch platformtype {
	case "AWS", "Azure", "GCP", "OpenStack":
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.Equal("2"))
		err = wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 2 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					e2e.Logf("Continue to next round")
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 2")
	case "None", "VSphere":
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).Should(o.Equal("1"))
		err = wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 1 {
				e2e.Logf("Continue to next round")
				return false, nil
			} else if podList.Items[0].Status.Phase != corev1.PodRunning {
				e2e.Logf("Continue to next round")
				return false, nil
			} else {
				return true, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 1")
	default:
		e2e.Failf("Failed to recover registry pods")
	}
}

func checkRegistrypodsRemoved(oc *exutil.CLI) {
	err := wait.Poll(25*time.Second, 3*time.Minute, func() (bool, error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(podList.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Image registry pods are not removed")
}

type staSource struct {
	name      string
	namespace string
	template  string
}

func (stafulsrc *staSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", stafulsrc.template, "-p", "NAME="+stafulsrc.name, "NAMESPACE="+stafulsrc.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkPodsRunningWithLabel(oc *exutil.CLI, namespace string, label string, number int) {
	err := wait.Poll(20*time.Second, 3*time.Minute, func() (bool, error) {
		podList, _ := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: label})
		if len(podList.Items) != number {
			e2e.Logf("the pod number is not %d, Continue to next round", number)
			return false, nil
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning {
				e2e.Logf("Continue to next round")
				return false, nil
			}
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("pods list are not %d", number))
}

type icspSource struct {
	name     string
	template string
}

func (icspsrc *icspSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", icspsrc.template, "-p", "NAME="+icspsrc.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (icspsrc *icspSource) delete(oc *exutil.CLI) {
	e2e.Logf("deleting icsp: %s", icspsrc.name)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagecontentsourcepolicy", icspsrc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRegistryDefaultRoute(oc *exutil.CLI) (defaultroute string) {
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		defroute, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route", "-n", "openshift-image-registry", "default-route", "-o=jsonpath={.spec.host}").Output()
		if len(defroute) == 0 || err != nil {
			e2e.Logf("Continue to next round")
			return false, nil
		}
		defaultroute = defroute
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "Did not find registry route")
	return defaultroute
}

func setImageregistryConfigs(oc *exutil.CLI, pathinfo string, matchlogs string) bool {
	foundInfo := false
	defer recoverRegistrySwiftSet(oc)
	err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{`+pathinfo+`}}}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("co/image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, matchlogs) {
			foundInfo = true
			return true, nil
		}
		e2e.Logf("Continue to next round")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "No image registry error info found")
	return foundInfo
}

func recoverRegistrySwiftSet(oc *exutil.CLI) {
	matchInfo := "True False False"
	err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{"authURL":null, "regionName":null, "regionID":null, "domainID":null, "domain":null, "tenantID":null}}}}`, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(4*time.Second, 20*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/image-registry", "-o=jsonpath={.status.conditions[*].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, matchInfo) {
			return true, nil
		}
		e2e.Logf("Continue to next round")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Image registry is degrade")
}

type podSource struct {
	name      string
	namespace string
	image     string
	template  string
}

func (podsrc *podSource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podsrc.template, "-p", "NAME="+podsrc.name, "NAMESPACE="+podsrc.namespace, "IMAGE="+podsrc.image)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkRegistryUsingFSVolume(oc *exutil.CLI) bool {
	storageinfo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("config.image", "cluster", "-o=jsonpath={.spec.storage}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(storageinfo, "pvc") || strings.Contains(storageinfo, "emptyDir") {
		return true
	}
	return false
}

func saveImageMetadataName(oc *exutil.CLI, image string) string {
	imagemetadata, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("images").OutputToFile("imagemetadata.txt")
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.Remove("imagemetadata.txt")
	manifest, err := exec.Command("bash", "-c", "cat "+imagemetadata+" | grep "+image+" | awk '{print $1}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.TrimSuffix(string(manifest), "\n")
}

func checkRegistryFunctionFine(oc *exutil.CLI, bcname string, namespace string) {
	//Check if could push images to image registry
	err := oc.AsAdmin().WithoutNamespace().Run("new-build").Args("-D", "FROM quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", "--to="+bcname, "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(namespace), bcname+"-1", nil, nil, nil)
	if err != nil {
		exutil.DumpBuildLogs(bcname, oc)
	}
	exutil.AssertWaitPollNoErr(err, "build is not complete")
	err = exutil.WaitForAnImageStreamTag(oc, namespace, bcname, "latest")
	o.Expect(err).NotTo(o.HaveOccurred())

	//Check if could pull images from image registry
	imagename := "image-registry.openshift-image-registry.svc:5000/" + namespace + "/" + bcname + ":latest"
	err = oc.AsAdmin().WithoutNamespace().Run("run").Args(bcname, "--image", imagename, "-n", namespace, "--command", "--", "/bin/sleep", "120").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", bcname, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, `Successfully pulled image "image-registry.openshift-image-registry.svc:5000`) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Image registry is broken, can't pull image")
}

func checkRegistryDegraded(oc *exutil.CLI) bool {
	status := "TrueFalseFalse"
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co/image-registry", "-o=jsonpath={.status.conditions[?(@.type==\"Available\")].status}{.status.conditions[?(@.type==\"Progressing\")].status}{.status.conditions[?(@.type==\"Degraded\")].status}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(output, status) {
		return false
	}
	return true
}

func getCreditFromCluster(oc *exutil.CLI) (string, string, string) {
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIDBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).Str, gjson.Get(credential, `data.aws_secret_access_key`).Str
	accessKeyID, err1 := base64.StdEncoding.DecodeString(accessKeyIDBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	clusterRegion, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	return string(accessKeyID), string(secureKey), string(clusterRegion)
}

func getAWSClient(oc *exutil.CLI) *s3.Client {
	accessKeyID, secureKey, clusterRegion := getCreditFromCluster(oc)
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secureKey, "")),
		config.WithRegion(clusterRegion))

	o.Expect(err).NotTo(o.HaveOccurred())
	return s3.NewFromConfig(cfg)
}

func awsGetBucketTagging(client *s3.Client, bucket string) (string, error) {
	tagOutput, err := client.GetBucketTagging(context.TODO(), &s3.GetBucketTaggingInput{Bucket: &bucket})
	if err != nil {
		outputGetTag := fmt.Sprintf("Got an error GetBucketTagging for %s: %v", bucket, err)
		return outputGetTag, err
	}
	outputGetTag := ""
	for _, t := range tagOutput.TagSet {
		outputGetTag += *t.Key + " " + *t.Value + "\n"
	}
	return outputGetTag, nil
}

//the method is to make newCheck object.
//the method paramter is expect, it will check something is expceted or not
//the method paramter is present, it will check something exists or not
//the executor is asAdmin, it will exectue oc with Admin
//the executor is asUser, it will exectue oc with User
//the inlineNamespace is withoutNamespace, it will execute oc with WithoutNamespace()
//the inlineNamespace is withNamespace, it will execute oc with WithNamespace()
//the expectAction take effective when method is expect, if it is contain, it will check if the strings contain substring with expectContent parameter
//                                                       if it is compare, it will check the strings is samme with expectContent parameter
//the expectContent is the content we expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
//the expect is ok, resource existing is OK for method == present, no error raise. if resource not existing, error raise
//the expect is nok, resource not existing is OK for method == present, no error raise. if resource existing, error raise
func newCheck(method string, executor bool, inlineNamespace bool, expectAction bool,
	expectContent string, expect bool, resource []string) checkDescription {
	return checkDescription{
		method:          method,
		executor:        executor,
		inlineNamespace: inlineNamespace,
		expectAction:    expectAction,
		expectContent:   expectContent,
		expect:          expect,
		resource:        resource,
	}
}

type checkDescription struct {
	method          string
	executor        bool
	inlineNamespace bool
	expectAction    bool
	expectContent   string
	expect          bool
	resource        []string
}

//the method is to check the resource per definition of the above described newCheck.
func (ck checkDescription) check(oc *exutil.CLI) {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		o.Expect(ok).To(o.BeTrue())
	case "expect":
		err := expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("expected content %s not found by %v", ck.expectContent, ck.resource))
	default:
		err := fmt.Errorf("unknown method")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//the method is to check the presence of the resource
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//present means if you expect the resource presence or not. if it is ok, expect presence. if it is nok, expect not present.
func isPresentResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, present bool, parameters ...string) bool {
	parameters = append(parameters, "--ignore-not-found")
	err := wait.Poll(3*time.Second, 70*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		if !present && strings.Compare(output, "") == 0 {
			return true, nil
		}
		if present && strings.Compare(output, "") != 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}
	return true
}

//the method is to check one resource's attribution is expected or not.
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//isCompare means if containing or exactly comparing. if it is contain, it check result contain content. if it is compare, it compare the result with content exactly.
//content is the substing to be expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
func expectedResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, isCompare bool, content string, expect bool, parameters ...string) error {
	expectMap := map[bool]string{
		true:  "do",
		false: "do not",
	}

	cc := func(a, b string, ic bool) bool {
		bs := strings.Split(b, "+2+")
		ret := false
		for _, s := range bs {
			if (ic && strings.Compare(a, s) == 0) || (!ic && strings.Contains(a, s)) {
				ret = true
			}
		}
		return ret
	}
	e2e.Logf("Running: oc get asAdmin(%t) withoutNamespace(%t) %s", asAdmin, withoutNamespace, strings.Join(parameters, " "))
	return wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("---> we %v expect value: %s, in returned value: %s", expectMap[expect], content, output)
		if isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s matches one of the content %s, expected", output, content)
			return true, nil
		}
		if isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not matche the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s contains one of the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not contain the content %s, expected", output, content)
			return true, nil
		}
		e2e.Logf("---> Not as expected! Return false")
		return false, nil
	})
}
func exposeService(oc *exutil.CLI, ns, resource, name, port string) {
	err := oc.WithoutNamespace().Run("expose").Args(resource, "--name="+name, "--port="+port, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func exposeEdgeRoute(oc *exutil.CLI, ns, route, service string) string {
	err := oc.WithoutNamespace().Run("create").Args("route", "edge", route, "--service="+service, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	regRoute, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", route, "-n", ns, "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return regRoute
}

func listRepositories(oc *exutil.CLI, regRoute, expect string) {
	curlCmd := fmt.Sprintf("curl -k  https://%s/v2/_catalog | grep %s", regRoute, expect)
	result, err := exec.Command("bash", "-c", curlCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(string(result)).To(o.ContainSubstring(expect))
}

func setSecureRegistryWithoutAuth(oc *exutil.CLI, ns, regName, image, port string) string {
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("deploy", regName, "--image="+image, "--port=5000", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	checkPodsRunningWithLabel(oc, ns, "app="+regName, 1)
	exposeService(oc, ns, "deploy/"+regName, regName, port)
	regRoute := exposeEdgeRoute(oc, ns, regName, regName)
	listRepositories(oc, regRoute, "repositories")
	return regRoute
}

func setSecureRegistryEnableAuth(oc *exutil.CLI, ns, regName, htpasswdFile, image string) string {
	regRoute := setSecureRegistryWithoutAuth(oc, ns, regName, image, "5000")
	ge1 := saveGeneration(oc, ns, "deployment/"+regName)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", "htpasswd", "--from-file="+htpasswdFile, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WithoutNamespace().Run("set").Args("volume", "deployment/"+regName, "--add", "--mount-path=/auth", "--type=secret", "--secret-name=htpasswd", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WithoutNamespace().Run("set").Args("env", "deployment/"+regName, "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd", "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "REGISTRY_AUTH=htpasswd", "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		ge2 := saveGeneration(oc, ns, "deployment/"+regName)
		if ge2 == ge1 {
			e2e.Logf("Continue to next round")
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "Custom registry does not update")
	newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", "-n", ns, "-l", "app=" + regName}).check(oc)
	return regRoute
}

func generateHtpasswdFile(tempDataDir, user, pass string) (string, error) {
	htpasswdFile := filepath.Join(tempDataDir, "htpasswd")
	generateCMD := fmt.Sprintf("htpasswd -Bbn %s %s > %s", user, pass, htpasswdFile)
	_, err := exec.Command("bash", "-c", generateCMD).Output()
	if err != nil {
		e2e.Logf("Fail to generate htpasswd file: %v", err)
		return htpasswdFile, err
	}
	return htpasswdFile, nil
}

func extractPullSecret(oc *exutil.CLI) (string, error) {
	tempDataDir := filepath.Join("/tmp/", fmt.Sprintf("ir-%s", getRandomString()))
	err := os.Mkdir(tempDataDir, 0755)
	if err != nil {
		e2e.Logf("Fail to create directory: %v", err)
		return tempDataDir, err
	}
	err = oc.AsAdmin().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--confirm", "--to="+tempDataDir).Execute()
	if err != nil {
		e2e.Logf("Fail to extract dockerconfig: %v", err)
		return tempDataDir, err
	}
	return tempDataDir, nil
}

func appendPullSecretAuth(authFile, regRouter, newRegUser, newRegPass string) (string, error) {
	fieldValue := newRegUser + ":" + newRegPass
	regToken := base64.StdEncoding.EncodeToString([]byte(fieldValue))
	authDir, _ := filepath.Split(authFile)
	newAuthFile := filepath.Join(authDir, fmt.Sprintf("%s.json", getRandomString()))
	jqCMD := fmt.Sprintf(`cat %s | jq '.auths += {"%s":{"auth":"%s"}}' > %s`, authFile, regRouter, regToken, newAuthFile)
	_, err := exec.Command("bash", "-c", jqCMD).Output()
	if err != nil {
		e2e.Logf("Fail to extract dockerconfig: %v", err)
		return newAuthFile, err
	}
	return newAuthFile, nil
}

func updatePullSecret(oc *exutil.CLI, authFile string) {
	err := oc.AsAdmin().WithoutNamespace().Run("set").Args("data", "secret/pull-secret", "-n", "openshift-config", "--from-file=.dockerconfigjson="+authFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func foundAffinityRules(oc *exutil.CLI, affinityRules string) bool {
	podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
	for _, pod := range podList.Items {
		out, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("pod/"+pod.Name, "-n", pod.Namespace, "-o=jsonpath={.spec.affinity.podAntiAffinity}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(out, affinityRules) {
			return false
			break
		}
	}
	return true
}

func saveGlobalProxy(oc *exutil.CLI) (string, string, string) {
	httpProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
	httpsProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
	noProxy := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.noProxy}")
	return httpProxy, httpsProxy, noProxy
}

func createSimpleRunPod(oc *exutil.CLI, image, expectInfo string) {
	podName := getRandomString()
	err := oc.AsAdmin().WithoutNamespace().Run("run").Args(podName, "--image="+image, "-n", oc.Namespace(), "--image-pull-policy=Always", "--", "sleep", "300").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(3*time.Second, 2*time.Minute, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", podName, "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, expectInfo) {
			return true, nil
		}
		e2e.Logf("Continue to next round")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Pod doesn't show expected log %v", expectInfo))
}

func newAppUseImageStream(oc *exutil.CLI, ns, imagestream, expectInfo string) {
	appName := getRandomString()
	err := oc.AsAdmin().WithoutNamespace().Run("new-app").Args("-i", imagestream, "--name="+appName, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", "-l", "deployment="+appName, "-n", ns).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(output, expectInfo) {
			return true, nil
		}
		e2e.Logf("Continue to next round")
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "Pod doesn't pull expected image")
}

//Save deployment or daemonset generation to judge if update applied
func saveGeneration(oc *exutil.CLI, ns, resource string) string {
	num, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(resource, "-n", ns, "-o=jsonpath={.metadata.generation}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return num
}

//Create route to expose the registry
func createRouteExposeRegistry(oc *exutil.CLI) {
	//Don't forget to restore the environment use func restoreRouteExposeRegistry
	output, err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":true}}`, "--type=merge").Output()
	if err != nil {
		e2e.Logf(output)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("patched"))
}

func restoreRouteExposeRegistry(oc *exutil.CLI) {
	output, err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":false}}`, "--type=merge").Output()
	if err != nil {
		e2e.Logf(output)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("patched"))
}

func getPodNodeListByLabel(oc *exutil.CLI, namespace string, labelKey string) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-o", "wide", "-n", namespace, "-l", labelKey, "-o=jsonpath={.items[*].spec.nodeName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNameList := strings.Fields(output)
	return nodeNameList
}

func getImageRegistryPodNumber(oc *exutil.CLI) int {
	podNum, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.spec.replicas}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	intPodNum, _ := strconv.Atoi(podNum)
	return intPodNum
}

func saveImageRegistryAuth(oc *exutil.CLI, regRoute, ns string) (string, error) {
	tempDataFile := filepath.Join("/tmp/", fmt.Sprintf("ir-auth-%s", getRandomString()))
	err := oc.AsAdmin().WithoutNamespace().Run("registry").Args("login", "--registry="+regRoute, "-z", "builder", "--to="+tempDataFile, "--insecure", "-n", ns).Execute()
	if err != nil {
		e2e.Logf("Fail to login image registry: %v", err)
		return tempDataFile, err
	}
	return tempDataFile, nil
}

func getSAToken(oc *exutil.CLI, sa, ns string) (string, error) {
	e2e.Logf("Getting a token assgined to specific serviceaccount from %s namespace...", ns)
	token, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("token", sa, "-n", ns).Output()
	if err != nil {
		if strings.Contains(token, "unknown command") { // oc client is old version, create token is not supported
			e2e.Logf("oc create token is not supported by current client, use oc sa get-token instead")
			token, err = oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", sa, "-n", ns).Output()
		} else {
			return "", err
		}
	}

	return token, err
}

type machineConfig struct {
	name       string
	pool       string
	source     string
	path       string
	template   string
	parameters []string
}

func (mc *machineConfig) waitForMCPComplete(oc *exutil.CLI) {
	machineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mc.pool, "-ojsonpath={.status.machineCount}").Output()
	e2e.Logf("machineCount: %v", machineCount)
	o.Expect(err).NotTo(o.HaveOccurred())
	mcCount, _ := strconv.Atoi(machineCount)
	timeToWait := time.Duration(10*mcCount) * time.Minute
	e2e.Logf("Waiting %s for MCP %s to be completed.", timeToWait, mc.pool)
	err = wait.Poll(1*time.Minute, timeToWait, func() (bool, error) {
		mcpStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mc.pool, `-ojsonpath={.status.conditions[?(@.type=="Updated")].status}`).Output()
		e2e.Logf("mcpStatus: %v", mcpStatus)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(mcpStatus, "True") {
			// i.e. mcp updated=true, mc is applied successfully
			e2e.Logf("mc operation is completed on mcp %s", mc.pool)
			return true, nil
		}
		return false, nil
	})

	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("mc operation is not completed on mcp %s", mc.pool))

}

func (mc *machineConfig) createWithCheck(oc *exutil.CLI) {
	mc.name = mc.name + "-" + exutil.GetRandomString()
	params := []string{"--ignore-unknown-parameters=true", "-f", mc.template, "-p", "NAME=" + mc.name, "POOL=" + mc.pool, "SOURCE=" + mc.source, "PATH=" + mc.path}
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

	mc.waitForMCPComplete(oc)

}

func (mc *machineConfig) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	mc.waitForMCPComplete(oc)
}

type runtimeClass struct {
	name     string
	handler  string
	template string
}

func (rtc *runtimeClass) createWithCheck(oc *exutil.CLI) {
	rtc.name = rtc.name + "-" + exutil.GetRandomString()
	params := []string{"--ignore-unknown-parameters=true", "-f", rtc.template, "-p", "NAME=" + rtc.name, "HANDLER=" + rtc.handler}
	exutil.CreateClusterResourceFromTemplate(oc, params...)

	rtcerr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("runtimeclass/"+rtc.name, "-o", "jsonpath='{.metadata.name}'").Output()
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, rtc.name) {
			e2e.Logf("runtimeClass %s is created successfully", rtc.name)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(rtcerr, fmt.Sprintf("create runtimeClass %v failed", rtc.name))

}

func (rtc *runtimeClass) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("runtimeclass", rtc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

type prometheusImageregistryOperations struct {
	Data struct {
		Result []struct {
			Metric []struct {
				Name      string `json:"__name__"`
				Operation string `json:"operation"`
				Resource  string `json:"resource_type"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

type prometheusImageregistryStorageType struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name      string `json:"__name__"`
				Container string `json:"container"`
				Endpoint  string `json:"endpoint"`
				Instance  string `json:"instance"`
				Job       string `json:"job"`
				Namespace string `json:"namespace"`
				Pod       string `json:"pod"`
				Service   string `json:"service"`
				Storage   string `json:"storage"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}
