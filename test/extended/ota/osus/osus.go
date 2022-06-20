package osus

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-updates] OTA osus should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("osus", exutil.KubeConfigPath())

	//author: jiajliu@redhat.com
	g.It("Author:jiajliu-High-35869-install/uninstall osus operator from OperatorHub through CLI [Flaky]", func() {

		testDataDir := exutil.FixturePath("testdata", "ota/osus")
		ogTemp := filepath.Join(testDataDir, "operatorgroup.yaml")
		subTemp := filepath.Join(testDataDir, "subscription.yaml")

		oc.SetupProject()

		og := operatorGroup{
			name:      "osus-og",
			namespace: oc.Namespace(),
			template:  ogTemp,
		}

		sub := subscription{
			name:            "osus-sub",
			namespace:       oc.Namespace(),
			channel:         "v1",
			approval:        "Automatic",
			operatorName:    "cincinnati-operator",
			sourceName:      "qe-app-registry",
			sourceNamespace: "openshift-marketplace",
			template:        subTemp,
		}

		g.By("Create OperatorGroup...")
		og.create(oc)

		g.By("Create Subscription...")
		sub.create(oc)

		g.By("Check updateservice operator installed successully!")
		e2e.Logf("Waiting for osus operator pod creating...")
		err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "--selector=name=updateservice-operator", "-n", oc.Namespace()).Output()
			if err != nil || strings.Contains(output, "No resources found") {
				e2e.Logf("error: %v, keep trying!", err)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod with name=updateservice-operator is not found")

		e2e.Logf("Waiting for osus operator pod running...")
		err = wait.Poll(5*time.Second, 15*time.Second, func() (bool, error) {
			status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "--selector=name=updateservice-operator", "-n", oc.Namespace(), "-o=jsonpath={.items[0].status.phase}").Output()
			if err != nil || strings.Compare(status, "Running") != 0 {
				e2e.Logf("error: %v; status: %w", err, status)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod with name=updateservice-operator is not Running")

		g.By("Delete OperatorGroup...")
		og.delete(oc)

		g.By("Delete Subscription...")
		sub.delete(oc)

		g.By("Delete CSV...")
		installedCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", sub.namespace, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(installedCSV).NotTo(o.BeEmpty())
		removeResource(oc, "-n", sub.namespace, "csv", installedCSV)

		g.By("Check updateservice operator uninstalled successully!")
		err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("all", "-n", oc.Namespace()).Output()
			if err != nil || !strings.Contains(output, "No resources found") {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "updateservice operator is not uninstalled")
	})

})
