package workloads

import (
	"reflect"
	"regexp"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-apps] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	//It is destructive case, will make kube-scheduler roll out, so adding [Disruptive]. One rollout costs about 5mins, so adding [Slow]
	g.It("Author:yinzhou-Medium-31939-Verify logLevel settings in kube scheduler operator [Disruptive][Slow]", func() {
		patchYamlToRestore := `[{"op": "replace", "path": "/spec/logLevel", "value":"Normal"}]`

		g.By("Set the loglevel to TraceAll")
		patchYamlTraceAll := `[{"op": "replace", "path": "/spec/logLevel", "value":"TraceAll"}]`
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			e2e.Logf("Restoring the scheduler cluster's logLevel")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check the scheduler operator should be in Progressing")
			err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not Progressing")

			g.By("Wait for the scheduler operator to rollout")
			err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
				if err != nil {
					e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
					return false, nil
				}
				if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
					e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not recovered to normal")
		}()
		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not Progressing")

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not recovered to normal")

		g.By("Check the loglevel setting for the pod")
		output, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=10", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 10\n")
		}

		g.By("Set the loglevel to Trace")
		patchYamlTrace := `[{"op": "replace", "path": "/spec/logLevel", "value":"Trace"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlTrace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not Progressing")

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not recovered to normal")

		g.By("Check the loglevel setting for the pod")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=6", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 6\n")
		}

		g.By("Set the loglevel to Debug")
		patchYamlDebug := `[{"op": "replace", "path": "/spec/logLevel", "value":"Debug"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubescheduler", "cluster", "--type=json", "-p", patchYamlDebug).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the scheduler operator should be in Progressing")
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("clusteroperator kube-scheduler not start new progress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*True.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is Progressing:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not Progressing")

		g.By("Wait for the scheduler operator to rollout")
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-scheduler").Output()
			if err != nil {
				e2e.Logf("Fail to get clusteroperator kube-scheduler, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString("True.*False.*False", output); matched {
				e2e.Logf("clusteroperator kube-scheduler is recover to normal:\n%s", output)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "clusteroperator kube-scheduler is not recovered to normal")

		g.By("Check the loglevel setting for the pod")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("pods", "-n", "openshift-kube-scheduler", "-l", "app=openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("-v=4", output); matched {
			e2e.Logf("clusteroperator kube-scheduler is running with logLevel 4\n")
		}
	})

	g.It("Author:knarra-High-44049-DefaultPodTopologySpread doesn't work in non-CloudProvider env in OpenShift 4.7 [Flaky]", func() {
		nodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		// Label nodes
		g.By("Label Node1 & Node2")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "topology.kubernetes.io/zone")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[0].Name, "topology.kubernetes.io/zone", "ocp44049zoneA")
		defer e2e.RemoveLabelOffNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "topology.kubernetes.io/zone")
		e2e.AddOrUpdateLabelOnNode(oc.KubeFramework().ClientSet, nodeList.Items[1].Name, "topology.kubernetes.io/zone", "ocp44049zoneB")

		// Test starts here
		// Test for Large pods
		err = oc.Run("create").Args("deployment", "ocp44049large", "--image", "gcr.io/google-samples/node-hello:1.0", "--replicas", "0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("resources", "deployment/ocp44049large", "--limits=cpu=2,memory=4Gi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("deployment/ocp44049large", "--replicas", "2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check all the pods should running")
		if ok := waitForAvailableRsRunning(oc, "deployment", "ocp44049large", oc.Namespace(), "2"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		expectNodeList := []string{nodeList.Items[0].Name, nodeList.Items[1].Name}
		g.By("Geting the node list where pods running")
		lpodNodeList := getPodNodeListByLabel(oc, oc.Namespace(), "app=ocp44049large")
		sort.Strings(lpodNodeList)

		if reflect.DeepEqual(lpodNodeList, expectNodeList) {
			e2e.Logf("All large pods have spread properly, which is expected")
		} else {
			e2e.Failf("Large pods have not been spread properly")
		}

		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		// Test for small pods
		err = oc.Run("create").Args("deployment", "ocp44049small", "--image", "gcr.io/google-samples/node-hello:1.0", "--replicas", "0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("resources", "deployment/ocp44049small", "--limits=cpu=0.1,memory=128Mi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("deployment/ocp44049small", "--replicas", "6").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check all the pods should running")
		if ok := waitForAvailableRsRunning(oc, "deployment", "ocp44049small", oc.Namespace(), "2"); ok {
			e2e.Logf("All pods are runnnig now\n")
		}

		spodNodeList := getPodNodeListByLabel(oc, oc.Namespace(), "app=ocp44049small")
		spodNodeList = removeDuplicateElement(spodNodeList)
		sort.Strings(spodNodeList)

		if reflect.DeepEqual(spodNodeList, expectNodeList) {
			e2e.Logf("All small pods have spread properly, which is expected")
		} else {
			e2e.Failf("small pods have not been spread properly")
		}

	})

	//It is destructive case, will make kube-scheduler roll out, so adding [Disruptive]. One rollout costs about 5mins, so adding [Slow]
	g.It("Longduration-NonPreRelease-Author:knarra-High-50931-Validate HighNodeUtilization profile 4.10 and above [Disruptive][Slow]", func() {
		patchYamlToRestore := `[{"op": "remove", "path": "/spec/profile"}]`

		g.By("Set profile to HighNodeUtilization")
		patchYamlTraceAll := `[{"op": "add", "path": "/spec/profile", "value":"HighNodeUtilization"}]`
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			e2e.Logf("Restoring the scheduler cluster's logLevel")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Checking KSO operator should be in Progressing and Available after rollout and recovery")
			e2e.Logf("Checking kube-scheduler operator should be in Progressing in 100 seconds")
			expectedStatus := map[string]string{"Progressing": "True"}
			err = waitCoBecomes(oc, "kube-scheduler", 100, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not start progressing in 100 seconds")
			e2e.Logf("Checking kube-scheduler operator should be Available in 1500 seconds")
			expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			err = waitCoBecomes(oc, "kube-scheduler", 1500, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not becomes available in 1500 seconds")

		}()

		g.By("Checking KSO operator should be in Progressing and Available after rollout and recovery")
		e2e.Logf("Checking kube-scheduler operator should be in Progressing in 100 seconds")
		expectedStatus := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "kube-scheduler", 100, expectedStatus)
		exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not start progressing in 100 seconds")
		e2e.Logf("Checking kube-scheduler operator should be Available in 1500 seconds")
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "kube-scheduler", 1500, expectedStatus)
		exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not becomes available in 1500 seconds")

		//Get the kube-scheduler pod name & check logs
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-kube-scheduler", "pods", "-l", "app=openshift-kube-scheduler", "-o=jsonpath={.items[0].metadata.name}").Output()
		schedulerLogs, err := oc.WithoutNamespace().AsAdmin().Run("logs").Args(podName, "-n", "openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if match, _ := regexp.MatchString("score.*\n.*disabled.*\n.*NodeResourcesBalancedAllocation.*\n.*weight.*0.*", schedulerLogs); !match {
			e2e.Failf("Enabling HighNodeUtilization Profile failed: %v", err)
		}
	})

	//It is destructive case, will make kube-scheduler roll out, so adding [Disruptive]. One rollout costs about 5mins, so adding [Slow]
	g.It("Longduration-NonPreRelease-Author:knarra-High-50932-Validate NoScoring profile 4.10 and above [Disruptive][Slow]", func() {
		patchYamlToRestore := `[{"op": "remove", "path": "/spec/profile"}]`

		g.By("Set profile to NoScoring")
		patchYamlTraceAll := `[{"op": "add", "path": "/spec/profile", "value":"NoScoring"}]`
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patchYamlTraceAll).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			e2e.Logf("Restoring the scheduler cluster's logLevel")
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("Scheduler", "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Checking KSO operator should be in Progressing and Available after rollout and recovery")
			e2e.Logf("Checking kube-scheduler operator should be in Progressing in 100 seconds")
			expectedStatus := map[string]string{"Progressing": "True"}
			err = waitCoBecomes(oc, "kube-scheduler", 100, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not start progressing in 100 seconds")
			e2e.Logf("Checking kube-scheduler operator should be Available in 1500 seconds")
			expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			err = waitCoBecomes(oc, "kube-scheduler", 1500, expectedStatus)
			exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not becomes available in 1500 seconds")

		}()

		g.By("Checking KSO operator should be in Progressing and Available after rollout and recovery")
		e2e.Logf("Checking kube-scheduler operator should be in Progressing in 100 seconds")
		expectedStatus := map[string]string{"Progressing": "True"}
		err = waitCoBecomes(oc, "kube-scheduler", 100, expectedStatus)
		exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not start progressing in 100 seconds")
		e2e.Logf("Checking kube-scheduler operator should be Available in 1500 seconds")
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = waitCoBecomes(oc, "kube-scheduler", 1500, expectedStatus)
		exutil.AssertWaitPollNoErr(err, "kube-scheduler operator is not becomes available in 1500 seconds")

		//Get the kube-scheduler pod name and check logs
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-kube-scheduler", "pods", "-l", "app=openshift-kube-scheduler", "-o=jsonpath={.items[0].metadata.name}").Output()
		schedulerLogs, err := oc.WithoutNamespace().AsAdmin().Run("logs").Args(podName, "-n", "openshift-kube-scheduler").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if match, _ := regexp.MatchString("score.*\n.*disabled.*\n.*name:.'*'.*\n.*weight.*0.*", schedulerLogs); !match {
			e2e.Failf("Enabling NoScoring Profile failed: %v", err)
		}
	})
})
