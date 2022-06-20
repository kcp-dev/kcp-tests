package nfd

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	nfdNamespace           = "openshift-nfd"
	nfd_namespace_file     = exutil.FixturePath("testdata", "psap", "nfd", "nfd-namespace.yaml")
	nfd_operatorgroup_file = exutil.FixturePath("testdata", "psap", "nfd", "nfd-operatorgroup.yaml")
	nfd_sub_file           = exutil.FixturePath("testdata", "psap", "nfd", "nfd-sub.yaml")
	nfd_instance_file      = exutil.FixturePath("testdata", "psap", "nfd", "nfd-instance.yaml")
)

// isPodInstalled will return true if any pod is found in the given namespace, and false otherwise
func isPodInstalled(oc *exutil.CLI, namespace string) bool {
	e2e.Logf("Checking if pod is found in namespace %s...", namespace)
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(podList.Items) == 0 {
		e2e.Logf("No pod found in namespace %s :(", namespace)
		return false
	}
	e2e.Logf("Pod found in namespace %s!", namespace)
	return true
}

// createYAMLFromMachineSet creates a YAML file with a given filename from a given machineset name in a given namespace, throws an error if creation fails
func createYAMLFromMachineSet(oc *exutil.CLI, namespace string, machineSetName string, filename string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("get").Args(exutil.MapiMachineset, "-n", namespace, machineSetName, "-o", "yaml").OutputToFile(filename)
}

// createMachineSetFromYAML creates a new machineset from the YAML configuration in a given filename, throws an error if creation fails
func createMachineSetFromYAML(oc *exutil.CLI, filename string) error {
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", filename).Execute()
}

// deleteMachineSet will delete a given machineset name from a given namespace
func deleteMachineSet(oc *exutil.CLI, namespace string, machineSetName string) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(exutil.MapiMachineset, machineSetName, "-n", namespace).Execute()
}
