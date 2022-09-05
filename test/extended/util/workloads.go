package util

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Deployment struct definition
type Deployment struct {
	Name      string
	Namespace string
	Replicas  string
	AppLabel  string
	Image     string
}

// DeployOption uses function option mode to change the default value of deployment parameters, E.g. name, namespace, replicas etc.
type DeployOption func(*Deployment)

// SetDeploymentName sets the deployment's name
func SetDeploymentName(name string) DeployOption {
	return func(dep *Deployment) {
		dep.Name = name
	}
}

// SetDeploymentNameSpace sets the deployment's namespace
func SetDeploymentNameSpace(namespace string) DeployOption {
	return func(dep *Deployment) {
		dep.Namespace = namespace
	}
}

// SetDeploymentReplicas sets the deployment's replicas
func SetDeploymentReplicas(replicas string) DeployOption {
	return func(dep *Deployment) {
		dep.Replicas = replicas
	}
}

// SetDeploymentAppLabel sets the deployment's appLabel
func SetDeploymentAppLabel(appLabel string) DeployOption {
	return func(dep *Deployment) {
		dep.AppLabel = appLabel
	}
}

// SetDeploymentImage sets the deployment's image
func SetDeploymentImage(image string) DeployOption {
	return func(dep *Deployment) {
		dep.Image = image
	}
}

// NewDeployment creates a new customized deployment object
func NewDeployment(opts ...DeployOption) Deployment {
	defaultDeployment := Deployment{
		Name:      "e2e-dep-" + GetRandomString(),
		Namespace: "default",
		Replicas:  "1",
		AppLabel:  "e2e-app-" + GetRandomString(),
		Image:     "gcr.io/kuar-demo/kuard-amd64:blue",
	}

	for _, o := range opts {
		o(&defaultDeployment)
	}
	return defaultDeployment
}

// Create the deployment
func (dep *Deployment) Create(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("create").Args("deployment", dep.Name, "-n", dep.Namespace, "--image", dep.Image).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete the deployment
func (dep *Deployment) Delete(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("deployment", dep.Name, "-n", dep.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Clean the deployment resource
func (dep *Deployment) Clean(k *CLI) {
	k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("deployment", dep.Name, "-n", dep.Namespace).Execute()
}

// GetFieldByJSONPath gets specific field value of the deployment by jsonpath
func (dep *Deployment) GetFieldByJSONPath(k *CLI, JSONPath string) (string, error) {
	return k.WithoutNamespace().WithoutKubeconf().Run("get").Args("deployment", dep.Name, "-n", dep.Namespace, "-o", "jsonpath="+JSONPath).Output()
}

// GetReplicasNum gets replicas of the deployment
func (dep *Deployment) GetReplicasNum(k *CLI) string {
	replicas, err := dep.GetFieldByJSONPath(k, "{.spec.replicas}")
	o.Expect(err).NotTo(o.HaveOccurred())
	dep.Replicas = replicas
	return dep.Replicas
}

// Describe the deployment
func (dep *Deployment) Describe(k *CLI) (string, error) {
	deploymentDescribe, err := k.WithoutKubeconf().WithoutNamespace().Run("describe").Args("-n", dep.Namespace, "deployment", dep.Name).Output()
	return deploymentDescribe, err
}

// CheckReady checks whether the deployment is ready
func (dep *Deployment) CheckReady(k *CLI) (bool, error) {
	dep.Replicas = dep.GetReplicasNum(k)
	readyReplicas, err := dep.GetFieldByJSONPath(k, "{.status.availableReplicas}")
	if err != nil {
		e2e.Logf("Get deployment/%s readyReplicas faied of \"%v\"", dep.Name, err)
		return false, err
	}
	if dep.Replicas == "0" && readyReplicas == "" {
		readyReplicas = "0"
	}
	e2e.Logf("deployment/%s readyReplicas is %s", dep.Name, readyReplicas)
	return strings.EqualFold(dep.Replicas, readyReplicas), nil
}

// WaitUntilReady waits the deployment become ready
func (dep *Deployment) WaitUntilReady(k *CLI) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		deploymentReady, err := dep.CheckReady(k)
		if err != nil {
			return deploymentReady, err
		}
		if !deploymentReady {
			return deploymentReady, nil
		}
		e2e.Logf(dep.Name + " availableReplicas is as expected")
		return deploymentReady, nil
	})
	if err != nil {
		describeInfo, _ := dep.Describe(k)
		e2e.Logf("$ oc describe pod %s:\n%s", dep.Name, describeInfo)
	}
	AssertWaitPollNoErr(err, fmt.Sprintf("Deployment %s not ready", dep.Name))
}

// ScaleReplicas scales the deployment's replicas number
func (dep *Deployment) ScaleReplicas(k *CLI, replicasNum string) {
	err := k.WithoutKubeconf().WithoutNamespace().Run("scale").Args("deployment", dep.Name, "--replicas="+replicasNum, "-n", dep.Namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	dep.Replicas = replicasNum
}

// CheckDisplayAttributes checks the deployment info showing the expected columns
func (dep *Deployment) CheckDisplayAttributes(k *CLI) {
	deploymentInfo, err := k.WithoutKubeconf().WithoutNamespace().Run("get").Args("-n", dep.Namespace, "deployment", dep.Name).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(deploymentInfo).Should(o.And(
		o.ContainSubstring("NAME"),
		o.ContainSubstring("READY"),
		o.ContainSubstring("UP-TO-DATE"),
		o.ContainSubstring("AVAILABLE"),
		o.ContainSubstring("AGE"),
	))
}
