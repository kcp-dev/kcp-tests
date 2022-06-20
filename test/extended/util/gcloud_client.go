package util

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	o "github.com/onsi/gomega"
)

// Gcloud struct
type Gcloud struct {
	ProjectID string
}

// Login logins to the gcloud. This function needs to be used only once to login into the GCP.
// the gcloud client is only used for the cluster which is on gcp platform.
func (gcloud *Gcloud) Login() *Gcloud {
	checkCred, err := exec.Command("bash", "-c", `gcloud auth list --format="value(account)"`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if string(checkCred) != "" {
		return gcloud
	}
	credErr := exec.Command("bash", "-c", "gcloud auth login --cred-file=$GOOGLE_APPLICATION_CREDENTIALS").Run()
	o.Expect(credErr).NotTo(o.HaveOccurred())
	projectErr := exec.Command("bash", "-c", fmt.Sprintf("gcloud config set project %s", gcloud.ProjectID)).Run()
	o.Expect(projectErr).NotTo(o.HaveOccurred())
	return gcloud
}

// GetIntSvcExternalIP returns the int svc external IP
func (gcloud *Gcloud) GetIntSvcExternalIP(infraID string) (string, error) {
	externalIP, err := exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute instances list --filter="%s-int-svc"  --format="value(EXTERNAL_IP)"`, infraID)).Output()
	if string(externalIP) == "" {
		return "", errors.New("additional VM is not found")
	}
	return strings.Trim(string(externalIP), "\n"), err
}

// GetIntSvcInternalIP returns the int svc internal IP
func (gcloud *Gcloud) GetIntSvcInternalIP(infraID string) (string, error) {
	internalIP, err := exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute instances list --filter="%s-int-svc"  --format="value(networkInterfaces.networkIP)"`, infraID)).Output()
	if string(internalIP) == "" {
		return "", errors.New("additional VM is not found")
	}
	return strings.Trim(string(internalIP), "\n"), err
}

// GetFirewallAllowPorts returns firewall allow ports
func (gcloud *Gcloud) GetFirewallAllowPorts(ruleName string) (string, error) {
	ports, err := exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute firewall-rules list --filter="name=(%s)" --format="value(ALLOW)"`, ruleName)).Output()
	return strings.Trim(string(ports), "\n"), err
}

// UpdateFirewallAllowPorts updates the firewall allow ports
func (gcloud *Gcloud) UpdateFirewallAllowPorts(ruleName string, ports string) error {
	return exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute firewall-rules update %s --allow %s`, ruleName, ports)).Run()
}

// GetZone get zone information for an instance
func (gcloud *Gcloud) GetZone(infraID string, workerName string) (string, error) {
	output, err := exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute instances list --filter="%s" --format="value(ZONE)"`, workerName)).Output()
	if string(output) == "" {
		return "", errors.New("Zone info for the instance is not found")
	}
	return string(output), err
}

// StartInstance Bring GCP node/instance back up
func (gcloud *Gcloud) StartInstance(nodeName string, zoneName string) error {
	return exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute instances start %s --zone=%s`, nodeName, zoneName)).Run()
}

// StopInstance Shutdown GCP node/instance
func (gcloud *Gcloud) StopInstance(nodeName string, zoneName string) error {
	return exec.Command("bash", "-c", fmt.Sprintf(`gcloud compute instances stop %s --zone=%s`, nodeName, zoneName)).Run()
}
