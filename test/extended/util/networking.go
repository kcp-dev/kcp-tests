package util

import (
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func CheckNetworkType(oc *CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.ToLower(output)
}

// check until CNO operator status reports True, False, False for Available, Progressing, Degraded status,
func CheckNetworkOperatorStatus(oc *CLI) error {
	err := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "network").Output()
		if err != nil {
			e2e.Logf("Fail to get clusteroperator network, error:%s. Trying again", err)
			return false, nil
		}
		matched, _ := regexp.MatchString("True.*False.*False", output)
		if matched {
			return true, nil
		}
		e2e.Logf("Network operator state is:%s", output)
		return false, nil
	})
	return err
}
