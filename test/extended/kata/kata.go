//Kata operator tests
package kata

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-kata] Kata", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("kata", exutil.KubeConfigPath())
		opNamespace          = "openshift-sandboxed-containers-operator"
		commonKataConfigName = "example-kataconfig"
		testDataDir          = exutil.FixturePath("testdata", "kata")
		iaasPlatform         string
		kcTemplate           = filepath.Join(testDataDir, "kataconfig.yaml")
		defaultDeployment    = filepath.Join(testDataDir, "deployment-example.yaml")
		subTemplate          = filepath.Join(testDataDir, "subscription_template.yaml")
		kcMonitorImageName   = ""
	)

	subscription := subscriptionDescription{
		subName:                "sandboxed-containers-operator",
		namespace:              opNamespace,
		catalogSourceName:      "redhat-operators",
		catalogSourceNamespace: "openshift-marketplace",
		channel:                "stable-1.2",
		ipApproval:             "Automatic",
		operatorPackage:        "sandboxed-containers-operator",
		template:               subTemplate,
	}

	if subscription.channel == "stable-1.2" {
		kcMonitorImageName = "registry.redhat.io/openshift-sandboxed-containers/osc-monitor-rhel8:1.2.0"
	}

	g.BeforeEach(func() {
		// Creating/deleting kataconfig reboots all worker node and extended-platform-tests may timeout after 20m.
		// add --timeout 50m
		// tag with [Slow][Serial][Disruptive] when deleting/recreating kataconfig
		var (
			err error
			msg string
		)

		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		iaasPlatform = strings.ToLower(msg)
		e2e.Logf("the current platform is %v", iaasPlatform)

		ns := filepath.Join(testDataDir, "namespace.yaml")
		og := filepath.Join(testDataDir, "operatorgroup.yaml")

		msg, err = subscribeFromTemplate(oc, subscription, subTemplate, ns, og)
		e2e.Logf("---------- subscription %v succeeded with channel %v %v", subscription.subName, subscription.channel, err)

		msg, err = createKataConfig(oc, kcTemplate, commonKataConfigName, kcMonitorImageName, subscription.namespace)
		e2e.Logf("---------- kataconfig %v create succeeded %v %v", commonKataConfigName, msg, err)
	})

	g.It("Author:abhbaner-High-39499-Operator installation", func() {
		g.By("Checking sandboxed-operator operator installation")
		e2e.Logf("Operator install check successfull as part of setup !!!!!")
		g.By("SUCCESSS - sandboxed-operator operator installed")

	})

	g.It("Author:abhbaner-High-43522-Common Kataconfig installation", func() {
		g.By("Install Common kataconfig and verify it")
		e2e.Logf("common kataconfig %v is installed", commonKataConfigName)
		g.By("SUCCESSS - kataconfig installed")

	})

	g.It("Author:abhbaner-High-41566-High-41574-deploy & delete a pod with kata runtime", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)
		g.By("SUCCESS - Pod with kata runtime installed")
		g.By("TEARDOWN - deleting the kata pod")
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-43238-Operator prohibits creation of multiple kataconfigs", func() {
		var (
			kataConfigName2 = commonKataConfigName + "2"
			configFile      string
			msg             string
			err             error
			kcTemplate      = filepath.Join(testDataDir, "kataconfig.yaml")
		)
		g.By("Create 2nd kataconfig file")
		configFile, err = oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", kcTemplate, "-p", "NAME="+kataConfigName2, "-n", subscription.namespace).OutputToFile(getRandomString() + "kataconfig-common.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the file of resource is %s", configFile)

		g.By("Apply 2nd kataconfig")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Output()
		o.Expect(msg).To(o.ContainSubstring("KataConfig instance already exists"))
		e2e.Logf("err %v, msg %v", err, msg)

		g.By("Success - cannot apply 2nd kataconfig")

	})

	g.It("Author:abhbaner-High-41263-Namespace check", func() {
		g.By("Checking if ns 'openshift-sandboxed-containers-operator' exists")
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring(opNamespace))
		g.By("SUCCESS - Namespace check complete")

	})

	g.It("Author:abhbaner-High-43620-validate podmetrics for pod running kata", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)

		errCheck := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			podMetrics, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("podmetrics", newPodName, "-n", podNs).Output()
			if err != nil {
				e2e.Logf("error  %v, please try next round", err)
				return false, nil
			}
			e2e.Logf("Pod metrics output below  \n %s ", podMetrics)
			o.Expect(podMetrics).To(o.ContainSubstring("Cpu"))
			o.Expect(podMetrics).To(o.ContainSubstring("Memory"))
			o.Expect(podMetrics).To(o.ContainSubstring("Events"))
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("can not describe podmetrics %v in ns %v", newPodName, podNs))
		g.By("SUCCESS - Podmetrics for pod with kata runtime validated")
		g.By("TEARDOWN - deleting the kata pod")
	})

	g.It("Author:abhbaner-High-43617-High-43616-CLI checks pod logs & fetching pods in podNs", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)

		/* checkKataPodStatus prints the pods with the podNs and validates if
		its running or not thus verifying OCP-43616 */

		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)
		errCheck := wait.Poll(10*time.Second, 200*time.Second, func() (bool, error) {
			podlogs, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+newPodName, "-n", podNs).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podlogs).NotTo(o.BeEmpty())
			if strings.Contains(podlogs, "httpd") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Pod logs are not getting generated"))
		g.By("SUCCESS - Logs for pods with kata validated")
		g.By("TEARDOWN - deleting the kata pod")
	})

	g.It("Author:abhbaner-High-43514-kata pod displaying correct overhead", func() {
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		defer deleteKataPod(oc, podNs, newPodName)
		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)

		g.By("Checking Pod Overhead")
		podoverhead, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("runtimeclass", "kata").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podoverhead).NotTo(o.BeEmpty())
		o.Expect(podoverhead).To(o.ContainSubstring("Overhead"))
		o.Expect(podoverhead).To(o.ContainSubstring("Cpu"))
		o.Expect(podoverhead).To(o.ContainSubstring("Memory"))
		g.By("SUCCESS - kata pod overhead verified")
		g.By("TEARDOWN - deleting the kata pod")
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-43619-oc admin top pod works for pods that use kata runtime", func() {

		oc.SetupProject()
		var (
			commonPodTemplate = filepath.Join(testDataDir, "example.yaml")
			podNs             = oc.Namespace()
			podName           string
			err               error
			msg               string
			waitErr           error
			metricCount       = 0
		)

		g.By("Deploy a pod with kata runtime")
		podName = createKataPod(oc, podNs, commonPodTemplate, "admtop")
		defer deleteKataPod(oc, podNs, podName)
		checkKataPodStatus(oc, podNs, podName)

		g.By("Get oc top adm metrics for the pod")
		snooze = 360
		waitErr = wait.Poll(10*time.Second, snooze*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "pod", "-n", podNs, podName, "--no-headers").Output()
			if err == nil { // Will get error with msg: error: metrics not available yet
				metricCount = len(strings.Fields(msg))
			}
			if metricCount == 3 {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "metrics never appeared")
		if metricCount == 3 {
			e2e.Logf("metrics for pod %v", msg)
		}
		o.Expect(metricCount).To(o.Equal(3))

		g.By("Success")

	})

	g.It("Author:abhbaner-High-43516-operator is available in CatalogSource", func() {

		g.By("Checking catalog source for the operator")
		opMarketplace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifests", "-n", "openshift-marketplace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(opMarketplace).NotTo(o.BeEmpty())
		o.Expect(opMarketplace).To(o.ContainSubstring("sandboxed-containers-operator"))
		o.Expect(opMarketplace).To(o.ContainSubstring("Red Hat Operators"))
		g.By("SUCCESS -  'sandboxed-containers-operator' is present in packagemanifests")

	})

	g.It("Longduration-NonPreRelease-Author:abhbaner-High-43523-Monitor Kataconfig deletion[Disruptive][Serial][Slow]", func() {
		g.By("Delete kataconfig and verify it")
		msg, err := deleteKataConfig(oc, commonKataConfigName)
		e2e.Logf("kataconfig %v was deleted\n--------- %v %v", commonKataConfigName, msg, err)

		g.By("Recreating kataconfig in 43523 for the remaining test cases")
		msg, err = createKataConfig(oc, kcTemplate, commonKataConfigName, kcMonitorImageName, subscription.namespace)
		e2e.Logf("recreated kataconfig %v: %v %v", commonKataConfigName, msg, err)

		g.By("SUCCESS")
	})

	g.It("Longduration-NonPreRelease-Author:abhbaner-High-41813-Build Acceptance test[Disruptive][Serial][Slow]", func() {
		//This test will install operator,kataconfig,pod with kata - delete pod, delete kataconfig
		commonPodName := "example"
		commonPod := filepath.Join(testDataDir, "example.yaml")

		oc.SetupProject()
		podNs := oc.Namespace()

		g.By("Deploying pod with kata runtime and verify it")
		newPodName := createKataPod(oc, podNs, commonPod, commonPodName)
		checkKataPodStatus(oc, podNs, newPodName)
		e2e.Logf("Pod (with Kata runtime) with name -  %v , is installed", newPodName)
		deleteKataPod(oc, podNs, newPodName)
		g.By("Kata Pod deleted - now deleting kataconfig")

		msg, err := deleteKataConfig(oc, commonKataConfigName)
		e2e.Logf("common kataconfig %v was deleted %v %v", commonKataConfigName, msg, err)
		g.By("SUCCESSS - build acceptance passed")

		g.By("Recreating kataconfig for the remaining test cases")
		msg, err = createKataConfig(oc, kcTemplate, commonKataConfigName, kcMonitorImageName, subscription.namespace)
		e2e.Logf("recreated kataconfig %v: %v %v", commonKataConfigName, msg, err)
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-46235-Kata Metrics Verify that Namespace is labeled to enable monitoring", func() {
		var (
			err        error
			msg        string
			s          string
			label      = ""
			hasMetrics = false
		)

		g.By("Get labels of openshift-sandboxed-containers-operator namespace to check for monitoring")
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", "openshift-sandboxed-containers-operator", "-o=jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, s = range strings.SplitAfter(msg, ",") {
			if strings.Contains(s, "openshift.io/cluster-monitoring") {
				label = s
				if strings.Contains(strings.SplitAfter(s, ":")[1], "true") {
					hasMetrics = true
				}
			}
		}
		o.Expect(strings.Contains(msg, "openshift.io/cluster-monitoring")).To(o.BeTrue())
		e2e.Logf("Label is %v", label)
		o.Expect(hasMetrics).To(o.BeTrue())

		g.By("Success")
	})

	g.It("Author:abhbaner-High-43524-Existing deployments (with runc) should restart normally after kata runtime install", func() {
		g.By("Creating a deployment")

		oc.SetupProject()
		ns := oc.Namespace()
		newDeployName := "dep-43524"
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", defaultDeployment, "-p", "NAME="+newDeployName).OutputToFile(getRandomString() + "dep-common.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		deployMsg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", ns, "-o=jsonpath={..name}").Output()
		e2e.Logf(" get pods for ns %v, output - %v", ns, deployMsg)
		o.Expect(deployMsg).To(o.ContainSubstring(newDeployName))

		defaultPodName := strings.Split(deployMsg, " example")[0]
		//deleting pod from the deployment and checking its status
		e2e.Logf("delete pod %s in namespace %s", defaultPodName, "%s ns", ns)
		oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", defaultPodName, "-n", ns).Execute()
		errCheck := wait.Poll(10*time.Second, 200*time.Second, func() (bool, error) {
			deployAfterDeleteMsg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "-n", ns).Output()
			if strings.Contains(deployAfterDeleteMsg, "3/3") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Pod replica could not be restarted"))
		g.By("SUCCESSS - kataconfig installed and post that pod with runc successfully restarted ")
	})

})
