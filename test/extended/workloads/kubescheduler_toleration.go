package workloads

import (
	"path/filepath"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-scheduling] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-13538-Check Existing pods with matched NoExecute will stay on node for time of tolerationSeconds [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podtolerateT := filepath.Join(buildPruningBaseDir, "pod_tolerationseconds.yaml")

		g.By("Test for case OCP-13538")
		g.By("create new namespace")
		oc.SetupProject()

		pod13538 := podTolerate{
			namespace:      oc.Namespace(),
			keyName:        "key1",
			operatorPolicy: "Equal",
			valueName:      "value1",
			effectPolicy:   "NoExecute",
			tolerateTime:   120,
			template:       podtolerateT,
		}

		g.By("Trying to launch a pod with toleration")
		pod13538.createPodTolerate(oc)
		pod13538nodename := getPodNodeName(oc, oc.Namespace(), "tolerationseconds-1")

		g.By("Add a matched taint on the node")
		err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod13538nodename, "key1=value1:NoExecute").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod13538nodename, "key1:NoExecute-").Execute()

		g.By("wait for 60 seconds then untaint the node")
		time.Sleep(60 * time.Second)
		oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod13538nodename, "key1:NoExecute-").Execute()

		g.By("check the pod should still running on the node")
		output, err := oc.WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace(), "tolerationseconds-1", "-o=wide").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString(pod13538nodename, output); matched {
			e2e.Logf("pods is still running on node :\n%s", output)
		}

		g.By("Add the matched taint to node1 again")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("taint", "node", pod13538nodename, "key1=value1:NoExecute").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("wait for 100 seconds check the pod still running on the node")
		time.Sleep(100 * time.Second)
		output, err = oc.WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace(), "tolerationseconds-1", "-o=wide").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString(pod13538nodename, output); matched {
			e2e.Logf("pods is still running on node after 100s:\n%s", output)
		}

		g.By("wait for more 30 seconds check the pod should be deleted")
		err = wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
			output, _ = oc.WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace(), "tolerationseconds-1", "-o=wide").Output()
			if matched, _ := regexp.MatchString("pods.*not found", output); matched {
				e2e.Logf("pods is deleted on node after 120s:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pods is not deleted on node after 120s")
	})

})
