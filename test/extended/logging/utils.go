package logging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/tidwall/gjson"
)

/*
// TBD: a common receiver interface
type logReceiver interface {
	infrastructureLogsFound() bool
	auditLogsFound() bool
	applicationLogsFound() bool
	logsFound() bool
}
*/

// SubscriptionObjects objects are used to create operators via OLM
type SubscriptionObjects struct {
	OperatorName  string
	Namespace     string
	OperatorGroup string // the file used to create operator group
	Subscription  string // the file used to create subscription
	PackageName   string
	CatalogSource CatalogSourceObjects `json:",omitempty"`
}

// CatalogSourceObjects defines the source used to subscribe an operator
type CatalogSourceObjects struct {
	Channel         string `json:",omitempty"`
	SourceName      string `json:",omitempty"`
	SourceNamespace string `json:",omitempty"`
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

// contain checks if the array a contains string b
func contain(a []string, b string) bool {
	for _, c := range a {
		if c == b {
			return true
		}
	}
	return false
}

func processTemplate(oc *exutil.CLI, parameters ...string) (string, error) {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + ".json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	return configFile, err
}

func (so *SubscriptionObjects) getChannelName(oc *exutil.CLI) string {
	var channelName string
	if so.CatalogSource.Channel != "" {
		channelName = so.CatalogSource.Channel
	} else {
		/*
			clusterVersion, err := oc.AsAdmin().AdminConfigClient().ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			e2e.Logf("clusterversion is: %v\n", clusterVersion.Status.Desired.Version)
			channelName = strings.Join(strings.Split(clusterVersion.Status.Desired.Version, ".")[0:2], ".")
		*/
		channelName = "stable"
	}
	e2e.Logf("the channel name is: %v\n", channelName)
	return channelName
}

func (so *SubscriptionObjects) getSourceNamespace(oc *exutil.CLI) string {
	var catsrcNamespaceName string
	if so.CatalogSource.SourceNamespace != "" {
		catsrcNamespaceName = so.CatalogSource.SourceNamespace
	} else {
		catsrcNamespaceName = "openshift-marketplace"
	}
	e2e.Logf("The source namespace name is: %v\n", catsrcNamespaceName)
	return catsrcNamespaceName
}

func (so *SubscriptionObjects) getCatalogSourceName(oc *exutil.CLI) string {
	var catsrcName, catsrcNamespaceName string
	catsrcNamespaceName = so.getSourceNamespace(oc)
	catsrc, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("catsrc", "-n", catsrcNamespaceName, "qe-app-registry").Output()

	err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", so.PackageName).Execute()
	if err != nil {
		e2e.Logf("Can't check the packagemanifest %s existence: %v", so.PackageName, err)
	}
	if so.CatalogSource.SourceName != "" {
		catsrcName = so.CatalogSource.SourceName
	} else if catsrc != "" && !(strings.Contains(catsrc, "NotFound")) {
		catsrcName = "qe-app-registry"
	} else {
		catsrcName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", so.PackageName, "-o", "jsonpath={.status.catalogSource}").Output()
		if err != nil {
			e2e.Logf("error getting catalog source name: %v", err)
		}
	}
	e2e.Logf("The catalog source name of %s is: %v\n", so.PackageName, catsrcName)
	return catsrcName
}

// SubscribeOperator is used to subcribe the CLO and EO
func (so *SubscriptionObjects) SubscribeOperator(oc *exutil.CLI) {
	// check if the namespace exists, if it doesn't exist, create the namespace
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(so.Namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			e2e.Logf("The project %s is not found, create it now...", so.Namespace)
			namespaceTemplate := exutil.FixturePath("testdata", "logging", "subscription", "namespace.yaml")
			namespaceFile, err := processTemplate(oc, "-f", namespaceTemplate, "-p", "NAMESPACE_NAME="+so.Namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
				output, err := oc.AsAdmin().Run("apply").Args("-f", namespaceFile).Output()
				if err != nil {
					if strings.Contains(output, "AlreadyExists") {
						return true, nil
					}
					return false, err
				}
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create project %s", so.Namespace))
		}
	}

	// check the operator group, if no object found, then create an operator group in the project
	og, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", so.Namespace, "og").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	msg := fmt.Sprintf("%v", og)
	if strings.Contains(msg, "No resources found") {
		// create operator group
		ogFile, err := processTemplate(oc, "-n", so.Namespace, "-f", so.OperatorGroup, "-p", "OG_NAME="+so.Namespace, "NAMESPACE="+so.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
			output, err := oc.AsAdmin().Run("apply").Args("-f", ogFile, "-n", so.Namespace).Output()
			if err != nil {
				if strings.Contains(output, "AlreadyExists") {
					return true, nil
				}
				return false, err
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create operatorgroup %s in %s project", so.Namespace, so.Namespace))
	}

	// check subscription, if there is no subscription objets, then create one
	sub, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", so.Namespace, so.PackageName).Output()
	if err != nil {
		msg := fmt.Sprint("v%", sub)
		if strings.Contains(msg, "NotFound") {
			catsrcNamespaceName := so.getSourceNamespace(oc)
			catsrcName := so.getCatalogSourceName(oc)
			channelName := so.getChannelName(oc)
			//check if the packagemanifest is exists in the source namespace or not
			packages, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", catsrcNamespaceName, "packagemanifests", "-l", "catalog="+catsrcName, "-o", "name").Output()
			o.Expect(packages).Should(o.ContainSubstring(so.PackageName))
			//create subscription object
			subscriptionFile, err := processTemplate(oc, "-n", so.Namespace, "-f", so.Subscription, "-p", "PACKAGE_NAME="+so.PackageName, "NAMESPACE="+so.Namespace, "CHANNEL="+channelName, "SOURCE="+catsrcName, "SOURCE_NAMESPACE="+catsrcNamespaceName)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
				output, err := oc.AsAdmin().Run("apply").Args("-f", subscriptionFile, "-n", so.Namespace).Output()
				if err != nil {
					if strings.Contains(output, "AlreadyExists") {
						return true, nil
					}
					return false, err
				}
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can't create subscription %s in %s project", so.PackageName, so.Namespace))
		}
	}
	//WaitForDeploymentPodsToBeReady(oc, so.Namespace, so.OperatorName)
	waitForPodReadyWithLabel(oc, so.Namespace, "name="+so.OperatorName)
}

func (so *SubscriptionObjects) uninstallOperator(oc *exutil.CLI) {
	resource{"subscription", so.PackageName, so.Namespace}.clear(oc)
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", so.Namespace, "csv", "--all").Execute()
	// do not remove namespace openshift-logging and openshift-operators-redhat, and preserve the operatorgroup as there may have several operators deployed in one namespace
	// for example: loki-operator and elasticsearch-operator
	if so.Namespace != "openshift-logging" && so.Namespace != "openshift-operators-redhat" && !strings.HasPrefix(so.Namespace, "e2e-test-") {
		deleteNamespace(oc, so.Namespace)
	}
}

func (so *SubscriptionObjects) getInstalledCSV(oc *exutil.CLI) string {
	installedCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", so.Namespace, "sub", so.PackageName, "-ojsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return installedCSV
}

//WaitForDeploymentPodsToBeReady waits for the specific deployment to be ready
func WaitForDeploymentPodsToBeReady(oc *exutil.CLI, namespace string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment/%s\n", name)
				return false, nil
			}
			return false, err
		}
		if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas && deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas {
			e2e.Logf("Deployment %s available (%d/%d)\n", name, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas)
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s deployment (%d/%d)\n", name, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment %s is not availabile", name))
}

func waitForStatefulsetReady(oc *exutil.CLI, namespace string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		ss, err := oc.AdminKubeClient().AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s statefulset\n", name)
				return false, nil
			}
			return false, err
		}
		if ss.Status.ReadyReplicas == *ss.Spec.Replicas && ss.Status.UpdatedReplicas == *ss.Spec.Replicas {
			e2e.Logf("statefulset %s available (%d/%d)\n", name, ss.Status.ReadyReplicas, *ss.Spec.Replicas)
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s statefulset (%d/%d)\n", name, ss.Status.ReadyReplicas, *ss.Spec.Replicas)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("statefulset %s is not availabile", name))
}

//WaitForDaemonsetPodsToBeReady waits for all the pods controlled by the ds to be ready
func WaitForDaemonsetPodsToBeReady(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of daemonset/%s\n", name)
				return false, nil
			}
			return false, err
		}
		if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled && daemonset.Status.UpdatedNumberScheduled == daemonset.Status.DesiredNumberScheduled {
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s daemonset (%d/%d)\n", name, daemonset.Status.NumberReady, daemonset.Status.DesiredNumberScheduled)
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Daemonset %s is not availabile", name))
	e2e.Logf("Daemonset %s is available\n", name)
}

