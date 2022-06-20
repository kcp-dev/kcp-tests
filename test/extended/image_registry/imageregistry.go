package imageregistry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc                   = exutil.NewCLI("default-image-registry", exutil.KubeConfigPath())
		errInfo              = "http.response.status=404"
		logInfo              = `Unsupported value: "abc": supported values: "", "Normal", "Debug", "Trace", "TraceAll"`
		updatePolicy         = `"maxSurge":0,"maxUnavailable":"10%"`
		monitoringns         = "openshift-monitoring"
		promPod              = "prometheus-k8s-0"
		patchAuthURL         = `"authURL":"invalid"`
		patchRegion          = `"regionName":"invaild"`
		patchDomain          = `"domain":"invaild"`
		patchDomainID        = `"domainID":"invalid"`
		patchTenantID        = `"tenantID":"invalid"`
		authErrInfo          = `Get "invalid/": unsupported`
		regionErrInfo        = "No suitable endpoint could be found"
		domainErrInfo        = "Failed to authenticate provider client"
		domainIDErrInfo      = "You must provide exactly one of DomainID or DomainName"
		tenantIDErrInfo      = "Authentication failed"
		queryCredentialMode  = "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query=cco_credentials_mode"
		imageRegistryBaseDir = exutil.FixturePath("testdata", "image_registry")
		requireRules         = "requiredDuringSchedulingIgnoredDuringExecution"
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-High-39027-Check AWS secret and access key with an OpenShift installed in a regular way", func() {
		output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Skip test when the cluster is with STS credential")
		token, err := getSAToken(oc, "prometheus-k8s", "openshift-monitoring")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())
		result, err := getBearerTokenURLViaPod(monitoringns, promPod, queryCredentialMode, token)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(result, "manualpodidentity") {
			g.Skip("Skip for the aws cluster with STS credential")
		}

		g.By("Check AWS secret and access key inside image registry pod")
		result, err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "cat", "/var/run/secrets/cloud/credentials").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("aws_access_key_id"))
		o.Expect(result).To(o.ContainSubstring("aws_secret_access_key"))
		g.By("Check installer-cloud-credentials secret")
		credentials, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/installer-cloud-credentials", "-n", "openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		fmt.Sprintf("credentials is %s", credentials)
		sDec, err := base64.StdEncoding.DecodeString(credentials)
		if err != nil {
			fmt.Printf("Error decoding string: %s ", err.Error())
		}
		o.Expect(sDec).To(o.ContainSubstring("aws_access_key_id"))
		o.Expect(sDec).To(o.ContainSubstring("aws_secret_access_key"))
		g.By("push/pull image to registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-39027", oc.Namespace())
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-34992-Add logLevel to registry config object with invalid value", func() {
		g.By("Change spec.loglevel with invalid values")
		out, _ := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"logLevel":"abc"}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring(logInfo))
		g.By("Change spec.operatorLogLevel with invalid values")
		out, _ = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"operatorLogLevel":"abc"}}`, "--type=merge").Output()
		o.Expect(out).To(o.ContainSubstring(logInfo))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Critial-24262-Image registry operator can read/overlap gloabl proxy setting [Disruptive]", func() {
		var (
			buildFile = filepath.Join(imageRegistryBaseDir, "inputimage.yaml")
			buildsrc  = bcSource{
				outname:   "inputimage",
				namespace: "",
				name:      "imagesourcebuildconfig",
				template:  buildFile,
			}
		)

		g.By("Check if it's a proxy cluster")
		output, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("proxy/cluster", "-o=jsonpath={.spec}").Output()
		if !strings.Contains(output, "httpProxy") {
			g.Skip("Skip for non-proxy platform")
		}
		g.By("Start a build and pull image from internal registry")
		oc.SetupProject()
		buildsrc.namespace = oc.Namespace()
		g.By("Create buildconfig")
		buildsrc.create(oc)
		g.By("starting a build to output internal imagestream")
		err := oc.Run("start-build").Args(buildsrc.outname).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), fmt.Sprintf("%s-1", buildsrc.outname), nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs(buildsrc.outname, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("starting a build using internal registry image")
		err = oc.Run("start-build").Args(buildsrc.name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildsrc.name+"-1", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs(buildsrc.name, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set wrong proxy to imageregistry cluster")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"proxy":{"http": "http://test:3128","https":"http://test:3128","noProxy":"test.no-proxy.com"}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Remove proxy for imageregistry cluster")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec": {"proxy": null}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
			result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "env").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).NotTo(o.ContainSubstring("HTTP_PROXY=http://test:3128"))
			o.Expect(result).NotTo(o.ContainSubstring("HTTPS_PROXY=http://test:3128"))
		}()
		recoverRegistryDefaultPods(oc)
		result, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "env").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("HTTP_PROXY=http://test:3128"))
		o.Expect(result).To(o.ContainSubstring("HTTPS_PROXY=http://test:3128"))
		g.By("starting  a build again and waiting for failure")
		br, err := exutil.StartBuildAndWait(oc, buildsrc.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		br.AssertFailure()
		g.By("expecting the build logs to indicate the image was rejected")
		buildLog, err := br.LogsNoTimestamp()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(buildLog).To(o.MatchRegexp("[Ee]rror.*initializing source docker://image-registry.openshift-image-registry.svc:5000"))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Critial-22893-PodAntiAffinity should work for image registry pod[Serial]", func() {
		g.Skip("According devel comments: https://bugzilla.redhat.com/show_bug.cgi?id=2014940, still not work,when find a solution, will enable it")
		g.By("Check platforms")
		//We set registry use pv on openstack&disconnect cluster, the case will fail on this scenario.
		//Skip all the fs volume test, only run on object storage backend.
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		var numi, numj int
		g.By("Add podAntiAffinity in image registry config")
		err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"podAffinityTerm":{"topologyKey":"kubernetes.io/hostname"},"weight":100}]}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"affinity":null}}`, "--type=merge").Execute()

		g.By("Set image registry replica to 3")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":3}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry replica to 2")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
		}()

		g.By("Confirm 3 pods scaled up")
		err = wait.Poll(1*time.Minute, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 3 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					e2e.Logf("Continue to next round")
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 3")

		g.By("At least 2 pods in different nodes")
		_, numj = comparePodHostIP(oc)
		o.Expect(numj >= 2).To(o.BeTrue())

		g.By("Set image registry replica to 4")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":4}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Set image registry replica to 2")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
		}()

		g.By("Confirm 4 pods scaled up")
		err = wait.Poll(50*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if len(podList.Items) != 4 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					e2e.Logf("Continue to next round")
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry pod list is not 4")

		g.By("Check 2 pods in the same node")
		numi, _ = comparePodHostIP(oc)
		o.Expect(numi >= 1).To(o.BeTrue())
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Low-43669-Update openshift-image-registry/node-ca DaemonSet using maxUnavailable", func() {
		g.By("Check node-ca updatepolicy")
		out := getResource(oc, asAdmin, withoutNamespace, "daemonset/node-ca", "-n", "openshift-image-registry", "-o=jsonpath={.spec.updateStrategy.rollingUpdate}")
		o.Expect(out).To(o.ContainSubstring(updatePolicy))
	})

	// author: xiuwang@redhat.com
	g.It("DisconnectedOnly-Author:xiuwang-High-43715-Image registry pullthough should support pull image from the mirror registry with auth via imagecontentsourcepolicy", func() {
		g.By("Create a imagestream using payload image with pullthrough policy")
		oc.SetupProject()
		err := exutil.WaitForAnImageStreamTag(oc, "openshift", "tools", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("tag").Args("openshift/tools:latest", "mytools:latest", "--reference-policy=local", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "mytools", "latest")

		g.By("Check the imagestream imported with digest id using pullthrough policy")
		err = oc.Run("set").Args("image-lookup", "mytools", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		expectInfo := `Successfully pulled image "image-registry.openshift-image-registry.svc:5000/` + oc.Namespace()
		createSimpleRunPod(oc, "mytools", expectInfo)
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-ConnectedOnly-27961-Create imagestreamtag with insufficient permissions [Disruptive]", func() {
		var (
			roleFile = filepath.Join(imageRegistryBaseDir, "role.yaml")
			rolesrc  = authRole{
				namespace: "",
				rolename:  "tag-bug-role",
				template:  roleFile,
			}
		)
		g.By("Import an image")
		oc.SetupProject()
		err := oc.Run("import-image").Args("test-img", "--from", "registry.access.redhat.com/rhel7", "--confirm").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create role with insufficient permissions")
		rolesrc.namespace = oc.Namespace()
		rolesrc.create(oc)
		err = oc.Run("create").Args("sa", "tag-bug-sa").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("policy").Args("add-role-to-user", "view", "-z", "tag-bug-sa", "--role-namespace", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("policy").Args("remove-role-from-user", "view", "tag-bug-sa", "--role-namespace", oc.Namespace()).Execute()
		out, _ := oc.Run("get").Args("sa", "tag-bug-sa", "-o=jsonpath={.secrets[0].name}", "-n", oc.Namespace()).Output()
		token, _ := oc.Run("get").Args("secret/"+out, "-o", `jsonpath={.data.\.dockercfg}`).Output()
		sDec, err := base64.StdEncoding.DecodeString(token)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("config").Args("set-credentials", "tag-bug-sa", fmt.Sprintf("--token=%s", sDec)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defuser, err := oc.Run("config").Args("get-users").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.Run("config").Args("current-context").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("config").Args("set-context", out, "--user=tag-bug-sa").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("config").Args("set-context", out, "--user="+defuser).Execute()

		g.By("Create imagestreamtag with insufficient permissions")
		err = oc.AsAdmin().Run("tag").Args("test-img:latest", "test-img:v1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check if new imagestreamtag created")
		out = getResource(oc, true, withoutNamespace, "istag", "-n", oc.Namespace())
		o.Expect(out).To(o.ContainSubstring("test-img:latest"))
		o.Expect(out).To(o.ContainSubstring("test-img:v1"))
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Medium-43664-Check ServiceMonitor of registry which will not hotloop CVO", func() {
		g.By("Check the servicemonitor of openshift-image-registry")
		out := getResource(oc, asAdmin, withoutNamespace, "servicemonitor", "-n", "openshift-image-registry", "-o=jsonpath={.items[1].spec.selector.matchLabels.name}")
		o.Expect(out).To(o.ContainSubstring("image-registry-operator"))

		g.By("Check CVO not hotloop due to registry")
		masterlogs, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("node-logs", "--role", "master", "--path=kube-apiserver/audit.log", "--raw").OutputToFile("audit.log")
		o.Expect(err).NotTo(o.HaveOccurred())

		result, err := exec.Command("bash", "-c", "cat "+masterlogs+" | grep verb.*update.*resource.*servicemonitors").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).NotTo(o.ContainSubstring("image-registry"))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-27985-Image with invalid resource name can be pruned", func() {
		//When registry configured pvc or emptryDir, the replicas is 1 and with recreate pod policy.
		//This is not suitable for the defer recoverage. Only run this case on cloud storage.
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		g.By("Config image registry to emptydir")
		defer recoverRegistryStorageConfig(oc)
		defer recoverRegistryDefaultReplicas(oc)
		configureRegistryStorageToEmptyDir(oc)

		g.By("Import image to internal registry")
		oc.SetupProject()
		//Change to use qe image to create build so we can run this on disconnect cluster.
		var invalidInfo = "Invalid image name foo/bar/" + oc.Namespace() + "/test-27985"
		checkRegistryFunctionFine(oc, "test-27985", oc.Namespace())

		g.By("Add system:image-pruner role to system:serviceaccount:openshift-image-registry:registry")
		err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "system:image-pruner", "system:serviceaccount:openshift-image-registry:registry").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-cluster-role-from-user", "system:image-pruner", "system:serviceaccount:openshift-image-registry:registry").Execute()

		g.By("Check invaild image source can be pruned")
		err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "mkdir", "-p", "/registry/docker/registry/v2/repositories/foo/bar").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "cp", "-r", "/registry/docker/registry/v2/repositories/"+oc.Namespace(), "/registry/docker/registry/v2/repositories/foo/bar").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "/bin/bash", "-c", "/usr/bin/dockerregistry -prune=check").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(invalidInfo))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-41414-There are 2 replicas for image registry on HighAvailable workers S3/Azure/GCS/Swift storage", func() {
		g.By("Check image registry pod")
		//We set registry use pv on openstack&disconnect cluster, the case will fail on this scenario.
		//Skip all the fs volume test, only run on object storage backend.
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 2)
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-41414", oc.Namespace())
	})

	//author: xiuwang@redhat.com
	g.It("Author:xiuwang-Critial-34680-Image registry storage cannot be removed if set to Unamanaged when image registry is set to Removed [Disruptive]", func() {
		g.By("Get registry storage info")
		var storageinfo1, storageinfo2, storageinfo3 string
		_, storageinfo1 = restoreRegistryStorageConfig(oc)
		g.By("Set image registry storage to Unmanaged, image registry operator to Removed")
		err := oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Removed","storage":{"managementState":"Unmanaged"}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			g.By("Recover image registry change")
			err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed","storage":{"managementState":"Managed"}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
		}()
		err = wait.Poll(25*time.Second, 2*time.Minute, func() (bool, error) {
			podList, err1 := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if err1 != nil {
				e2e.Logf("Error listing pods: %v", err)
				return false, nil
			}
			if len(podList.Items) != 0 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is not removed")
		_, storageinfo2 = restoreRegistryStorageConfig(oc)
		if strings.Compare(storageinfo1, storageinfo2) != 0 {
			e2e.Failf("Image stroage has changed")
		}
		g.By("Set image registry operator to Managed again")
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(25*time.Second, 2*time.Minute, func() (bool, error) {
			podList, err1 := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
			if err1 != nil {
				e2e.Logf("Error listing pods: %v", err)
				return false, nil
			}
			if len(podList.Items) == 0 {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is not recovered")
		_, storageinfo3 = restoreRegistryStorageConfig(oc)
		if strings.Compare(storageinfo1, storageinfo3) != 0 {
			e2e.Failf("Image stroage has changed")
		}
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Critical-21593-Check registry status by changing managementState for image-registry [Disruptive]", func() {
		g.By("Check platforms")
		//We set registry use pv on openstack&disconnect cluster, the case will fail on this scenario.
		//Skip all the fs volume test, only run on object storage backend.
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		g.By("Change managementSet from Managed -> Removed")
		defer func() {
			g.By("Set image registry cluster Managed")
			oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
			recoverRegistryDefaultPods(oc)
		}()
		err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Removed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check image-registry pods are removed")
		checkRegistrypodsRemoved(oc)

		g.By("Change managementSet from Removed to Managed")
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		recoverRegistryDefaultPods(oc)

		g.By("Change managementSet from Managed to Unmanaged")
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Update replicas to 1")
		defer oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas": 2}}`, "--type=merge").Execute()
		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"replicas": 1}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check image registry pods are still 2")
		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-image-registry").List(metav1.ListOptions{LabelSelector: "docker-registry=default"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(podList.Items)).Should(o.Equal(2))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-High-45952-ConnectedOnly-Imported imagestreams should success in deploymentconfig", func() {
		var (
			statefulsetFile = filepath.Join(imageRegistryBaseDir, "statefulset.yaml")
			statefulsetsrc  = staSource{
				namespace: "",
				name:      "example-statefulset",
				template:  statefulsetFile,
			}
		)
		g.By("Import an image stream and set image-lookup")
		oc.SetupProject()
		err := oc.Run("import-image").Args("registry.access.redhat.com/ubi8/ubi", "--scheduled", "--confirm", "--reference-policy=local").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "ubi", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("image-lookup", "ubi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the initial statefulset")
		statefulsetsrc.namespace = oc.Namespace()
		g.By("Create statefulset")
		statefulsetsrc.create(oc)
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)

		g.By("Reapply the sample yaml")
		applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", statefulsetsrc.template, "-p", "NAME="+statefulsetsrc.name, "NAMESPACE="+statefulsetsrc.namespace)
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)

		g.By("setting a trigger, pods are still running")
		err = oc.Run("set").Args("triggers", "statefulset/example-statefulset", "--from-image=ubi:latest", "--containers", "example-statefulset").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check the pods are running")
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=example-statefulset", 3)
		interReg := "image-registry.openshift-image-registry.svc:5000/" + oc.Namespace() + "/ubi"
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("pods", "-o=jsonpath={.items[*].spec.containers[*].image}", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(interReg))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-39028-Check aws secret and access key with an openShift installed with an STS credential", func() {
		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check if the cluster is with STS credential")
		token, err := getSAToken(oc, "prometheus-k8s", "openshift-monitoring")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())
		result, err := getBearerTokenURLViaPod(monitoringns, promPod, queryCredentialMode, token)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(result, "manualpodidentity") {
			g.Skip("Skip for the aws cluster without STS credential")
		}

		g.By("Check role_arn/web_identity_token_file inside image registry pod")
		result, err = oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-image-registry", "deployment.apps/image-registry", "cat", "/var/run/secrets/cloud/credentials").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("role_arn"))
		o.Expect(result).To(o.ContainSubstring("web_identity_token_file"))

		g.By("Check installer-cloud-credentials secret")
		credentials, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/installer-cloud-credentials", "-n", "openshift-image-registry", "-o=jsonpath={.data.credentials}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		sDec, _ := base64.StdEncoding.DecodeString(credentials)
		if !strings.Contains(string(sDec), "role_arn") {
			e2e.Failf("credentials does not contain role_arn")
		}
		if !strings.Contains(string(sDec), "web_identity_token_file") {
			e2e.Failf("credentials does not contain web_identity_token_file")
		}

		g.By("push/pull image to registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-39028", oc.Namespace())
	})

	//author: xiuwang@redhat.com
	g.It("NonPreRelease-Author:xiuwang-High-45540-Registry should fall back to secondary ImageContentSourcePolicy Mirror [Disruptive]", func() {
		var (
			icspFile = filepath.Join(imageRegistryBaseDir, "icsp-multi-mirrors.yaml")
			icspsrc  = icspSource{
				name:     "image-policy-fake",
				template: icspFile,
			}
		)
		g.By("Create imagecontentsourcepolicy with multiple mirrors")
		defer icspsrc.delete(oc)
		icspsrc.create(oc)

		g.By("Get all nodes list")
		nodeList, err := exutil.GetAllNodes(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check registry configs in all nodes")
		err = wait.Poll(25*time.Second, 2*time.Minute, func() (bool, error) {
			for _, nodeName := range nodeList {
				output, err := exutil.DebugNodeWithChroot(oc, nodeName, "bash", "-c", "cat /etc/containers/registries.conf | grep fake.rhcloud.com")
				o.Expect(err).NotTo(o.HaveOccurred())
				if !strings.Contains(output, "fake.rhcloud.com") {
					e2e.Logf("Continue to next round")
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "registry configs are not changed")

		g.By("Create a pod to check pulling issue")
		oc.SetupProject()
		err = exutil.WaitForAnImageStreamTag(oc, "openshift", "cli", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("deployment", "cli-test", "--image", "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get project events")
		err = wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
			events, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(events, `Successfully pulled image "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest"`) {
				e2e.Logf("Continue to next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image pulls failed")
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-23583-Registry should not try to pullthrough himself by any name [Serial]", func() {
		g.By("Create route to expose the registry")
		defer restoreRouteExposeRegistry(oc)
		createRouteExposeRegistry(oc)

		g.By("Get server host")
		defroute := getRegistryDefaultRoute(oc)
		userroute := strings.Replace(defroute, "default", "extra", 1)
		patchInfo := fmt.Sprintf("{\"spec\":{\"routes\":[{\"hostname\": \"%s\", \"name\":\"extra-image-registry\", \"secretName\":\"\"}]}}", userroute)
		defer oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"routes":null}}`, "--type=merge").Execute()
		err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", patchInfo, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get token from secret")
		oc.SetupProject()
		token, err := getSAToken(oc, "builder", oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())

		g.By("Create a secret for user-defined route")
		err = oc.WithoutNamespace().AsAdmin().Run("create").Args("secret", "docker-registry", "mysecret", "--docker-server="+userroute, "--docker-username="+oc.Username(), "--docker-password="+token, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Import an image")
		//Use multiarch image with digest, so it could be test on ARM cluster and disconnect cluster.
		err = oc.WithoutNamespace().AsAdmin().Run("import-image").Args("myimage", "--from=quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", "--confirm", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "myimage", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Tag the image point to itself address")
		err = oc.WithoutNamespace().AsAdmin().Run("tag").Args(userroute+"/"+oc.Namespace()+"/myimage", "myimage:test", "--insecure=true", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "myimage", "test")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check import successfully")
		err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			successInfo := userroute + "/" + oc.Namespace() + "/myimage@sha256"
			output, err := oc.WithoutNamespace().AsAdmin().Run("describe").Args("is", "myimage", "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if o.Expect(output).To(o.ContainSubstring(successInfo)) {
				return true, nil
			}
			e2e.Logf("Continue to next round")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Import failed"))
		g.By("Get blobs from the default registry")
		getURL := "curl -Lks -u \"" + oc.Username() + ":" + token + "\" -I HEAD https://" + defroute + "/v2/" + oc.Namespace() + "/myimage@sha256:0000000000000000000000000000000000000000000000000000000000000000"
		curlOutput, err := exec.Command("bash", "-c", getURL).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(curlOutput)).To(o.ContainSubstring("404 Not Found"))
		podsOfImageRegistry := []corev1.Pod{}
		podsOfImageRegistry = listPodStartingWith("image-registry", oc, "openshift-image-registry")
		if len(podsOfImageRegistry) == 0 {
			e2e.Failf("Error retrieving logs")
		}
		foundErrLog := false
		foundErrLog = dePodLogs(podsOfImageRegistry, oc, errInfo)
		o.Expect(foundErrLog).To(o.BeTrue())
	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Longduration-Author:jitli-ConnectedOnly-VMonly-Medium-33051-Images can be imported from an insecure registry without 'insecure: true' if it is in insecureRegistries in image.config/cluster [Disruptive]", func() {

		workNode, _ := exutil.GetFirstWorkerNode(oc)
		e2e.Logf(workNode)
		defer func() {
			err := wait.Poll(30*time.Second, 6*time.Minute, func() (bool, error) {
				regStatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf | grep \"docker.io\"")
				if !strings.Contains(regStatus, "location = \"docker.io\"") {
					e2e.Logf("registries.conf updated")
					return true, nil
				}
				e2e.Logf("registries.conf not update")
				return false, nil

			})
			exutil.AssertWaitPollNoErr(err, "registries.conf not contains docker.io")
		}()
		g.By("import image from an insecure registry directly without --insecure=true")
		output, err := oc.WithoutNamespace().AsAdmin().Run("import-image").Args("image-33051", "--from=registry.access.redhat.com/rhel7").Output()
		o.Expect(err).To(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}

		g.By("Create route to expose the registry")
		defer restoreRouteExposeRegistry(oc)
		createRouteExposeRegistry(oc)

		g.By("Get server host")
		host := getRegistryDefaultRoute(oc)

		g.By("Get token from secret")
		oc.SetupProject()
		token, err := getSAToken(oc, "builder", oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())

		g.By("Create a secret for user-defined route")
		err = oc.WithoutNamespace().AsAdmin().Run("create").Args("secret", "docker-registry", "secret33051", "--docker-server="+host, "--docker-username="+oc.Username(), "--docker-password="+token, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Add the insecure registry to images.config.openshift.io cluster")
		defer oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"registrySources": null}}`, "--type=merge").Execute()
		output, err = oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"registrySources": {"insecureRegistries": ["`+host+`"]}}}`, "--type=merge").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("registries.conf gets updated")
		err = wait.Poll(30*time.Second, 6*time.Minute, func() (bool, error) {
			registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf | grep default-route-openshift-image-registry.apps")
			if strings.Contains(registriesstatus, "default-route-openshift-image-registry.apps") {
				e2e.Logf("registries.conf updated")
				return true, nil
			}
			e2e.Logf("registries.conf not update")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "registries.conf not update")

		g.By("Tag the image")
		output, err = oc.WithoutNamespace().AsAdmin().Run("tag").Args(host+"/openshift/ruby:latest", "ruby:33051", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Tag ruby:33051 set"))

		g.By("Add docker.io to blockedRegistries list")
		defer oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"additionalTrustedCA": null,"registrySources": null}}`, "--type=merge").Execute()
		output, err = oc.AsAdmin().Run("patch").Args("images.config.openshift.io/cluster", "-p", `{"spec": {"additionalTrustedCA": {"name": ""},"registrySources": {"blockedRegistries": ["docker.io"]}}}`, "--type=merge").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("registries.conf gets updated")
		err = wait.Poll(30*time.Second, 6*time.Minute, func() (bool, error) {
			registriesstatus, _ := exutil.DebugNodeWithChroot(oc, workNode, "bash", "-c", "cat /etc/containers/registries.conf | grep \"docker.io\"")
			if strings.Contains(registriesstatus, "location = \"docker.io\"") {
				e2e.Logf("registries.conf updated")
				return true, nil
			}
			e2e.Logf("registries.conf not update")
			return false, nil

		})
		exutil.AssertWaitPollNoErr(err, "registries.conf not contains docker.io")

		g.By("Import an image from docker.io")
		output, err = oc.WithoutNamespace().AsAdmin().Run("import-image").Args("image2-33051", "--from=docker.io/centos/ruby-22-centos7", "--confirm=true").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("error: Import failed (Forbidden): forbidden: registry docker.io blocked"))

	})

	// author: wewang@redhat.com
	g.It("NonPreRelease-Author:wewang-Critical-24838-Registry OpenStack Storage test with invalid settings [Disruptive]", func() {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "OpenStack") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Set a different container in registry config")
		oricontainer, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("configs.imageregistry/cluster", "-o=jsonpath={.spec.storage.swift.container}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		newcontainer := strings.Replace(oricontainer, "image", "images", 1)
		defer func() {
			err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{"container": "`+oricontainer+`"}}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			recoverRegistryDefaultPods(oc)
		}()
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"swift":{"container": "`+newcontainer+`"}}}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		recoverRegistryDefaultPods(oc)

		g.By("Set invalid authURL in image registry crd")
		foundErrLog := false
		foundErrLog = setImageregistryConfigs(oc, patchAuthURL, authErrInfo)
		o.Expect(foundErrLog).To(o.BeTrue())

		g.By("Set invalid regionName")
		foundErrLog = false
		foundErrLog = setImageregistryConfigs(oc, patchRegion, regionErrInfo)
		o.Expect(foundErrLog).To(o.BeTrue())

		g.By("Set invalid domain")
		foundErrLog = false
		foundErrLog = setImageregistryConfigs(oc, patchDomain, domainErrInfo)
		o.Expect(foundErrLog).To(o.BeTrue())

		g.By("Set invalid domainID")
		foundErrLog = false
		foundErrLog = setImageregistryConfigs(oc, patchDomainID, domainIDErrInfo)
		o.Expect(foundErrLog).To(o.BeTrue())

		g.By("Set invalid tenantID")
		foundErrLog = false
		foundErrLog = setImageregistryConfigs(oc, patchTenantID, tenantIDErrInfo)
		o.Expect(foundErrLog).To(o.BeTrue())
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Critical-47274-Image registry works with OSS storage on alibaba cloud", func() {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AlibabaCloud") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Check OSS storage")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.storage.oss}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("bucket"))
		o.Expect(output).To(o.ContainSubstring(`"endpointAccessibility":"Internal"`))
		o.Expect(output).To(o.ContainSubstring("region"))
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.conditions[?(@.type==\"StorageEncrypted\")].message}{.status.conditions[?(@.type==\"StorageEncrypted\")].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Default AES256 encryption was successfully enabled on the OSS bucketTrue"))

		g.By("Check if registry operator degraded")
		registryDegrade := checkRegistryDegraded(oc)
		if registryDegrade {
			e2e.Failf("Image registry is degraded")
		}

		g.By("Check if registry works well")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-47274", oc.Namespace())

		g.By("Check if registry interact with OSS used the internal endpoint")
		output, err = oc.WithoutNamespace().AsAdmin().Run("logs").Args("deploy/image-registry", "--since=30s", "-n", "openshift-image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("internal.aliyuncs.com"))

	})

	// author: xiuwang@redhat.com
	g.It("NonPreRelease-Author:xiuwang-Medium-47342-Configure image registry works with OSS parameters [Disruptive]", func() {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AlibabaCloud") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Configure OSS with Public endpoint")
		defer oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"oss":{"endpointAccessibility":null}}}}`, "--type=merge").Execute()
		output, err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"oss":{"endpointAccessibility":"Public"}}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
			registryDegrade := checkRegistryDegraded(oc)
			if registryDegrade {
				e2e.Logf("wait for next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is degraded")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-47342", oc.Namespace())
		output, err = oc.WithoutNamespace().AsAdmin().Run("logs").Args("deploy/image-registry", "--since=1m", "-n", "openshift-image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("internal.aliyuncs.com"))

		g.By("Configure registry to use KMS encryption type")
		defer oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"oss":{"encryption":null}}}}`, "--type=merge").Execute()
		output, err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"storage":{"oss":{"encryption":{"method":"KMS","kms":{"keyID":"invalidid"}}}}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
			output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image", "cluster", "-o=jsonpath={.status.conditions[?(@.type==\"StorageEncrypted\")].message}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(output, "Default KMS encryption was successfully enabled on the OSS bucket") {
				e2e.Logf("wait for next round")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Default encryption can't be changed")
		br, err := exutil.StartBuildAndWait(oc, "test-47342")
		o.Expect(err).NotTo(o.HaveOccurred())
		br.AssertFailure()
		output, err = oc.WithoutNamespace().AsAdmin().Run("logs").Args("deploy/image-registry", "--since=1m", "-n", "openshift-image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("The specified parameter KMS keyId is not valid"))
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Critical-45345-Image registry works with ibmcos storage on IBM cloud", func() {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "IBMCloud") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Check ibmcos storage")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.storage.ibmcos}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("bucket"))
		o.Expect(output).To(o.ContainSubstring("location"))
		o.Expect(output).To(o.ContainSubstring("resourceGroupName"))
		o.Expect(output).To(o.ContainSubstring("resourceKeyCRN"))
		o.Expect(output).To(o.ContainSubstring("serviceInstanceCRN"))

		g.By("Check if registry operator degraded")
		registryDegrade := checkRegistryDegraded(oc)
		if registryDegrade {
			e2e.Failf("Image registry is degraded")
		}

		g.By("Check if registry works well")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-45345", oc.Namespace())
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ConnectedOnly-Medium-41398-Users providing custom AWS tags are set with bucket creation [Disruptive]", func() {

		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}
		g.By("Check the cluster is with resourceTags")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.aws}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "resourceTags") {
			g.Skip("Skip for no resourceTags")
		}
		g.By("Get bucket name")
		bucket, err := oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bucket).NotTo(o.BeEmpty())

		g.By("Check the tags")
		aws := getAWSClient(oc)
		tag, err := awsGetBucketTagging(aws, bucket)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))

		g.By("Removed managementState")
		defer func() {
			status, err := oc.AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.spec.managementState}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if status != "Managed" {
				e2e.Logf("recover config.image cluster is Managed")
				output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"managementState": "Managed"}}`, "--type=merge").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(string(output)).To(o.ContainSubstring("patched"))
			} else {
				e2e.Logf("config.image cluster is Managed")
			}
		}()
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"managementState": "Removed"}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check bucket has been deleted")
		err = wait.Poll(2*time.Second, 10*time.Second, func() (bool, error) {
			tag, err = awsGetBucketTagging(aws, bucket)
			if err != nil && strings.Contains(tag, "The specified bucket does not exist") {
				return true, nil
			}
			e2e.Logf("bucket still exist, go next round")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "the bucket isn't been deleted")

		g.By("Managed managementState")
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"managementState": "Managed"}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Get new bucket name and check")
		err = wait.Poll(10*time.Second, 2*time.Minute, func() (bool, error) {
			bucket, _ = oc.AsAdmin().Run("get").Args("config.image", "-o=jsonpath={..spec.storage.s3.bucket}").Output()
			if strings.Compare(bucket, "") != 0 {
				return true, nil
			}
			e2e.Logf("not update")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "Can't get bucket")

		tag, err = awsGetBucketTagging(aws, bucket)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(tag)).To(o.ContainSubstring("customTag"))
		o.Expect(string(tag)).To(o.ContainSubstring("installer-qe"))
	})

	// author: tbuskey@redhat.com
	g.It("Author:tbuskey-High-22056-Registry operator configure prometheus metric gathering", func() {
		var (
			authHeader         string
			after              = make(map[string]int)
			before             = make(map[string]int)
			data               prometheusImageregistryQueryHTTP
			err                error
			fails              = 0
			failItems          = ""
			l                  int
			msg                string
			prometheusURL      = "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1"
			prometheusURLQuery string
			query              string
			token              string
			metrics            = []string{"imageregistry_http_request_duration_seconds_count",
				"imageregistry_http_request_size_bytes_count",
				"imageregistry_http_request_size_bytes_sum",
				"imageregistry_http_response_size_bytes_count",
				"imageregistry_http_response_size_bytes_sum",
				"imageregistry_http_request_size_bytes_count",
				"imageregistry_http_request_size_bytes_sum",
				"imageregistry_http_requests_total",
				"imageregistry_http_response_size_bytes_count",
				"imageregistry_http_response_size_bytes_sum"}
		)

		g.By("Get Prometheus token")
		token, err = getSAToken(oc, "prometheus-k8s", "openshift-monitoring")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())
		authHeader = fmt.Sprintf("Authorization: Bearer %v", token)

		g.By("Collect metrics at start")
		for _, query = range metrics {
			prometheusURLQuery = fmt.Sprintf("%v/query?query=%v", prometheusURL, query)
			msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "-i", "--", "curl", "-k", "-H", authHeader, prometheusURLQuery).Outputs()
			o.Expect(msg).NotTo(o.BeEmpty())
			json.Unmarshal([]byte(msg), &data)
			l = len(data.Data.Result) - 1
			before[query], err = strconv.Atoi(data.Data.Result[l].Value[1].(string))
			// e2e.Logf("query %v ==  %v", query, before[query])
		}
		g.By("pause to get next metrics")
		time.Sleep(60 * time.Second)

		g.By("Collect metrics again")
		for _, query = range metrics {
			prometheusURLQuery = fmt.Sprintf("%v/query?query=%v", prometheusURL, query)
			msg, _, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-monitoring", "-c", "prometheus", "prometheus-k8s-0", "-i", "--", "curl", "-k", "-H", authHeader, prometheusURLQuery).Outputs()
			o.Expect(msg).NotTo(o.BeEmpty())
			json.Unmarshal([]byte(msg), &data)
			l = len(data.Data.Result) - 1
			after[query], err = strconv.Atoi(data.Data.Result[l].Value[1].(string))
			// e2e.Logf("query %v ==  %v", query, before[query])
		}

		g.By("results")
		for _, query = range metrics {
			msg = "."
			if before[query] > after[query] {
				fails++
				failItems = fmt.Sprintf("%v%v ", failItems, query)
			}
			e2e.Logf("%v -> %v %v", before[query], after[query], query)
			// need to test & compare
		}
		if fails != 0 {
			e2e.Failf("\nFAIL: %v metrics decreasesd: %v\n\n", fails, failItems)
		}

		g.By("Success")
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Medium-47933-DeploymentConfigs template should respect resolve-names annotation", func() {
		var (
			imageRegistryBaseDir = exutil.FixturePath("testdata", "image_registry")
			podFile              = filepath.Join(imageRegistryBaseDir, "dc-template.yaml")
			podsrc               = podSource{
				name:      "mydc",
				namespace: "",
				image:     "myis",
				template:  podFile,
			}
		)

		g.By("Use source imagestream to create dc")
		oc.SetupProject()
		err := oc.AsAdmin().WithoutNamespace().Run("tag").Args("quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", "myis:latest", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), podsrc.image, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		podsrc.namespace = oc.Namespace()
		podsrc.create(oc)
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploymentconfig/mydc", "-o=jsonpath={..spec.containers[*].image}", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("quay.io/openshifttest/busybox"))

		g.By("Use pullthrough imagestream to create dc")
		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args("quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", "myis:latest", "--reference-policy=local", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), podsrc.image, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		podsrc.create(oc)
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deploymentconfig/mydc", "-o=jsonpath={..spec.template.spec.containers[*].image}", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("image-registry.openshift-image-registry.svc:5000/" + oc.Namespace() + "/" + podsrc.image))
	})

	g.It("NonPreRelease-Author:xiuwang-VMonly-Critical-43260-Image registry pod could report to processing after openshift-apiserver reports unconnect quickly[Disruptive][Slow]", func() {
		firstMaster, err := exutil.GetFirstMasterNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		clusterID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if exutil.CheckPlatform(oc) == "none" && strings.HasPrefix(firstMaster, "master") && !strings.HasPrefix(firstMaster, clusterID) && !strings.HasPrefix(firstMaster, "internal") {
			defer oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"tolerations":[]}}`, "--type=merge").Output()
			output, err := oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"tolerations":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"}]}}`, "--type=merge").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(output)
			newCheck("expect", asAdmin, withoutNamespace, contain, "Running", ok, []string{"pods", "-n", "openshift-image-registry", "-l", "docker-registry=default"}).check(oc)
			names, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-image-registry", "-l", "docker-registry=default", "-o", "name").Output()
			if err != nil {
				e2e.Failf("Fail to get the image-registry pods' name")
			}
			podNames := strings.Split(names, "\n")
			privateKeyPath := "/root/openshift-qe.pem"
			var nodeNames []string

			for _, podName := range podNames {
				e2e.Logf("get the node name of pod name: %s", podName)
				nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-image-registry", podName, "-o=jsonpath={.spec.nodeName}").Output()
				e2e.Logf("node name: %s", nodeName)
				if err != nil {
					e2e.Failf("Fail to get the node name")
				}
				nodeNames = append(nodeNames, nodeName)
			}

			for _, nodeName := range nodeNames {

				e2e.Logf("stop crio service of node: %s", nodeName)
				defer exec.Command("bash", "-c", "ssh -o StrictHostKeyChecking=no -i "+privateKeyPath+" core@"+nodeName+" sudo systemctl start crio").CombinedOutput()
				defer exec.Command("bash", "-c", "ssh -o StrictHostKeyChecking=no -i "+privateKeyPath+" core@"+nodeName+" sudo systemctl start kubelet").CombinedOutput()
				output, _ := exec.Command("bash", "-c", "ssh -o StrictHostKeyChecking=no -i "+privateKeyPath+" core@"+nodeName+" sudo systemctl stop crio").CombinedOutput()
				e2e.Logf("stop crio command result : %s", output)
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("stop service of node: %s", nodeName)
				output, _ = exec.Command("bash", "-c", "ssh -o StrictHostKeyChecking=no -i "+privateKeyPath+" core@"+nodeName+" sudo systemctl stop kubelet").CombinedOutput()
				e2e.Logf("stop kubelet command result : %s", output)
				o.Expect(err).NotTo(o.HaveOccurred())
				newCheck("expect", asAdmin, withoutNamespace, contain, "NodeStatusUnknown", ok, []string{"node", nodeName, "-o=jsonpath={.status.conditions..reason}"}).check(oc)
			}
			newCheck("expect", asAdmin, withoutNamespace, contain, "True", ok, []string{"co", "image-registry", "-o=jsonpath={.status.conditions[?(@.type==\"Progressing\")].status}"}).check(oc)
			err = wait.Poll(10*time.Second, 330*time.Second, func() (bool, error) {
				res, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "image-registry", "-o=jsonpath={.status.conditions[?(@.type==\"Available\")].status}").Output()
				if strings.Contains(res, "True") {
					return true, nil
				}
				e2e.Logf(" Available command result : %s", res)
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		e2e.Logf("Only baremetal platform supported for the test")
	})

	g.It("NonPreRelease-VMonly-Author:xiuwang-Medium-48045-Update global pull secret for additional private registries[Disruptive]", func() {
		g.By("Setup a private registry")
		oc.SetupProject()
		var regUser, regPass = "testuser", getRandomString()
		tempDataDir, err := extractPullSecret(oc)
		defer os.RemoveAll(tempDataDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		originAuth := filepath.Join(tempDataDir, ".dockerconfigjson")
		htpasswdFile, err := generateHtpasswdFile(tempDataDir, regUser, regPass)
		defer os.RemoveAll(htpasswdFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		regRoute := setSecureRegistryEnableAuth(oc, oc.Namespace(), "myregistry", htpasswdFile, "quay.io/openshifttest/registry@sha256:01493571d994fd021da18c1f87aba1091482df3fc20825f443b4e60b3416c820")

		g.By("Push image to private registry")
		newAuthFile, err := appendPullSecretAuth(originAuth, regRoute, regUser, regPass)
		o.Expect(err).NotTo(o.HaveOccurred())
		myimage := regRoute + "/" + oc.Namespace() + "/myimage:latest"
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", "quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", myimage, "--insecure", "-a", newAuthFile, "--keep-manifest-list=true", "--filter-by-os=.*").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Make sure the image can't be pulled without auth")
		output, err := oc.AsAdmin().WithoutNamespace().Run("import-image").Args("firstis:latest", "--from="+myimage, "--reference-policy=local", "--insecure", "--confirm", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(output)).To(o.ContainSubstring("Unauthorized"))

		g.By("Update pull secret")
		updatePullSecret(oc, newAuthFile)
		defer updatePullSecret(oc, originAuth)
		err = wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			podList, _ := oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver").List(metav1.ListOptions{LabelSelector: "apiserver=true"})
			for _, pod := range podList.Items {
				output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-apiserver", pod.Name, "--", "bash", "-c", "cat /var/lib/kubelet/config.json").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if !strings.Contains(output, oc.Namespace()) {
					e2e.Logf("Go to next round")
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Failed to update apiserver")

		g.By("Make sure the image can be pulled after add auth")
		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(myimage, "newis:latest", "--reference-policy=local", "--insecure", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "newis", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-Medium-43731-Image registry pods should have anti-affinity rules", func() {
		g.By("Check platforms")
		//We set registry use pv on openstack&disconnect cluster, the case will fail on this scenario.
		//Skip all the fs volume test, only run on object storage backend.
		//pods anti-affinity sets when registry's replicas is 2
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		g.By("Check pods anti-affinity match requiredDuringSchedulingIgnoredDuringExecution rule when replicas is 2")
		foundrequiredRules := false
		foundrequiredRules = foundAffinityRules(oc, requireRules)
		o.Expect(foundrequiredRules).To(o.BeTrue())

		/*
		   when https://bugzilla.redhat.com/show_bug.cgi?id=2000940 is fixed, will open this part
		   		g.By("Set deployment.apps replica to 0")
		   		err = oc.WithoutNamespace().AsAdmin().Run("patch").Args("deployment.apps/image-registry", "-p", `{"spec":{"replicas":0}}`, "--type=merge", "-n", "openshift-image-registry").Execute()
		   		o.Expect(err).NotTo(o.HaveOccurred())
		   		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("co/image-registry", "-o=jsonpath={.status.conditions[0]}").Output()
		   		o.Expect(err).NotTo(o.HaveOccurred())
		   		o.Expect(output).To(o.ContainSubstring("\"status\":\"False\""))
		   		o.Expect(output).To(o.ContainSubstring("The deployment does not have available replicas"))
		   		o.Expect(output).To(o.ContainSubstring("\"type\":\"Available\""))
		   		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.imageregistry/cluster", "-o=jsonpath={.status.readyReplicas}").Output()
		   		o.Expect(err).NotTo(o.HaveOccurred())
		   		o.Expect(output).To(o.Equal("0"))
		*/
	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Author:jitli-Critical-34895-Image registry can work well on Gov Cloud with custom endpoint defined [Disruptive]", func() {

		g.By("Check platforms")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure.config.openshift.io", "-o=jsonpath={..status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "AWS") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Check the cluster is with us-gov")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.storage.s3.region}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "us-gov") {
			g.Skip("Skip for wrong region")
		}

		g.By("Set regionEndpoint if it not set")
		regionEndpoint, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.storage.s3.regionEndpoint}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(regionEndpoint, "https://s3.us-gov-west-1.amazonaws.com") {
			defer func() {
				output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"storage":{"s3":{"regionEndpoint": null ,"virtualHostedStyle": null}}}}`, "--type=merge").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(string(output)).To(o.ContainSubstring("patched"))
			}()
			output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"storage":{"s3":{"regionEndpoint": "https://s3.us-gov-west-1.amazonaws.com" ,"virtualHostedStyle": true}}}}`, "--type=merge").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))
		}

		err = wait.Poll(2*time.Second, 10*time.Second, func() (bool, error) {
			regionEndpoint, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("config.image/cluster", "-o=jsonpath={.status.storage.s3.regionEndpoint}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(regionEndpoint, "https://s3.us-gov-west-1.amazonaws.com") {
				return true, nil
			}
			e2e.Logf("regionEndpoint not found, go next round")
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "regionEndpoint not found")

		g.By("Check if registry operator degraded")
		err = wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			registryDegrade := checkRegistryDegraded(oc)
			if registryDegrade {
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Image registry is degraded")

		g.By("Import an image with reference-policy=local")
		oc.SetupProject()
		err = oc.WithoutNamespace().AsAdmin().Run("import-image").Args("image-34895", "--from=registry.access.redhat.com/rhel7", "--reference-policy=local", "--confirm", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Start a build")
		checkRegistryFunctionFine(oc, "test-34895", oc.Namespace())

	})

	// author: xiuwangredhat.com
	g.It("VMonly-Author:xiuwang-Critical-48744-Pull through for images that have dots in their namespace", func() {

		g.By("Setup a private registry")
		oc.SetupProject()
		var regUser, regPass = "testuser", getRandomString()
		tempDataDir := filepath.Join("/tmp/", fmt.Sprintf("ir-%s", getRandomString()))
		defer os.RemoveAll(tempDataDir)
		err := os.Mkdir(tempDataDir, 0755)
		if err != nil {
			e2e.Logf("Fail to create directory: %v", err)
		}
		htpasswdFile, err := generateHtpasswdFile(tempDataDir, regUser, regPass)
		defer os.RemoveAll(htpasswdFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		regRoute := setSecureRegistryEnableAuth(oc, oc.Namespace(), "myregistry", htpasswdFile, "quay.io/openshifttest/registry@sha256:01493571d994fd021da18c1f87aba1091482df3fc20825f443b4e60b3416c820")

		g.By("Create secret for the private registry")
		err = oc.WithoutNamespace().AsAdmin().Run("create").Args("secret", "docker-registry", "myregistry-auth", "--docker-username="+regUser, "--docker-password="+regPass, "--docker-server="+regRoute, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("extract").Args("secret/myregistry-auth", "-n", oc.Namespace(), "--confirm", "--to="+tempDataDir).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Mirror image with dots in namespace")
		myimage := regRoute + "/" + fmt.Sprintf("48744-test.%s", getRandomString()) + ":latest"
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", "quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", myimage, "--insecure", "-a", tempDataDir+"/.dockerconfigjson", "--keep-manifest-list=true", "--filter-by-os=.*").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		myimage1 := regRoute + "/" + fmt.Sprintf("48744-test1.%s", getRandomString()) + "/rh-test:latest"
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", myimage, myimage1, "--insecure", "-a", tempDataDir+"/.dockerconfigjson", "--keep-manifest-list=true", "--filter-by-os=.*").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create imagestream with pull through")
		err = oc.AsAdmin().WithoutNamespace().Run("import-image").Args("first:latest", "--from="+myimage, "--reference-policy=local", "--insecure", "--confirm", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "first", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(myimage1, "second:latest", "--reference-policy=local", "--insecure", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "second", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create pod with the imagestreams")
		err = oc.Run("set").Args("image-lookup", "--all", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		expectInfo := `Successfully pulled image "image-registry.openshift-image-registry.svc:5000/` + oc.Namespace()
		createSimpleRunPod(oc, "first:latest", expectInfo)
		createSimpleRunPod(oc, "second:latest", expectInfo)
	})

	// author: xiuwang@redhat.com
	g.It("DisconnectedOnly-Author:xiuwang-High-48739-Pull through works with icsp which source and mirror without full path", func() {
		g.By("Check if image-policy-aosqe created")
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("imagecontentsourcepolicy").Output()
		if !strings.Contains(output, "image-policy-aosqe") {
			e2e.Failf("image-policy-aosqe is not created in this disconnect cluster")
		}

		g.By("Create imagestream using source image only match to mirrors namespace in icsp")
		oc.SetupProject()
		err = oc.WithoutNamespace().AsAdmin().Run("import-image").Args("skopeo:latest", "--from=quay.io/openshifttest/skopeo@sha256:426196e376cf045012289d53fec986554241496ed7f38e347fc56505aa8ad322", "--reference-policy=local", "--confirm", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "skopeo", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("image-lookup", "skopeo", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		expectInfo := `Successfully pulled image "image-registry.openshift-image-registry.svc:5000/` + oc.Namespace()
		createSimpleRunPod(oc, "skopeo:latest", expectInfo)

		g.By("Create imagestream using source image which use the whole mirrors")
		manifest := saveImageMetadataName(oc, "rhel8/mysql-80")
		mysqlImage := "registry.redhat.io/rhel8/mysql-80@" + manifest
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(mysqlImage, "mysqlx:latest", "--reference-policy=local", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "mysqlx", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("set").Args("image-lookup", "mysqlx", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		createSimpleRunPod(oc, "mysqlx:latest", expectInfo)
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ConnectedOnly-VMonly-High-48710-Should be able to deploy an existing image from private docker.io registry", func() {

		oc.SetupProject()
		g.By("Create the secret for docker private image")
		dockerConfig := filepath.Join("/home", "cloud-user", ".docker", "auto", "48710.json")
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", oc.Namespace(), "secret", "docker-registry", "secret48710", fmt.Sprintf("--from-file=.dockerconfigjson=%s", dockerConfig)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create a imagestream with a docker private image")
		output, err := oc.AsAdmin().WithoutNamespace().Run("tag").Args("docker.io/irqe/busybox:latest", "test48710:latest", "--reference-policy=local", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Tag test48710:latest set"))
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "test48710", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create pod with the imagestream")
		expectInfo := `Successfully pulled image "image-registry.openshift-image-registry.svc:5000/` + oc.Namespace()
		newAppUseImageStream(oc, oc.Namespace(), "test48710:latest", expectInfo)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-Critical-48959-Should be able to get public images connect to the server and have basic auth credentials [Serial]", func() {

		g.By("Create route to expose the registry")
		defer restoreRouteExposeRegistry(oc)
		createRouteExposeRegistry(oc)

		g.By("Get server host")
		host := getRegistryDefaultRoute(oc)

		g.By("Grant public access to the openshift namespace")
		defer oc.AsAdmin().WithoutNamespace().Run("policy").Args("remove-role-from-group", "system:image-puller", "system:unauthenticated", "--namespace", "openshift").Execute()
		output, err := oc.AsAdmin().WithoutNamespace().Run("policy").Args("add-role-to-group", "system:image-puller", "system:unauthenticated", "--namespace", "openshift").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("clusterrole.rbac.authorization.k8s.io/system:image-puller added: \"system:unauthenticated\""))

		g.By("Try to fetch image metadata")
		output, err = oc.AsAdmin().Run("image").Args("info", "--insecure", host+"/openshift/tools:latest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("error: unauthorized: authentication required"))
		o.Expect(output).NotTo(o.ContainSubstring("Unable to connect to the server: no basic auth credentials"))
		o.Expect(output).To(o.ContainSubstring(host + "/openshift/tools:latest"))

	})

	// author: yyou@redhat.com
	g.It("VMonly-NonPreRelease-Author:yyou-Critical-44037-Could configure swift authentication using application credentials [Disruptive]", func() {
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(output, "OpenStack") {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Configure image-registry-private-configuration  secret to use new application credentials")
		defer oc.AsAdmin().WithoutNamespace().Run("set").Args("data", "secret/image-registry-private-configuration", "--from-literal=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALID='' ", "--from-literal=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALNAME='' ", "--from-literal=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALSECRET='' ", "-n", "openshift-image-registry").Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("set").Args("data", "secret/image-registry-private-configuration", "--from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALID=/root/auto/44037/applicationid", "--from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALNAME=/root/auto/44037/applicationname", "--from-file=REGISTRY_STORAGE_SWIFT_APPLICATIONCREDENTIALSECRET=/root/auto/44037/applicationsecret", "-n", "openshift-image-registry").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check image registry pod")
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 2)

		g.By("push/pull image to registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-44037", oc.Namespace())

	})

	// author: jitli@redhat.com
	// Cover test case: OCP-46069 and 49886
	g.It("NonPreRelease-Author:jitli-Critical-46069-Could override the default topology constraints and Topology Constraints works well in non zone cluster [Disruptive]", func() {

		g.By("Get image registry pod replicas num")
		podNum := getImageRegistryPodNumber(oc)

		g.By("Check cluster whose nodes have no zone label set")
		if !checkRegistryUsingFSVolume(oc) {

			g.By("Platform with zone, Check the image-registry default topology")
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"DoNotSchedule"}`))
			o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}`))
			o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"node-role.kubernetes.io/worker","whenUnsatisfiable":"DoNotSchedule"}`))

			g.By("Check whether these two registry pods are running in different workers")
			NodeList := getPodNodeListByLabel(oc, "openshift-image-registry", "docker-registry=default")
			o.Expect(NodeList[0]).NotTo(o.Equal(NodeList[1]))

			g.By("Configure topology")
			defer oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"topologySpreadConstraints":null}}`, "--type=merge").Execute()
			output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"topologySpreadConstraints":[{"labelSelector":{"matchLabels":{"docker-registry":"bar"}},"maxSkew":2,"topologyKey":"zone","whenUnsatisfiable":"ScheduleAnyway"}]}}`, "--type=merge").Output()
			if err != nil {
				e2e.Logf(output)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))

			g.By("Check if the topology has been override in image registry deploy")
			err = wait.Poll(3*time.Second, 9*time.Second, func() (bool, error) {
				output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(output, `{"labelSelector":{"matchLabels":{"docker-registry":"bar"}},"maxSkew":2,"topologyKey":"zone","whenUnsatisfiable":"ScheduleAnyway"}`) {
					return true, nil
				}
				e2e.Logf("Continue to next round")
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, "The topology has not been overridden")

			g.By("Check if image registry pods go to running")
			checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)

			g.By("check registry working well")
			oc.SetupProject()
			checkRegistryFunctionFine(oc, "test-46069", oc.Namespace())

		} else {

			g.By("Platform without zone, Check the image-registry default topology in non zone label cluster")
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}`))
			o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"node-role.kubernetes.io/worker","whenUnsatisfiable":"DoNotSchedule"}`))

			g.By("Check registry pod")
			checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)

			g.By("check registry working well")
			oc.SetupProject()
			checkRegistryFunctionFine(oc, "test-49886", oc.Namespace())
		}

	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Author:jitli-Medium-46082-Increase replicas to match one zone have one pod [Disruptive]", func() {

		g.By("Check platforms")
		if checkRegistryUsingFSVolume(oc) {
			g.Skip("Skip for fs volume")
		}

		g.By("Check the nodes with Each zone have one worker")
		workerNodes, _ := exutil.GetClusterNodesBy(oc, "worker")
		if len(workerNodes) != 3 {
			g.Skip("Skip for not three workers")
		}
		zone, err := oc.AsAdmin().Run("get").Args("node", "-l", "node-role.kubernetes.io/worker", `-o=jsonpath={.items[*].metadata.labels.topology\.kubernetes\.io\/zone}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		zoneList := strings.Fields(zone)
		if strings.EqualFold(zoneList[0], zoneList[1]) || strings.EqualFold(zoneList[0], zoneList[2]) || strings.EqualFold(zoneList[1], zoneList[2]) {

			e2e.Logf("Zone: %v . Doesn't conform Each zone have one worker", zone)
			g.By("Only check pods on different worker")
			samenum, diffnum := comparePodHostIP(oc)
			e2e.Logf("%v %v", samenum, diffnum)
			o.Expect(samenum == 0).To(o.BeTrue())
			o.Expect(diffnum == 1).To(o.BeTrue())

		} else {

			e2e.Logf("Zone: %v . Each zone have one worker", zone)
			g.By("Scale registry pod to 3 then to 4")
			/*
				replicas=2 has the pod affinity configure
				When change replicas=2 to other number, the registry pods will be recreated.
				When changed replicas to 3, the Pods will be scheduled to each worker.
				Due to the RollingUpdate policy, it will also count the old pods that are running.
				So there is a certain probability that two pods are running in the same worker
				To deal with the problem, we change replicas to 3 then 4 to monitor the new pod followoing topologyspread.

				When the kubernetes issues fixed, we can update the checkpoint ,
				https://bugzilla.redhat.com/show_bug.cgi?id=2024888#c11
			*/
			defer oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Execute()
			output, err := oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":3}}`, "--type=merge").Output()
			if err != nil {
				e2e.Logf(output)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))
			checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 3)

			output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":4}}`, "--type=merge").Output()
			if err != nil {
				e2e.Logf(output)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))
			checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 4)

			g.By("Check if image registry pods run in each zone")
			samenum, diffnum := comparePodHostIP(oc)
			e2e.Logf("%v %v", samenum, diffnum)
			o.Expect(samenum == 1).To(o.BeTrue())
			o.Expect(diffnum == 5).To(o.BeTrue())

		}

		g.By("check registry working well")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-46082", oc.Namespace())

	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Author:jitli-Critical-46083-Topology Constraints works well in SNO environment [Disruptive]", func() {

		g.By("Check platforms")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		platforms := map[string]bool{
			"AWS":          true,
			"Azure":        true,
			"GCP":          true,
			"OpenStack":    true,
			"AlibabaCloud": true,
			"IBMCloud":     true,
		}
		if !platforms[platformtype] {
			g.Skip("Skip for non-supported platform")
		}

		g.By("Check whether the environment is SNO")
		//Only 1 master, 1 worker node and with the same hostname.
		masterNodes, _ := exutil.GetClusterNodesBy(oc, "master")
		workerNodes, _ := exutil.GetClusterNodesBy(oc, "worker")
		if len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
			e2e.Logf("This is a SNO cluster")
		} else {
			g.Skip("Not SNO cluster - skipping test ...")
		}

		g.By("Check the image-registry default topology in SNO cluster")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"DoNotSchedule"}`))
		o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}`))
		o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"node-role.kubernetes.io/worker","whenUnsatisfiable":"DoNotSchedule"}`))

		g.By("Check registry pod")
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 1)

		g.By("Scale registry pod to 2")
		defer oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":1}}`, "--type=merge").Execute()
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":2}}`, "--type=merge").Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check registry new pods")
		err = wait.Poll(20*time.Second, 1*time.Minute, func() (bool, error) {
			podsStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-o", "wide", "-n", "openshift-image-registry", "-l", "docker-registry=default", "--sort-by={.status.phase}", "-o=jsonpath={.items[*].status.phase}").Output()
			if podsStatus != "Pending Pending Running" {
				e2e.Logf("the pod status is %v, continue to next round", podsStatus)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "Pods list are not one Running two Pending")

		g.By("Scale registry pod to 3")
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"replicas":3}}`, "--type=merge").Output()
		if err != nil {
			e2e.Logf(output)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check if all pods are running well")
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", 3)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-High-NonPreRelease-50219-Setting nodeSelector and tolerations on nodes with taints registry works well [Disruptive]", func() {

		g.By("Check the image-registry default topology")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}`))
		o.Expect(output).To(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"node-role.kubernetes.io/worker","whenUnsatisfiable":"DoNotSchedule"}`))

		g.By("Setting both nodeSelector and tolerations on nodes with taints")
		defer oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":null,"tolerations":null}}`, "--type=merge").Output()
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":{"node-role.kubernetes.io/master": ""},"tolerations":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"}]}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check registry pod well")
		podNum := getImageRegistryPodNumber(oc)
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)

		g.By("Check the image-registry default topology removed")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "image-registry", "-n", "openshift-image-registry", "-o=jsonpath={.spec.template.spec.topologySpreadConstraints}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"DoNotSchedule"}`))
		o.Expect(output).NotTo(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}`))
		o.Expect(output).NotTo(o.ContainSubstring(`{"labelSelector":{"matchLabels":{"docker-registry":"default"}},"maxSkew":1,"topologyKey":"node-role.kubernetes.io/worker","whenUnsatisfiable":"DoNotSchedule"}`))

		g.By("check registry working well")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test1-50219", oc.Namespace())

		g.By("Setting nodeSelector on node without taints")
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":null,"tolerations":null}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))
		output, err = oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":{"node-role.kubernetes.io/worker": ""}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check registry pod well")
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)

		g.By("check registry working well")
		checkRegistryFunctionFine(oc, "test2-50219", oc.Namespace())
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Critical-NonPreRelease-49455-disableRedirect should work when image registry configured object storage [Serial]", func() {
		g.By("Get registry storage info")
		storagetype, _ := restoreRegistryStorageConfig(oc)
		if storagetype == "pvc" || storagetype == "emptyDir" {
			g.Skip("Skip disableRedirect test for fs volume")
		}

		g.By("Set object storage client accordingly")
		var storageclient string
		switch storagetype {
		case "azure":
			storageclient = "blob.core.windows.net"
		case "gcs":
			storageclient = "storage.googleapis.com"
		case "ibmocs":
			storageclient = "storage.appdomain.cloud"
		case "oss":
			storageclient = "aliyuncs.com"
		case "swift":
			storageclient = "redhat.com"
		case "s3":
			storageclient = "amazonaws.com"
		default:
			e2e.Failf("Image Registry is using unknown storage type")
		}

		g.By("Create route to expose the registry")
		defer restoreRouteExposeRegistry(oc)
		createRouteExposeRegistry(oc)
		regRoute := getRegistryDefaultRoute(oc)

		g.By("push image to registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-49455", oc.Namespace())
		authFile, err := saveImageRegistryAuth(oc, regRoute, oc.Namespace())
		defer os.RemoveAll(authFile)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check disableRedirect function")
		//disableRedirect: Controls whether to route all data through the registry, rather than redirecting to the back end. Defaults to false.
		myimage := regRoute + "/" + oc.Namespace() + "/test-49455:latest"
		cmd := "oc image info " + myimage + " -ojson -a " + authFile + " --insecure|jq -r '.layers[0].digest'"
		imageblob, err := exec.Command("bash", "-c", cmd).Output()
		e2e.Logf(" imageblob is %s", imageblob)
		o.Expect(err).NotTo(o.HaveOccurred())
		token, err := getSAToken(oc, "builder", oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).NotTo(o.BeEmpty())
		cmd = "curl -Lks -u " + oc.Username() + ":" + token + " -I HEAD https://" + regRoute + "/v2/" + oc.Namespace() + "/test-49455/blobs/" + string(imageblob)
		curlOutput, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(curlOutput)).To(o.ContainSubstring(storageclient))
	})

	// author: xiuwang@redhat.com
	g.It("Author:xiuwang-Low-51055-Image pullthrough does pass 429 errors back to capable clients", func() {

		g.By("Create a registry could limit quota")
		oc.SetupProject()
		regRoute := setSecureRegistryWithoutAuth(oc, oc.Namespace(), "myregistry", "quay.io/openshifttest/registry-toomany-request@sha256:ca50d2c9b289b0bf5a22f7aa73f68586cf38de2878c63465eacf74c032a6211d", "8080")
		o.Expect(regRoute).NotTo(o.BeEmpty())
		err := oc.Run("set").Args("resources", "deploy/myregistry", "--requests=cpu=100m,memory=128Mi").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		checkPodsRunningWithLabel(oc, oc.Namespace(), "app=myregistry", 1)

		limitURL := "curl -k -XPOST -d 'c=150' https://" + regRoute
		_, err = exec.Command("bash", "-c", limitURL).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Push image to the limit registry")
		myimage := regRoute + "/" + oc.Namespace() + "/myimage:latest"
		err = oc.AsAdmin().WithoutNamespace().Run("image").Args("mirror", "quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", myimage, "--insecure", "--keep-manifest-list=true", "--filter-by-os=.*").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("import-image").Args("test-51055", "--from", myimage, "--confirm", "--reference-policy=local", "--insecure").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "test-51055", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Limit the registry quota")
		limitURL = "curl -k -XPOST -d 'c=1' https://" + regRoute
		_, err = exec.Command("bash", "-c", limitURL).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create pod with the imagestream")
		err = oc.Run("set").Args("image-lookup", "test-51055", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		expectInfo := "429 Too Many Requests"
		createSimpleRunPod(oc, "test-51055:latest", expectInfo)
		output, err := oc.WithoutNamespace().AsAdmin().Run("logs").Args("deploy/image-registry", "--since=10s", "-n", "openshift-image-registry").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("unable to pullthrough manifest"))
		o.Expect(output).To(o.ContainSubstring("err.code=toomanyrequests"))
	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Longduration-Author:jitli-Medium-49747-Configure image registry to skip volume SELinuxLabel [Disruptive]", func() {

		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "image_registry")
			machineConfigSource = filepath.Join(buildPruningBaseDir, "machineconfig.yaml")
			runtimeClassSource  = filepath.Join(buildPruningBaseDir, "runtimeClass.yaml")
			mc                  = machineConfig{
				name:     "49747-worker-selinux-configuration",
				pool:     "worker",
				source:   "data:text/plain;charset=utf-8;base64,W2NyaW8ucnVudGltZS5ydW50aW1lcy5zZWxpbnV4XQpydW50aW1lX3BhdGggPSAiL3Vzci9iaW4vcnVuYyIKcnVudGltZV9yb290ID0gIi9ydW4vcnVuYyIKcnVudGltZV90eXBlID0gIm9jaSIKYWxsb3dlZF9hbm5vdGF0aW9ucyA9IFsiaW8ua3ViZXJuZXRlcy5jcmktby5UcnlTa2lwVm9sdW1lU0VMaW51eExhYmVsIl0K",
				path:     "/etc/crio/crio.conf.d/01-runtime-selinux.conf",
				template: machineConfigSource,
			}

			rtc = runtimeClass{
				name:     "selinux-49747",
				handler:  "selinux",
				template: runtimeClassSource,
			}
		)

		g.By("Register defer block to delete mc and wait for mcp/worker rollback to complete")
		defer mc.delete(oc)

		g.By("Create machineconfig to add selinux cri-o config and wait for mcp update to complete")
		mc.createWithCheck(oc)

		g.By("verify new crio drop-in file exists and content is correct")
		workerNode, workerNodeErr := exutil.GetFirstWorkerNode(oc)
		o.Expect(workerNodeErr).NotTo(o.HaveOccurred())
		o.Expect(workerNode).NotTo(o.BeEmpty())
		err := wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
			selinuxStatus, statusErr := exutil.DebugNodeWithChroot(oc, workerNode, "cat", "/etc/crio/crio.conf.d/01-runtime-selinux.conf")
			if statusErr == nil {
				if strings.Contains(selinuxStatus, "io.kubernetes.cri-o.TrySkipVolumeSELinuxLabel") {
					e2e.Logf("runtime-selinux.conf updated")
					return true, nil
				}
			}
			e2e.Logf("runtime-selinux.conf not update, err: %v", statusErr)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "runtime-selinux.conf not update")

		g.By("Register defer block to delete new runtime class")
		defer rtc.delete(oc)

		g.By("Create new runtimeClass from template and verify it's done")
		rtc.createWithCheck(oc)

		g.By("Override the image registry annonation and runtimeclass")
		defer oc.AsAdmin().Run("patch").Args("config.imageregistry.operator.openshift.io/cluster", "-p", `{"spec":{"unsupportedConfigOverrides":null}}`, "--type=merge").Execute()
		configPatchStatus, configPatchErr := oc.AsAdmin().Run("patch").Args("config.imageregistry.operator.openshift.io/cluster", "-p", `{"spec":{"unsupportedConfigOverrides":{"deployment":{"annotations":{"io.kubernetes.cri-o.TrySkipVolumeSELinuxLabel":"true"},"runtimeClassName":"`+rtc.name+`"}}}}`, "--type=merge").Output()
		o.Expect(configPatchErr).NotTo(o.HaveOccurred())
		o.Expect(configPatchStatus).To(o.ContainSubstring("patched"))
		podNum := getImageRegistryPodNumber(oc)
		checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)

		g.By("Check the registry files label")
		err = wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
			selinuxLabel, selinuxLabelErr := oc.AsAdmin().Run("get").Args("pod", "-n", "openshift-image-registry", "-l", "docker-registry=default", `-ojsonpath={.items..metadata.annotations.io\.kubernetes\.cri-o\.TrySkipVolumeSELinuxLabel}`).Output()
			getRuntimeClassName, runtimeClassNameErr := oc.AsAdmin().Run("get").Args("pod", "-n", "openshift-image-registry", "-l", "docker-registry=default", `-ojsonpath={.items..spec.runtimeClassName}`).Output()

			if strings.Contains(selinuxLabel, "true") && strings.Contains(getRuntimeClassName, rtc.name) {
				e2e.Logf("pod metadata updated")
				return true, nil
			}
			e2e.Logf("pod metadata not update, selinuxLabel:%v %v, runtimeClassName:%v %v", selinuxLabel, selinuxLabelErr, getRuntimeClassName, runtimeClassNameErr)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod metadata not update ")

		g.By("check registry working well")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-49747", oc.Namespace())
	})

	// author: jitli@redhat.com
	g.It("NonPreRelease-Author:jitli-Medium-22032-Config NodeSelector for internal registry [Disruptive]", func() {

		g.By("Set up internal registry NodeSelector")
		initialConfig, err := oc.AsAdmin().Run("get").Args("config.image/cluster", "-ojsonpath={.spec.nodeSelector}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if initialConfig == "" {
			initialConfig = `null`
		}

		defer func() {
			g.By("Remove nodeSelector for imageregistry")
			output, err := oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":`+initialConfig+`}}`, "--type=merge").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))
			g.By("Check registry pod well")
			podNum := getImageRegistryPodNumber(oc)
			checkPodsRunningWithLabel(oc, "openshift-image-registry", "docker-registry=default", podNum)
		}()
		output, err := oc.AsAdmin().Run("patch").Args("config.image/cluster", "-p", `{"spec":{"nodeSelector":{"node-role.kubernetes.io/master": "test22032"}}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("Check the image-registry default topology")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			nodeSelector, nodeSelectorErr := oc.AsAdmin().Run("get").Args("pod", "-n", "openshift-image-registry", "-l", "docker-registry=default", `-ojsonpath={.items..spec.nodeSelector}`).Output()

			if strings.Contains(nodeSelector, "test22032") {
				e2e.Logf("pod metadata updated")
				return true, nil
			}
			e2e.Logf("pod metadata not update, nodeSelector:%v %v", nodeSelector, nodeSelectorErr)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(err, "pod metadata not update")
	})

	//author: yyou@redhat.com
	g.It("Author:yyou-Medium-50925-Add prometheusrules for image_registry_image_stream_tags_total and registry operations metrics", func() {
		var (
			operationData   prometheusImageregistryOperations
			storageTypeData prometheusImageregistryStorageType
		)
		g.By("Push 1 images to non-openshift project to image registry")
		oc.SetupProject()
		checkRegistryFunctionFine(oc, "test-50925", oc.Namespace())

		g.By("Collect metrics of tag")
		mo, err := exutil.NewPrometheusMonitor(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		tagQueryParams := exutil.MonitorInstantQueryParams{Query: "imageregistry:imagestreamtags_count:sum"}
		tagMsg, err := mo.InstantQuery(tagQueryParams)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tagMsg).NotTo(o.BeEmpty())

		g.By("Collect metrics of operations")
		opQueryParams := exutil.MonitorInstantQueryParams{Query: "imageregistry:operations_count:sum"}
		operationMsg, err := mo.InstantQuery(opQueryParams)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(operationMsg).NotTo(o.BeEmpty())
		jsonerr := json.Unmarshal([]byte(operationMsg), &storageTypeData)
		if jsonerr != nil {
			e2e.Failf("operation data is not in json format")
		}
		operationLen := len(operationData.Data.Result)
		beforeOperationData := make([]int, operationLen)
		for i := 0; i < operationLen; i++ {
			beforeOperationData[i], err = strconv.Atoi(operationData.Data.Result[i].Value[1].(string))
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("the operation array %v is %v", i, beforeOperationData[i])
		}

		g.By("Tag 2 imagestream to non-openshift project")
		err = oc.AsAdmin().Run("tag").Args("quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f", "is50925-1:latest", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "is50925-1", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("import-image").Args("is50925-2:latest", "--from", "registry.access.redhat.com/rhel7", "--confirm", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "is50925-2", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Collect metrics of storagetype")
		fullStorageType := "S3 EmptyDir PVC Azure GCS Swift OSS IBMCOS"
		storageQuertParams := exutil.MonitorInstantQueryParams{Query: "image_registry_storage_type"}
		storageTypeMsg, err := mo.InstantQuery(storageQuertParams)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(storageTypeMsg).NotTo(o.BeEmpty())
		jsonerr = json.Unmarshal([]byte(storageTypeMsg), &storageTypeData)
		if jsonerr != nil {
			e2e.Failf("storage type data is not in json format")
		}
		storageType := storageTypeData.Data.Result[0].Metric.Storage
		o.Expect(fullStorageType).To(o.ContainSubstring(storageType))

		g.By("Collect metrics of operation again")
		err = wait.Poll(20*time.Second, 3*time.Minute, func() (bool, error) {
			opQueryParams = exutil.MonitorInstantQueryParams{Query: "imageregistry:operations_count:sum"}
			operationMsg, err = mo.InstantQuery(opQueryParams)
			o.Expect(err).NotTo(o.HaveOccurred())
			jsonerr = json.Unmarshal([]byte(operationMsg), &storageTypeData)
			if jsonerr != nil {
				e2e.Failf("operation data is not in json format")
			}
			afterOperationData := make([]int, operationLen)
			for i := 0; i < operationLen; i++ {
				afterOperationData[i], err = strconv.Atoi(operationData.Data.Result[i].Value[1].(string))
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("the operation array %v is %v", i, beforeOperationData[i])
				if afterOperationData[i] >= beforeOperationData[i] {
					e2e.Logf("%v -> %v", beforeOperationData[i], afterOperationData[i])
				} else {
					return false, nil
				}
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(err, "The operation metric don't get expect value")
	})

})
