package clusterinfrastructure

import (
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle] Cluster_Infrastructure", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("cluster-machine-approver", exutil.KubeConfigPath())
	)

	// author: huliu@redhat.com
	g.It("Author:huliu-Medium-45420-Cluster Machine Approver should use leader election", func() {
		attemptAcquireLeaderLeaseStr := "attempting to acquire leader lease openshift-cluster-machine-approver/cluster-machine-approver-leader..."
		acquiredLeaseStr := "successfully acquired lease openshift-cluster-machine-approver/cluster-machine-approver-leader"

		g.By("Check default pod is leader")
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-cluster-machine-approver").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(podName) == 0 {
			g.Skip("Skip for no pod!")
		}
		logsOfPod, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podName, "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logsOfPod).To(o.ContainSubstring(attemptAcquireLeaderLeaseStr))
		o.Expect(logsOfPod).To(o.ContainSubstring(acquiredLeaseStr))

		defer oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", "machine-approver", "--replicas=1", "-n", "openshift-cluster-machine-approver").Execute()

		g.By("Scale the replica of ClusterMachineApprover to 2")
		err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment", "machine-approver", "--replicas=2", "-n", "openshift-cluster-machine-approver").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for ClusterMachineApprover to scale")
		err = wait.Poll(3*time.Second, 90*time.Second, func() (bool, error) {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "machine-approver", "-o=jsonpath={.status.availableReplicas}", "-n", "openshift-cluster-machine-approver").Output()
			readyReplicas, _ := strconv.Atoi(output)
			if readyReplicas != 2 {
				e2e.Logf("The scaled pod is not ready yet and waiting up to 3 seconds ...")
				return false, nil
			}
			e2e.Logf("The deployment machine-approver is successfully scaled")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Check pod failed")

		podNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].metadata.name}", "-n", "openshift-cluster-machine-approver").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podNameList := strings.Split(podNames, " ")

		var logsOfPod1, logsOfPod2 string

		g.By("Wait both pods are attempting to acquire leader lease")
		err = wait.Poll(5*time.Second, 90*time.Second, func() (bool, error) {
			logsOfPod1, _ = oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameList[0], "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
			logsOfPod2, _ = oc.AsAdmin().WithoutNamespace().Run("logs").Args(podNameList[1], "-n", "openshift-cluster-machine-approver", "-c", "machine-approver-controller").Output()
			if !strings.Contains(logsOfPod1, attemptAcquireLeaderLeaseStr) || !strings.Contains(logsOfPod2, attemptAcquireLeaderLeaseStr) {
				e2e.Logf("At least one pod is not attempting to acquire leader lease and waiting up to 5 seconds ...")
				return false, nil
			}
			e2e.Logf("Both pods are attempting to acquire leader lease")
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Check pod attempting to acquire leader lease failed")

		g.By("Check only one pod is leader")
		o.Expect((strings.Contains(logsOfPod1, acquiredLeaseStr) && !strings.Contains(logsOfPod2, acquiredLeaseStr)) || (!strings.Contains(logsOfPod1, acquiredLeaseStr) && strings.Contains(logsOfPod2, acquiredLeaseStr))).To(o.BeTrue())
	})
})