func waitForPodReadyWithLabel(oc *exutil.CLI, ns string, label string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			return false, err
		}
		if len(pods.Items) == 0 {
			e2e.Logf("Waiting for pod with label %s to appear\n", label)
			return false, nil
		}
		ready := true
		for _, pod := range pods.Items {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					ready = false
					break
				}
			}
		}
		if !ready {
			e2e.Logf("Waiting for pod with label %s to be ready...\n", label)
		}
		return ready, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The pod with label %s is not availabile", label))
}

//GetDeploymentsNameByLabel retruns a list of deployment name which have specific labels
func GetDeploymentsNameByLabel(oc *exutil.CLI, ns string, label string) []string {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment\n")
				return false, nil
			}
			return false, err
		}
		if len(deployList.Items) > 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("deployment with label %s is not availabile", label))
	if err == nil {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedDeployments := make([]string, 0, len(deployList.Items))
		for _, deploy := range deployList.Items {
			expectedDeployments = append(expectedDeployments, deploy.Name)
		}
		return expectedDeployments
	}
	return nil
}

//WaitForECKPodsToBeReady checks if the EFK pods could become ready or not
func WaitForECKPodsToBeReady(oc *exutil.CLI, ns string) {
	//wait for ES
	esDeployNames := GetDeploymentsNameByLabel(oc, ns, "cluster-name=elasticsearch")
	for _, name := range esDeployNames {
		WaitForDeploymentPodsToBeReady(oc, ns, name)
	}
	// wait for Kibana
	WaitForDeploymentPodsToBeReady(oc, ns, "kibana")
	// wait for collector
	WaitForDaemonsetPodsToBeReady(oc, ns, "collector")
}

type resource struct {
	kind      string
	name      string
	namespace string
}

//WaitUntilResourceIsGone waits for the resource to be removed cluster
func (r resource) WaitUntilResourceIsGone(oc *exutil.CLI) error {
	return wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			errstring := fmt.Sprintf("%v", output)
			if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
}

//delete the objects in the cluster
func (r resource) clear(oc *exutil.CLI) error {
	msg, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, r.kind, r.name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
			return nil
		}
		return err
	}
	err = r.WaitUntilResourceIsGone(oc)
	return err
}

func (r resource) WaitForResourceToAppear(oc *exutil.CLI) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", r.namespace, r.kind, r.name).Output()
		if err != nil {
			msg := fmt.Sprintf("%v", output)
			if strings.Contains(msg, "NotFound") {
				return false, nil
			}
			return false, err
		}
		e2e.Logf("Find %s %s", r.kind, r.name)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("resource %s/%s is not appear", r.kind, r.name))
}

func (r resource) applyFromTemplate(oc *exutil.CLI, parameters ...string) error {
	parameters = append(parameters, "-n", r.namespace)
	file, err := processTemplate(oc, parameters...)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Can not process %v", parameters))
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", file, "-n", r.namespace).Execute()
	r.WaitForResourceToAppear(oc)
	return err
}

//DeleteClusterLogging deletes the clusterlogging instance and ensures the related resources are removed
func (r resource) deleteClusterLogging(oc *exutil.CLI) {
	err := r.clear(oc)
	if err != nil {
		e2e.Logf("could not delete %s/%s", r.kind, r.name)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("could not delete %s/%s", r.kind, r.name))
	//make sure other resources are removed
	resources := []resource{{"elasticsearches.logging.openshift.io", "elasticsearch", r.namespace}, {"kibanas.logging.openshift.io", "kibana", r.namespace}, {"daemonset", "collector", r.namespace}}
	for i := 0; i < len(resources); i++ {
		err = resources[i].WaitUntilResourceIsGone(oc)
		if err != nil {
			e2e.Logf("%s/%s is not deleted", resources[i].kind, resources[i].name)
		}
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s/%s is not deleted", resources[i].kind, resources[i].name))
	}
	// remove all the pvcs in the namespace
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", r.namespace, "pvc", "-l", "logging-cluster=elasticsearch").Execute()
}

func (r resource) createClusterLogging(oc *exutil.CLI, parameters ...string) {
	// delete clusterlogging instance first
	r.deleteClusterLogging(oc)
	err := r.applyFromTemplate(oc, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteNamespace(oc *exutil.CLI, ns string) {
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = nil
		}
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Namespace %s is not deleted in 3 minutes", ns))
}

// WaitForIMCronJobToAppear checks if the cronjob exists or not
func WaitForIMCronJobToAppear(oc *exutil.CLI, ns string, name string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().BatchV1beta1().CronJobs(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of cronjob\n")
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cronjob %s is not availabile", name))
}

func waitForIMJobsToComplete(oc *exutil.CLI, ns string, timeout time.Duration) {
	// wait for jobs to appear
	err := wait.Poll(5*time.Second, timeout, func() (done bool, err error) {
		jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of jobs\n")
				return false, nil
			}
			return false, err
		}
		if len(jobList.Items) > 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("jobs with label %s are not exist", "component=indexManagement"))
	// wait for jobs to complete
	jobList, err := oc.AdminKubeClient().BatchV1().Jobs(ns).List(metav1.ListOptions{LabelSelector: "component=indexManagement"})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, job := range jobList.Items {
		err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			job, err := oc.AdminKubeClient().BatchV1().Jobs(ns).Get(job.Name, metav1.GetOptions{})
			//succeeded, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "job", job.Name, "-o=jsonpath={.status.succeeded}").Output()
			if err != nil {
				return false, err
			}
			if job.Status.Succeeded == 1 {
				e2e.Logf("job %s completed successfully", job.Name)
				return true, nil
			}
			e2e.Logf("job %s is not completed yet", job.Name)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("job %s is not completed yet", job.Name))
	}
}

func getStorageClassName(oc *exutil.CLI) (string, error) {
	var scName string
	defaultSC := ""
	SCs, err := oc.AdminKubeClient().StorageV1().StorageClasses().List(metav1.ListOptions{})
	for _, sc := range SCs.Items {
		if sc.ObjectMeta.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultSC = sc.Name
			break
		}
	}
	if defaultSC != "" {
		scName = defaultSC
	} else {
		scName = SCs.Items[0].Name
	}
	return scName, err
}

