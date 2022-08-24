package util

import (
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Deployment struct defination
type Deployment struct {
	name      string
	namespace string
	replicas  string
	appLabel  string
	image     string
}

// DeployOption uses function option mode to change the default value of deployment parameters, E.g. name, namespace, replicas etc.
type DeployOption func(*Deployment)

// SetDeploymentName replaces the default value of Deployment name parameter
func SetDeploymentName(name string) DeployOption {
	return func(dep *Deployment) {
		dep.name = name
	}
}

// SetDeploymentNameSpace replaces the default value of deployment namespace parameter
func SetDeploymentNameSpace(namespace string) DeployOption {
	return func(dep *Deployment) {
		dep.namespace = namespace
	}
}

// SetDeploymentReplicas replaces the default value of deployment replicas parameter
func SetDeploymentReplicas(replicas string) DeployOption {
	return func(dep *Deployment) {
		dep.replicas = replicas
	}
}

// SetDeploymentAppLabel replaces the default value of deployment appLabel parameter
func SetDeploymentAppLabel(appLabel string) DeployOption {
	return func(dep *Deployment) {
		dep.appLabel = appLabel
	}
}

// SetDeploymentImage replaces the default value of deployment Image parameter
func SetDeploymentImage(image string) DeployOption {
	return func(dep *Deployment) {
		dep.image = image
	}
}

// NewDeployment creates a new customized deployment object
func NewDeployment(opts ...DeployOption) Deployment {
	defaultDeployment := Deployment{
		name:      "e2e-dep-" + GetRandomString(),
		namespace: "default",
		replicas:  "1",
		appLabel:  "e2e-app-" + GetRandomString(),
		image:     "gcr.io/kuar-demo/kuard-amd64:blue",
	}

	for _, o := range opts {
		o(&defaultDeployment)
	}
	return defaultDeployment
}

// Create the deployment
func (dep *Deployment) Create(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("create").Args("deployment", dep.name, "-n", dep.namespace, "--image", dep.image).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete the deployment
func (dep *Deployment) Delete(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("deployment", dep.name, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// GetFieldByJSONPath gets specific field value of the deployment by jsonpath
func (dep *Deployment) GetFieldByJSONPath(k *CLI, JSONPath string) (string, error) {
	return k.WithoutNamespace().WithoutKubeconf().Run("get").Args("deployment", dep.name, "-n", dep.namespace, "-o", "jsonpath="+JSONPath).Output()
}

// GetReplicasNum gets replicas of the deployment
func (dep *Deployment) GetReplicasNum(k *CLI) string {
	replicas, err := dep.GetFieldByJSONPath(k, "{.spec.replicas}")
	o.Expect(err).NotTo(o.HaveOccurred())
	dep.replicas = replicas
	return dep.replicas
}

// Describe the deployment
func (dep *Deployment) Describe(k *CLI) (string, error) {
	deploymentDescribe, err := k.WithoutKubeconf().WithoutNamespace().Run("describe").Args("-n", dep.namespace, "deployment", dep.name).Output()
	return deploymentDescribe, err
}

// CheckReady checks whether the deployment is ready
func (dep *Deployment) CheckReady(k *CLI) (bool, error) {
	dep.replicas = dep.GetReplicasNum(k)
	readyReplicas, err := dep.GetFieldByJSONPath(k, "{.status.availableReplicas}")
	if err != nil {
		e2e.Logf("Get deployment/%s readyReplicas faied of \"%v\"", dep.name, err)
		return false, err
	}
	if dep.replicas == "0" && readyReplicas == "" {
		readyReplicas = "0"
	}
	e2e.Logf("deployment/%s readyReplicas is %s", dep.name, readyReplicas)
	return strings.EqualFold(dep.replicas, readyReplicas), nil
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
		e2e.Logf(dep.name + " availableReplicas is as expected")
		return deploymentReady, nil
	})
	if err != nil {
		describeInfo, _ := dep.Describe(k)
		e2e.Logf("$ oc describe pod %s:\n%s", dep.name, describeInfo)
	}
	AssertWaitPollNoErr(err, fmt.Sprintf("Deployment %s not ready", dep.name))
}
