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

// DeployOption use function option mode to change the default value of deployment parameters, E.g. name, namespace, replicas etc.
type DeployOption func(*Deployment)

// SetDeploymentName replace the default value of Deployment name parameter
func SetDeploymentName(name string) DeployOption {
	return func(dep *Deployment) {
		dep.name = name
	}
}

// SetDeploymentNameSpace replace the default value of Deployment namespace parameter
func SetDeploymentNameSpace(namespace string) DeployOption {
	return func(dep *Deployment) {
		dep.namespace = namespace
	}
}

// SetDeploymentReplicas replace the default value of Deployment replicas parameter
func SetDeploymentReplicas(replicas string) DeployOption {
	return func(dep *Deployment) {
		dep.replicas = replicas
	}
}

// SetDeploymentAppLabel replace the default value of Deployment appLabel parameter
func SetDeploymentAppLabel(appLabel string) DeployOption {
	return func(dep *Deployment) {
		dep.appLabel = appLabel
	}
}

// SetDeploymentImage replace the default value of Deployment Image parameter
func SetDeploymentImage(image string) DeployOption {
	return func(dep *Deployment) {
		dep.image = image
	}
}

// NewDeployment create a new customized Deployment object
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

// Create new Deployment with customized parameters
func (dep *Deployment) Create(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("create").Args("deployment", dep.name, "-n", dep.namespace, "--image", dep.image).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete the Deployment
func (dep *Deployment) Delete(k *CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("deployment", dep.name, "-n", dep.namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// GetSpecificField get Specific Field of the Deployment
func (dep *Deployment) GetSpecificField(k *CLI, JSONPath string) (string, error) {
	return k.WithoutNamespace().WithoutKubeconf().Run("get").Args("deployment", dep.name, "-n", dep.namespace, "-o", "jsonpath="+JSONPath).Output()
}

// GetReplicasNum get replicas of the Deployment
func (dep *Deployment) GetReplicasNum(k *CLI) string {
	replicas, err := dep.GetSpecificField(k, "{.spec.replicas}")
	o.Expect(err).NotTo(o.HaveOccurred())
	dep.replicas = replicas
	return dep.replicas
}

// Describe the deployment
func (dep *Deployment) Describe(k *CLI) (string, error) {
	deploymentDescribe, err := k.WithoutKubeconf().WithoutNamespace().Run("describe").Args("-n", dep.namespace, "deployment", dep.name).Output()
	return deploymentDescribe, err
}

// CheckReady check whether the deployment is ready
func (dep *Deployment) CheckReady(k *CLI) (bool, error) {
	dep.replicas = dep.GetReplicasNum(k)
	readyReplicas, err := dep.GetSpecificField(k, "{.status.availableReplicas}")
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

// WaitReady waiting the deployment become ready
func (dep *Deployment) WaitReady(k *CLI) {
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
