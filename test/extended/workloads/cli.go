package workloads

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-cli] Workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("oc", exutil.KubeConfigPath())
	)

	g.It("Author:yinzhou-Medium-28007-Checking oc version show clean as gitTreeState value", func() {
		out, err := oc.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		versionInfo := &VersionInfo{}
		if err := json.Unmarshal([]byte(out), &versionInfo); err != nil {
			e2e.Failf("unable to decode version with error: %v", err)
		}
		if match, _ := regexp.MatchString("clean", versionInfo.ClientInfo.GitTreeState); !match {
			e2e.Failf("varification GitTreeState with error: %v", err)
		}

	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-43030-oc get events always show the timestamp as LAST SEEN", func() {
		g.By("Get all the namespace")
		output, err := oc.AsAdmin().Run("get").Args("projects", "-o=custom-columns=NAME:.metadata.name", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		projectList := strings.Fields(output)

		g.By("check the events per project")
		for _, projectN := range projectList {
			output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", projectN).Output()
			if match, _ := regexp.MatchString("No resources found", string(output)); match {
				e2e.Logf("No events in project: %v", projectN)
			} else {
				result, _ := exec.Command("bash", "-c", "cat "+output+" | awk '{print $1}'").Output()
				if match, _ := regexp.MatchString("unknown", string(result)); match {
					e2e.Failf("Does not show timestamp as expected: %v", result)
				}
			}
		}

	})
	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-42983-always delete the debug pod when the oc debug node command exist [Flaky]", func() {
		g.By("Get all the node name list")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeList := strings.Fields(out)

		g.By("Create a new namespace")
		oc.SetupProject()

		g.By("Run debug node")
		for _, nodeName := range nodeList {
			err = oc.AsAdmin().Run("debug").Args("node/"+nodeName, "--", "chroot", "/host", "date").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Make sure debug pods have been deleted")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.Run("get").Args("pods", "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if matched, _ := regexp.MatchString("No resources found", output); !matched {
				e2e.Logf("pods still not deleted :\n%s, try again ", output)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "pods still not deleted")

	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-43032-oc adm release mirror generating correct imageContentSources when using --to and --to-release-image [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podMirrorT := filepath.Join(buildPruningBaseDir, "pod_mirror.yaml")
		g.By("create new namespace")
		oc.SetupProject()

		registry := registry{
			dockerImage: "quay.io/openshifttest/registry:2",
			namespace:   oc.Namespace(),
		}

		g.By("Trying to launch a registry app")
		serInfo := registry.createregistry(oc)
		defer registry.deleteregistry(oc)

		g.By("Get the cli image from openshift")
		cliImage := getCliImage(oc)

		g.By("Create the  pull secret from the localfile")
		createPullSecret(oc, oc.Namespace())
		defer oc.Run("delete").Args("secret/my-secret", "-n", oc.Namespace()).Execute()

		imageSouceS := "--from=quay.io/openshift-release-dev/ocp-release:4.5.8-x86_64"
		imageToS := "--to=" + serInfo.serviceURL + "/zhouytest/test-release"
		imageToReleaseS := "--to-release-image=" + serInfo.serviceURL + "/zhouytest/ocptest-release:4.5.8-x86_64"
		imagePullSecretS := "-a " + "/etc/foo/" + ".dockerconfigjson"

		pod43032 := podMirror{
			name:            "mypod43032",
			namespace:       oc.Namespace(),
			cliImageID:      cliImage,
			imagePullSecret: imagePullSecretS,
			imageSource:     imageSouceS,
			imageTo:         imageToS,
			imageToRelease:  imageToReleaseS,
			template:        podMirrorT,
		}

		g.By("Trying to launch the mirror pod")
		pod43032.createPodMirror(oc)
		defer oc.Run("delete").Args("pod/mypod43032", "-n", oc.Namespace()).Execute()
		g.By("check the mirror pod status")
		err := wait.Poll(5*time.Second, 600*time.Second, func() (bool, error) {
			out, err := oc.Run("get").Args("-n", oc.Namespace(), "pod", pod43032.name, "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Logf("Fail to get pod: %s, error: %s and try again", pod43032.name, err)
			}
			if matched, _ := regexp.MatchString("Succeeded", out); matched {
				e2e.Logf("Mirror completed: %s", out)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Mirror is not completed")

		g.By("Check the mirror result")
		mirrorOutFile, err := oc.Run("logs").Args("-n", oc.Namespace(), "pod/"+pod43032.name).OutputToFile(getRandomString() + "workload-mirror.txt")
		o.Expect(err).NotTo(o.HaveOccurred())

		reg := regexp.MustCompile(`(?m:^  -.*/zhouytest/test-release$)`)
		reg2 := regexp.MustCompile(`(?m:^  -.*/zhouytest/ocptest-release$)`)
		if reg == nil && reg2 == nil {
			e2e.Failf("regexp err")
		}
		b, err := ioutil.ReadFile(mirrorOutFile)
		if err != nil {
			e2e.Failf("failed to read the file ")
		}
		s := string(b)
		match := reg.FindString(s)
		match2 := reg2.FindString(s)
		if match != "" && match2 != "" {
			e2e.Logf("mirror succeed %v and %v ", match, match2)
		} else {
			e2e.Failf("Failed to mirror")
		}

	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-44797-Could define a Command for DC", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("Create the dc with define command")
		err := oc.WithoutNamespace().Run("create").Args("deploymentconfig", "-n", oc.Namespace(), "dc44797", "--image="+"quay.io/openshifttest/busybox@sha256:afe605d272837ce1732f390966166c2afff5391208ddd57de10942748694049d", "--", "tail", "-f", "/dev/null").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the command should be defined")
		comm, err := oc.Run("get").WithoutNamespace().Args("dc/dc44797", "-n", oc.Namespace(), "-o=jsonpath={.spec.template.spec.containers[0].command[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.ExpectEqual("tail", comm)

		g.By("Create the deploy with define command")
		err = oc.WithoutNamespace().Run("create").Args("deployment", "-n", oc.Namespace(), "deploy44797", "--image="+"quay.io/openshifttest/busybox@sha256:afe605d272837ce1732f390966166c2afff5391208ddd57de10942748694049d", "--", "tail", "-f", "/dev/null").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check the command should be defined")
		comm1, err := oc.Run("get").WithoutNamespace().Args("deploy/deploy44797", "-n", oc.Namespace(), "-o=jsonpath={.spec.template.spec.containers[0].command[0]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.ExpectEqual("tail", comm1)

	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-43034-should not show signature verify error msgs while trying to mirror OCP image repository to [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		podMirrorT := filepath.Join(buildPruningBaseDir, "pod_mirror.yaml")
		g.By("create new namespace")
		oc.SetupProject()

		registry := registry{
			dockerImage: "quay.io/openshifttest/registry:2",
			namespace:   oc.Namespace(),
		}

		g.By("Trying to launch a registry app")
		defer registry.deleteregistry(oc)
		serInfo := registry.createregistry(oc)

		g.By("Get the cli image from openshift")
		cliImage := getCliImage(oc)

		g.By("Create the  pull secret from the localfile")
		defer oc.Run("delete").Args("secret/my-secret", "-n", oc.Namespace()).Execute()
		createPullSecret(oc, oc.Namespace())

		g.By("Add the cluster admin role for the default sa")
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-cluster-role-from-user", "cluster-admin", "-z", "default", "-n", oc.Namespace()).Execute()
		err1 := oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", "-z", "default", "-n", oc.Namespace()).Execute()
		o.Expect(err1).NotTo(o.HaveOccurred())

		imageSouceS := "--from=quay.io/openshift-release-dev/ocp-release:4.5.5-x86_64"
		imageToS := "--to=" + serInfo.serviceURL + "/zhouytest/test-release"
		imageToReleaseS := "--apply-release-image-signature"
		imagePullSecretS := "-a " + "/etc/foo/" + ".dockerconfigjson"

		pod43034 := podMirror{
			name:            "mypod43034",
			namespace:       oc.Namespace(),
			cliImageID:      cliImage,
			imagePullSecret: imagePullSecretS,
			imageSource:     imageSouceS,
			imageTo:         imageToS,
			imageToRelease:  imageToReleaseS,
			template:        podMirrorT,
		}

		g.By("Trying to launch the mirror pod")
		defer oc.Run("delete").Args("pod/mypod43034", "-n", oc.Namespace()).Execute()
		pod43034.createPodMirror(oc)
		g.By("check the mirror pod status")
		err := wait.Poll(5*time.Second, 600*time.Second, func() (bool, error) {
			out, err := oc.Run("get").Args("-n", oc.Namespace(), "pod", pod43034.name, "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Logf("Fail to get pod: %s, error: %s and try again", pod43034.name, err)
			}
			if matched, _ := regexp.MatchString("Succeeded", out); matched {
				e2e.Logf("Mirror completed: %s", out)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Mirror is not completed")

		g.By("Get the created configmap")
		newConfigmapS, err := oc.Run("logs").Args("-n", oc.Namespace(), "pod/"+pod43034.name, "--tail=1").Output()
		newConfigmapN := strings.Split(newConfigmapS, " ")[0]
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-config-managed", newConfigmapN).Execute()

		g.By("Check the mirror result")
		mirrorOutFile, err := oc.Run("logs").Args("-n", oc.Namespace(), "pod/"+pod43034.name).OutputToFile(getRandomString() + "workload-mirror.txt")
		o.Expect(err).NotTo(o.HaveOccurred())

		reg := regexp.MustCompile(`(unable to retrieve signature)`)
		if reg == nil {
			e2e.Failf("regexp err")
		}
		b, err := ioutil.ReadFile(mirrorOutFile)
		if err != nil {
			e2e.Failf("failed to read the file ")
		}
		s := string(b)
		match := reg.FindString(s)
		if match != "" {
			e2e.Failf("Mirror failed %v", match)
		} else {
			e2e.Logf("Succeed with the apply-release-image-signature option")
		}

	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-33648-must gather pod should not schedule on windows node", func() {
		go checkMustgatherPodNode(oc)
		g.By("Create the must-gather pod")
		oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--timeout="+"30s", "--dest-dir=/tmp/mustgatherlog", "--", "/etc/resolv.conf").Execute()
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-34155-oc get events sorted by lastTimestamp", func() {
		g.By("Get events sorted by lastTimestamp")
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", "openshift-operator-lifecycle-manager", "--sort-by="+".lastTimestamp").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-47555-Should not update data when use oc set data with dry-run as server", func() {
		g.By("create new namespace")
		oc.SetupProject()
		g.By("Create new configmap")
		err := oc.Run("create").Args("configmap", "cm-47555", "--from-literal=name=abc").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Save the data for configmap")
		beforeSetcm, err := oc.Run("get").Args("cm", "cm-47555", "-o=jsonpath={.data.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Run the set with server dry-run")
		err = oc.Run("set").Args("data", "cm", "cm-47555", "--from-literal=name=def", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		afterSetcm, err := oc.Run("get").Args("cm", "cm-47555", "-o=jsonpath={.data.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if match, _ := regexp.MatchString(beforeSetcm, afterSetcm); !match {
			e2e.Failf("Should not persistent update configmap with server dry-run")
		}
		g.By("Create new secret")
		err = oc.Run("create").Args("secret", "generic", "secret-47555", "--from-literal=name=abc").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Save the data for secret")
		beforeSetse, err := oc.Run("get").Args("secret", "secret-47555", "-o=jsonpath={.data.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Run the set with server dry-run")
		err = oc.Run("set").Args("data", "secret", "secret-47555", "--from-literal=name=def", "--dry-run=server").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		afterSetse, err := oc.Run("get").Args("secret", "secret-47555", "-o=jsonpath={.data.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if match, _ := regexp.MatchString(beforeSetse, afterSetse); !match {
			e2e.Failf("Should not persistent update secret with server dry-run")
		}

	})

	// author: knarra@redhat.com
	g.It("Author:knarra-Medium-48681-Could start debug pod using pod definition yaml", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		debugPodUsingDefinitionT := filepath.Join(buildPruningBaseDir, "debugpod_48681.yaml")

		g.By("create new namespace")
		oc.SetupProject()
		g.By("Get the cli image from openshift")
		cliImage := getCliImage(oc)

		pod48681 := debugPodUsingDefinition{
			name:       "pod48681",
			namespace:  oc.Namespace(),
			cliImageID: cliImage,
			template:   debugPodUsingDefinitionT,
		}

		g.By("Create test pod")
		pod48681.createDebugPodUsingDefinition(oc)
		defer oc.Run("delete").Args("pod/pod48681", "-n", oc.Namespace()).Execute()
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-49116-oc debug should remove startupProbe when create debug pod", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("Create the deploy")
		err := oc.Run("create").Args("deploy", "d49116", "--image", "quay.io/openshifttest/hello-openshift@sha256:b1aabe8c8272f750ce757b6c4263a2712796297511e0c6df79144ee188933623", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("patch the deploy with startupProbe")
		patchS := `[{"op": "add", "path": "/spec/template/spec/containers/0/startupProbe", "value":{ "exec": {"command": [ "false" ]}}}]`
		err = oc.Run("patch").Args("deploy", "d49116", "--type=json", "-p", patchS, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("run the debug with jsonpath")
		out, err := oc.Run("debug").Args("deploy/d49116", "-o=jsonpath='{.spec.containers[0].startupProbe}'", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if out != "''" {
			e2e.Failf("The output should be empty, but not: %v", out)
		}
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-NonPreRelease-High-45307-Critical-45327-check oc adm prune deployments to prune RS [Slow][Disruptive]", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("Create deployments and trigger more times")
		createDeployment(oc, oc.Namespace(), "mydep45307")
		triggerSucceedDeployment(oc, oc.Namespace(), "mydep45307", 6, 20)
		triggerFailedDeployment(oc, oc.Namespace(), "mydep45307")

		g.By("get the completed rs infomation")
		totalCompletedRsList, totalCompletedRsListNum := getCompeletedRsInfo(oc, oc.Namespace(), "mydep45307")

		g.By("Dry run the prune deployments for RS")
		keepCompletedRsNum := 3
		pruneRsNumCMD := fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true  |grep %s |wc -l", keepCompletedRsNum, oc.Namespace())
		pruneRsDryCMD := fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true  |grep %s|awk '{print $2}'", keepCompletedRsNum, oc.Namespace())
		rsListFromPrune := getShouldPruneRSFromPrune(oc, pruneRsNumCMD, pruneRsDryCMD, (totalCompletedRsListNum - keepCompletedRsNum))
		shouldPruneRsList := getShouldPruneRSFromCreateTime(totalCompletedRsList, totalCompletedRsListNum, keepCompletedRsNum)
		if comparePrunedRS(shouldPruneRsList, rsListFromPrune) {
			e2e.Logf("Checked the pruned rs is expected")
		} else {
			e2e.Failf("Pruned the wrong RS with dry run")
		}

		g.By("Make sure never prune RS with replicas num >0")
		//before prune ,check the running rs list
		runningRsList := checkRunningRsList(oc, oc.Namespace(), "mydep45307")

		//checking the should prune rs list
		completedRsNum := 0
		pruneRsNumCMD = fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true  |grep %s |wc -l", completedRsNum, oc.Namespace())
		pruneRsDryCMD = fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true  |grep %s|awk '{print $2}'", completedRsNum, oc.Namespace())

		rsListFromPrune = getShouldPruneRSFromPrune(oc, pruneRsNumCMD, pruneRsDryCMD, (totalCompletedRsListNum - completedRsNum))
		shouldPruneRsList = getShouldPruneRSFromCreateTime(totalCompletedRsList, totalCompletedRsListNum, completedRsNum)
		if comparePrunedRS(shouldPruneRsList, rsListFromPrune) {
			e2e.Logf("dry run prune all completed rs is expected")
		} else {
			e2e.Failf("Pruned the wrong RS with dry run")
		}

		//prune all the completed rs list
		pruneCompletedRs(oc, "prune", "deployments", "--keep-complete=0", "--keep-younger-than=10s", "--replica-sets=true", "--confirm")

		//after prune , check the remaining rs list
		remainingRsList := getRemainingRs(oc, oc.Namespace(), "mydep45307")
		if comparePrunedRS(runningRsList, remainingRsList) {
			e2e.Logf("pruned all completed rs is expected")
		} else {
			e2e.Failf("Pruned the wrong")
		}
	})
	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-NonPreRelease-High-45308-check oc adm prune deployments command with the orphans options works well [Slow][Disruptive]", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("Create deployments and trigger more times")
		createDeployment(oc, oc.Namespace(), "mydep45308")
		triggerSucceedDeployment(oc, oc.Namespace(), "mydep45308", 6, 20)
		triggerFailedDeployment(oc, oc.Namespace(), "mydep45308")

		g.By("get the completed rs infomation")
		totalCompletedRsList, totalCompletedRsListNum := getCompeletedRsInfo(oc, oc.Namespace(), "mydep45308")

		g.By("delete the deploy with ")
		err := oc.Run("delete").Args("-n", oc.Namespace(), "deploy", "mydep45308", "--cascade=orphan").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("prune the rs with orphans=true")
		//before prune ,check the running rs list
		runningRsList := checkRunningRsList(oc, oc.Namespace(), "mydep45308")

		//checking the should prune rs list
		completedRsNum := 0
		pruneRsNumCMD := fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true --orphans=true |grep %s |wc -l", completedRsNum, oc.Namespace())
		pruneRsDryCMD := fmt.Sprintf("oc adm prune deployments --keep-complete=%v --keep-younger-than=10s --replica-sets=true --orphans=true |grep %s|awk '{print $2}'", completedRsNum, oc.Namespace())

		rsListFromPrune := getShouldPruneRSFromPrune(oc, pruneRsNumCMD, pruneRsDryCMD, (totalCompletedRsListNum - completedRsNum))
		shouldPruneRsList := getShouldPruneRSFromCreateTime(totalCompletedRsList, totalCompletedRsListNum, completedRsNum)
		if comparePrunedRS(shouldPruneRsList, rsListFromPrune) {
			e2e.Logf("dry run prune all completed rs is expected")
		} else {
			e2e.Failf("Pruned the wrong RS with dry run")
		}

		//prune all the completed rs list
		pruneCompletedRs(oc, "prune", "deployments", "--keep-complete=0", "--keep-younger-than=10s", "--replica-sets=true", "--confirm", "--orphans=true")

		//after prune , check the remaining rs list
		remainingRsList := getRemainingRs(oc, oc.Namespace(), "mydep45308")
		if comparePrunedRS(runningRsList, remainingRsList) {
			e2e.Logf("pruned all completed rs is expected")
		} else {
			e2e.Failf("Pruned the wrong")
		}
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-49859-should failed when oc import-image setting with Garbage values for --reference-policy", func() {
		g.By("create new namespace")
		oc.SetupProject()

		g.By("import image with garbage values set for reference-policy")
		out, err := oc.Run("import-image").Args("registry.redhat.io/openshift3/jenkins-2-rhel7", "--reference-policy=sdfsdfds", "--confirm").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("reference policy values are source or local"))

		g.By("check should no imagestream created")
		out, err = oc.Run("get").Args("is").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("No resources found"))
	})

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-44061-Check the default registry credential path for oc", func() {
		g.By("check the help info for the registry config locations")
		clusterImage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "version", "-o=jsonpath={.status.desired.image}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("image").Args("info", clusterImage).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("unauthorized: authentication required"))

		g.By("Set podman registry config")
		dirname := "/tmp/case44061"
		err = os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(dirname)
		err = locatePodmanCred(oc, dirname)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("info", clusterImage).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-Medium-50399-oc apply could update EgressNetworkPolicy resource", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		egressnetworkP := filepath.Join(buildPruningBaseDir, "egressnetworkpolicy.yaml")
		updateegressnetworkP := filepath.Join(buildPruningBaseDir, "update_egressnetworkpolicy.yaml")

		g.By("create new namespace")
		oc.SetupProject()
		out, err := oc.AsAdmin().Run("apply").Args("-f", egressnetworkP).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("default-egress-egressnetworkpolicy created"))
		out, err = oc.AsAdmin().Run("apply").Args("-f", updateegressnetworkP).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("default-egress-egressnetworkpolicy configured"))
	})
	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-42982-Describe quota output should always show units", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		deploymentconfigF := filepath.Join(buildPruningBaseDir, "deploymentconfig_with_quota.yaml")
		clusterresourceF := filepath.Join(buildPruningBaseDir, "clusterresource_for_user.yaml")
		g.By("create new namespace")
		oc.SetupProject()
		err := oc.AsAdmin().Run("create").Args("quota", "compute-resources-42982", "--hard=requests.cpu=4,requests.memory=8Gi,pods=4,limits.cpu=4,limits.memory=8Gi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", deploymentconfigF).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for pod running
		checkPodStatus(oc, "deploymentconfig=hello-openshift", oc.Namespace(), "Running")
		checkPodStatus(oc, "openshift.io/deployer-pod-for.name=hello-openshift-1", oc.Namespace(), "Succeeded")
		output, err := oc.Run("describe").Args("quota", "compute-resources-42982").Output()
		if matched, _ := regexp.MatchString("requests.memory.*Ki.*8Gi", output); matched {
			e2e.Logf("describe the quota with units:\n%s", output)
		}

		//check for clusterresourcequota
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterresourcequota", "for-user42982").Execute()
		userName, err := oc.Run("whoami").Args("").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", clusterresourceF).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("current user name is %v", userName)
		patchPath := fmt.Sprintf("-p=[{\"op\": \"replace\", \"path\": \"/spec/selector/annotations\", \"value\":{ \"openshift.io/requester\": \"%s\" }}]", userName)
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterresourcequota", "for-user42982", "--type=json", patchPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("new-project").Args("p42982-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", "p42982-1").Execute()
		err = oc.WithoutNamespace().Run("create").Args("-f", deploymentconfigF, "-n", "p42982-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for pod running
		checkPodStatus(oc, "deploymentconfig=hello-openshift", "p42982-1", "Running")
		checkPodStatus(oc, "openshift.io/deployer-pod-for.name=hello-openshift-1", "p42982-1", "Succeeded")
		output, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("clusterresourcequota", "for-user42982").Output()
		if matched, _ := regexp.MatchString("requests.memory.*Ki.*8Gi", output); matched {
			e2e.Logf("describe the quota with units:\n%s", output)
		}

	})
})

// ClientVersion ...
type ClientVersion struct {
	BuildDate    string `json:"buildDate"`
	Compiler     string `json:"compiler"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GitVersion   string `json:"gitVersion"`
	GoVersion    string `json:"goVersion"`
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	Platform     string `json:"platform"`
}

// ServerVersion ...
type ServerVersion struct {
	BuildDate    string `json:"buildDate"`
	Compiler     string `json:"compiler"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GitVersion   string `json:"gitVersion"`
	GoVersion    string `json:"goVersion"`
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	Platform     string `json:"platform"`
}

// VersionInfo ...
type VersionInfo struct {
	ClientInfo       ClientVersion `json:"ClientVersion"`
	OpenshiftVersion string        `json:"openshiftVersion"`
	ServerInfo       ServerVersion `json:"ServerVersion"`
}
