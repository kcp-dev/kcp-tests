package mco

import (
	"fmt"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"regexp"
	"strings"
)

const (
	// ControllerDeployment name of the deployment deploying the machine config controller
	ControllerDeployment = "machine-config-controller"
	// ControllerContainer name of the controller container in the controller pod
	ControllerContainer = "machine-config-controller"
	// ControllerLabel label used to identify the controller pod
	ControllerLabel = "k8s-app"
	// ControllerLabelValue value used to identify the controller pod
	ControllerLabelValue = "machine-config-controller"
	// MCONamespace namespace where the MCO controller is deployed
	MCONamespace = "openshift-machine-config-operator"
)

// Controller handles the functinalities related to the MCO controller pod
type Controller struct {
	oc             *exutil.CLI
	logsCheckPoint string
	podName        string
}

// NewController creates a new Controller struct
func NewController(oc *exutil.CLI) *Controller {
	return &Controller{oc: oc, logsCheckPoint: "", podName: ""}
}

// GetCachedPodName returns the cached value of the MCO controller pod name. If there is no value available it tries to execute a command to get the pod name from the cluster
func (mcc *Controller) GetCachedPodName() (string, error) {
	if mcc.podName == "" {
		podName, err := mcc.GetPodName()
		if err != nil {
			e2e.Logf("Error trying to get the machine-config-controller pod name. Error: %s", err)
			return "", err
		}

		mcc.podName = podName
	}

	return mcc.podName, nil
}

// GetPodName executed a command to get the current pod name of the MCO controller pod. Updateds the cached value of the pod name
// This function refreshes the pod name cache
func (mcc *Controller) GetPodName() (string, error) {
	podName, err := mcc.oc.WithoutNamespace().Run("get").Args("pod", "-n", MCONamespace, "-l", ControllerLabel+"="+ControllerLabelValue, "-o", "jsonpath={.items[0].metadata.name}").Output()
	if err != nil {
		return "", err
	}
	mcc.podName = podName
	return podName, nil
}

// IgnoreLogsBeforeNow when it is called all logs generated before calling it will be ignored by "GetLogs"
func (mcc *Controller) IgnoreLogsBeforeNow() error {
	mcc.logsCheckPoint = ""
	logsUptoNow, err := mcc.GetLogs()
	if err != nil {
		return err
	}
	mcc.logsCheckPoint = logsUptoNow

	return nil
}

// StopIgnoringLogs when it is called "IgnoreLogsBeforeNow" effect will not be taken into account anymore, and "GetLogs" will return full logs in MCO controller
func (mcc *Controller) StopIgnoringLogs() {
	mcc.logsCheckPoint = ""
}

// GetIgnoredLogs returns the logs that will be ignored after calling "IgnoreLogsBeforeNow"
func (mcc Controller) GetIgnoredLogs() string {
	return mcc.logsCheckPoint
}

// GetLogs returns the MCO controller logs. Logs generated before calling the function "IgnoreLogsBeforeNow" will not be returned
// This function can return big log so, please, try not to print the returned value in your tests
func (mcc Controller) GetLogs() (string, error) {
	cachedPodName, err := mcc.GetCachedPodName()
	if err != nil {
		return "", err
	}
	if cachedPodName == "" {
		err := fmt.Errorf("Cannot get controller pod name. Failed getting MCO controller logs")
		e2e.Logf("Error getting controller pod name. Error: %s", err)
		return "", err
	}
	podAllLogs, err := exutil.GetSpecificPodLogs(mcc.oc, MCONamespace, ControllerContainer, cachedPodName, "")
	if err != nil {
		e2e.Logf("Error getting log lines. Error: %s", err)
		return "", err
	}
	// Remove the logs before the check point
	return strings.Replace(podAllLogs, mcc.logsCheckPoint, "", 1), nil
}

// GetLogsAsList returns the MCO controller logs as a list strings. One string per line
func (mcc Controller) GetLogsAsList() ([]string, error) {
	logs, err := mcc.GetLogs()
	if err != nil {
		return nil, err
	}

	return strings.Split(logs, "\n"), nil
}

// GetFilteredLogsAsList returns the filtered logs as a lit of strings, one string per line.
func (mcc Controller) GetFilteredLogsAsList(regex string) ([]string, error) {
	logs, err := mcc.GetLogsAsList()
	if err != nil {
		return nil, err
	}

	filteredLogs := []string{}
	for _, line := range logs {
		match, err := regexp.MatchString(regex, line)
		if err != nil {
			e2e.Logf("Error filtering log lines. Error: %s", err)
			return nil, err
		}

		if match {
			filteredLogs = append(filteredLogs, line)
		}
	}

	return filteredLogs, nil
}

// GetFilteredLogs returns the logs filtered by a regexp applied to every line. If the match is ok the log line is accepted.
// This function can return big log so, please, try not to print the returned value in your tests
func (mcc Controller) GetFilteredLogs(regex string) (string, error) {
	logs, err := mcc.GetFilteredLogsAsList(regex)
	if err != nil {
		return "", err
	}

	return strings.Join(logs, "\n"), nil
}
