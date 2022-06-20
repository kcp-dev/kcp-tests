package hypershift

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-hypershift] Hypershift", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("hypershift", exutil.KubeConfigPath())
	var guestClusterName, guestClusterNamespace string

	g.BeforeEach(func() {
		operator := doOcpReq(oc, OcpGet, false, []string{"pods", "-n", "hypershift", "-ojsonpath={.items[*].metadata.name}"})
		if len(operator) <= 0 {
			g.Skip("hypershift operator not found, skip test run")
		}

		clusterNames := doOcpReq(oc, OcpGet, false,
			[]string{"-n", "clusters", "hostedcluster", "-o=jsonpath={.items[*].metadata.name}"})
		if len(clusterNames) <= 0 {
			g.Skip("hypershift guest cluster not found, skip test run")
		}

		//get first guest cluster to run test
		guestClusterName = strings.Split(clusterNames, " ")[0]
		guestClusterNamespace = "clusters-" + guestClusterName

		res := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "hypershift", "pod", "-o=jsonpath={.items[0].status.phase}"})
		checkSubstring(res, []string{"Running"})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-42855-Check Status Conditions for HostedControlPlane", func() {
		g.By("hypershift OCP-42855 check hostedcontrolplane condition status")

		res := doOcpReq(oc, OcpGet, true,
			[]string{"-n", guestClusterNamespace, "hostedcontrolplane", guestClusterName,
				"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}"})
		checkSubstring(res,
			[]string{"ValidHostedControlPlaneConfiguration True",
				"EtcdAvailable True", "KubeAPIServerAvailable True", "InfrastructureReady True"})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43555-Allow direct ingress on guest clusters on AWS", func() {
		g.By("hypershift OCP-43555 allow direct ingress on guest cluster")
		guestClusterKubeconfigFile := "/tmp/guestcluster-kubeconfig-43555"

		var bashClient = NewCmdClient()
		defer os.Remove(guestClusterKubeconfigFile)
		_, err := bashClient.Run(fmt.Sprintf("hypershift create kubeconfig --name %s > %s", guestClusterName, guestClusterKubeconfigFile)).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		res := doOcpReq(oc, OcpGet, true,
			[]string{"clusteroperator", "kube-apiserver", fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile),
				"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}"})
		checkSubstring(res, []string{"Degraded False"})

		ingressDomain := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "openshift-ingress-operator", "ingresscontrollers", "-ojsonpath={.items[0].spec.domain}",
				fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile)})
		e2e.Logf("The guest cluster ingress domain is : %s\n", ingressDomain)

		console := doOcpReq(oc, OcpWhoami, true,
			[]string{fmt.Sprintf("--kubeconfig=%s", guestClusterKubeconfigFile), "--show-console"})

		pwdbase64 := doOcpReq(oc, OcpGet, true,
			[]string{"-n", guestClusterNamespace, "secret", "kubeadmin-password", "-ojsonpath={.data.password}"})
		pwd, err := base64.StdEncoding.DecodeString(pwdbase64)
		o.Expect(err).ShouldNot(o.HaveOccurred())

		parms := fmt.Sprintf("curl -u admin:%s %s  -k  -LIs -o /dev/null -w %s ", string(pwd), console, "%{http_code}")
		res, err = bashClient.Run(parms).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		checkSubstring(res, []string{"200"})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43282-Implement versioning API and report version status in hostedcluster[Serial][Disruptive]", func() {
		g.By("hypershift OCP-43282 Implement versioning API and report version status in hostedcluster")

		oriImage := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "clusters", "hostedcluster", guestClusterName, "-ojsonpath={.status.version.desired.image}"})
		e2e.Logf("hostedcluster %s image: %s", guestClusterName, oriImage)

		defer func() {
			//change back
			patchOption := fmt.Sprintf("-p=[{\"op\": \"replace\", \"path\": \"/spec/release/image\",\"value\": \"%s\"}]", oriImage)
			doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "hostedcluster", guestClusterName, "--type=json", patchOption})

			err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				//check hostedcluster status image, check by spec/release/image
				res := doOcpReq(oc, OcpGet, true,
					[]string{"-n", "clusters", "hostedcluster", guestClusterName, "-ojsonpath={.spec.release.image}"})
				if strings.Contains(res, oriImage) {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "release image of hostedcluster change back error")
		}()

		//change image version to quay.io/openshift-release-dev/ocp-release:4.9.0-x86_64
		desiredImage := "quay.io/openshift-release-dev/ocp-release:4.9.10-x86_64"
		patchOption := fmt.Sprintf("-p=[{\"op\": \"replace\", \"path\": \"/spec/release/image\",\"value\": \"%s\"}]", desiredImage)
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "hostedcluster", guestClusterName, "--type=json", patchOption})

		err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			//check hostedcluster status image
			res := doOcpReq(oc, OcpGet, true,
				[]string{"-n", "clusters", "hostedcluster", guestClusterName, "-ojsonpath={.status.version.desired.image}"})
			if !strings.Contains(res, desiredImage) {
				return false, nil
			}

			//check hostedcontrolplane spec.releaseImage
			res = doOcpReq(oc, OcpGet, true,
				[]string{"-n", guestClusterNamespace, "hostedcontrolplane", guestClusterName, "-ojsonpath={.spec.releaseImage}"})
			if !strings.Contains(res, desiredImage) {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "release image of hostedcluster update error")
	})

	// author: heli@redhat.com
	g.It("Longduration-NonPreRelease-Author:heli-Critical-43272-Test cluster autoscaler via hostedCluster autoScaling settings[Serial][Slow]", func() {
		g.By("Author:jiezhao-Critical-43272-Test cluster autoscaler via hostedCluster autoScaling settings")

		nodeCountJsonPath := fmt.Sprintf("-ojsonpath={.items[?(@.spec.clusterName==\"%s\")].spec.nodeCount}", guestClusterName)
		nodeCount := doOcpReq(oc, OcpGet, true, []string{"-n", "clusters", "nodepools", nodeCountJsonPath})
		e2e.Logf("The nodepool size is : %s\n", nodeCount)

		var bashClient = NewCmdClient().WithShowInfo(true)
		npCount := 2
		npName := "jz-43272-test-01"
		npName2 := "jz-43272-test-02"

		defer func() {
			res := doOcpReq(oc, OcpGet, false, []string{"-n", "clusters", "nodepools", npName, "--ignore-not-found"})
			if res != "" {
				doOcpReq(oc, OcpDelete, false, []string{"-n", "clusters", "nodepools", npName})
			}
			res = doOcpReq(oc, OcpGet, false, []string{"-n", "clusters", "nodepools", npName2, "--ignore-not-found"})
			if res != "" {
				doOcpReq(oc, OcpDelete, false, []string{"-n", "clusters", "nodepools", npName2})
			}
		}()

		cmd := fmt.Sprintf("hypershift create nodepool aws --name %s --cluster-name %s --node-count %d", npName, guestClusterName, npCount)
		_, err := bashClient.Run(cmd).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		_, err = bashClient.Run(fmt.Sprintf(
			"hypershift create nodepool aws --name %s --cluster-name %s --node-count %d", npName2, guestClusterName, npCount)).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		//remove nodeCount and set autoscaling max:4, min:1
		autoScalingMax := "4"
		autoScalingMin := "1"
		removeNpConfig := "[{\"op\": \"remove\", \"path\": \"/spec/nodeCount\"}]"
		autoscalConfig := fmt.Sprintf("--patch={\"spec\": {\"autoScaling\":   {\"max\": %s, \"min\":%s}}}", autoScalingMax, autoScalingMin)

		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, "--type=json", "-p", removeNpConfig})
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, autoscalConfig, "--type=merge"})
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName2, "--type=json", "-p", removeNpConfig})
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName2, autoscalConfig, "--type=merge"})

		res := doOcpReq(oc, OcpGet, true, []string{"-n", "clusters", "nodepools", npName, "-ojsonpath={.spec.autoScaling.max}"})
		o.Expect(res).To(o.ContainSubstring(autoScalingMax))
		res = doOcpReq(oc, OcpGet, true, []string{"-n", "clusters", "nodepools", npName, "-ojsonpath={.spec.autoScaling.min}"})
		o.Expect(res).To(o.ContainSubstring(autoScalingMin))
		res = doOcpReq(oc, OcpGet, true, []string{"-n", "clusters", "nodepools", npName2, "-ojsonpath={.spec.autoScaling.max}"})
		o.Expect(res).To(o.ContainSubstring(autoScalingMax))
		res = doOcpReq(oc, OcpGet, true, []string{"-n", "clusters", "nodepools", npName2, "-ojsonpath={.spec.autoScaling.min}"})
		o.Expect(res).To(o.ContainSubstring(autoScalingMin))

		guestClusterKubeconfigFile := "/tmp/guestcluster-kubeconfig-43272"
		defer os.Remove(guestClusterKubeconfigFile)

		//get hostedcluster kubeconfig
		_, err = bashClient.Run(fmt.Sprintf("hypershift create kubeconfig --name=%s > %s", guestClusterName, guestClusterKubeconfigFile)).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		hypershiftTeamBaseDir := exutil.FixturePath("testdata", "hypershift")
		workloadTemplate := filepath.Join(hypershiftTeamBaseDir, "workload.yaml")

		// workload is deployed on guest cluster default namespace, and will be cleared in the end
		wl := workload{
			name:      "workload",
			namespace: "default",
			template:  workloadTemplate,
		}

		//create workload
		parsedWorkloadFile := "ocp-43272-workload-template.config"
		defer wl.delete(oc, guestClusterKubeconfigFile, parsedWorkloadFile)
		wl.create(oc, guestClusterKubeconfigFile, parsedWorkloadFile)

		//wait a bit for nodepool scale up, max=20mins
		err = wait.Poll(30*time.Second, 20*time.Minute, func() (bool, error) {
			resNp := doOcpReq(oc, OcpGet, false, []string{"nodepool", npName, "-n", "clusters", "-ojsonpath={.status.nodeCount}"})
			resNp2 := doOcpReq(oc, OcpGet, false, []string{"nodepool", npName2, "-n", "clusters", "-ojsonpath={.status.nodeCount}"})
			if resNp == autoScalingMax && resNp2 == autoScalingMax {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "nodepool auto scaling check error")

		//clear
		doOcpReq(oc, OcpDelete, true, []string{"-n", "clusters", "nodepools", npName})
		doOcpReq(oc, OcpDelete, true, []string{"-n", "clusters", "nodepools", npName2})
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43829-Test autoscaling status in nodePool conditions[Serial]", func() {
		g.By("hypershift OCP-43829-Test autoscaling status in nodePool conditions")

		//check nodepool autoscale status
		npNameJsonPath := fmt.Sprintf("-ojsonpath={.items[?(@.spec.clusterName==\"%s\")].metadata.name}", guestClusterName)
		existingNodePools := doOcpReq(oc, OcpGet, false, []string{"nodepool", "-n", "clusters", npNameJsonPath})
		existNp := strings.Split(existingNodePools, " ")[0]
		res := doOcpReq(oc, OcpGet, true, []string{"nodepool", existNp, "-n", "clusters",
			"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}}"})
		o.Expect(res).To(o.ContainSubstring("AutoscalingEnabled False"))

		nodeCount := doOcpReq(oc, OcpGet, true,
			[]string{"-n", "clusters", "nodepools", existNp, "-ojsonpath={.spec.nodeCount}"})
		e2e.Logf("The nodepool size is : %s\n", nodeCount)

		//create nodepool
		var bashClient = NewCmdClient().WithShowInfo(true)
		npCount := 2
		npName := "jz-43829-test-01"

		defer func() {
			res := doOcpReq(oc, OcpGet, false, []string{"-n", "clusters", "nodepools", npName, "--ignore-not-found"})
			if res != "" {
				doOcpReq(oc, OcpDelete, false, []string{"-n", "clusters", "nodepools", npName})
			}
		}()

		cmd := fmt.Sprintf("hypershift create nodepool aws --name %s --cluster-name %s --node-count %d", npName, guestClusterName, npCount)
		_, err := bashClient.Run(cmd).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		//check nodepool autoscale status
		res = doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters",
			"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}}"})
		o.Expect(res).To(o.ContainSubstring("AutoscalingEnabled False"))

		//Set autoscaling and keep nodeCount in the nodepool:
		autoScalingMax := "4"
		autoScalingMin := "1"
		autoscalConfig := fmt.Sprintf("--patch={\"spec\": {\"autoScaling\":   {\"max\": %s, \"min\":%s}}}", autoScalingMax, autoScalingMin)
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, autoscalConfig, "--type=merge"})

		//Check autoscaling status
		res = doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters",
			"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}}"})
		o.Expect(res).To(o.ContainSubstring("AutoscalingEnabled False"))

		//Remove nodeCount, keep autoscaling in the nodepool:
		removeNpConfig := "[{\"op\": \"remove\", \"path\": \"/spec/nodeCount\"}]"
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, "--type=json", "-p", removeNpConfig})

		//Check autoscaling status
		res = doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters",
			"-ojsonpath={range .status.conditions[*]}{@.type}{\" \"}{@.status}{\" \"}{end}}"})
		o.Expect(res).To(o.ContainSubstring("AutoscalingEnabled True"))
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43268-Expose nodePoolManagement API to enable rolling upgrade[Serial][Disruptive]", func() {
		g.By("hypershift OCP-43268-Expose nodePoolManagement API to enable rolling upgrade")

		//create nodepool
		var bashClient = NewCmdClient().WithShowInfo(true)
		npCount := 2
		npName := "jz-43268-test-01"

		defer func() {
			res := doOcpReq(oc, OcpGet, false, []string{"-n", "clusters", "nodepools", npName, "--ignore-not-found"})
			if res != "" {
				doOcpReq(oc, OcpDelete, false, []string{"-n", "clusters", "nodepools", npName})
			}
		}()

		cmd := fmt.Sprintf("hypershift create nodepool aws --name %s --cluster-name %s --node-count %d", npName, guestClusterName, npCount)
		_, err := bashClient.Run(cmd).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		//Get nodepool yaml file, edit spec.release.image, change it to 4.8.5
		npImage := doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters",
			"-ojsonpath={.spec.release.image}"})
		e2e.Logf("The original image of nodepool %s is : %s\n", npName, npImage)

		desiredImage := "quay.io/openshift-release-dev/ocp-release:4.8.5-x86_64"
		patchOption := fmt.Sprintf("-p=[{\"op\": \"replace\", \"path\": \"/spec/release/image\",\"value\": \"%s\"}]", desiredImage)
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepool", npName, "--type=json", patchOption})

		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			//Get nodepool yaml file
			newImage := doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters", "-ojsonpath={.spec.release.image}"})
			if !strings.Contains(newImage, desiredImage) {
				return false, nil
			}

			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "enable rolling update image error")

		//remove nodeCount, add autoscaling, change spec.release.image back
		autoScalingMax := "4"
		autoScalingMin := "1"
		removeNpConfig := "[{\"op\": \"remove\", \"path\": \"/spec/nodeCount\"}]"
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, "--type=json", "-p", removeNpConfig})
		autoscalConfig := fmt.Sprintf("--patch={\"spec\": {\"autoScaling\":   {\"max\": %s, \"min\":%s}}}", autoScalingMax, autoScalingMin)
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepools", npName, autoscalConfig, "--type=merge"})

		patchOption = fmt.Sprintf("-p=[{\"op\": \"replace\", \"path\": \"/spec/release/image\",\"value\": \"%s\"}]", npImage)
		doOcpReq(oc, OcpPatch, true, []string{"-n", "clusters", "nodepool", npName, "--type=json", patchOption})

		err = wait.Poll(1*time.Second, 60*time.Second, func() (bool, error) {
			//Get nodepool yaml file
			newImage := doOcpReq(oc, OcpGet, true, []string{"nodepool", npName, "-n", "clusters", "-ojsonpath={.spec.release.image}"})
			if !strings.Contains(newImage, npImage) {
				return false, nil
			}

			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "enable rolling update node count error")
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-43554-Check FIPS support in the Hosted Cluster", func() {
		g.By("hypershift OCP-43554-Check FIPS support in the Hosted Cluster")

		res := doOcpReq(oc, OcpGet, false, []string{"-n", "clusters", "hostedcluster", guestClusterName, "-ojsonpath={.spec.fips}"})
		if res != "true" {
			g.Skip("only for the fip enabled hostedcluster, skip test run")
		}

		guestClusterKubeconfigFile := "/tmp/guestcluster-kubeconfig-43554"
		defer os.Remove(guestClusterKubeconfigFile)
		var bashClient = NewCmdClient()
		_, err := bashClient.Run(fmt.Sprintf("hypershift create kubeconfig --name %s > %s", guestClusterName, guestClusterKubeconfigFile)).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		hostedNodes := doOcpReq(oc, OcpGet, true, []string{"--kubeconfig=" + guestClusterKubeconfigFile, "node", "-ojsonpath={.items[*].metadata.name}"})
		for _, nodename := range strings.Split(hostedNodes, " ") {
			na := strings.TrimSpace(nodename)
			//check node FIP mode
			res = doOcpReq(oc, OcpDebug, true, []string{"--kubeconfig=" + guestClusterKubeconfigFile, "node/" + na, "-q", "--", "fips-mode-setup", "--check"})
			o.Expect(res).To(o.ContainSubstring("FIPS mode is enabled"))

			//ignore cat /etc/system-fips because chroot /host failed
			res = doOcpReq(oc, OcpDebug, true, []string{"--kubeconfig=" + guestClusterKubeconfigFile, "node/" + na, "-q", "--", "cat", "/proc/sys/crypto/fips_enabled"})
			o.Expect(res).To(o.ContainSubstring("1"))

			res = doOcpReq(oc, OcpDebug, true, []string{"--kubeconfig=" + guestClusterKubeconfigFile, "node/" + na, "-q", "--", "sysctl", "crypto.fips_enabled"})
			o.Expect(res).To(o.ContainSubstring("crypto.fips_enabled = 1"))
		}
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-45770-Test basic fault resilient HA-capable etcd[Serial][Disruptive]", func() {
		g.By("hypershift OCP-45770-Test basic fault resilient HA-capable etcd")
		controlplaneMode := doOcpReq(oc, OcpGet, true, []string{"hostedcluster", guestClusterName, "-n", "clusters", "-ojsonpath={.spec.controllerAvailabilityPolicy}"})
		e2e.Logf("get hostedcluster %s controllerAvailabilityPolicy: %s ", guestClusterName, controlplaneMode)
		if controlplaneMode != "HighlyAvailable" {
			g.Skip("this is for guest cluster HA mode testrun, skip...")
		}

		//check etcd
		antiAffinityJsonPath := ".spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution"
		topologyKeyJsonPath := antiAffinityJsonPath + "[*].topologyKey"
		desiredTopogyKey := "topology.kubernetes.io/zone"

		etcdSts := "etcd"
		doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "statefulset", etcdSts, "-ojsonpath={" + antiAffinityJsonPath + "}"})
		res := doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "statefulset", etcdSts, "-ojsonpath={" + topologyKeyJsonPath + "}"})
		o.Expect(res).To(o.ContainSubstring(desiredTopogyKey))

		//check etcd healthy
		etcdCmd := "ETCDCTL_API=3 /usr/bin/etcdctl --cacert /etc/etcd/tls/client/etcd-client-ca.crt " +
			"--cert /etc/etcd/tls/client/etcd-client.crt --key /etc/etcd/tls/client/etcd-client.key --endpoints=localhost:2379"
		etcdHealthCmd := etcdCmd + " endpoint health"
		etcdStatusCmd := etcdCmd + " endpoint status"
		for i := 0; i < 3; i++ {
			res = doOcpReq(oc, OcpExec, true, []string{"-n", guestClusterNamespace, "etcd-" + strconv.Itoa(i), "--", "sh", "-c", etcdHealthCmd})
			o.Expect(res).To(o.ContainSubstring("localhost:2379 is healthy"))
		}

		for i := 0; i < 3; i++ {
			etcdPodName := "etcd-" + strconv.Itoa(i)
			res = doOcpReq(oc, OcpExec, true, []string{"-n", guestClusterNamespace, etcdPodName, "--", "sh", "-c", etcdStatusCmd})
			if strings.Contains(res, "false, false") {
				e2e.Logf("find etcd follower etcd-%d, begin to delete this pod", i)

				//delete the first follower
				doOcpReq(oc, OcpDelete, true, []string{"-n", guestClusterNamespace, "pod", etcdPodName})

				//check the follower can be restarted and keep health
				err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
					status := doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "pod", etcdPodName, "-ojsonpath={.status.phase}"})
					if status == "Running" {
						return true, nil
					}
					return false, nil
				})
				exutil.AssertWaitPollNoErr(err, "etcd cluster health check error")

				//check the follower pod running
				status := doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "pod", etcdPodName, "-ojsonpath={.status.phase}"})
				o.Expect(status).To(o.ContainSubstring("Running"))

				//check the follower health
				execEtcdHealthCmd := append([]string{"-n", guestClusterNamespace, etcdPodName, "--", "sh", "-c"}, etcdHealthCmd)
				res = doOcpReq(oc, OcpExec, true, execEtcdHealthCmd)
				o.Expect(res).To(o.ContainSubstring("localhost:2379 is healthy"))

				break
			}
		}
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-46711-Test HCP components to use service account tokens", func() {
		g.By("hypershift OCP-46711-Test HCP components to use service account tokens")
		secrets := []string{
			//capi secret
			guestClusterName + "-node-mgmt-creds",
			//controlplaneoperator Secret
			guestClusterName + "-cpo-creds",
			//kubeapiSecret
			guestClusterName + "-cloud-ctrl-creds",
		}

		for _, sec := range secrets {
			cre := doOcpReq(oc, OcpGet, true, []string{"secret", sec, "-n", guestClusterNamespace, "-ojsonpath={.data.credentials}"})
			roleInfo, err := base64.StdEncoding.DecodeString(cre)
			o.Expect(err).ShouldNot(o.HaveOccurred())
			checkSubstring(string(roleInfo), []string{"role_arn", "web_identity_token_file"})
		}
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-44824-Resource requests/limit configuration for critical control plane workloads[Serial][Disruptive]", func() {
		g.By("hypershift OCP-44824-Resource requests/limit configuration for critical control plane workloads")

		cpuRequest := doOcpReq(oc, OcpGet, true, []string{"deployment", "kube-apiserver", "-n",
			guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.cpu}"})
		memoryRequest := doOcpReq(oc, OcpGet, true, []string{"deployment", "kube-apiserver", "-n",
			guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.memory}"})
		e2e.Logf("cpu request: %s, memory request: %s\n", cpuRequest, memoryRequest)

		defer func() {
			//change back to original cpu, memory value
			patchOptions := fmt.Sprintf("{\"spec\":{\"template\":{\"spec\": {\"containers\":"+
				"[{\"name\":\"kube-apiserver\",\"resources\":{\"requests\":{\"cpu\":\"%s\", \"memory\": \"%s\"}}}]}}}}", cpuRequest, memoryRequest)
			doOcpReq(oc, OcpPatch, true, []string{"deploy", "kube-apiserver", "-n", guestClusterNamespace, "-p", patchOptions})

			//check new value of cpu, memory resource
			err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
				cpuRes := doOcpReq(oc, OcpGet, true, []string{"deployment", "kube-apiserver", "-n",
					guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.cpu}"})
				if cpuRes != cpuRequest {
					return false, nil
				}

				memoryRes := doOcpReq(oc, OcpGet, true, []string{"deployment", "kube-apiserver", "-n",
					guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.memory}"})
				if memoryRes != memoryRequest {
					return false, nil
				}
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, "kube-apiserver cpu & memory resource change back error")
		}()

		//change cpu, memory resources
		desiredCpuRequest := "200m"
		desiredMemoryReqeust := "1700Mi"
		patchOptions := fmt.Sprintf("{\"spec\":{\"template\":{\"spec\": {\"containers\":"+
			"[{\"name\":\"kube-apiserver\",\"resources\":{\"requests\":{\"cpu\":\"%s\", \"memory\": \"%s\"}}}]}}}}", desiredCpuRequest, desiredMemoryReqeust)
		doOcpReq(oc, OcpPatch, true, []string{"deploy", "kube-apiserver", "-n", guestClusterNamespace, "-p", patchOptions})

		//check new value of cpu, memory resource
		err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			cpuRes := doOcpReq(oc, OcpGet, false, []string{"deployment", "kube-apiserver", "-n",
				guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.cpu}"})
			if cpuRes != desiredCpuRequest {
				return false, nil
			}

			memoryRes := doOcpReq(oc, OcpGet, false, []string{"deployment", "kube-apiserver", "-n",
				guestClusterNamespace, "-ojsonpath={.spec.template.spec.containers[?(@.name==\"kube-apiserver\")].resources.requests.memory}"})
			if memoryRes != desiredMemoryReqeust {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "kube-apiserver cpu & memory resource update error")
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-44926-Test priority classes for Hypershift control plane workloads", func() {
		g.By("hypershift OCP-44926-Test priority classes for Hypershift control plane workloads")

		//deployment
		priorityClasses := map[string][]string{
			"hypershift-api-critical": []string{
				"kube-apiserver",
				"oauth-openshift",
				"openshift-oauth-apiserver",
				"openshift-apiserver",
				"packageserver",
			},
			"hypershift-control-plane": []string{
				"capi-provider",
				"catalog-operator",
				"cluster-api",
				"cluster-autoscaler",
				"cluster-policy-controller",
				"control-plane-operator",
				"ignition-server",
				"kube-controller-manager",
				"kube-scheduler",
				"olm-operator",
				"openshift-controller-manager",
				//no etcd operator yet
				//"etcd-operator",
				"konnectivity-agent",
				"konnectivity-server",
				"cluster-version-operator",
				"hosted-cluster-config-operator",
				"certified-operators-catalog",
				"community-operators-catalog",
				"redhat-marketplace-catalog",
				"redhat-operators-catalog",
			},
		}

		for priority, components := range priorityClasses {
			e2e.Logf("priorityClass: %s %v\n", priority, components)
			for _, c := range components {
				res := doOcpReq(oc, OcpGet, true, []string{"deploy", c, "-n", guestClusterNamespace, "-ojsonpath={.spec.template.spec.priorityClassName}"})
				o.Expect(res).To(o.Equal(priority))
			}
		}

		//check statefulset for etcd
		etcdSts := "etcd"
		etcdPriorityClass := "hypershift-etcd"
		res := doOcpReq(oc, OcpGet, true, []string{"statefulset", etcdSts, "-n", guestClusterNamespace, "-ojsonpath={.spec.template.spec.priorityClassName}"})
		o.Expect(res).To(o.Equal(etcdPriorityClass))
	})

	// author: heli@redhat.com
	g.It("Author:heli-NonPreRelease-Critical-44942-Enable control plane deployment restart on demand[Serial]", func() {
		g.By("hypershift OCP-44942-Enable control plane deployment restart on demand")

		res := doOcpReq(oc, OcpGet, false, []string{"hostedcluster", guestClusterName, "-n", "clusters", "-ojsonpath={.metadata.annotations}"})
		e2e.Logf("get hostedcluster %s annotation: %s ", guestClusterName, res)

		var cmdClient = NewCmdClient()
		var restartDate string
		var err error

		systype := runtime.GOOS
		if systype == "darwin" {
			restartDate, err = cmdClient.Run("gdate --rfc-3339=date").Output()
			o.Expect(err).ShouldNot(o.HaveOccurred())
		} else if systype == "linux" {
			restartDate, err = cmdClient.Run("date --rfc-3339=date").Output()
			o.Expect(err).ShouldNot(o.HaveOccurred())
		} else {
			g.Skip("only available on linux or mac system")
		}

		annotationKey := "hypershift.openshift.io/restart-date"
		//value to be annotated
		restartAnnotation := fmt.Sprintf("%s=%s", annotationKey, restartDate)
		//annotations to be verified
		desiredAnnotation := fmt.Sprintf("\"%s\":\"%s\"", annotationKey, restartDate)

		//delete if already has this annotation
		existingAnno := doOcpReq(oc, OcpGet, false, []string{"hostedcluster", guestClusterName, "-n", "clusters", "-ojsonpath={.metadata.annotations}"})
		e2e.Logf("get hostedcluster %s annotation: %s ", guestClusterName, existingAnno)
		if strings.Contains(existingAnno, desiredAnnotation) {
			removeAnno := annotationKey + "-"
			doOcpReq(oc, OcpAnnotate, true, []string{"hostedcluster", guestClusterName, "-n", "clusters", removeAnno})
		}

		doOcpReq(oc, OcpAnnotate, true, []string{"hostedcluster", guestClusterName, "-n", "clusters", restartAnnotation})
		e2e.Logf("set hostedcluster %s annotation %s done ", guestClusterName, restartAnnotation)

		res = doOcpReq(oc, OcpGet, true, []string{"hostedcluster", guestClusterName, "-n", "clusters", "-ojsonpath={.metadata.annotations}"})
		e2e.Logf("get hostedcluster %s annotation: %s ", guestClusterName, res)
		o.Expect(res).To(o.ContainSubstring(desiredAnnotation))

		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			res = doOcpReq(oc, OcpGet, true, []string{"deploy", "kube-apiserver", "-n", guestClusterNamespace, "-ojsonpath={.spec.template.metadata.annotations}"})
			if strings.Contains(res, desiredAnnotation) {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "ocp-44942 hostedcluster restart annotation not found error")
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-44988-Colocate control plane components by default", func() {
		g.By("hypershift OCP-44988-Colocate control plane components by default")

		//deployment
		controlplaneComponents := []string{
			"kube-apiserver",
			"oauth-openshift",
			"openshift-oauth-apiserver",
			"openshift-apiserver",
			"capi-provider",
			"catalog-operator",
			"cluster-api",
			"cluster-autoscaler",
			"cluster-policy-controller",
			"control-plane-operator",
			"ignition-server",
			"kube-controller-manager",
			"kube-scheduler",
			"olm-operator",
			"openshift-controller-manager",
			//no etcd operator yet
			//"etcd-operator",
			"konnectivity-agent",
			"konnectivity-server",
			"cluster-version-operator",
			"hosted-cluster-config-operator",
			"packageserver",
			"certified-operators-catalog",
			"community-operators-catalog",
			"redhat-marketplace-catalog",
			"redhat-operators-catalog",
		}

		controlplanAffinityLabelKey := "hypershift.openshift.io/hosted-control-plane"
		controlplanAffinityLabelValue := guestClusterNamespace
		ocJsonpath := "-ojsonpath={.spec.template.spec.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels}"

		for _, component := range controlplaneComponents {
			res := doOcpReq(oc, OcpGet, true, []string{"deploy", component, "-n", guestClusterNamespace, ocJsonpath})
			o.Expect(res).To(o.ContainSubstring(controlplanAffinityLabelKey))
			o.Expect(res).To(o.ContainSubstring(controlplanAffinityLabelValue))
		}

		res := doOcpReq(oc, OcpGet, true, []string{"pod", "-n", guestClusterNamespace, "-l", controlplanAffinityLabelKey + "=" + controlplanAffinityLabelValue})
		checkSubstring(res, controlplaneComponents)
	})

	// author: heli@redhat.com
	g.It("Author:heli-Critical-44924-Test multi-zonal control plane components spread with HA mode enabled", func() {
		g.By("hypershift OCP-44924-Test multi-zonal control plane components spread with HA mode enabled")

		controlplaneMode := doOcpReq(oc, OcpGet, true, []string{"hostedcluster", guestClusterName, "-n", "clusters", "-ojsonpath={.spec.controllerAvailabilityPolicy}"})
		e2e.Logf("get hostedcluster %s controllerAvailabilityPolicy: %s ", guestClusterName, controlplaneMode)
		if controlplaneMode != "HighlyAvailable" {
			g.Skip("this is for guest cluster HA mode testrun, skip...")
		}

		//deployment
		controlplaneComponents := []string{
			"kube-apiserver",
			"oauth-openshift",
			"openshift-oauth-apiserver",
			"openshift-apiserver",
			"capi-provider",
			"cluster-api",
			"cluster-policy-controller",
			"ignition-server",
			"kube-controller-manager",
			"kube-scheduler",
			"openshift-controller-manager",
			"konnectivity-agent",
			"packageserver",
			//"certified-operators-catalog",
			//"cluster-version-operator",
			//"cluster-autoscaler",
			//"control-plane-operator",
			//"olm-operator",
			//"etcd-operator",
			//"konnectivity-server",
			//"catalog-operator",
			//"community-operators-catalog",
			//"redhat-marketplace-catalog",
			//"redhat-operators-catalog",
			//"hosted-cluster-config-operator",
		}

		antiAffinityJsonPath := ".spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution"
		topologyKeyJsonPath := antiAffinityJsonPath + "[*].topologyKey"
		desiredTopogyKey := "topology.kubernetes.io/zone"

		for _, c := range controlplaneComponents {
			doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "deploy", c, "-ojsonpath={" + antiAffinityJsonPath + "}"})
			res := doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "deploy", c, "-ojsonpath={" + topologyKeyJsonPath + "}"})
			o.Expect(res).To(o.ContainSubstring(desiredTopogyKey))
		}

		//check etcd
		etcdSts := "etcd"
		doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "statefulset", etcdSts, "-ojsonpath={" + antiAffinityJsonPath + "}"})
		res := doOcpReq(oc, OcpGet, true, []string{"-n", guestClusterNamespace, "statefulset", etcdSts, "-ojsonpath={" + topologyKeyJsonPath + "}"})
		o.Expect(res).To(o.ContainSubstring(desiredTopogyKey))
	})
})
