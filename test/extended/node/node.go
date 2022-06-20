package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	//e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] NODE initContainer policy,volume,readines,quota", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("node-"+getRandomString(), exutil.KubeConfigPath())
		buildPruningBaseDir  = exutil.FixturePath("testdata", "node")
		customTemp           = filepath.Join(buildPruningBaseDir, "pod-modify.yaml")
		podTerminationTemp   = filepath.Join(buildPruningBaseDir, "pod-termination.yaml")
		podOOMTemp           = filepath.Join(buildPruningBaseDir, "pod-oom.yaml")
		podInitConTemp       = filepath.Join(buildPruningBaseDir, "pod-initContainer.yaml")
		podSleepTemp         = filepath.Join(buildPruningBaseDir, "sleepPod46306.yaml")
		kubeletConfigTemp    = filepath.Join(buildPruningBaseDir, "kubeletconfig-hardeviction.yaml")
		memHogTemp           = filepath.Join(buildPruningBaseDir, "mem-hog-ocp11600.yaml")
		podTwoContainersTemp = filepath.Join(buildPruningBaseDir, "pod-with-two-containers.yaml")
		podUserNSTemp        = filepath.Join(buildPruningBaseDir, "pod-user-namespace.yaml")

		podUserNS47663 = podUserNSDescription{
			name:      "",
			namespace: "",
			template:  podUserNSTemp,
		}

		podModify = podModifyDescription{
			name:          "",
			namespace:     "",
			mountpath:     "",
			command:       "",
			args:          "",
			restartPolicy: "",
			user:          "",
			role:          "",
			level:         "",
			template:      customTemp,
		}

		podTermination = podTerminationDescription{
			name:      "",
			namespace: "",
			template:  podTerminationTemp,
		}

		podOOM = podOOMDescription{
			name:      "",
			namespace: "",
			template:  podOOMTemp,
		}

		podInitCon38271 = podInitConDescription{
			name:      "",
			namespace: "",
			template:  podInitConTemp,
		}

		podSleep = podSleepDescription{
			namespace: "",
			template:  podSleepTemp,
		}

		kubeletConfig = kubeletConfigDescription{
			name:       "",
			labelkey:   "",
			labelvalue: "",
			template:   kubeletConfigTemp,
		}

		memHog = memHogDescription{
			name:       "",
			namespace:  "",
			labelkey:   "",
			labelvalue: "",
			template:   memHogTemp,
		}

		podTwoContainers = podTwoContainersDescription{
			name:      "",
			namespace: "",
			template:  podTwoContainersTemp,
		}
	)
	// author: pmali@redhat.com
	g.It("Author:pmali-High-12893-Init containers with restart policy Always", func() {
		oc.SetupProject()
		podModify.name = "init-always-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Always"

		g.By("create FAILED init container with pod restartPolicy Always")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Always")

		podModify.name = "init-always-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Always"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12894-Init containers with restart policy OnFailure", func() {
		oc.SetupProject()
		podModify.name = "init-onfailure-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "OnFailure"

		g.By("create FAILED init container with pod restartPolicy OnFailure")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy OnFailure")

		podModify.name = "init-onfailure-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "OnFailure"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12896-Init containers with restart policy Never", func() {
		oc.SetupProject()
		podModify.name = "init-never-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with pod restartPolicy Never")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Never")

		podModify.name = "init-never-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12911-App container status depends on init containers exit code	", func() {
		oc.SetupProject()
		podModify.name = "init-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/false"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with exit code and command /bin/false")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		exutil.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with command /bin/true")
		podModify.name = "init-success"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/true"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-High-12913-Init containers with volume work fine", func() {

		oc.SetupProject()
		podModify.name = "init-volume"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "echo This is OCP volume test > /work-dir/volume-test"
		podModify.restartPolicy = "Never"

		g.By("Create a pod with initContainer using volume\n")
		podModify.create(oc)
		g.By("Check pod status")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Check Vol status\n")
		err = volStatus(oc)
		exutil.AssertWaitPollNoErr(err, "Init containers with volume do not work fine")
		g.By("Delete Pod\n")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-Medium-30521-CRIO Termination Grace Period test", func() {

		oc.SetupProject()
		podTermination.name = "pod-termination"
		podTermination.namespace = oc.Namespace()

		g.By("Create a pod with termination grace period\n")
		podTermination.create(oc)
		g.By("Check pod status\n")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Check container TimeoutStopUSec\n")
		err = podTermination.getTerminationGrace(oc)
		exutil.AssertWaitPollNoErr(err, "terminationGracePeriodSeconds is not valid")
		g.By("Delete Pod\n")
		podTermination.delete(oc)
	})

	// author: minmli@redhat.com
	g.It("Author:minmli-High-38271-Init containers should not restart when the exited init container is removed from node", func() {
		g.By("Test for case OCP-38271")
		oc.SetupProject()
		podInitCon38271.name = "initcon-pod"
		podInitCon38271.namespace = oc.Namespace()

		g.By("Create a pod with init container")
		podInitCon38271.create(oc)
		defer podInitCon38271.delete(oc)

		g.By("Check pod status")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Check init container exit normally")
		err = podInitCon38271.containerExit(oc)
		exutil.AssertWaitPollNoErr(err, "conainer not exit normally")

		g.By("Delete init container")
		err = podInitCon38271.deleteInitContainer(oc)
		exutil.AssertWaitPollNoErr(err, "fail to delete container")

		g.By("Check init container not restart again")
		err = podInitCon38271.initContainerNotRestart(oc)
		exutil.AssertWaitPollNoErr(err, "init container restart")
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-Medium-40558-oom kills must be monitored and logged", func() {

		oc.SetupProject()
		podOOM.name = "pod-oom"
		podOOM.namespace = oc.Namespace()

		g.By("Create a pod which will be killed with OOM\n")
		podOOM.create(oc)
		g.By("Check pod status\n")
		err := podOOM.podOOMStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is running")
		g.By("Delete Pod\n")
		podOOM.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("NonPreRelease-Author:pmali-High-46306-Node should not becomes NotReady with error creating container storage layer not known[Disruptive][Slow]", func() {

		oc.SetupProject()
		podSleep.namespace = oc.Namespace()

		g.By("Get Worker Node and Add label app=sleep\n")
		workerNodeName := getSingleWorkerNode(oc)
		addLabelToNode(oc, "app=sleep", workerNodeName, "nodes")
		defer removeLabelFromNode(oc, "app-", workerNodeName, "nodes")

		g.By("Create a 50 pods on the same node\n")
		for i := 0; i < 50; i++ {
			podSleep.create(oc)
		}

		g.By("Check pod status\n")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is NOT running")

		g.By("Delete project\n")
		go podSleep.deleteProject(oc)

		g.By("Reboot Worker node\n")
		go rebootNode(oc, workerNodeName)

		g.By("Check Nodes Status\n")
		err = checkNodeStatus(oc, workerNodeName)
		exutil.AssertWaitPollNoErr(err, "node is not ready")

		g.By("Get Master node\n")
		masterNode := getSingleMasterNode(oc)

		g.By("Check Master Node Logs\n")
		err = masterNodeLog(oc, masterNode)
		exutil.AssertWaitPollNoErr(err, "Logs Found, Test Failed")
	})

	// author: pmali@redhat.com
	g.It("Longduration-NonPreRelease-Author:pmali-Medium-11600-kubelet will evict pod immediately when met hard eviction threshold memory [Disruptive][Slow]", func() {

		oc.SetupProject()
		kubeletConfig.name = "kubeletconfig-ocp11600"
		kubeletConfig.labelkey = "custom-kubelet-ocp11600"
		kubeletConfig.labelvalue = "hard-eviction"

		memHog.name = "mem-hog-ocp11600"
		memHog.namespace = oc.Namespace()
		memHog.labelkey = kubeletConfig.labelkey
		memHog.labelvalue = kubeletConfig.labelvalue

		g.By("Get Worker Node and Add label custom-kubelet-ocp11600=hard-eviction\n")
		addLabelToNode(oc, "custom-kubelet-ocp11600=hard-eviction", "worker", "mcp")
		defer removeLabelFromNode(oc, "custom-kubelet-ocp11600-", "worker", "mcp")

		g.By("Create Kubelet config \n")
		kubeletConfig.create(oc)
		defer getmcpStatus(oc, "worker") // To check all the Nodes are in Ready State after deleteing kubeletconfig
		defer cleanupObjectsClusterScope(oc, objectTableRefcscope{"kubeletconfig", "kubeletconfig-ocp11600"})

		g.By("Make sure Worker mcp is Updated correctly\n")
		err := getmcpStatus(oc, "worker")
		exutil.AssertWaitPollNoErr(err, "mcp is not updated")

		g.By("Create a 10 pods on the same node\n")
		for i := 0; i < 10; i++ {
			memHog.create(oc)
		}
		defer cleanupObjectsClusterScope(oc, objectTableRefcscope{"ns", oc.Namespace()})

		g.By("Check worker Node events\n")
		workerNodeName := getSingleWorkerNode(oc)
		err = getWorkerNodeDescribe(oc, workerNodeName)
		exutil.AssertWaitPollNoErr(err, "Logs did not Found memory pressure, Test Failed")
	})

	// author: weinliu@redhat.com
	g.It("Author:weinliu-Critical-11055-/dev/shm can be automatically shared among all of a pod's containers", func() {
		g.By("Test for case OCP-11055")
		oc.SetupProject()
		podTwoContainers.name = "pod-twocontainers"
		podTwoContainers.namespace = oc.Namespace()
		g.By("Create a pod with two containers")
		podTwoContainers.create(oc)
		defer podTwoContainers.delete(oc)
		g.By("Check pod status")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Enter container 1 and write files")
		_, err = exutil.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift", "echo 'written_from_container1' > /dev/shm/c1")
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Enter container 2 and check whether it can share container 1 shared files")
		containerFile1, err := exutil.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift-fedora", "cat /dev/shm/c1")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Container1 File Content is: %v", containerFile1)
		o.Expect(containerFile1).To(o.Equal("written_from_container1"))
		g.By("Enter container 2 and write files")
		_, err = exutil.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift-fedora", "echo 'written_from_container2' > /dev/shm/c2")
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Enter container 1 and check whether it can share container 2 shared files")
		containerFile2, err := exutil.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift", "cat /dev/shm/c2")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Container2 File Content is: %v", containerFile2)
		o.Expect(containerFile2).To(o.Equal("written_from_container2"))
	})
	
	// author: minmli@redhat.com
	g.It("Author:minmli-High-47663-run pods in user namespaces via crio workload annotation", func() {
		oc.SetupProject()
		g.By("Test for case OCP-47663")
		podUserNS47663.name = "userns-47663"
		podUserNS47663.namespace = oc.Namespace()

		g.By("Check workload of openshift-builder exist in crio config")
		err := podUserNS47663.crioWorkloadConfigExist(oc)
		exutil.AssertWaitPollNoErr(err, "crio workload config not exist")

		g.By("Check user containers exist in /etc/sub[ug]id")
		err = podUserNS47663.userContainersExistForNS(oc)
		exutil.AssertWaitPollNoErr(err, "user containers not exist for user namespace")

		g.By("Create a pod with annotation of openshift-builder workload")
		podUserNS47663.createPodUserNS(oc)
		defer podUserNS47663.deletePodUserNS(oc)

		g.By("Check pod status")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Check pod run in user namespace")
		err = podUserNS47663.podRunInUserNS(oc)
		exutil.AssertWaitPollNoErr(err, "pod not run in user namespace")		
	})
})
