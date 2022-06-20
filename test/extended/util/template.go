package util

import (
	"fmt"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ApplyClusterResourceFromTemplate apply the changes to the cluster resource.
// For ex: ApplyClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func ApplyClusterResourceFromTemplate(oc *CLI, parameters ...string) {
	resourceFromTemplate(oc, false, "", parameters...)
}

// ApplyNsResourceFromTemplate apply changes to the ns resource.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: ApplyNsResourceFromTemplate(oc, "NAMESPACE", "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func ApplyNsResourceFromTemplate(oc *CLI, namespace string, parameters ...string) {
	resourceFromTemplate(oc, false, namespace, parameters...)
}

// CreateClusterResourceFromTemplate create resource from the template.
// For ex: CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func CreateClusterResourceFromTemplate(oc *CLI, parameters ...string) {
	resourceFromTemplate(oc, true, "", parameters...)
}

// CreateNsResourceFromTemplate create ns resource from the template.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: CreateNsResourceFromTemplate(oc, "NAMESPACE", "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func CreateNsResourceFromTemplate(oc *CLI, namespace string, parameters ...string) {
	resourceFromTemplate(oc, true, namespace, parameters...)
}

func resourceFromTemplate(oc *CLI, create bool, namespace string, parameters ...string) {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		fileName := GetRandomString() + "config.json"
		stdout, _, err := oc.AsAdmin().Run("process").Args(parameters...).OutputsToFiles(fileName)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}

		configFile = stdout
		return true, nil
	})
	AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)

	var resourceErr error
	if create {
		if namespace != "" {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile, "-n", namespace).Execute()
		} else {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		}
	} else {
		if namespace != "" {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", namespace).Execute()
		} else {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		}
	}
	AssertWaitPollNoErr(resourceErr, fmt.Sprintf("fail to create/apply resource %v", resourceErr))
}

func GetRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}