//Assert the status of a resource
func (r resource) assertResourceStatus(oc *exutil.CLI, content string, exptdStatus string) {
	err := wait.Poll(10*time.Second, 180*time.Second, func() (done bool, err error) {
		clStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(r.kind, r.name, "-n", r.namespace, "-o", content).Output()
		if err != nil {
			return false, err
		}
		if strings.Compare(clStatus, exptdStatus) != 0 {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s %s value for %s is not %s", r.kind, r.name, content, exptdStatus))
}

func getRouteAddress(oc *exutil.CLI, ns, routeName string) string {
	route, err := oc.AdminRouteClient().RouteV1().Routes(ns).Get(routeName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return route.Spec.Host
}

func getSAToken(oc *exutil.CLI, name, ns string) string {
	secrets, err := oc.AdminKubeClient().CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return ""
	}
	var secret string
	for _, s := range secrets.Items {
		if strings.Contains(s.Name, name+"-token") {
			secret = s.Name
			break
		}
	}
	dirname := "/tmp/" + oc.Namespace() + "-sa"
	defer os.RemoveAll(dirname)
	err = os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/"+secret, "-n", ns, "--confirm", "--to="+dirname).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	bearerToken, err := os.ReadFile(dirname + "/token")
	o.Expect(err).NotTo(o.HaveOccurred())
	return string(bearerToken)
}

type prometheusQueryResult struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name              string `json:"__name__"`
				Cluster           string `json:"cluster,omitempty"`
				Container         string `json:"container,omitempty"`
				ContainerName     string `json:"containername,omitempty"`
				Endpoint          string `json:"endpoint,omitempty"`
				Instance          string `json:"instance,omitempty"`
				Job               string `json:"job,omitempty"`
				Namespace         string `json:"namespace,omitempty"`
				Path              string `json:"path,omitempty"`
				Pod               string `json:"pod,omitempty"`
				PodName           string `json:"podname,omitempty"`
				Service           string `json:"service,omitempty"`
				ExportedNamespace string `json:"exported_namespace,omitempty"`
				State             string `json:"state,omitempty"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

// queryPrometheus returns the promtheus metrics which match the query string
// token: the user token used to run the http request, if it's not specified, it will use the token of sa/prometheus-k8s in openshift-monitoring project
// path: the api path, for example: /api/v1/query?
// query: the metric/alert you want to search, e.g.: es_index_namespaces_total
// action: it can be "GET", "get", "Get", "POST", "post", "Post"
func queryPrometheus(oc *exutil.CLI, token string, path string, query string, action string) (prometheusQueryResult, error) {
	var bearerToken string
	var err error
	if token == "" {
		bearerToken = getSAToken(oc, "prometheus-k8s", "openshift-monitoring")
	} else {
		bearerToken = token
	}
	prometheusURL := "https://" + getRouteAddress(oc, "openshift-monitoring", "prometheus-k8s") + path
	if query != "" {
		prometheusURL = prometheusURL + "query=" + url.QueryEscape(query)
	}

	var tr *http.Transport
	if os.Getenv("http_proxy") != "" || os.Getenv("https_proxy") != "" {
		var proxy string
		if os.Getenv("http_proxy") != "" {
			proxy = os.Getenv("http_proxy")
		} else {
			proxy = os.Getenv("https_proxy")
		}
		proxyURL, err := url.Parse(proxy)
		o.Expect(err).NotTo(o.HaveOccurred())
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyURL),
		}
	} else {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client := &http.Client{Transport: tr}
	var request *http.Request
	switch action {
	case "GET", "get", "Get":
		request, err = http.NewRequest("GET", prometheusURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	case "POST", "post", "Post":
		request, err = http.NewRequest("POST", prometheusURL, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	default:
		e2e.Failf("Unrecogonized action: %s", action)
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+bearerToken)
	response, err := client.Do(request)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer response.Body.Close()
	responseData, err := ioutil.ReadAll(response.Body)
	res := prometheusQueryResult{}
	json.Unmarshal(responseData, &res)
	return res, err
}

//WaitUntilPodsAreGone waits for pods selected with labelselector to be removed
func WaitUntilPodsAreGone(oc *exutil.CLI, namespace string, labelSelector string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "--selector="+labelSelector, "-n", namespace).Output()
		if err != nil {
			return false, err
		}
		errstring := fmt.Sprintf("%v", output)
		if strings.Contains(errstring, "No resources found") {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Error waiting for pods to be removed using label selector %s", labelSelector))
}

//Check logs from resource
func (r resource) checkLogsFromRs(oc *exutil.CLI, expected string, containerName string) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(r.kind+`/`+r.name, "-n", r.namespace, "-c", containerName).Output()
		if err != nil {
			e2e.Logf("Can't get logs from resource, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.Match(expected, []byte(output)); !matched {
			e2e.Logf("Can't find the expected string\n")
			return false, nil
		}
		e2e.Logf("Check the logs succeed!!\n")
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s is not expected for %s", expected, r.name))
}

func getCurrentCSVFromPackage(oc *exutil.CLI, channel string, packagemanifest string) string {
	var currentCSV string
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", packagemanifest, "-ojson").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	PM := PackageManifest{}
	json.Unmarshal([]byte(output), &PM)
	for _, channels := range PM.Status.Channels {
		if channels.Name == channel {
			currentCSV = channels.CurrentCSV
			break
		}
	}
	return currentCSV
}

func chkMustGather(oc *exutil.CLI, ns string) {
	cloImg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "deployment.apps/cluster-logging-operator", "-o", "jsonpath={.spec.template.spec.containers[?(@.name == \"cluster-logging-operator\")].image}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The cloImg is: " + cloImg)

	cloPodList, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: "name=cluster-logging-operator"})
	o.Expect(err).NotTo(o.HaveOccurred())
	cloImgID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "pods", cloPodList.Items[0].Name, "-o", "jsonpath={.status.containerStatuses[0].imageID}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The cloImgID is: " + cloImgID)

	mgDest := "must-gather-" + getRandomString()
	baseDir := exutil.FixturePath("testdata", "logging")
	TestDataPath := filepath.Join(baseDir, mgDest)
	defer exec.Command("rm", "-r", TestDataPath).Output()
	err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("-n", ns, "must-gather", "--image="+cloImg, "--dest-dir="+TestDataPath).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	replacer := strings.NewReplacer(".", "-", "/", "-", ":", "-", "@", "-")
	cloImgDir := replacer.Replace(cloImgID)
	checkPath := []string{
		"timestamp",
		"event-filter.html",
		cloImgDir + "/gather-debug.log",
		cloImgDir + "/cluster-scoped-resources",
		cloImgDir + "/namespaces",
		cloImgDir + "/cluster-logging/clo",
		cloImgDir + "/cluster-logging/collector",
		cloImgDir + "/cluster-logging/eo",
		cloImgDir + "/cluster-logging/eo/elasticsearch-operator.logs",
		cloImgDir + "/cluster-logging/es",
		cloImgDir + "/cluster-logging/install",
		cloImgDir + "/cluster-logging/kibana/",
		cloImgDir + "/cluster-logging/clo/clf-events.yaml",
		cloImgDir + "/cluster-logging/clo/clo-events.yaml",
	}

	for _, v := range checkPath {
		pathStat, err := os.Stat(filepath.Join(TestDataPath, v))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pathStat.Size() > 0).To(o.BeTrue(), "The path %s is empty", v)
	}
}

func checkNetworkType(oc *exutil.CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.ToLower(output)
}

type certsConf struct {
	serverName string
	namespace  string
	passPhrase string //client private key passphrase
}

func (certs certsConf) generateCerts(keysPath string) {
	generateCertsSH := exutil.FixturePath("testdata", "logging", "external-log-stores", "cert_generation.sh")
	cmd := []string{generateCertsSH, keysPath, certs.namespace, certs.serverName}
	if certs.passPhrase != "" {
		cmd = append(cmd, certs.passPhrase)
	}
	err := exec.Command("sh", cmd...).Run()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//expect: true means we want the resource contain/compare with the expectedContent, false means the resource is expected not to compare with/contain the expectedContent;
//compare: true means compare the expectedContent with the resource content, false means check if the resource contains the expectedContent;
//args are the arguments used to execute command `oc.AsAdmin.WithoutNamespace().Run("get").Args(args...).Output()`;
func checkResource(oc *exutil.CLI, expect bool, compare bool, expectedContent string, args []string) {
	err := wait.Poll(10*time.Second, 180*time.Second, func() (done bool, err error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
		if err != nil {
			if strings.Contains(output, "NotFound") {
				return false, nil
			}
			return false, err
		}
		if compare {
			res := strings.Compare(output, expectedContent)
			if (res == 0 && expect) || (res != 0 && !expect) {
				return true, nil
			}
			return false, nil
		}
		res := strings.Contains(output, expectedContent)
		if (res && expect) || (!res && !expect) {
			return true, nil
		}
		return false, nil
	})
	if expect {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The content doesn't match/contain %s", expectedContent))
	} else {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The %s still exists in the resource", expectedContent))
	}
}

type rsyslog struct {
	serverName string //the name of the rsyslog server, it's also used to name the svc/cm/sa/secret
	namespace  string //the namespace where the rsyslog server deployed in
	tls        bool
	secretName string //the name of the secret for the collector to use
	loggingNS  string //the namespace where the collector pods deployed in
}

func (r rsyslog) createPipelineSecret(oc *exutil.CLI, keysPath string) {
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", r.secretName, "-n", r.loggingNS, "--from-file=ca-bundle.crt="+keysPath+"/ca.crt").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	resource{"secret", r.secretName, r.loggingNS}.WaitForResourceToAppear(oc)
}

func (r rsyslog) deploy(oc *exutil.CLI) {
	// create SA
	sa := resource{"serviceaccount", r.serverName, r.namespace}
	err := oc.WithoutNamespace().Run("create").Args("serviceaccount", sa.name, "-n", sa.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	sa.WaitForResourceToAppear(oc)
	err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:%s", r.namespace, r.serverName), "-n", r.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	filePath := []string{"testdata", "logging", "external-log-stores", "rsyslog"}
	// create secrets if needed
	if r.tls {
		o.Expect(r.secretName).NotTo(o.BeEmpty())
		// create a temporary directory
		baseDir := exutil.FixturePath("testdata", "logging")
		keysPath := filepath.Join(baseDir, "temp"+getRandomString())
		defer exec.Command("rm", "-r", keysPath).Output()
		err = os.MkdirAll(keysPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		cert := certsConf{r.serverName, r.namespace, ""}
		cert.generateCerts(keysPath)
		// create pipelinesecret
		r.createPipelineSecret(oc, keysPath)
		// create secret for rsyslog server
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", r.serverName, "-n", r.namespace, "--from-file=server.key="+keysPath+"/server.key", "--from-file=server.crt="+keysPath+"/server.crt", "--from-file=ca_bundle.crt="+keysPath+"/ca.crt").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		filePath = append(filePath, "secure")
	} else {
		filePath = append(filePath, "insecure")
	}

	// create configmap/deployment/svc
	cm := resource{"configmap", r.serverName, r.namespace}
	cmFilePath := append(filePath, "configmap.yaml")
	cmFile := exutil.FixturePath(cmFilePath...)
	err = cm.applyFromTemplate(oc, "-f", cmFile, "-n", r.namespace, "-p", "NAMESPACE="+r.namespace, "-p", "NAME="+r.serverName)
	o.Expect(err).NotTo(o.HaveOccurred())

	deploy := resource{"deployment", r.serverName, r.namespace}
	deployFilePath := append(filePath, "deployment.yaml")
	deployFile := exutil.FixturePath(deployFilePath...)
	err = deploy.applyFromTemplate(oc, "-f", deployFile, "-n", r.namespace, "-p", "NAMESPACE="+r.namespace, "-p", "NAME="+r.serverName)
	o.Expect(err).NotTo(o.HaveOccurred())
	WaitForDeploymentPodsToBeReady(oc, r.namespace, r.serverName)

	svc := resource{"svc", r.serverName, r.namespace}
	svcFilePath := append(filePath, "svc.yaml")
	svcFile := exutil.FixturePath(svcFilePath...)
	err = svc.applyFromTemplate(oc, "-f", svcFile, "-n", r.namespace, "-p", "NAMESPACE="+r.namespace, "-p", "NAME="+r.serverName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (r rsyslog) remove(oc *exutil.CLI) {
	resource{"serviceaccount", r.serverName, r.namespace}.clear(oc)
	if r.tls {
		resource{"secret", r.serverName, r.namespace}.clear(oc)
		resource{"secret", r.secretName, r.loggingNS}.clear(oc)
	}
	resource{"configmap", r.serverName, r.namespace}.clear(oc)
	resource{"deployment", r.serverName, r.namespace}.clear(oc)
	resource{"svc", r.serverName, r.namespace}.clear(oc)
}

func (r rsyslog) getPodName(oc *exutil.CLI) string {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(r.namespace).List(metav1.ListOptions{LabelSelector: "component=" + r.serverName})
	o.Expect(err).NotTo(o.HaveOccurred())
	var names []string
	for i := 0; i < len(pods.Items); i++ {
		names = append(names, pods.Items[i].Name)
	}
	return names[0]
}

func (r rsyslog) checkData(oc *exutil.CLI, expect bool, filename string) {
	cmd := "ls -l /var/log/clf/" + filename
	err := wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
		stdout, err := e2e.RunHostCmdWithRetries(r.namespace, r.getPodName(oc), cmd, 3*time.Second, 15*time.Second)
		if err != nil {
			return false, err
		}
		if (strings.Contains(stdout, filename) && expect) || (!strings.Contains(stdout, filename) && !expect) {
			return true, nil
		}
		return false, nil
	})
	if expect {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The %s doesn't exist", filename))
	} else {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The %s exists", filename))
	}
}

func searchLogsInLoki(oc *exutil.CLI, cloNS string, lokiNS string, pod string, logType string) lokiQueryResponse {
	//This function to be used only for audit or infra (Journal system) logs only
	cmd := ""
	if logType == "audit" {
		// audit logs
		cmd = "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={ log_type=\"" + logType + "\"}'"
	} else if logType == "infra" {
		// infrastructure logs
		cmd = "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={ log_type=\"infrastructure\"}'"
	} else {
		// Journal system Infra logs
		cmd = "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={ tag=\"journal.system\"}'"
	}
	stdout, err := e2e.RunHostCmdWithRetries(cloNS, pod, cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := lokiQueryResponse{}
	json.Unmarshal([]byte(stdout), &res)
	return res
}
func searchAppLogsInLokiByNamespace(oc *exutil.CLI, cloNS string, lokiNS string, pod string, appNS string) lokiQueryResponse {
	cmd := "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={ kubernetes_namespace_name=\"" + appNS + "\"}'"
	stdout, err := e2e.RunHostCmdWithRetries(cloNS, pod, cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := lokiQueryResponse{}
	json.Unmarshal([]byte(stdout), &res)
	return res
}
func searchAppLogsInLokiByTenantKey(oc *exutil.CLI, cloNS string, lokiNS string, pod string, tenantKey string, tenantKeyID string) lokiQueryResponse {
	cmd := "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={" + tenantKey + "=\"" + tenantKeyID + "\"}'"
	stdout, err := e2e.RunHostCmdWithRetries(cloNS, pod, cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := lokiQueryResponse{}
	json.Unmarshal([]byte(stdout), &res)
	return res
}
func searchAppLogsInLokiByLabelKeys(oc *exutil.CLI, cloNS string, lokiNS string, pod string, labelKeys string, podLabel string) lokiQueryResponse {
	cmd := "curl -G -s  \"http://loki-server." + lokiNS + ".svc:3100/loki/api/v1/query?limit=3\" --data-urlencode 'query={" + labelKeys + "=\"" + podLabel + "\"}'"
	stdout, err := e2e.RunHostCmdWithRetries(cloNS, pod, cmd, 3*time.Second, 30*time.Second)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	res := lokiQueryResponse{}
	json.Unmarshal([]byte(stdout), &res)
	return res
}
func deployExternalLokiServer(oc *exutil.CLI, lokiConfigMapName string, lokiServerName string) string {
	//create project to deploy Loki Server
	oc.SetupProject()
	lokiProj := oc.Namespace()

	//creating sa for loki
	err := oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", "loki-sa", "-n", lokiProj).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:loki-sa", lokiProj)).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	//Create configmap for Loki
	CMTemplate := exutil.FixturePath("testdata", "logging", "external-log-stores", "loki", "loki-configmap.yaml")
	lokiCM := resource{"configmap", "loki-config", lokiProj}
	err = lokiCM.applyFromTemplate(oc, "-n", lokiProj, "-f", CMTemplate, "-p", "LOKINAMESPACE="+lokiProj, "-p", "LOKICMNAME="+lokiConfigMapName)
	o.Expect(err).NotTo(o.HaveOccurred())

	//Create Deployment for Loki
	deployTemplate := exutil.FixturePath("testdata", "logging", "external-log-stores", "loki", "loki-deployment.yaml")
	lokiDeploy := resource{"Deployment", "loki-server", lokiProj}
	err = lokiDeploy.applyFromTemplate(oc, "-n", lokiProj, "-f", deployTemplate, "-p", "LOKISERVERNAME="+lokiServerName, "-p", "LOKINAMESPACE="+lokiProj, "-p", "LOKICMNAME="+lokiConfigMapName)
	o.Expect(err).NotTo(o.HaveOccurred())

	//Expose Loki as a Service
	WaitForDeploymentPodsToBeReady(oc, lokiProj, "loki-server")
	err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("-n", lokiProj, "deployment", "loki-server").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	return lokiProj
}

type fluentdServer struct {
	serverName                 string //the name of the fluentd server, it's also used to name the svc/cm/sa/secret
	namespace                  string //the namespace where the fluentd server deployed in
	serverAuth                 bool
	clientAuth                 bool   // only can be set when serverAuth is true
	clientPrivateKeyPassphrase string //only can be set when clientAuth is true
	sharedKey                  string //if it's not empty, means the shared_key is set, only works when serverAuth is true
	secretName                 string //the name of the secret for the collector to use
	loggingNS                  string //the namespace where the collector pods deployed in
}

func (f fluentdServer) createPipelineSecret(oc *exutil.CLI, keysPath string) {
	secret := resource{"secret", f.secretName, f.loggingNS}
	cmd := []string{"secret", "generic", secret.name, "-n", secret.namespace, "--from-file=ca-bundle.crt=" + keysPath + "/ca.crt"}
	if f.clientAuth {
		cmd = append(cmd, "--from-file=tls.key="+keysPath+"/client.key", "--from-file=tls.crt="+keysPath+"/client.crt")
	}
	if f.clientPrivateKeyPassphrase != "" {
		cmd = append(cmd, "--from-literal=passphrase="+f.clientPrivateKeyPassphrase)
	}
	if f.sharedKey != "" {
		cmd = append(cmd, "--from-literal=shared_key="+f.sharedKey)
	}
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args(cmd...).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	secret.WaitForResourceToAppear(oc)
}

func (f fluentdServer) deploy(oc *exutil.CLI) {
	// create SA
	sa := resource{"serviceaccount", f.serverName, f.namespace}
	err := oc.WithoutNamespace().Run("create").Args("serviceaccount", sa.name, "-n", sa.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	sa.WaitForResourceToAppear(oc)
	//err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:%s", f.namespace, f.serverName), "-n", f.namespace).Execute()
	//o.Expect(err).NotTo(o.HaveOccurred())
	filePath := []string{"testdata", "logging", "external-log-stores", "fluentd"}

	// create secrets if needed
	if f.serverAuth {
		o.Expect(f.secretName).NotTo(o.BeEmpty())
		filePath = append(filePath, "secure")
		// create a temporary directory
		baseDir := exutil.FixturePath("testdata", "logging")
		keysPath := filepath.Join(baseDir, "temp"+getRandomString())
		defer exec.Command("rm", "-r", keysPath).Output()
		err = os.MkdirAll(keysPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		//generate certs
		cert := certsConf{f.serverName, f.namespace, f.clientPrivateKeyPassphrase}
		cert.generateCerts(keysPath)
		//create pipelinesecret
		f.createPipelineSecret(oc, keysPath)
		//create secret for fluentd server
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", f.serverName, "-n", f.namespace, "--from-file=ca-bundle.crt="+keysPath+"/ca.crt", "--from-file=tls.key="+keysPath+"/server.key", "--from-file=tls.crt="+keysPath+"/server.crt", "--from-file=ca.key="+keysPath+"/ca.key").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

	} else {
		filePath = append(filePath, "insecure")
	}

	// create configmap/deployment/svc
	cm := resource{"configmap", f.serverName, f.namespace}
	var cmFileName string
	if !f.serverAuth {
		cmFileName = "configmap.yaml"
	} else {
		if f.clientAuth {
			if f.sharedKey != "" && f.clientPrivateKeyPassphrase == "" {
				cmFileName = "cm-mtls-share.yaml"
			} else if f.sharedKey == "" && f.clientPrivateKeyPassphrase == "" {
				cmFileName = "cm-mtls.yaml"
			} else if f.sharedKey != "" && f.clientPrivateKeyPassphrase != "" {
				cmFileName = "cm-mtls-passphrase-share.yaml"
			} else {
				cmFileName = "cm-mtls-passphrase.yaml"
			}
		} else {
			if f.sharedKey != "" {
				cmFileName = "cm-serverauth-share.yaml"
			} else {
				cmFileName = "cm-serverauth.yaml"
			}
		}
	}
	cmFilePath := append(filePath, cmFileName)
	cmFile := exutil.FixturePath(cmFilePath...)
	cCmCmd := []string{"-f", cmFile, "-n", f.namespace, "-p", "NAMESPACE=" + f.namespace, "-p", "NAME=" + f.serverName}
	if f.sharedKey != "" {
		cCmCmd = append(cCmCmd, "-p", "SHARED_KEY="+f.sharedKey)
	}
	if f.clientPrivateKeyPassphrase != "" {
		cCmCmd = append(cCmCmd, "-p", "PRIVATE_KEY_PASSPHRASE="+f.clientPrivateKeyPassphrase)
	}
	err = cm.applyFromTemplate(oc, cCmCmd...)
	o.Expect(err).NotTo(o.HaveOccurred())

	deploy := resource{"deployment", f.serverName, f.namespace}
	deployFilePath := append(filePath, "deployment.yaml")
	deployFile := exutil.FixturePath(deployFilePath...)
	err = deploy.applyFromTemplate(oc, "-f", deployFile, "-n", f.namespace, "-p", "NAMESPACE="+f.namespace, "-p", "NAME="+f.serverName)
	o.Expect(err).NotTo(o.HaveOccurred())
	WaitForDeploymentPodsToBeReady(oc, f.namespace, f.serverName)

	err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("-n", f.namespace, "deployment", f.serverName, "--name="+f.serverName).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (f fluentdServer) remove(oc *exutil.CLI) {
	resource{"serviceaccount", f.serverName, f.namespace}.clear(oc)
	if f.serverAuth {
		resource{"secret", f.serverName, f.namespace}.clear(oc)
		resource{"secret", f.secretName, f.loggingNS}.clear(oc)
	}
	resource{"configmap", f.serverName, f.namespace}.clear(oc)
	resource{"deployment", f.serverName, f.namespace}.clear(oc)
	resource{"svc", f.serverName, f.namespace}.clear(oc)
}

func (f fluentdServer) getPodName(oc *exutil.CLI) string {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(f.namespace).List(metav1.ListOptions{LabelSelector: "component=" + f.serverName})
	o.Expect(err).NotTo(o.HaveOccurred())
	var names []string
	for i := 0; i < len(pods.Items); i++ {
		names = append(names, pods.Items[i].Name)
	}
	return names[0]
}

// check the data in fluentd server
// filename is the name of a file you want to check
// expect true means you expect the file to exist, false means the file is not expected to exist
func (f fluentdServer) checkData(oc *exutil.CLI, expect bool, filename string) {
	cmd := "ls -l /fluentd/log/" + filename
	err := wait.Poll(5*time.Second, 60*time.Second, func() (done bool, err error) {
		stdout, err := e2e.RunHostCmdWithRetries(f.namespace, f.getPodName(oc), cmd, 3*time.Second, 15*time.Second)
		if err != nil {
			return false, err
		}
		if (strings.Contains(stdout, filename) && expect) || (!strings.Contains(stdout, filename) && !expect) {
			return true, nil
		}
		return false, nil
	})
	if expect {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The %s doesn't exist", filename))
	} else {
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("The %s exists", filename))
	}

}

// return the infrastructureName. For example:  anli922-jglp4
func getInfrastructureName(oc *exutil.CLI) string {
	infrastructureName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure/cluster", "-o=jsonpath={.status.infrastructureName}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return infrastructureName
}

// return the nodeNames
func getNodeNames(oc *exutil.CLI, nodeLabel string) []string {
	var nodeNames []string
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", nodeLabel, "-o=jsonpath={.items[*].metadata.name}").Output()
	if err == nil {
		nodeNames = strings.Split(output, " ")
	} else {
		e2e.Logf("Warning: failed to get nodes names ")
	}
	return nodeNames
}

// cloudWatchSpec the basic object which describe all common test options
type cloudwatchSpec struct {
	groupPrefix       string   // the prefix of the cloudwatch group, the default values is the cluster infrastructureName. For example: anli23ovn-fwm5l
	groupType         string   // `default: "logType"`, the group type to classify logs. logType,namespaceName,namespaceUUID
	secretName        string   // `default: "cw-secret"`, the name of the secret for the collector to use
	secretNamespace   string   // `default: "openshift-logging"`, the namespace where the clusterloggingfoward deployed
	awsKeyID          string   // aws_access_key_id file
	awsKey            string   // aws_access_key file
	awsRegion         string   // `default: "us-east-2"` //aws region
	selNamespacesUUID []string // The app namespaces should be collected
	//disNamespacesUUID []string // The app namespaces should not be collected
	//Generical variables
	nodes            []string // Cluster Nodes Names
	ovnEnabled       bool     //`default: "false"`//  if ovn is enabled. default: false
	logTypes         []string //`default: "['infrastructure','application', 'audit']"` // logTypes in {"application","infrastructure","audit"}
	selAppNamespaces []string //The app namespaces should be collected and verified
	//selInfraNamespaces []string //The infra namespaces should be collected and verified
	//disAppNamespaces   []string //The namespaces should not be collected and verified
	//selInfraPods       []string // The infra pods should be collected and verified.
	//selAppPods         []string // The app pods should be collected and verified
	//disAppPods         []string // The pods shouldn't be collected and verified
	//selInfraContainres []string // The infra containers should be collected and verified
	//selAppContainres   []string // The app containers should be collected and verified
	//disAppContainers   []string // The containers shouldn't be collected verified
	//jsonPods           []string // pods which produce json logs
	//multilinePods      []string // pods which produce multilines logs
}

/*
// TBD: The Spec of the logs records
type logRecordsSpec struct {
	namespace     string //The namespace of the pod generate this record
	podname       string //The name of the pod this record
	containerName string //The container name generate this record
	format        string //The log format of this record. Flat, json, multiline and etc
	content       string //The content of record, only one record can be specified. most of time, we use format and content together to determine the final result. For example: enable json
	number        int    //The total number of the records.
}
*/

// Set the default values to the cloudwatchSpec Object, you need to change the default in It if needs
func (cw cloudwatchSpec) init(oc *exutil.CLI) cloudwatchSpec {
	cw.groupPrefix = getInfrastructureName(oc)
	cw.groupType = "logType"
	cw.secretName = "cw-secret"
	cw.secretNamespace = "openshift-logging"
	cw.awsRegion = "us-east-2"
	cw.nodes = getNodeNames(oc, "kubernetes.io/os=linux")
	cw.logTypes = []string{"infrastructure", "application", "audit"}
	cw.setRegion("us-east-2")
	cw.ovnEnabled = false
	/* May enable it after OVN audit logs producer is enabled by default
	if checkNetworkType(oc) == "ovnkubernetes" {
		cw.ovnEnabled = true
	}
	*/
	e2e.Logf("Init cloudwatchSpec done ")
	return cw
}

// Get the AWS key from cluster
func getAWSKey(oc *exutil.CLI) (string, string) {
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIDBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).Str, gjson.Get(credential, `data.aws_secret_access_key`).Str
	accessKeyID, err1 := base64.StdEncoding.DecodeString(accessKeyIDBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	return string(accessKeyID), string(secureKey)
}

// Create Cloudwatch Secret. note: use credential files can avoid leak in output
func (cw cloudwatchSpec) createClfSecret(oc *exutil.CLI) {
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)

	f1, err1 := os.Create(dirname + "/aws_access_key_id")
	o.Expect(err1).NotTo(o.HaveOccurred())
	defer f1.Close()

	_, err = f1.WriteString(cw.awsKeyID)
	o.Expect(err).NotTo(o.HaveOccurred())

	f2, err2 := os.Create(dirname + "/aws_secret_access_key")
	o.Expect(err2).NotTo(o.HaveOccurred())
	defer f2.Close()

	_, err = f2.WriteString(cw.awsKey)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", cw.secretName, "--from-file="+dirname+"/aws_access_key_id", "--from-file="+dirname+"/aws_secret_access_key", "-n", cw.secretNamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Set AWS Region Env
func (cw cloudwatchSpec) setRegion(regionName string) {
	if regionName == "" {
		regionName = cw.awsRegion
	}
	os.Setenv("AWS_DEFAULT_REGION", regionName)
}

// Return Cloudwatch GroupNames
func (cw cloudwatchSpec) getGroupNames(client *cloudwatchlogs.Client, groupPrefix string) []string {
	var groupNames []string
	if groupPrefix == "" {
		groupPrefix = cw.groupPrefix
	}

	logGroupDesc, err := client.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(groupPrefix),
	})

	if err != nil {
		e2e.Logf("Warn: DescribeLogGroups failed \n %v", err)
		return groupNames
	}
	for _, group := range logGroupDesc.LogGroups {
		groupNames = append(groupNames, *group.LogGroupName)
	}

	e2e.Logf("Found cloudWatchLog groupNames %v", groupNames)
	return groupNames
}

// trigger DeleteLogGroup once the case is over
func (cw cloudwatchSpec) deleteGroups() {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		// Hard coded credentials.
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: cw.awsKeyID, SecretAccessKey: cw.awsKey,
			},
		}))
	if err != nil {
		e2e.Logf("Warn: failed to login to AWS\n delete groups are skipped")
		return
	}
	// Create a Cloudwatch service client
	client := cloudwatchlogs.NewFromConfig(cfg)

	logGroupDesc, err := client.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(cw.groupPrefix)})

	if err != nil {
		e2e.Logf("Warn: DescribeLogGroups failed \n delete groups are skipped \n %v", err)
	} else {
		for _, group := range logGroupDesc.LogGroups {
			e2e.Logf("Delete LogGroups" + *group.LogGroupName)
			_, err := client.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
				LogGroupName: group.LogGroupName,
			})
			if err != nil {
				e2e.Logf("Waring: " + *group.LogGroupName + " is not deleted")
			}
		}
	}
}

/*
// TBD: Get groups storage size matching the groupNamePrefix, But sometimes, storebyte is zero, although  there is logs under it.  Research Needs .
func (cw cloudwatchSpec) getGroupSize(client *cloudwatchlogs.Client, groupName string) int64 {
	const int64zero int64 = 0
	logGroupDesc, err := client.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(groupName),
	})
	if err != nil {
		e2e.Logf("Warn: DescribeLogGroups failed \n %v", err)
		return int64zero
	}
	var totalStoreBytes int64 = int64zero
	for _, group := range logGroupDesc.LogGroups {
		if *group.StoredBytes > int64zero {
			totalStoreBytes = totalStoreBytes + *group.StoredBytes
		}
	}
	return totalStoreBytes
}
*/

// Get Stream names matching the logTypes and containerName.
func (cw cloudwatchSpec) getStreamNames(client *cloudwatchlogs.Client, groupName string, streamPrefix string) []string {
	var logStreamNames []string
	var err error
	var logStreamDesc *cloudwatchlogs.DescribeLogStreamsOutput
	if streamPrefix == "" {
		logStreamDesc, err = client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(groupName),
		})
	} else {
		logStreamDesc, err = client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(groupName), LogStreamNamePrefix: aws.String(streamPrefix),
		})
	}
	if err != nil {
		e2e.Logf("Warn: DescribeLogStreams failed \n %v", err)
		return logStreamNames
	}

	for _, stream := range logStreamDesc.LogStreams {
		logStreamNames = append(logStreamNames, *stream.LogStreamName)
	}
	return logStreamNames
}

/*
// TBD: Checking the byte is the correct way to ensure the logs can be sent to Cloudwatchlogs. But sometimes, the logs are under group, but storebyte is zero.  Research Needs .
func (cw cloudwatchSpec) storeByteFound(client *cloudwatchlogs.Client) bool {
	const int64zero int64 = 0
	var firstLogSize int64 = int64zero

	if len(cw.logTypes) == 0 {
		e2e.Logf("Warning: No LogTypes")
		return false
	}

	for _, logType := range cw.logTypes {
		if logType == "infrastructure" {
			firstLogSize = firstLogSize + cw.getGroupSize(client, cw.groupPrefix+".infrastructure")
		}
		if logType == "audit" {
			firstLogSize = firstLogSize + cw.getGroupSize(client, cw.groupPrefix+".audit")
		}
		if logType == "application" && cw.groupType == "logType" {
			firstLogSize = firstLogSize + cw.getGroupSize(client, cw.groupPrefix+".application")
		}
		if logType == "application" && cw.groupType == "namespaceName" {
			for _, projectName := range cw.selAppNamespaces {
				e2e.Logf(cw.groupPrefix + "." + projectName)
				firstLogSize = firstLogSize + cw.getGroupSize(client, cw.groupPrefix+"."+projectName)
			}
		}
		if logType == "application" && cw.groupType == "namespaceUUID" {
			for _, projectUUID := range cw.selNamespacesUUID {
				firstLogSize = firstLogSize + cw.getGroupSize(client, cw.groupPrefix+"."+projectUUID)
			}
		}
	}

	if firstLogSize > int64zero {
		return true
	} else {
		e2e.Logf("Warning: LogSize <= 0")
		return false
	}
}
*/

// The stream present status
type cloudwatchStreamResult struct {
	streamPattern string
	logType       string //container,journal, audit
	streamFound   bool
}

// In this function, verify all infra logs from all nodes infra (both journal and container) are present on Cloudwatch
func (cw cloudwatchSpec) infrastructureLogsFound(client *cloudwatchlogs.Client) bool {
	var infraLogGroupNames []string
	var logFoundAll bool = true
	var streamsToVerify []*cloudwatchStreamResult

	logGroupNames := cw.getGroupNames(client, cw.groupPrefix)
	for _, e := range logGroupNames {
		r, _ := regexp.Compile(`.*\.infrastructure$`)
		match := r.MatchString(e)
		//match1, _ := regexp.MatchString(".*\\.infrastructure$", e)
		if match {
			infraLogGroupNames = append(infraLogGroupNames, e)
		}
	}
	if len(infraLogGroupNames) == 0 {
		return false
	}
	//Construct the stream pattern
	for _, e := range cw.nodes {
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: strings.Split(e, ".")[0] + ".journal.system", logType: "journal", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: e + ".kubernetes.var.log.pods", logType: "container", streamFound: false})
	}

	for _, e := range streamsToVerify {
		logStreams := cw.getStreamNames(client, infraLogGroupNames[0], e.streamPattern)
		if len(logStreams) > 0 {
			e.streamFound = true
		}
	}

	for _, e := range streamsToVerify {
		if !e.streamFound {
			e2e.Logf("Warn: can not find the stream matching " + e.streamPattern)
			logFoundAll = false
		}
	}
	return logFoundAll
}

// In this function, verify all type of audit logs can be found.
// Note: ovc-audit logs only be present when OVN are enabled
// LogStream Example:
//    anli48022-gwbb4-master-2.k8s-audit.log
//    anli48022-gwbb4-master-2.openshift-audit.log
//    anli48022-gwbb4-master-1.k8s-audit.log
//    ip-10-0-136-31.us-east-2.compute.internal.linux-audit.log
func (cw cloudwatchSpec) auditLogsFound(client *cloudwatchlogs.Client) bool {
	var logFoundAll bool = true
	var auditLogGroupNames []string
	var streamsToVerify []*cloudwatchStreamResult

	for _, e := range cw.getGroupNames(client, cw.groupPrefix) {
		r, _ := regexp.Compile(`.*\.audit$`)
		match := r.MatchString(e)
		//match1, _ := regexp.MatchString(".*\\.audit$", e)
		if match {
			auditLogGroupNames = append(auditLogGroupNames, e)
		}
	}

	if len(auditLogGroupNames) == 0 {
		return false
	}

	var ovnFoundInit bool = true
	if cw.ovnEnabled {
		ovnFoundInit = false
	}

	//Method 1: Not all type of audit logs can be are produced on each node. so this method is comment comment
	/*for _, e := range cw.masters {
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".k8s-audit.log", logType: "k8saudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".openshift-audit.log", logType: "ocpaudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".linux-audit.log", logType: "linuxaudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".ovn-audit.log", logType: "ovnaudit", streamFound: ovnFoundInit})
	}

	for _, e := range cw.workers {
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".k8s-audit.log", logType: "k8saudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".openshift-audit.log", logType: "ocpaudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".linux-audit.log", logType: "linuxaudit", streamFound: false})
		streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{ streamPattern: e+".ovn-audit.log", logType: "ovnaudit", streamFound: ovnFoundInit})
	}


	for _, e := range streamsToVerify {
		logStreams := cw.getStreamNames(client, auditLogGroupNames[0], e.streamPattern)
		if len(logStreams)>0 {
			e.streamFound=true
		}
	}*/

	// Method 2: Only search logstream whose suffix is audit.log. the potential issues 1) No audit log on all nodes 2) The stream size > the buffer to large cluster.
	// TBD: produce audit message on every node
	streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: ".k8s-audit.log$", logType: "k8saudit", streamFound: false})
	streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: ".openshift-audit.log$", logType: "ocpaudit", streamFound: false})
	streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: ".linux-audit.log$", logType: "linuxaudit", streamFound: false})
	streamsToVerify = append(streamsToVerify, &cloudwatchStreamResult{streamPattern: ".ovn-audit.log", logType: "ovnaudit", streamFound: ovnFoundInit})

	logStreams := cw.getStreamNames(client, auditLogGroupNames[0], "")

	for _, e := range streamsToVerify {
		for _, streamName := range logStreams {
			match, _ := regexp.MatchString(e.streamPattern, streamName)
			if match {
				e.streamFound = true
			}
		}
	}

	for _, e := range streamsToVerify {
		if !e.streamFound {
			e2e.Logf("Warn: failed to find stream matching " + e.streamPattern)
			logFoundAll = false
		}
	}
	return logFoundAll
}

// In this function, verify the pod's groupNames can be found in cloudwatch
// GroupName example:
//   uuid-.0471c739-e38c-4590-8a96-fdd5298d47ae,uuid-.audit,uuid-.infrastructure
func (cw cloudwatchSpec) applicationLogsFoundUUID(client *cloudwatchlogs.Client) bool {
	var appLogGroupNames []string
	var logFound bool = true
	if len(cw.selNamespacesUUID) == 0 {
		logGroupNames := cw.getGroupNames(client, cw.groupPrefix)
		for _, e := range logGroupNames {
			r1, _ := regexp.Compile(`.*\.infrastructure$`)
			match1 := r1.MatchString(e)
			//match1, _ := regexp.MatchString(".*\\.infrastructure$", e)
			if match1 {
				continue
			}
			r2, _ := regexp.Compile(`.*\.audit$`)
			match2 := r2.MatchString(e)
			//match2, _ := regexp.MatchString(".*\\.audit$", e)
			if match2 {
				continue
			}
			appLogGroupNames = append(appLogGroupNames, e)
		}
		return len(appLogGroupNames) > 0
	}

	for _, projectUUID := range cw.selNamespacesUUID {
		logGroupNames := cw.getGroupNames(client, cw.groupPrefix+"."+projectUUID)
		if len(logGroupNames) == 0 {
			e2e.Logf("Warn: Can not find groupnames for project " + projectUUID)
			logFound = false
		}
	}
	return logFound
}

// In this function, we verify the pod's groupNames can be found in cloudwatch
// GroupName:
//   prefix.aosqe-log-json-1638788875,prefix.audit,prefix.infrastructure
func (cw cloudwatchSpec) applicationLogsFoundNamespaceName(client *cloudwatchlogs.Client) bool {
	var appLogGroupNames []string
	var logFoundAll bool = true
	if len(cw.selAppNamespaces) == 0 {
		logGroupNames := cw.getGroupNames(client, cw.groupPrefix)
		for _, e := range logGroupNames {
			r1, _ := regexp.Compile(`.*\.infrastructure$`)
			match1 := r1.MatchString(e)
			//match1, _ := regexp.MatchString(".*\\.infrastructure$", e)
			if match1 {
				continue
			}
			r2, _ := regexp.Compile(`.*\.audit$`)
			match2 := r2.MatchString(e)
			//match2, _ := regexp.MatchString(".*\\.audit$", e)
			if match2 {
				continue
			}
			appLogGroupNames = append(appLogGroupNames, e)
		}
		return len(appLogGroupNames) > 0
	}

	for _, projectName := range cw.selAppNamespaces {
		logGroupNames := cw.getGroupNames(client, cw.groupPrefix+"."+projectName)
		if len(logGroupNames) == 0 {
			e2e.Logf("Warn: Can not find groupnames for project " + projectName)
			logFoundAll = false
		}
	}
	return logFoundAll
}

// In this function, verify the logStream can be found under application groupName
// GroupName Example:
//    anli48022-gwbb4.application
// logStream Example:
//    kubernetes.var.log.containers.centos-logtest-tvffh_aosqe-log-json-1638427743_centos-logtest-56a00a8f6a2e43281bce6d44d33e93b600352f2234610a093c4d254a49d9bf4e.log
//    kubernetes.var.log.containers.loki-server-6f8485b8ff-b4p8w_loki-aosqe_loki-c7a4e4fa4370062e53803ac5acecc57f6217eb2bb603143ac013755819ed5fdb.log
//    The stream name changed from containers to pods
//    kubernetes.var.log.pods.openshift-image-registry_image-registry-7f5dbdbc69-vwddg_425a4fbc-6a20-4919-8cd2-8bebd5d9b5cd.registry.0.log
//    pods.
func (cw cloudwatchSpec) applicationLogsFoundLogType(client *cloudwatchlogs.Client) bool {
	var logFoundAll bool = true
	var appLogGroupNames []string

	logGroupNames := cw.getGroupNames(client, "")
	for _, e := range logGroupNames {
		r, _ := regexp.Compile(`.*\.application$`)
		match := r.MatchString(e)
		//match1, _ := regexp.MatchString(".*\\.application$", e)
		if match {
			appLogGroupNames = append(appLogGroupNames, e)
		}
	}
	// Retrun false if can not find app group
	if len(appLogGroupNames) == 0 {
		e2e.Logf("Warn: Can not find app groupnames")
		return false
	}

	if len(appLogGroupNames) > 1 {
		//e2e.Logf("Error: multiple App GroupNames found [%v ], Please clean up LogGroup in Cloudwatch", strings.Join(appLogGroupNames,","))
		e2e.Logf("Warn: multiple App GroupNames found [%v ], Please clean up LogGroup in Cloudwatch", appLogGroupNames)
		return false
	}
	e2e.Logf("Found logGroup", appLogGroupNames[0])

	//Return true, if no selNamespaces is pre-defined, Else search the defined namespaces
	if len(cw.selAppNamespaces) == 0 {
		return true
	}

	logStreams := cw.getStreamNames(client, appLogGroupNames[0], "")
	for _, projectName := range cw.selAppNamespaces {
		var streamFields []string
		var projectFound bool = false
		for _, e := range logStreams {
			streamFields = strings.Split(e, "_")
			if streamFields[1] == projectName {
				projectFound = true
			}
		}
		if !projectFound {
			logFoundAll = false
			e2e.Logf("Warn: Can not find the logStream for project " + projectName)

		}
	}
	// TBD: disSelAppNamespaces, select by pod, containers ....
	return logFoundAll
}

// The index to find application logs
// GroupType
//   logType: anli48022-gwbb4.application
//   namespaceName:  anli48022-gwbb4.aosqe-log-json-1638788875
//   namespaceUUID:   anli48022-gwbb4.0471c739-e38c-4590-8a96-fdd5298d47ae,uuid.audit,uuid.infrastructure
func (cw cloudwatchSpec) applicationLogsFound(client *cloudwatchlogs.Client) bool {
	var logFound bool = true
	switch cw.groupType {
	case "logType":
		logFound = cw.applicationLogsFoundLogType(client)
	case "namespaceName":
		logFound = cw.applicationLogsFoundNamespaceName(client)
	case "namespaceUUID":
		logFound = cw.applicationLogsFoundUUID(client)
	default:
		logFound = false
	}
	return logFound
}

// The common function to verify if logs can be found or not. In general, customized the cloudwatchSpec before call this function
func (cw cloudwatchSpec) logsFound() bool {
	var appFound bool = true
	var infraFound bool = true
	var auditFound bool = true

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: cw.awsKeyID, SecretAccessKey: cw.awsKey,
			},
		}))

	if err != nil {
		e2e.Logf("Error: LoadDefaultConfig failed to AWS  \n %v", err)
		return false
	}

	// Create a Cloudwatch service client
	client := cloudwatchlogs.NewFromConfig(cfg)

	for _, logType := range cw.logTypes {
		if logType == "infrastructure" {
			err1 := wait.Poll(15*time.Second, 90*time.Second, func() (done bool, err error) {
				return cw.infrastructureLogsFound(client), nil
			})
			if err1 != nil {
				infraFound = false
				e2e.Logf("Failed to find infrastructure in given time\n %v", err1)
			} else {
				e2e.Logf("Found InfraLogs finally")
			}
		}
		if logType == "audit" {
			err2 := wait.Poll(15*time.Second, 90*time.Second, func() (done bool, err error) {
				return cw.auditLogsFound(client), nil
			})
			if err2 != nil {
				auditFound = false
				e2e.Logf("Failed to find auditLogs in given time\n %v", err2)
			} else {
				e2e.Logf("Found auditLogs finally")
			}
		}
		if logType == "application" {
			err3 := wait.Poll(15*time.Second, 90*time.Second, func() (done bool, err error) {
				return cw.applicationLogsFound(client), nil
			})
			if err3 != nil {
				appFound = false
				e2e.Logf("Failed to find AppLogs in given time\n %v", err3)
			} else {
				e2e.Logf("Found AppLogs finally")
			}
		}
	}

	if infraFound && auditFound && appFound {
		e2e.Logf("Found all expected logs")
		return true
	}
	e2e.Logf("Error: couldn't find some type of logs. Possible reason: logs weren't generated; connect to AWS failure/timeout; Logging Bugs")
	e2e.Logf("infraFound: %t", infraFound)
	e2e.Logf("auditFound: %t", auditFound)
	e2e.Logf("appFound: %t", appFound)
	return false
}

func getDataFromKafkaConsumerPod(oc *exutil.CLI, ns string, consumerName string) (string, error) {
	consumerPods, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: "job-name=" + consumerName})
	if err != nil {
		return "", err
	}
	output, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ns, consumerPods.Items[0].Name, "--since=2m").Output()
	return output, err
}

type kafka struct {
	namespace    string
	kafkasvcName string
	zoosvcName   string
	authtype     string //Name the kafka folders under testdata same as the authtype (options: sasl_plaintext, sasl_ssl, ssl, plaintext, mutual_sasl_ssl)
}

func (r kafka) deployZookeeper(oc *exutil.CLI) {
	kafkaFilePath := exutil.FixturePath("testdata", "logging", "external-log-stores", "kafka")
	zookeeperConfigDir := filepath.Join(kafkaFilePath, "zookeeper-configmap")

	//create zookeeper configmap/svc/StatefulSet
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", r.zoosvcName, "-n", r.namespace, "--from-file=init.sh="+zookeeperConfigDir+"/init.sh", "--from-file=log4j.properties="+zookeeperConfigDir+"/log4j.properties", "--from-file=zookeeper.properties="+zookeeperConfigDir+"/zookeeper.properties").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	zoosvcFile := filepath.Join(kafkaFilePath, "zookeeper-svc.yaml")
	zoosvc := resource{"Service", r.zoosvcName, r.namespace}
	err = zoosvc.applyFromTemplate(oc, "-n", r.namespace, "-f", zoosvcFile, "-p", "NAME="+r.zoosvcName, "-p", "NAMESPACE="+r.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())

	zoosfsFile := filepath.Join(kafkaFilePath, "zookeeper-statefulset.yaml")
	zoosfs := resource{"StatefulSet", r.zoosvcName, r.namespace}
	err = zoosfs.applyFromTemplate(oc, "-n", r.namespace, "-f", zoosfsFile, "-p", "NAME="+r.zoosvcName, "-p", "NAMESPACE="+r.namespace, "-p", "SERVICENAME="+zoosvc.name, "-p", "CM_NAME="+r.zoosvcName)
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForPodReadyWithLabel(oc, r.namespace, "app="+r.zoosvcName)
}

func (r kafka) deployKafka(oc *exutil.CLI) {
	kafkaFilePath := exutil.FixturePath("testdata", "logging", "external-log-stores", "kafka", r.authtype)
	kafkaConfigDir := filepath.Join(kafkaFilePath, "kafka-configmap")
	consumerConfigDir := filepath.Join(kafkaFilePath, "consumer-configmap")

	//create kafka secrets
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", "kafka-client-cert", "-n", r.namespace, "--from-literal=username=admin", "--from-literal=password=admin-secret").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", "kafka-fluentd", "-n", r.namespace, "--from-literal=username=admin", "--from-literal=password=admin-secret").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	//create ClusterRole
	crFile := filepath.Join(kafkaFilePath, "kafka-clusterrole.yaml")
	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", crFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	//create ClusterRoleBinding
	output, err := oc.AsAdmin().WithoutNamespace().Run("process").Args("-n", r.namespace, "-f", kafkaFilePath+"/kafka-clusterrolebinding.yaml", "-p", "NAMESPACE="+r.namespace).OutputToFile(getRandomString() + ".json")
	o.Expect(err).NotTo(o.HaveOccurred())
	oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", output, "-n", r.namespace).Execute()

	//create kafka configmap
	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", r.kafkasvcName, "-n", r.namespace, "--from-file=init.sh="+kafkaConfigDir+"/init.sh", "--from-file=log4j.properties="+kafkaConfigDir+"/log4j.properties", "--from-file=server.properties="+kafkaConfigDir+"/server.properties", "--from-file=kafka_server_jaas.conf="+kafkaConfigDir+"/kafka_server_jaas.conf").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	//create kafka svc
	svcFile := filepath.Join(kafkaFilePath, "kafka-svc.yaml")
	svc := resource{"Service", r.kafkasvcName, r.namespace}
	err = svc.applyFromTemplate(oc, "-f", svcFile, "-n", r.namespace, "-p", "NAME="+r.kafkasvcName, "-p", "NAMESPACE="+r.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())

	//create kafka StatefulSet
	sfsFile := filepath.Join(kafkaFilePath, "kafka-statefulset.yaml")
	sfs := resource{"StatefulSet", r.kafkasvcName, r.namespace}
	err = sfs.applyFromTemplate(oc, "-f", sfsFile, "-n", r.namespace, "-p", "NAME="+r.kafkasvcName, "-p", "NAMESPACE="+r.namespace, "-p", "CM_NAME="+r.kafkasvcName)
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForPodReadyWithLabel(oc, r.namespace, "app="+r.kafkasvcName)

	//create kafka-consumer deployment
	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", "kafka-client", "-n", r.namespace, "--from-file=client.properties="+consumerConfigDir+"/client.properties", "--from-file=kafka_client_jaas.conf="+consumerConfigDir+"/kafka_client_jaas.conf", "--from-file=sasl-producer.properties="+consumerConfigDir+"/sasl-producer.properties").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	//create kafka deployment
	deployFile := filepath.Join(kafkaFilePath, "kafka-consumer-deployment.yaml")
	deploy := resource{"deployment", "kafka-consumer-sals-plaintext", r.namespace}
	err = deploy.applyFromTemplate(oc, "-f", deployFile, "-n", r.namespace, "NAMESPACE="+r.namespace, "-p", "CM_NAME="+"kafka-client")
	o.Expect(err).NotTo(o.HaveOccurred())
	WaitForDeploymentPodsToBeReady(oc, r.namespace, "kafka-consumer-sals-plaintext")
}

func (r kafka) removeZookeeper(oc *exutil.CLI) {
	resource{"configmap", r.zoosvcName, r.namespace}.clear(oc)
	resource{"svc", r.zoosvcName, r.namespace}.clear(oc)
	resource{"statefulset", r.zoosvcName, r.namespace}.clear(oc)
}

func (r kafka) removeKafka(oc *exutil.CLI) {
	resource{"secret", "kafka-client-cert", r.namespace}.clear(oc)
	resource{"secret", "kafka-fluentd", r.namespace}.clear(oc)
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrole/kafka-node-reader").Execute()
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebinding/kafka-node-reader-binding").Execute()
	resource{"configmap", r.kafkasvcName, r.namespace}.clear(oc)
	resource{"svc", r.kafkasvcName, r.namespace}.clear(oc)
	resource{"statefulset", r.kafkasvcName, r.namespace}.clear(oc)
	resource{"configmap", "kafka-client", r.namespace}.clear(oc)
	resource{"deployment", "kafka-consumer-sals-plaintext", r.namespace}.clear(oc)
}

func deleteEventRouter(oc *exutil.CLI, namespace string) {
	e2e.Logf("Deleting Event Router and its resources")
	r := []resource{{"deployment", "", namespace}, {"configmaps", "", namespace}, {"serviceaccounts", "", namespace}}
	for i := 0; i < len(r); i++ {
		rName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, r[i].kind, "-l", "app=eventrouter", "-o=jsonpath={.items[0].metadata.name}").Output()
		if err != nil {
			errstring := fmt.Sprintf("%v", rName)
			if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") || strings.Contains(errstring, "array index out of bounds") {
				e2e.Logf("%s not found for Event Router", r[i].kind)
				continue
			}
		}
		r[i].name = rName
		err = r[i].clear(oc)
		if err != nil {
			e2e.Logf("could not delete %s/%s", r[i].kind, r[i].name)
		}
	}
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrole", "-l", "app=eventrouter").Execute()
	oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebindings", "-l", "app=eventrouter").Execute()
}

func (r resource) createEventRouter(oc *exutil.CLI, parameters ...string) {
	// delete Event Router first.
	deleteEventRouter(oc, r.namespace)
	parameters = append(parameters, "-l", "app=eventrouter", "-p", "EVENT_ROUTER_NAME="+r.name)
	err := r.applyFromTemplate(oc, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}
