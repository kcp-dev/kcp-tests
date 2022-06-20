package networking

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func getOvsHWOffloadWokerNodes(oc *exutil.CLI) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/offload", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeNameList := strings.Fields(output)
	return nodeNameList
}

func capturePacktes(oc *exutil.CLI, ns string, pod string, intf string, srcip string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", ns, pod, "bash", "-c",
		`timeout --preserve-status 10 tcpdump tcp -c 10 -vvv -i `+fmt.Sprintf("%s", intf)+` and src net `+fmt.Sprintf("%s", srcip)+`/32`).Output()
	e2e.Logf("start to capture packetes on pod %s using command 'tcpdump tcp -c 10 -vvv -i %s and src net %s/32`", pod, intf, srcip)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).NotTo(o.BeEmpty())
	return output
}

func chkCapturePacketsOnIntf(oc *exutil.CLI, ns string, pod string, intf string, srcip string, expectnum string) {
	errCheck := wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		capResOnIntf := capturePacktes(oc, ns, pod, intf, srcip)
		//e2e.Logf("The capture packtes result is %v", capResOnIntf)
		reg := regexp.MustCompile(`(\d+) packets captured`)
		match := reg.FindStringSubmatch(capResOnIntf)
		pktNum := match[1]
		e2e.Logf("captured %s packtes on this interface", pktNum)
		if pktNum != expectnum {
			e2e.Logf("doesn't capture the expected number packets, trying next round ... ")
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(errCheck, "can not capture expected number packets, please check")
}

func getPodVFPresentor(oc *exutil.CLI, ns string, pod string) string {
	nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", pod, "-o=jsonpath={.spec.nodeName}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	containerID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", pod, "-o=jsonpath={.status.containerStatuses[0].containerID}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The containerID is %v", containerID)
	initContainerID := string(containerID)[8:]
	e2e.Logf("The initContainerID is %s", initContainerID)
	sandBoxStr, err := oc.AsAdmin().Run("debug").Args(`node/`+fmt.Sprintf("%s", nodename), "--", "chroot", "/host", "crictl", "inspect", initContainerID).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	//e2e.Logf("The sandBoxStr is %v", sandBoxStr)
	re := regexp.MustCompile(`sandboxID":\s+"(\w+)"`)
	match := re.FindStringSubmatch(sandBoxStr)
	e2e.Logf("The match result is %v", match)
	sandBoxID := match[1]
	vfPreName := string(sandBoxID)[:15]
	e2e.Logf("The VF Presentor is %s", vfPreName)
	return vfPreName
}

func startIperfTraffic(oc *exutil.CLI, ns string, pod string, svrip string, duration string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", ns, pod, "iperf3", "-c", svrip, "-t", duration).Output()
	if err != nil {
		e2e.Logf("start iperf traffic failed, the error message is %s", output)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).NotTo(o.BeEmpty())
	re := regexp.MustCompile(`(\d+.\d+)\s+Gbits/sec\s+receiver`)
	match := re.FindStringSubmatch(output)
	bandWidth := match[1]
	e2e.Logf("iperf bandwidth %s", bandWidth)
	return bandWidth
}

func startIperfTrafficBackground(oc *exutil.CLI, ns string, pod string, svrip string, duration string) {
	e2e.Logf("start iperf traffic in background")
	_, _, _, err := oc.Run("exec").Args("-n", ns, pod, "-q", "--", "iperf3", "-c", svrip, "-t", duration).Background()
	o.Expect(err).NotTo(o.HaveOccurred())
	//wait for 5 seconds for iperf starting.
	time.Sleep(5 * time.Second)
}
