package util

import (
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetClusterVersion returns the cluster version as string value (Ex: 4.8) and cluster build (Ex: 4.8.0-0.nightly-2021-09-28-165247)
func GetClusterVersion(oc *CLI) (string, string, error) {
	clusterBuild, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.version}").Output()
	if err != nil {
		return "", "", err
	}
	splitValues := strings.Split(clusterBuild, ".")
	clusterVersion := splitValues[0] + "." + splitValues[1]
	return clusterVersion, clusterBuild, err
}

// GetInfraId returns the infra id
func GetInfraId(oc *CLI) (string, error) {
	infraId, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o", "jsonpath='{.status.infrastructureName}'").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(infraId, "'"), err
}

// GetGcpProjectId returns the gcp project id
func GetGcpProjectId(oc *CLI) (string, error) {
	projectId, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o", "jsonpath='{.status.platformStatus.gcp.projectID}'").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(projectId, "'"), err
}

func GetClusterPrefixName(oc *CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route", "console", "-n", "openshift-console", "-o=jsonpath={.spec.host}").Output()
	if err != nil {
		e2e.Logf("Get cluster console route failed with err %v .", err)
		return ""
	}
	return strings.Split(output, ".")[2]
}

func GetClusterArchitecture(oc *CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[0].status.nodeInfo.architecture}").Output()
	if err != nil {
		e2e.Logf("Get nodes failed with err %v .", err)
		return ""
	}
	return output
}
