package operatorsdk

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operators] Operator_SDK should", func() {
	defer g.GinkgoRecover()

	var operatorsdkCLI = NewOperatorSDKCLI()
	var makeCLI = NewMakeCLI()
	var oc = exutil.NewCLIWithoutNamespace("default")
	var ocpversion = "4.10"

	// author: jfan@redhat.com
	g.It("VMonly-Author:jfan-High-37465-SDK olm improve olm related sub commands", func() {

		operatorsdkCLI.showInfo = true
		g.By("check the olm status")
		output, _ := operatorsdkCLI.Run("olm").Args("status", "--olm-namespace", "openshift-operator-lifecycle-manager").Output()
		o.Expect(output).To(o.ContainSubstring("Successfully got OLM status for version"))
	})

	// author: jfan@redhat.com
	g.It("Author:jfan-High-37312-SDK olm improve manage operator bundles in new manifests metadata format", func() {

		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-37312 && cd /tmp/memcached-operator-37312 && operator-sdk init --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-37312").Output()
		result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-37312 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("Bundle manifests generated successfully in bundle"))
	})

	// author: jfan@redhat.com
	g.It("Author:jfan-High-37141-SDK Helm support simple structural schema generation for Helm CRDs", func() {

		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir /tmp/nginx-operator-37141 && cd /tmp/nginx-operator-37141 && operator-sdk init --project-name nginx-operator --plugins helm.sdk.operatorframework.io/v1").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/nginx-operator-37141").Output()
		result, err := exec.Command("bash", "-c", "cd /tmp/nginx-operator-37141 && operator-sdk create api --group apps --version v1beta1 --kind Nginx").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("Created helm-charts/nginx"))
		result, err = exec.Command("bash", "-c", "cat /tmp/nginx-operator-37141/config/crd/bases/apps.my.domain_nginxes.yaml | grep -E \"x-kubernetes-preserve-unknown-fields: true\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("x-kubernetes-preserve-unknown-fields: true"))
	})

	// author: jfan@redhat.com
	g.It("Author:jfan-High-37311-SDK ansible valid structural schemas for ansible based operators", func() {
		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir /tmp/ansible-operator-37311 && cd /tmp/ansible-operator-37311 && operator-sdk init --project-name nginx-operator --plugins ansible.sdk.operatorframework.io/v1").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/ansible-operator-37311").Output()
		_, err := exec.Command("bash", "-c", "cd /tmp/ansible-operator-37311 && operator-sdk create api --group apps --version v1beta1 --kind Nginx").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := exec.Command("bash", "-c", "cat /tmp/ansible-operator-37311/config/crd/bases/apps.my.domain_nginxes.yaml | grep -E \"x-kubernetes-preserve-unknown-fields: true\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("x-kubernetes-preserve-unknown-fields: true"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-37627-SDK run bundle upgrade test", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/upgradeoperator-bundle:v0.1", "-n", oc.Namespace(), "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
		output, err = operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/upgradeoperator-bundle:v0.2", "-n", oc.Namespace(), "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Successfully upgraded to"))
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "upgradeoperator.v0.0.2", "-n", oc.Namespace()).Output()
			if strings.Contains(msg, "Succeeded") {
				e2e.Logf("upgrade to 0.2 success")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("upgradeoperator upgrade failed in %s ", oc.Namespace()))
		output, err = operatorsdkCLI.Run("cleanup").Args("upgradeoperator", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-38054-SDK run bundle create pods and csv and registry image pod", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/podcsvcheck-bundle:v0.0.1", "-n", oc.Namespace(), "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))
		output, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace()).Output()
		o.Expect(output).To(o.ContainSubstring("quay-io-olmqe-podcsvcheck-bundle-v0-0-1"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "podcsvcheck.v0.0.1", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Succeeded"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "podcsvcheck-catalog", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("grpc"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("installplan", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("podcsvcheck.v0.0.1"))
		output, err = operatorsdkCLI.Run("cleanup").Args("podcsvcheck", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("ConnectedOnly-Author:jfan-High-38060-SDK run bundle detail message about failed", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		output, _ := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/etcd-bundle:0.0.1", "-n", oc.Namespace()).Output()
		o.Expect(output).To(o.ContainSubstring("quay.io/olmqe/etcd-bundle:0.0.1: not found"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-27977-SDK ansible Implement default Ansible content path in watches.yaml", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/contentpath-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment/contentpath-controller-manager", "-n", namespace, "-c", "manager").Output()
			if strings.Contains(msg, "Starting workers") {
				e2e.Logf("found Starting workers")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("contentpath-controller-manager deployment of %s has not Starting workers", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-34292-SDK ansible operator flags maxConcurrentReconciles by arg max concurrent reconciles", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		defer operatorsdkCLI.Run("cleanup").Args("max-concurrent-reconciles", "-n", namespace).Output()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/max-concurrent-reconciles-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check the reconciles number in logs")
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/max-concurrent-reconciles-controller-manager", "-c", "manager", "-n", namespace).Output()
			if strings.Contains(msg, "\"worker count\":4") {
				e2e.Logf("found worker count:4")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("log of deploy/max-concurrent-reconciles-controller-manager of %s doesn't have worker count:4", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-28157-SDK ansible blacklist supported in watches.yaml", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var blacklist = filepath.Join(buildPruningBaseDir, "cache1_v1_blacklist.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/blacklist-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createBlacklist, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", blacklist, "-p", "NAME=blacklist-sample").OutputToFile("config-28157.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createBlacklist, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "blacklist-sample") {
				e2e.Logf("found pod blacklist-sample")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("blacklist-sample pod in %s doesn't found", namespace))
		msg, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/blacklist-controller-manager", "-c", "manager", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("Skipping"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-28586-SDK ansible Content Collections Support in watches.yaml", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var collectiontest = filepath.Join(buildPruningBaseDir, "cache5_v1_collectiontest.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/contentcollections-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createCollection, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", collectiontest, "-p", "NAME=collectiontest").OutputToFile("config-28586.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCollection, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/contentcollections-controller-manager", "-c", "manager", "-n", namespace).Output()
			if strings.Contains(msg, "dummy : Create ConfigMap") {
				e2e.Logf("found dummy : Create ConfigMap")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("miss log dummy : Create ConfigMap in %s", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-29374-SDK ansible Migrate kubernetes Ansible modules to a collect", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var memcached = filepath.Join(buildPruningBaseDir, "cache2_v1_modulescollect.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/modules-to-collect-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createModules, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcached, "-p", "NAME=modulescollect-sample").OutputToFile("config-29374.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createModules, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", namespace).Output()
			if strings.Contains(msg, "test-secret") {
				e2e.Logf("found secret test-secret")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("doesn't get secret test-secret %s", namespace))
		//oc get secret test-secret -o yaml
		msg, err := oc.AsAdmin().Run("describe").Args("secret", "test-secret", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("test:  6 bytes"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-37142-SDK helm cr create deletion process", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var nginx = filepath.Join(buildPruningBaseDir, "demo_v1_nginx.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/nginx-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createNginx, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", nginx, "-p", "NAME=nginx-sample").OutputToFile("config-37142.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createNginx, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "nginx-sample") {
				e2e.Logf("found pod nginx-sample")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("miss pod nginx-sample in %s", namespace))
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("nginx.helmdemo.example.com", "nginx-sample", "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: jfan@redhat.com
	g.It("Author:jfan-High-34441-SDK commad operator sdk support init help message", func() {
		output, err := operatorsdkCLI.Run("init").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("--component-config"))
	})

	// author: jfan@redhat.com
	g.It("Author:jfan-Medium-40521-SDK olm improve manage operator bundles in new manifests metadata format", func() {
		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir /tmp/memcached-operator-40521 && cd /tmp/memcached-operator-40521 && operator-sdk init --plugins ansible.sdk.operatorframework.io/v1 --domain example.com --group cache --version v1alpha1 --kind Memcached --generate-playbook").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/memcached-operator-40521").Output()
		result, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("Bundle manifests generated successfully in bundle"))
		exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && sed -i '/icon/,+2d' ./bundle/manifests/memcached-operator-40521.clusterserviceversion.yaml").Output()
		msg, err := exec.Command("bash", "-c", "cd /tmp/memcached-operator-40521 && operator-sdk bundle validate ./bundle &> ./validateresult && cat validateresult").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("All validation tests have completed successfully"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-40520-SDK k8sutil 1123Label creates invalid values", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		msg, _ := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/raffaelespazzoli-proactive-node-scaling-operator-bundle:latest-", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(msg).To(o.ContainSubstring("Successfully created registry pod: raffaelespazzoli-proactive-node-scaling-operator-bundle-latest"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-35443-SDK run bundle InstallMode for own namespace [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		// install the operator without og with installmode
		msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v"+ocpversion, "--install-mode", "OwnNamespace", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
		o.Expect(msg).To(o.ContainSubstring(namespace))
		output, err := operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-ownsingleallsupport-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle can't be deleted in %s", namespace))
		// install the operator with og and installmode
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-own", "NAMESPACE="+namespace).OutputToFile("config-35443.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v"+ocpversion, "--install-mode", "OwnNamespace", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		output, _ = operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// install the operator with og without installmode
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-ownsingleallsupport-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle can't be deleted in %s", namespace))
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsingleallsupport-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		output, _ = operatorsdkCLI.Run("cleanup").Args("ownsingleallsupport", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// delete the og
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-own", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-ownsingleallsupport-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("quay-io-olmqe-ownsingleallsupport-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-ownsingleallsupport-bundle can't be deleted in %s", namespace))
		// install the operator without og and installmode, the csv support ownnamespace and singlenamespace
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/ownsinglesupport-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
		o.Expect(msg).To(o.ContainSubstring(namespace))
		output, _ = operatorsdkCLI.Run("cleanup").Args("ownsinglesupport", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-41064-SDK run bundle InstallMode for single namespace [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var operatorGroup = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", "test-sdk-41064").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", "test-sdk-41064").Execute()
		msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v"+ocpversion, "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-n", namespace, "-o=jsonpath={.spec.targetNamespaces}").Output()
		o.Expect(msg).To(o.ContainSubstring("test-sdk-41064"))
		output, err := operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all1support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle can't be deleted in %s", namespace))
		// install the operator with og and installmode
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-single", "NAMESPACE="+namespace, "KAKA=test-sdk-41064").OutputToFile("config-41064.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v"+ocpversion, "--install-mode", "SingleNamespace=test-sdk-41064", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		output, _ = operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// install the operator with og without installmode
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all1support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle can't be deleted in %s", namespace))
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all1support-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		output, _ = operatorsdkCLI.Run("cleanup").Args("all1support", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// delete the og
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-single", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all1support-bundle-v4-10", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all1support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all1support-bundle can't be deleted in %s", namespace))
		// install the operator without og and installmode, the csv only support singlenamespace
		msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/singlesupport-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(msg).To(o.ContainSubstring("AllNamespaces InstallModeType not supported"))
		output, _ = operatorsdkCLI.Run("cleanup").Args("singlesupport", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-41065-SDK run bundle InstallMode for all namespace [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
		var operatorGroup = filepath.Join(buildPruningBaseDir, "og-allns.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		// install the operator without og with installmode all namespace
		msg, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v"+ocpversion, "--install-mode", "AllNamespaces", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		defer operatorsdkCLI.Run("cleanup").Args("all2support").Output()
		msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("og", "operator-sdk-og", "-o=jsonpath={.spec.targetNamespaces}", "-n", namespace).Output()
		o.Expect(msg).To(o.ContainSubstring(""))
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
			if strings.Contains(msg, "all2support.v0.0.1") {
				e2e.Logf("csv all2support.v0.0.1")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
		output, err := operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-10", "--no-headers", "-n", namespace).Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all2support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle can't be deleted in %s", namespace))
		// install the operator with og and installmode
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=og-allnames", "NAMESPACE="+namespace).OutputToFile("config-41065.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v"+ocpversion, "--install-mode", "AllNamespaces", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
			if strings.Contains(msg, "all2support.v0.0.1") {
				e2e.Logf("csv all2support.v0.0.1")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
		output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// install the operator with og without installmode
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-10", "--no-headers", "-n", namespace).Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all2support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle can't be deleted in %s", namespace))
		msg, err = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
			if strings.Contains(msg, "all2support.v0.0.1") {
				e2e.Logf("csv all2support.v0.0.1")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
		output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
		// delete the og
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("og", "og-allnames", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "quay-io-olmqe-all2support-bundle-v4-10", "--no-headers", "-n", namespace).Output()
			if strings.Contains(msg, "not found") {
				e2e.Logf("not found pod quay-io-olmqe-all2support-bundle")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("pod quay-io-olmqe-all2support-bundle can't be deleted in %s", namespace))
		// install the operator without og and installmode, the csv support allnamespace and ownnamespace
		msg, _ = operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/all2support-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(msg).To(o.ContainSubstring("OLM has successfully installed"))
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", "openshift-operators").Output()
			if strings.Contains(msg, "all2support.v0.0.1") {
				e2e.Logf("csv all2support.v0.0.1")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv all2support.v0.0.1 in %s", namespace))
		output, _ = operatorsdkCLI.Run("cleanup").Args("all2support", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-41497-SDK ansible operatorsdk util k8s status in the task", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var k8sstatus = filepath.Join(buildPruningBaseDir, "cache3_v1_k8sstatus.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/k8sstatus-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createK8sstatus, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", k8sstatus, "-p", "NAME=k8sstatus-sample").OutputToFile("config-41497.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createK8sstatus, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("k8sstatus.cache3.k8sstatus.com", "k8sstatus-sample", "-n", namespace, "-o", "yaml").Output()
			if strings.Contains(msg, "hello world") {
				e2e.Logf("k8s_status test hello world")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get k8sstatus-sample hello world in %s", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-38757-SDK operator bundle upgrade from traditional operator installation", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var catalogofupgrade = filepath.Join(buildPruningBaseDir, "catalogsource.yaml")
		var ogofupgrade = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		var subofupgrade = filepath.Join(buildPruningBaseDir, "sub.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		// install operator from sub
		defer operatorsdkCLI.Run("cleanup").Args("upgradeindex", "-n", namespace).Output()
		createCatalog, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalogofupgrade, "-p", "NAME=upgradetest", "NAMESPACE="+namespace, "ADDRESS=quay.io/olmqe/upgradeindex-index:v0.1", "DISPLAYNAME=KakaTest").OutputToFile("catalogsource-41497.json")
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCatalog, "-n", namespace).Execute()
		createOg, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ogofupgrade, "-p", "NAME=kakatest-single", "NAMESPACE="+namespace, "KAKA="+namespace).OutputToFile("createog-41497.json")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg, "-n", namespace).Execute()
		createSub, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subofupgrade, "-p", "NAME=subofupgrade", "NAMESPACE="+namespace, "SOURCENAME=upgradetest", "OPERATORNAME=upgradeindex", "SOURCENAMESPACE="+namespace, "STARTINGCSV=upgradeindex.v0.0.1").OutputToFile("createsub-41497.json")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createSub, "-n", namespace).Execute()
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "upgradeindex.v0.0.1", "-o=jsonpath={.status.phase}", "-n", namespace).Output()
			if strings.Contains(msg, "Succeeded") {
				e2e.Logf("upgradeindexv0.1 installed successfully")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv upgradeindex.v0.0.1 %s", namespace))
		// upgrade by operator-sdk
		msg, err := operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/upgradeindex-bundle:v0.2", "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("Successfully upgraded to"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-42928-SDK support the previous base ansible image [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var previouscache = filepath.Join(buildPruningBaseDir, "cache_v1_previous.yaml")
		var previouscollection = filepath.Join(buildPruningBaseDir, "previous_v1_collectiontest.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/previousansiblebase-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createPreviouscache, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", previouscache, "-p", "NAME=previous-sample").OutputToFile("config-42928.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviouscache, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		// k8s status
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("previous.cache.previous.com", "previous-sample", "-n", namespace, "-o", "yaml").Output()
			if strings.Contains(msg, "hello world") {
				e2e.Logf("previouscache test hello world")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get previous-sample hello world in %s", namespace))

		// migrate test
		msg, err := oc.AsAdmin().Run("describe").Args("secret", "test-secret", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("test:  6 bytes"))

		// blacklist
		msg, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("Skipping"))

		// max concurrent reconciles
		msg, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("\"worker count\":4"))

		// content collection
		createPreviousCollection, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", previouscollection, "-p", "NAME=collectiontest-sample").OutputToFile("config1-42928.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviousCollection, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deploy/previousansiblebase-controller-manager", "-c", "manager", "-n", namespace).Output()
			if strings.Contains(msg, "dummy : Create ConfigMap") {
				e2e.Logf("found dummy : Create ConfigMap")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get log dummy create ConfigMap in %s", namespace))

		output, _ := operatorsdkCLI.Run("cleanup").Args("previousansiblebase", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-42929-SDK support the previous base helm image", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var nginx = filepath.Join(buildPruningBaseDir, "helmbase_v1_nginx.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		defer operatorsdkCLI.Run("cleanup").Args("previoushelmbase", "-n", namespace).Output()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/previoushelmbase-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createPreviousNginx, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", nginx, "-p", "NAME=previousnginx-sample").OutputToFile("config-42929.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createPreviousNginx, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "--no-headers").Output()
			if strings.Contains(msg, "nginx-sample") {
				e2e.Logf("found pod nginx-sample")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get pod nginx-sample in %s", namespace))
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("nginx.helmbase.previous.com", "previousnginx-sample", "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, _ := operatorsdkCLI.Run("cleanup").Args("previoushelmbase", "-n", namespace).Output()
		o.Expect(output).To(o.ContainSubstring("uninstalled"))
	})

	// author: jfan@redhat.com
	g.It("ConnectedOnly-VMonly-Author:jfan-High-42614-SDK validate the deprecated APIs and maxOpenShiftVersion", func() {
		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir -p /tmp/ocp-42614/traefikee-operator").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-42614").Output()
		exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-42614-data/bundle/ /tmp/ocp-42614/traefikee-operator/").Output()

		g.By("with deprecated api, with maxOpenShiftVersion")
		msg, err := operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
		o.Expect(msg).To(o.ContainSubstring("This bundle is using APIs which were deprecated and removed in"))
		o.Expect(msg).NotTo(o.ContainSubstring("error"))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("with deprecated api, with higher version maxOpenShiftVersion")
		exec.Command("bash", "-c", "sed -i 's/4.8/4.9/g' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
		msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
		o.Expect(msg).To(o.ContainSubstring("This bundle is using APIs which were deprecated and removed"))
		o.Expect(msg).To(o.ContainSubstring("error"))

		g.By("with deprecated api, with wrong maxOpenShiftVersion")
		exec.Command("bash", "-c", "sed -i 's/4.9/invalid/g' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
		msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
		o.Expect(msg).To(o.ContainSubstring("csv.Annotations.olm.properties has an invalid value"))
		o.Expect(msg).To(o.ContainSubstring("error"))

		g.By("with deprecated api, without maxOpenShiftVersion")
		exec.Command("bash", "-c", "sed -i '/invalid/d' /tmp/ocp-42614/traefikee-operator/bundle/manifests/traefikee-operator.v2.1.1.clusterserviceversion.yaml").Output()
		msg, _ = operatorsdkCLI.Run("bundle").Args("validate", "/tmp/ocp-42614/traefikee-operator/bundle", "--select-optional", "name=community", "-o", "json-alpha1").Output()
		o.Expect(msg).To(o.ContainSubstring("csv.Annotations not specified olm.maxOpenShiftVersion for an OCP version"))
		o.Expect(msg).To(o.ContainSubstring("error"))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-34462-SDK playbook ansible operator generate the catalog [Slow]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var catalogofcatalog = filepath.Join(buildPruningBaseDir, "catalogsource.yaml")
		var ogofcatalog = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		var subofcatalog = filepath.Join(buildPruningBaseDir, "sub.yaml")
		oc.SetupProject()
		namespace := oc.Namespace()
		containerCLI := container.NewPodmanCLI()
		g.By("Create the playbook ansible operator")
		_, err := exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34462/catalogtest && cd /tmp/ocp-34462/catalogtest && operator-sdk init --plugins=ansible --domain catalogtest.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34462").Output()
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && operator-sdk create api --group cache --version v1 --kind Catalogtest --generate-playbook").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/Dockerfile /tmp/ocp-34462/catalogtest/Dockerfile").Output()
		exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/config/default/manager_auth_proxy_patch.yaml /tmp/ocp-34462/catalogtest/config/default/manager_auth_proxy_patch.yaml").Output()
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make docker-build docker-push IMG=quay.io/olmqe/catalogtest-operator:v"+ocpversion).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-operator:v" + ocpversion)

		// ocp-40219
		g.By("Generate the bundle image and catalog index image")
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && sed -i 's#controller:latest#quay.io/olmqe/catalogtest-operator:v4.10#g' /tmp/ocp-34462/catalogtest/Makefile").Output()
		_, err = exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34462-data/manifests/bases/ /tmp/ocp-34462/catalogtest/config/manifests/").Output()
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make bundle").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && sed -i 's/--container-tool docker //g' Makefile").Output()
		_, err = exec.Command("bash", "-c", "cd /tmp/ocp-34462/catalogtest && make bundle-build bundle-push catalog-build catalog-push BUNDLE_IMG=quay.io/olmqe/catalogtest-bundle:v4.10 CATALOG_IMG=quay.io/olmqe/catalogtest-index:v4.10").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-bundle:v" + ocpversion)
		defer containerCLI.RemoveImage("quay.io/olmqe/catalogtest-index:v" + ocpversion)

		g.By("Install the operator through olm")
		createCatalog, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalogofcatalog, "-p", "NAME=cs-catalog", "NAMESPACE="+namespace, "ADDRESS=quay.io/olmqe/catalogtest-index:v"+ocpversion, "DISPLAYNAME=CatalogTest").OutputToFile("catalogsource-34462.json")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createCatalog, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		createOg, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ogofcatalog, "-p", "NAME=catalogtest-single", "NAMESPACE="+namespace, "KAKA="+namespace).OutputToFile("createog-34462.json")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createOg, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		createSub, _ := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subofcatalog, "-p", "NAME=cataloginstall", "NAMESPACE="+namespace, "SOURCENAME=cs-catalog", "OPERATORNAME=catalogtest", "SOURCENAMESPACE="+namespace, "STARTINGCSV=catalogtest.v0.0.1").OutputToFile("createsub-34462.json")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createSub, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "catalogtest.v0.0.1", "-o=jsonpath={.status.phase}", "-n", namespace).Output()
			if strings.Contains(msg, "Succeeded") {
				e2e.Logf("catalogtest installed successfully")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get csv catalogtest.v0.0.1 in %s", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-High-45141-SDK ansible add k8s event module to operator sdk util", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var k8sevent = filepath.Join(buildPruningBaseDir, "k8s_v1_k8sevent.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/k8sevent-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", "system:serviceaccount:"+namespace+":k8sevent-controller-manager").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		createK8sevent, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", k8sevent, "-p", "NAME=k8sevent-sample").OutputToFile("config-45141.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createK8sevent, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("event", "-n", namespace).Output()
			if strings.Contains(msg, "test-reason") {
				e2e.Logf("k8s_event test")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get k8s event test-name in %s", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-48359-SDK init plugin about hybird helm operator", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var hybirdtest = filepath.Join(buildPruningBaseDir, "cache_v1_hybirdtest.yaml")
		var memcachedbackup = filepath.Join(buildPruningBaseDir, "cache_v1_memcachedbackup.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		defer operatorsdkCLI.Run("cleanup").Args("hybird-operator", "-n", namespace).Output()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/hybird-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createHybird, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", hybirdtest, "-p", "NAME=hybirdtest-sample").OutputToFile("config-48359.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		createMemback, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", memcachedbackup, "-p", "NAME=memcachedbackup-sample").OutputToFile("config-backup-48359.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", "system:serviceaccount:"+namespace+":memcached-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createHybird, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace).Output()
			if strings.Contains(msg, "hybirdtest-sample") {
				e2e.Logf("hybirdtest created success")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get hybirdtest helm type pods in %s", namespace))

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMemback, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace).Output()
			if strings.Contains(msg, "memcachedbackup-sample") {
				e2e.Logf("memcachedbackup-sample created success")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get hybirdtest go type pods in %s", namespace))
	})

	// author: jfan@redhat.com
	g.It("VMonly-ConnectedOnly-Author:jfan-Medium-48366-SDK add ansible prometheus metrics", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		var metrics = filepath.Join(buildPruningBaseDir, "metrics_v1_testmetrics.yaml")
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		namespace := oc.Namespace()
		defer operatorsdkCLI.Run("cleanup").Args("ansiblemetrics", "-n", namespace).Output()
		_, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/testmetrics-bundle:v"+ocpversion, "-n", namespace, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		createMetrics, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", metrics, "-p", "NAME=metrics-sample").OutputToFile("config-48366.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", createMetrics, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace).Output()
			if strings.Contains(msg, "metrics-sample") {
				e2e.Logf("metrics created success")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("can't get metrics samples in %s", namespace))
		promeToken, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n", "openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		promeEp, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ep", "ansiblemetrics-controller-manager-metrics-service", "-o=jsonpath={.subsets[0].addresses[0].ip}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		metricsMsg, err := exec.Command("bash", "-c", "oc exec deployment/ansiblemetrics-controller-manager -n "+namespace+" -- curl -k -H \"Authorization: Bearer "+promeToken+"\" 'https://"+promeEp+":8443/metrics' | grep -E \"Observe|gague|my_counter\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metricsMsg).To(o.ContainSubstring("my gague and set it to 2"))
		o.Expect(metricsMsg).To(o.ContainSubstring("counter"))
		o.Expect(metricsMsg).To(o.ContainSubstring("Observe my histogram"))
		o.Expect(metricsMsg).To(o.ContainSubstring("Observe my summary"))
	})

	// author: chuo@redhat.com
	g.It("Author:chuo-Medium-27718-scorecard remove version flag", func() {
		operatorsdkCLI.showInfo = true
		output, _ := operatorsdkCLI.Run("scorecard").Args("--version").Output()
		o.Expect(output).To(o.ContainSubstring("unknown flag: --version"))
	})

	// author: chuo@redhat.com
	g.It("VMonly-Author:chuo-Critical-37655-run bundle upgrade connect to the Operator SDK CLI", func() {
		operatorsdkCLI.showInfo = true
		output, _ := operatorsdkCLI.Run("run").Args("bundle-upgrade", "-h").Output()
		o.Expect(output).To(o.ContainSubstring("help for bundle-upgrade"))
	})

	// author: chuo@redhat.com
	g.It("VMonly-Author:chuo-Medium-34945-ansible Add flag metricsaddr for ansible operator", func() {
		operatorsdkCLI.showInfo = true
		result, err := exec.Command("bash", "-c", "ansible-operator run --help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("--metrics-bind-address"))
	})
	// author: chuo@redhat.com
	g.It("Author:xzha-High-52126-Sync 1.24 to downstream", func() {
		operatorsdkCLI.showInfo = true
		output, _ := operatorsdkCLI.Run("version").Args().Output()
		o.Expect(output).To(o.ContainSubstring("v1.24"))
	})
	// author: chuo@redhat.com
	g.It("ConnectedOnly-VMonly-Author:chuo-High-34427-Ensure that Ansible Based Operators creation is working", func() {
		architecture := exutil.GetClusterArchitecture(oc)
		if architecture != "amd64" && architecture != "arm64" {
			g.Skip("Do not support " + architecture)
		}
		imageTag := "quay.io/olmqe/memcached-operator-ansible-base:v" + ocpversion + getRandomString()
		if architecture == "arm64" {
			imageTag = "quay.io/olmqe/memcached-operator-ansible-base:v4.11-34427"
		}
		nsSystem := "system-ocp34427" + getRandomString()
		nsOperator := "memcached-operator-34427-system-" + getRandomString()

		tmpBasePath := "/tmp/ocp-34427-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator-34427")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		if imageTag != "quay.io/olmqe/memcached-operator-ansible-base:v4.11-34427" {
			quayCLI := container.NewQuayCLI()
			defer quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		}

		defer func() {
			g.By("step: undeploy")
			_, err = makeCLI.Run("undeploy").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("step: init Ansible Based Operator")
		output, err := operatorsdkCLI.Run("init").Args("--plugins=ansible", "--domain", "example.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Next"))

		g.By("step: Create API.")
		output, err = operatorsdkCLI.Run("create").Args("api", "--group", "cache", "--version", "v1alpha1", "--kind", "Memcached34427", "--generate-role").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Writing kustomize manifests"))

		dataPath := "test/extended/util/operatorsdk/ocp-34427-data/roles/memcached/"
		err = copy(filepath.Join(dataPath, "tasks", "main.yml"), filepath.Join(tmpPath, "roles", "memcached34427", "tasks", "main.yml"))
		o.Expect(err).NotTo(o.HaveOccurred())
		err = copy(filepath.Join(dataPath, "defaults", "main.yml"), filepath.Join(tmpPath, "roles", "memcached34427", "defaults", "main.yml"))
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$d' %s/config/samples/cache_v1alpha1_memcached34427.yaml", tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$a\\  size: 3' %s/config/samples/cache_v1alpha1_memcached34427.yaml", tmpPath)).Output()

		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/name: system/name: %s/g' `grep -rl \"name: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: system/namespace: %s/g'  `grep -rl \"namespace: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: memcached-operator-34427-system/namespace: %s/g'  `grep -rl \"namespace: memcached-operator-34427-system\" %s`", nsOperator, tmpPath)).Output()

		g.By("step: Push the operator image")
		dockerFilePath := filepath.Join(tmpPath, "Dockerfile")
		replaceContent(dockerFilePath, "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml", "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml --force")
		tokenDir := "/tmp/ocp-34427-auth" + getRandomString()
		err = os.MkdirAll(tokenDir, os.ModePerm)
		defer os.RemoveAll(tokenDir)
		if err != nil {
			e2e.Failf("fail to create the token folder:%s", tokenDir)
		}
		_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", fmt.Sprintf("--to=%s", tokenDir), "--confirm").Output()
		if err != nil {
			e2e.Failf("Fail to get the cluster auth %v", err)
		}
		switch architecture {
		case "amd64":
			podmanCLI := container.NewPodmanCLI()
			podmanCLI.ExecCommandPath = tmpPath
			output, err := podmanCLI.Run("build").Args(tmpPath, "--arch", "amd64", "--tag", imageTag, "--authfile", fmt.Sprintf("%s/.dockerconfigjson", tokenDir)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Successfully"))
			output, err = podmanCLI.Run("push").Args(imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Storing signatures"))
		case "arm64":
			e2e.Logf("platfrom is arm64, IMG is " + imageTag)
		}

		g.By("step: Install the CRD")
		output, err = makeCLI.Run("install").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("memcached34427s.cache.example.com created"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "memcached34427s.cache.example.com").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("NotFound"))

		g.By("step: Deploy the operator")
		output, err = makeCLI.Run("deploy").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("deployment.apps/memcached-operator-34427-controller-manager"))

		waitErr := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			lines := strings.Split(podList, "\n")
			for _, line := range lines {
				if strings.Contains(line, "memcached-operator-34427-controller-manager") {
					e2e.Logf("found pod memcached-operator-34427-controller-manager")
					if strings.Contains(line, "Running") {
						e2e.Logf("the status of pod memcached-operator-34427-controller-manager is Running")
						return true, nil
					}
					e2e.Logf("the status of pod memcached-operator-34427-controller-manager is not Running")
					return false, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "No memcached-operator-34427-controller-manager")

		g.By("step: Create the resource")
		filePath := filepath.Join(tmpPath, "config", "samples", "cache_v1alpha1_memcached34427.yaml")
		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", filePath, "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "memcached34427-sample") {
				e2e.Logf("found pod memcached34427-sample")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "No pod memcached34427-sample")
		waitErr = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment/memcached34427-sample-memcached", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "3 desired | 3 updated | 3 total | 3 available | 0 unavailable") {
				e2e.Logf("deployment/memcached34427-sample-memcached is created successfully")
				return true, nil
			}
			return false, nil
		})
		if waitErr != nil {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment/memcached34427-sample-memcached", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(msg)
			msg, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", nsOperator).Output()
			e2e.Logf(msg)
		}
		exutil.AssertWaitPollNoErr(waitErr, "the status of deployment/memcached34427-sample-memcached is wrong")

		g.By("34427 SUCCESS")
	})

	// author: chuo@redhat.com
	g.It("ConnectedOnly-VMonly-Author:chuo-Medium-34366-change ansible operator flags from maxWorkers using env MAXCONCURRENTRECONCILES ", func() {
		architecture := exutil.GetClusterArchitecture(oc)
		if architecture != "amd64" && architecture != "arm64" {
			g.Skip("Do not support " + architecture)
		}
		imageTag := "quay.io/olmqe/memcached-operator-max-worker:v" + ocpversion + getRandomString()
		if architecture == "arm64" {
			imageTag = "quay.io/olmqe/memcached-operator-max-worker:v4.11-34366"
		}
		nsSystem := "system-ocp34366" + getRandomString()
		nsOperator := "memcached-operator-34366-system-" + getRandomString()

		tmpBasePath := "/tmp/ocp-34366-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator-34366")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		if imageTag != "quay.io/olmqe/memcached-operator-max-worker:v4.11-34366" {
			quayCLI := container.NewQuayCLI()
			defer quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		}

		defer func() {
			g.By("step: undeploy")
			_, err = makeCLI.Run("undeploy").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("step: init Ansible Based Operator")
		output, err := operatorsdkCLI.Run("init").Args("--plugins=ansible", "--domain", "example.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Next"))

		g.By("step: Create API.")
		output, err = operatorsdkCLI.Run("create").Args("api", "--group", "cache", "--version", "v1alpha1", "--kind", "Memcached34366", "--generate-role").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Writing kustomize manifests"))

		dataPath := "test/extended/util/operatorsdk/ocp-34366-data/"
		err = copy(filepath.Join(dataPath, "roles", "memcached", "tasks", "main.yml"), filepath.Join(tmpPath, "roles", "memcached34366", "tasks", "main.yml"))
		o.Expect(err).NotTo(o.HaveOccurred())
		err = copy(filepath.Join(dataPath, "config", "manager", "manager.yaml"), filepath.Join(tmpPath, "config", "manager", "manager.yaml"))
		o.Expect(err).NotTo(o.HaveOccurred())

		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/name: system/name: %s/g' `grep -rl \"name: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: system/namespace: %s/g'  `grep -rl \"namespace: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: memcached-operator-34366-system/namespace: %s/g'  `grep -rl \"namespace: memcached-operator-34366-system\" %s`", nsOperator, tmpPath)).Output()

		g.By("step: build and push img.")
		dockerFilePath := filepath.Join(tmpPath, "Dockerfile")
		replaceContent(dockerFilePath, "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml", "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml --force")
		tokenDir := "/tmp/ocp-34366-auth" + getRandomString()
		err = os.MkdirAll(tokenDir, os.ModePerm)
		defer os.RemoveAll(tokenDir)
		if err != nil {
			e2e.Failf("fail to create the token folder:%s", tokenDir)
		}
		_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", fmt.Sprintf("--to=%s", tokenDir), "--confirm").Output()
		if err != nil {
			e2e.Failf("Fail to get the cluster auth %v", err)
		}
		switch architecture {
		case "amd64":
			podmanCLI := container.NewPodmanCLI()
			podmanCLI.ExecCommandPath = tmpPath
			output, err = podmanCLI.Run("build").Args(tmpPath, "--arch", "amd64", "--tag", imageTag, "--authfile", fmt.Sprintf("%s/.dockerconfigjson", tokenDir)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Successfully"))
			output, err = podmanCLI.Run("push").Args(imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Storing signatures"))
		case "arm64":
			e2e.Logf("platfrom is arm64, IMG is " + imageTag)
		}

		g.By("step: Install the CRD")
		output, err = makeCLI.Run("install").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("memcached34366s.cache.example.com created"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "memcached34366s.cache.example.com").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("NotFound"))

		g.By("step: Deploy the operator")
		output, err = makeCLI.Run("deploy").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("deployment.apps/memcached-operator-34366-controller-manager"))
		_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", fmt.Sprintf("system:serviceaccount:%s:default", nsOperator)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		waitErr := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "Running") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "memcached-operator-34366-controller-manager has no Starting workers")

		output, err = oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment.apps/memcached-operator-34366-controller-manager", "-c", "manager", "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("\"worker count\":6"))
	})

	// author: chuo@redhat.com
	g.It("VMonly-Author:chuo-Medium-34883-SDK stamp on Operator bundle image", func() {
		operatorsdkCLI.showInfo = true
		exec.Command("bash", "-c", "mkdir -p /tmp/ocp-34883/memcached-operator && cd /tmp/ocp-34883/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-34883").Output()
		exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
		exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && mkdir -p /tmp/ocp-34883/memcached-operator/config/manifests/").Output()
		exec.Command("bash", "-c", "cp -rf test/extended/util/operatorsdk/ocp-34883-data/manifests/bases/ /tmp/ocp-34883/memcached-operator/config/manifests/").Output()
		waitErr := wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := exec.Command("bash", "-c", "cd /tmp/ocp-34883/memcached-operator && make bundle").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(msg), "operator-sdk bundle validate ./bundle") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("operator-sdk bundle generate failed"))

		output, err := exec.Command("bash", "-c", "cat /tmp/ocp-34883/memcached-operator/bundle/metadata/annotations.yaml  | grep -E \"operators.operatorframework.io.metrics.builder: operator-sdk\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("operators.operatorframework.io.metrics.builder: operator-sdk"))
	})

	// author: chuo@redhat.com
	g.It("VMonly-ConnectedOnly-Author:chuo-Critical-45431-Critical-45428-Medium-43973-Medium-43976-Medium-48630-scorecard basic test migration and migrate olm tests and proxy configurable and xunit adjustment and sa ", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		tmpBasePath := "/tmp/ocp-45431-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		g.By("step: init Ansible Based Operator")
		_, err = operatorsdkCLI.Run("init").Args("--plugins=ansible", "--domain", "example.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Create API.")
		_, err = operatorsdkCLI.Run("create").Args("api", "--group", "cache", "--version", "v1alpha1", "--kind", "Memcached", "--generate-role").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		manifestsPath := filepath.Join(tmpPath, "config", "manifests")
		err = os.MkdirAll(manifestsPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", fmt.Sprintf("cp -rf test/extended/util/operatorsdk/ocp-43973-data/manifests/bases/ %s", manifestsPath)).Output()

		waitErr := wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := makeCLI.Run("bundle").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(msg), "operator-sdk bundle validate ./bundle") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "operator-sdk bundle generate failed")

		output, _ := operatorsdkCLI.Run("version").Args("").Output()
		e2e.Logf("The OperatorSDK version is %s", output)
		//ocp-43973
		g.By("scorecard basic test migration")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=basic-check-spec-test", "-n", oc.Namespace()).Output()
		e2e.Logf(" scorecard bundle %v", err)
		o.Expect(output).To(o.ContainSubstring("State: fail"))
		o.Expect(output).To(o.ContainSubstring("spec missing from [memcached-sample]"))

		//ocp-43976
		g.By("migrate OLM tests-bundle validation")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: pass"))

		g.By("migrate OLM tests-crds have validation test")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-crds-have-validation-test", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: pass"))

		g.By("migrate OLM tests-crds have resources test")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-crds-have-resources-test", "-n", oc.Namespace()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: fail"))
		o.Expect(output).To(o.ContainSubstring("Owned CRDs do not have resources specified"))

		g.By("migrate OLM tests- spec descriptors test")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-spec-descriptors-test", "-n", oc.Namespace()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: fail"))

		g.By("migrate OLM tests- status descriptors test")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-status-descriptors-test", "-n", oc.Namespace()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: fail"))
		o.Expect(output).To(o.ContainSubstring("memcacheds.cache.example.com does not have a status descriptor"))

		//ocp-48630
		g.By("scorecard proxy container port should be configurable")
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$a\\proxy-port: 9001' %s/bundle/tests/scorecard/config.yaml", tmpPath)).Output()
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: pass"))

		//ocp-45428 xunit adjustments - add nested tags and attributes
		g.By("migrate OLM tests-bundle validation to generate a pass xunit output")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test", "-o", "xunit", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("<testsuite name=\"olm-bundle-validation-test\""))
		o.Expect(output).To(o.ContainSubstring("<testcase name=\"olm-bundle-validation\""))

		g.By("migrate OLM tests-status descriptors to generate a failed xunit output")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-status-descriptors-test", "-o", "xunit", "-n", oc.Namespace()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("<testcase name=\"olm-status-descriptors\""))
		o.Expect(output).To(o.ContainSubstring("<system-out>Loaded ClusterServiceVersion:"))
		o.Expect(output).To(o.ContainSubstring("failure"))

		// ocp-45431 bring in latest o-f/api to SDK BEFORE 1.13
		g.By("use an non-exist service account to run test")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test ", "-s", "testing", "-n", oc.Namespace()).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("serviceaccount \"testing\" not found"))
		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", "test/extended/util/operatorsdk/ocp-43973-data/sa_testing.yaml", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test ", "-s", "testing", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	// author: chuo@redhat.com
	g.It("VMonly-ConnectedOnly-Author:chuo-High-43660-scorecard support storing test output", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		tmpBasePath := "/tmp/ocp-43660-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator-43660")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		g.By("step: init Ansible Based Operator")
		_, err = operatorsdkCLI.Run("init").Args("--plugins=ansible", "--domain", "example.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Create API.")
		_, err = operatorsdkCLI.Run("create").Args("api", "--group", "cache", "--version", "v1alpha1", "--kind", "Memcached43660", "--generate-role").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: make bundle.")
		manifestsPath := filepath.Join(tmpPath, "config", "manifests")
		err = os.MkdirAll(manifestsPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		exec.Command("bash", "-c", fmt.Sprintf("cp -rf test/extended/util/operatorsdk/ocp-43660-data/manifests/bases/ %s", manifestsPath)).Output()

		waitErr := wait.Poll(30*time.Second, 120*time.Second, func() (bool, error) {
			msg, err := makeCLI.Run("bundle").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(msg), "operator-sdk bundle validate ./bundle") {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "operator-sdk bundle generate failed")

		g.By("run scorecard ")
		output, err := operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "4m", "--selector=test=olm-bundle-validation-test", "--test-output", "/testdata", "-n", oc.Namespace()).Output()
		e2e.Logf(" scorecard bundle %v", err)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: pass"))
		o.Expect(output).To(o.ContainSubstring("Name: olm-bundle-validation"))

		g.By("step: modify test config.")
		configFilePath := filepath.Join(tmpPath, "bundle", "tests", "scorecard", "config.yaml")
		replaceContent(configFilePath, "mountPath: {}", "mountPath:\n          path: /test-output")

		g.By("scorecard basic test migration")
		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=basic-check-spec-test", "-n", oc.Namespace()).Output()
		e2e.Logf(" scorecard bundle %v", err)
		o.Expect(output).To(o.ContainSubstring("State: fail"))
		o.Expect(output).To(o.ContainSubstring("spec missing from [memcached43660-sample]"))
		pathOutput := filepath.Join(tmpPath, "test-output", "basic", "basic-check-spec-test")
		_, err = os.Stat(pathOutput)
		o.Expect(err).NotTo(o.HaveOccurred())

		output, err = operatorsdkCLI.Run("scorecard").Args("./bundle", "-c", "./bundle/tests/scorecard/config.yaml", "-w", "60s", "--selector=test=olm-bundle-validation-test", "-n", oc.Namespace()).Output()
		e2e.Logf(" scorecard bundle %v", err)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("State: pass"))
		pathOutput = filepath.Join(tmpPath, "test-output", "olm", "olm-bundle-validation-test")
		_, err = os.Stat(pathOutput)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// author: chuo@redhat.com
	g.It("ConnectedOnly-Author:chuo-High-31219-scorecard bundle is mandatory ", func() {
		operatorsdkCLI.showInfo = true
		oc.SetupProject()
		exec.Command("bash", "-c", "mkdir -p /tmp/ocp-31219/memcached-operator && cd /tmp/ocp-31219/memcached-operator && operator-sdk init --plugins=ansible --domain example.com").Output()
		defer exec.Command("bash", "-c", "rm -rf /tmp/ocp-31219").Output()
		exec.Command("bash", "-c", "cd /tmp/ocp-31219/memcached-operator && operator-sdk create api --group cache --version v1alpha1 --kind Memcached --generate-role").Output()
		exec.Command("bash", "-c", "cd /tmp/ocp-31219/memcached-operator && operator-sdk generate bundle --deploy-dir=config --crds-dir=config/crds --version=0.0.1").Output()
		output, err := operatorsdkCLI.Run("scorecard").Args("/tmp/ocp-31219/memcached-operator/bundle", "-c", "/tmp/ocp-31219/memcached-operator/config/scorecard/bases/config.yaml", "-w", "60s", "--selector=test=basic-check-spec-test", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("tests selected"))
	})

	// author: chuo@redhat.com
	g.It("ConnectedOnly-VMonly-Author:chuo-High-34426-Ensure that Helm Based Operators creation is working ", func() {
		architecture := exutil.GetClusterArchitecture(oc)
		if architecture != "amd64" && architecture != "arm64" {
			g.Skip("Do not support " + architecture)
		}
		imageTag := "quay.io/olmqe/nginx-operator-base:v4.10-34426" + getRandomString()
		nsSystem := "system-34426-" + getRandomString()
		nsOperator := "nginx-operator-34426-system"

		tmpBasePath := "/tmp/ocp-34426-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "nginx-operator-34426")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		defer func() {
			quayCLI := container.NewQuayCLI()
			quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		}()

		defer func() {
			g.By("delete nginx-sample")
			filePath := filepath.Join(tmpPath, "config", "samples", "demo_v1_nginx34426.yaml")
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", filePath, "-n", nsOperator).Output()
			waitErr := wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("Nginx34426", "-n", nsOperator).Output()
				if strings.Contains(output, "nginx34426-sample") {
					e2e.Logf("nginx34426-sample still exists")
					return false, nil
				}
				return true, nil
			})
			if waitErr != nil {
				e2e.Logf("delete nginx-sample failed, still try to run make undeploy")
			}
			g.By("step: run make undeploy")
			_, err = makeCLI.Run("undeploy").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		g.By("step: init Helm Based Operators")
		output, err := operatorsdkCLI.Run("init").Args("--plugins=helm").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Next: define a resource with"))

		g.By("step: Create API.")
		output, err = operatorsdkCLI.Run("create").Args("api", "--group", "demo", "--version", "v1", "--kind", "Nginx34426").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("nginx"))

		g.By("step: modify namespace")
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/name: system/name: %s/g' `grep -rl \"name: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: system/namespace: %s/g'  `grep -rl \"namespace: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: nginx-operator-34426-system/namespace: %s/g'  `grep -rl \"namespace: nginx-operator-system\" %s`", nsOperator, tmpPath)).Output()

		g.By("step: build and Push the operator image")
		tokenDir := "/tmp/ocp-34426" + getRandomString()
		err = os.MkdirAll(tokenDir, os.ModePerm)
		defer os.RemoveAll(tokenDir)
		if err != nil {
			e2e.Failf("fail to create the token folder:%s", tokenDir)
		}
		_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", fmt.Sprintf("--to=%s", tokenDir), "--confirm").Output()
		if err != nil {
			e2e.Failf("Fail to get the cluster auth %v", err)
		}
		switch architecture {
		case "amd64":
			podmanCLI := container.NewPodmanCLI()
			podmanCLI.ExecCommandPath = tmpPath
			output, err := podmanCLI.Run("build").Args(tmpPath, "--arch", "amd64", "--tag", imageTag, "--authfile", fmt.Sprintf("%s/.dockerconfigjson", tokenDir)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Successfully"))
			output, err = podmanCLI.Run("push").Args(imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Storing signatures"))
		case "arm64":
			podmanCLI := container.NewPodmanCLI()
			podmanCLI.ExecCommandPath = tmpPath
			output, err := podmanCLI.Run("build").Args(tmpPath, "--arch", "arm64", "--tag", imageTag, "--authfile", fmt.Sprintf("%s/.dockerconfigjson", tokenDir)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Successfully"))
			output, err = podmanCLI.Run("push").Args(imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Storing signatures"))
		}

		g.By("step: Install the CRD")
		output, err = makeCLI.Run("install").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("nginx34426s.demo.my.domain"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "nginx34426s.demo.my.domain").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("NotFound"))

		g.By("step: Deploy the operator")
		output, err = makeCLI.Run("deploy").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("deployment.apps/nginx-operator-34426-controller-manager created"))
		waitErr := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			lines := strings.Split(podList, "\n")
			for _, line := range lines {
				if strings.Contains(line, "nginx-operator-34426-controller-manager") {
					e2e.Logf("found pod nginx-operator-34426-controller-manager")
					if strings.Contains(line, "Running") {
						e2e.Logf("the status of pod nginx-operator-34426-controller-manager is Running")
						return true, nil
					}
					e2e.Logf("the status of pod nginx-operator-34426-controller-manager is not Running")
					return false, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "No nginx-operator-34426-controller-manager")

		_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "anyuid", fmt.Sprintf("system:serviceaccount:%s:nginx34426-sample", nsOperator)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Create the resource")
		filePath := filepath.Join(tmpPath, "config", "samples", "demo_v1_nginx34426.yaml")
		replaceContent(filePath, "repository: nginx", "repository: quay.io/olmqe/nginx-docker")
		replaceContent(filePath, "tag: \"\"", "tag: multi-arch")

		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", filePath, "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		waitErr = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(podList, "nginx34426-sample") {
				e2e.Logf("No nginx34426-sample")
				return false, nil
			}
			lines := strings.Split(podList, "\n")
			for _, line := range lines {
				if strings.Contains(line, "nginx34426-sample") {
					e2e.Logf("found pod nginx34426-sample")
					if strings.Contains(line, "Running") {
						e2e.Logf("the status of pod nginx34426-sample is Running")
						return true, nil
					}
					e2e.Logf("the status of pod nginx34426-sample is not Running")
					return false, nil
				}
			}
			return false, nil
		})
		if waitErr != nil {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", nsOperator).Output()
			e2e.Logf(msg)
		}
		exutil.AssertWaitPollNoErr(waitErr, "No nginx34426-sample is in Running status")
	})

	// author: xzha@redhat.com
	g.It("VMonly-ConnectedOnly-Author:xzha-Critical-38101-implement IndexImageCatalogCreator", func() {
		operatorsdkCLI.showInfo = true
		g.By(fmt.Sprintf("0) check the cluster proxy configuration"))
		httpProxy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("proxy", "cluster", "-o=jsonpath={.status.httpProxy}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if httpProxy != "" {
			g.Skip("Skip for cluster with proxy")
		}

		g.By("step: create new project")
		oc.SetupProject()
		ns := oc.Namespace()

		g.By("step: run bundle 1")
		output, err := operatorsdkCLI.Run("run").Args("bundle", "quay.io/olmqe/kubeturbo-bundle:v8.4.0", "-n", ns, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("OLM has successfully installed"))

		g.By("step: check catsrc annotations")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("index-image"))
		o.Expect(output).To(o.ContainSubstring("injected-bundles"))
		o.Expect(output).To(o.ContainSubstring("registry-pod-name"))
		o.Expect(output).To(o.ContainSubstring("quay.io/olmqe/kubeturbo-bundle:v8.4.0"))
		o.Expect(output).NotTo(o.ContainSubstring("quay.io/olmqe/kubeturbo-bundle:v8.5.0"))

		g.By("step: check catsrc address")
		podname1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.metadata.annotations.operators\\.operatorframework\\.io/registry-pod-name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podname1).NotTo(o.BeEmpty())

		ip, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podname1, "-n", oc.Namespace(), "-o=jsonpath={.status.podIP}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip).NotTo(o.BeEmpty())

		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.spec.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal(ip + ":50051"))

		g.By("step: check catsrc sourceType")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.spec.sourceType}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("grpc"))

		g.By("step: check packagemanifest")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "kubeturbo", "-n", oc.Namespace(), "-o=jsonpath={.status.channels[*].currentCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubeturbo-operator.v8.4.0"))

		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "kubeturbo", "-n", oc.Namespace(), "-o=jsonpath={.status.channels[*].name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("8.4.0"))
		o.Expect(output).To(o.ContainSubstring("stable"))

		g.By("step: upgrade bundle")
		output, err = operatorsdkCLI.Run("run").Args("bundle-upgrade", "quay.io/olmqe/kubeturbo-bundle:v8.5.0", "-n", ns, "--timeout", "5m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Successfully upgraded to"))
		waitErr := wait.Poll(15*time.Second, 360*time.Second, func() (bool, error) {
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "kubeturbo-operator.v8.5.0", "-n", ns).Output()
			if strings.Contains(msg, "Succeeded") {
				e2e.Logf("upgrade to 8.5.0 success")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("upgradeoperator upgrade failed in %s ", ns))

		g.By("step: check catsrc annotations")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.metadata.annotations}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("index-image"))
		o.Expect(output).To(o.ContainSubstring("injected-bundles"))
		o.Expect(output).To(o.ContainSubstring("registry-pod-name"))
		o.Expect(output).To(o.ContainSubstring("quay.io/olmqe/kubeturbo-bundle:v8.4.0"))
		o.Expect(output).To(o.ContainSubstring("quay.io/olmqe/kubeturbo-bundle:v8.5.0"))

		g.By("step: check catsrc address")
		podname2, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.metadata.annotations.operators\\.operatorframework\\.io/registry-pod-name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podname2).NotTo(o.BeEmpty())
		o.Expect(podname2).NotTo(o.Equal(podname1))

		ip, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podname2, "-n", oc.Namespace(), "-o=jsonpath={.status.podIP}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ip).NotTo(o.BeEmpty())

		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.spec.address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal(ip + ":50051"))

		g.By("step: check catsrc sourceType")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "kubeturbo-catalog", "-n", oc.Namespace(), "-o=jsonpath={.spec.sourceType}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal("grpc"))

		g.By("step: check packagemanifest")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "kubeturbo", "-n", oc.Namespace(), "-o=jsonpath={.status.channels[*].currentCSV}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("kubeturbo-operator.v8.5.0"))
		o.Expect(output).To(o.ContainSubstring("kubeturbo-operator.v8.4.0"))

		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "kubeturbo", "-n", oc.Namespace(), "-o=jsonpath={.status.channels[*].name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("8.5.0"))
		o.Expect(output).To(o.ContainSubstring("8.4.0"))
		o.Expect(output).To(o.ContainSubstring("stable"))

		g.By("SUCCESS")

	})

	// author: xzha@redhat.com
	g.It("VMonly-ConnectedOnly-Author:xzha-High-42028-Update python kubernetes and python openshift to kubernetes 12.0.0", func() {
		if os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != "" {
			g.Skip("HTTP_PROXY is not empty - skipping test ...")
		}
		imageTag := "registry-proxy.engineering.redhat.com/rh-osbs/openshift-ose-ansible-operator:v4.10"
		containerCLI := container.NewPodmanCLI()
		e2e.Logf("create container with image %s", imageTag)
		id, err := containerCLI.ContainerCreate(imageTag, "test-42028", "/bin/sh", true)
		defer func() {
			e2e.Logf("stop container %s", id)
			containerCLI.ContainerStop(id)
			e2e.Logf("remove container %s", id)
			err := containerCLI.ContainerRemove(id)
			if err != nil {
				e2e.Failf("Defer: fail to remove container %s", id)
			}
		}()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("container id is %s", id)

		e2e.Logf("start container %s", id)
		err = containerCLI.ContainerStart(id)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("start container %s successful", id)

		commandStr := []string{"pip3", "show", "kubernetes"}
		e2e.Logf("run command %s", commandStr)
		output, err := containerCLI.Exec(id, commandStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Version:"))
		o.Expect(output).To(o.ContainSubstring("12.0."))

		commandStr = []string{"pip3", "show", "openshift"}
		e2e.Logf("run command %s", commandStr)
		output, err = containerCLI.Exec(id, commandStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Version:"))
		o.Expect(output).To(o.ContainSubstring("0.12."))

		e2e.Logf("OCP 42028 SUCCESS")
	})

	// author: xzha@redhat.com
	g.It("ConnectedOnly-VMonly-Author:xzha-High-44295-Ensure that Go type Operators creation is working [Slow]", func() {
		if os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != "" {
			g.Skip("HTTP_PROXY is not empty - skipping test ...")
		}
		architecture := exutil.GetClusterArchitecture(oc)
		if architecture != "amd64" {
			g.Skip("Do not support " + architecture)
		}
		buildPruningBaseDir := exutil.FixturePath("testdata", "operatorsdk")
		dataPath := filepath.Join(buildPruningBaseDir, "ocp-44295-data")
		quayCLI := container.NewQuayCLI()
		imageTag := "quay.io/olmqe/memcached-operator:44295-" + getRandomString()
		tmpBasePath := "/tmp/ocp-44295-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator-44295")
		nsSystem := "system-ocp44295" + getRandomString()
		nsOperator := "memcached-operator-system-ocp44295" + getRandomString()
		defer os.RemoveAll(tmpBasePath)
		defer quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		g.By("step: generate go type operator")
		defer func() {
			_, err = makeCLI.Run("undeploy").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		output, err := operatorsdkCLI.Run("init").Args("--domain=example.com", "--plugins=go.kubebuilder.io/v3", "--repo=github.com/example-inc/memcached-operator-44295").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Writing scaffold for you to edit"))
		o.Expect(output).To(o.ContainSubstring("Next: define a resource with"))

		g.By("step: Create a Memcached API.")
		output, err = operatorsdkCLI.Run("create").Args("api", "--resource=true", "--controller=true", "--group=cache", "--version=v1alpha1", "--kind=Memcached44295").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Update dependencies"))
		o.Expect(output).To(o.ContainSubstring("Next"))

		g.By("step: update API")
		err = copy(filepath.Join(dataPath, "memcached_types.go"), filepath.Join(tmpPath, "api", "v1alpha1", "memcached44295_types.go"))
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = makeCLI.Run("generate").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: make manifests")
		_, err = makeCLI.Run("manifests").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: modify namespace and controllers")
		crFilePath := filepath.Join(tmpPath, "config", "samples", "cache_v1alpha1_memcached44295.yaml")
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$d' %s", crFilePath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$a\\  size: 3' %s", crFilePath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/name: system/name: system-ocp44295/g' `grep -rl \"name: system\" %s`", tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: system/namespace: %s/g'  `grep -rl \"namespace: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: memcached-operator-44295-system/namespace: %s/g'  `grep -rl \"namespace: memcached-operator-44295-system\" %s`", nsOperator, tmpPath)).Output()
		err = copy(filepath.Join(dataPath, "memcached_controller.go"), filepath.Join(tmpPath, "controllers", "memcached44295_controller.go"))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Build the operator image")
		dockerFilePath := filepath.Join(tmpPath, "Dockerfile")
		replaceContent(dockerFilePath, "golang:", "quay.io/olmqe/golang:")
		output, err = makeCLI.Run("docker-build").Args("IMG=" + imageTag).Output()
		if (err != nil) && strings.Contains(output, "go mod tidy") {
			e2e.Logf("execute go mod tidy")
			exec.Command("bash", "-c", fmt.Sprintf("cd %s; go mod tidy", tmpPath)).Output()
			output, err = makeCLI.Run("docker-build").Args("IMG=" + imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("docker build -t"))
		} else {
			o.Expect(output).To(o.ContainSubstring("docker build -t"))
		}

		g.By("step: Push the operator image")
		_, err = makeCLI.Run("docker-push").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("step: Install kustomize")
		kustomizePath := "/root/kustomize"
		binPath := filepath.Join(tmpPath, "bin")
		exec.Command("bash", "-c", fmt.Sprintf("cp %s %s", kustomizePath, binPath)).Output()

		g.By("step: Install the CRD")
		output, err = makeCLI.Run("install").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("memcached44295s.cache.example.com"))

		g.By("step: Deploy the operator")
		output, err = makeCLI.Run("deploy").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("deployment.apps/memcached-operator-44295-controller-manager"))

		waitErr := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			lines := strings.Split(podList, "\n")
			for _, line := range lines {
				if strings.Contains(line, "memcached-operator-44295-controller-manager") {
					e2e.Logf("found pod memcached-operator-44295-controller-manager")
					if strings.Contains(line, "Running") {
						e2e.Logf("the status of pod memcached-operator-44295-controller-manager is Running")
						return true, nil
					}
					e2e.Logf("the status of pod memcached-operator-44295-controller-manager is not Running")
					return false, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("No memcached-operator-44295-controller-manager in project %s", nsOperator))
		msg, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args("deployment.apps/memcached-operator-44295-controller-manager", "-c", "manager", "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.ContainSubstring("Starting workers"))

		g.By("step: Create the resource")
		_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crFilePath, "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "memcached44295-sample") {
				e2e.Logf("found pod memcached44295-sample")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("No memcached44295-sample in project %s", nsOperator))

		waitErr = wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment/memcached44295-sample", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "3 desired | 3 updated | 3 total | 3 available | 0 unavailable") {
				e2e.Logf("deployment/memcached44295-sample is created successfully")
				return true, nil
			}
			return false, nil
		})
		if waitErr != nil {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("deployment/memcached44295-sample", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf(msg)
			msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", nsOperator).Output()
			e2e.Logf(msg)
		}
		exutil.AssertWaitPollNoErr(waitErr, "the status of deployment/memcached44295-sample is wrong")

		g.By("OCP 44295 SUCCESS")
	})

	// author: chuo@redhat.com
	g.It("VMonly-ConnectedOnly-Author:chuo-High-40341-Ansible operator needs a way to pass vars as unsafe ", func() {
		imageTag := "quay.io/olmqe/memcached-operator-pass-unsafe:v" + ocpversion + getRandomString()
		architecture := exutil.GetClusterArchitecture(oc)
		if architecture != "arm64" && architecture != "amd64" {
			g.Skip("Do not support " + architecture)
		}
		if architecture == "arm64" {
			imageTag = "quay.io/olmqe/memcached-operator-pass-unsafe:v4.11-40341"
		}
		nsSystem := "system-40341-" + getRandomString()
		nsOperator := "memcached-operator-40341-system-" + getRandomString()

		tmpBasePath := "/tmp/ocp-40341-" + getRandomString()
		tmpPath := filepath.Join(tmpBasePath, "memcached-operator-40341")
		err := os.MkdirAll(tmpPath, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpBasePath)
		operatorsdkCLI.ExecCommandPath = tmpPath
		makeCLI.ExecCommandPath = tmpPath

		defer func() {
			if imageTag != "quay.io/olmqe/memcached-operator-pass-unsafe:v4.11-40341" {
				quayCLI := container.NewQuayCLI()
				quayCLI.DeleteTag(strings.Replace(imageTag, "quay.io/", "", 1))
			}
		}()

		defer func() {
			deployfilepath := filepath.Join(tmpPath, "config", "samples", "cache_v1alpha1_memcached40341.yaml")
			_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", deployfilepath, "-n", nsOperator).Output()
			g.By("step: undeploy")
			_, err = makeCLI.Run("undeploy").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("step: init Ansible Based Operator")
		output, err := operatorsdkCLI.Run("init").Args("--plugins=ansible", "--domain", "example.com").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Next"))

		g.By("step: Create API.")
		output, err = operatorsdkCLI.Run("create").Args("api", "--group", "cache", "--version", "v1alpha1", "--kind", "Memcached40341", "--generate-role").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Writing kustomize manifests"))

		deployfilepath := filepath.Join(tmpPath, "config", "samples", "cache_v1alpha1_memcached40341.yaml")
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$d' %s", deployfilepath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$a\\  size: 3' %s", deployfilepath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i '$a\\  testKey: testVal' %s", deployfilepath)).Output()

		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/name: system/name: %s/g' `grep -rl \"name: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: system/namespace: %s/g'  `grep -rl \"namespace: system\" %s`", nsSystem, tmpPath)).Output()
		exec.Command("bash", "-c", fmt.Sprintf("sed -i 's/namespace: memcached-operator-40341-system/namespace: %s/g'  `grep -rl \"namespace: memcached-operator-40341-system\" %s`", nsOperator, tmpPath)).Output()

		g.By("step: build and Push the operator image")
		dockerFilePath := filepath.Join(tmpPath, "Dockerfile")
		replaceContent(dockerFilePath, "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml", "RUN ansible-galaxy collection install -r ${HOME}/requirements.yml --force")
		tokenDir := "/tmp/ocp-34426" + getRandomString()
		err = os.MkdirAll(tokenDir, os.ModePerm)
		defer os.RemoveAll(tokenDir)
		if err != nil {
			e2e.Failf("fail to create the token folder:%s", tokenDir)
		}
		_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", fmt.Sprintf("--to=%s", tokenDir), "--confirm").Output()
		if err != nil {
			e2e.Failf("Fail to get the cluster auth %v", err)
		}
		switch architecture {
		case "amd64":
			podmanCLI := container.NewPodmanCLI()
			podmanCLI.ExecCommandPath = tmpPath
			output, err := podmanCLI.Run("build").Args(tmpPath, "--arch", "amd64", "--tag", imageTag, "--authfile", fmt.Sprintf("%s/.dockerconfigjson", tokenDir)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Successfully"))
			output, err = podmanCLI.Run("push").Args(imageTag).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("Storing signatures"))
		case "arm64":
			e2e.Logf("platfrom is arm64, IMG is " + imageTag)
		}

		g.By("step: Install the CRD")
		output, err = makeCLI.Run("install").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("memcached40341s.cache.example.com created"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "memcached40341s.cache.example.com").Output()
		e2e.Logf(output)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("NotFound"))

		g.By("step: Deploy the operator")
		output, err = makeCLI.Run("deploy").Args("IMG=" + imageTag).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("deployment.apps/memcached-operator-40341-controller-manager"))

		waitErr := wait.Poll(30*time.Second, 180*time.Second, func() (bool, error) {
			podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			lines := strings.Split(podList, "\n")
			for _, line := range lines {
				if strings.Contains(line, "memcached-operator-40341-controller-manager") {
					e2e.Logf("found pod memcached-operator-40341-controller-manager")
					if strings.Contains(line, "Running") {
						e2e.Logf("the status of pod memcached-operator-40341-controller-manager is Running")
						return true, nil
					}
					e2e.Logf("the status of pod memcached-operator-40341-controller-manager is not Running")
					return false, nil
				}
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "No memcached-operator-40341-controller-manager")

		g.By("step: Create the resource")
		_, err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", deployfilepath, "-n", nsOperator).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitErr = wait.Poll(5*time.Second, 10*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("memcached40341s.cache.example.com", "-n", nsOperator).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(output, "memcached40341-sample") {
				e2e.Logf("cr memcached40341-sample is created")
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(waitErr, "No cr memcached40341-sample")

		g.By("step: check vars")
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("memcached40341s.cache.example.com/memcached40341-sample", "-n", nsOperator, "-o=jsonpath={.spec.size}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal("3"))
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("memcached40341s.cache.example.com/memcached40341-sample", "-n", nsOperator, "-o=jsonpath={.spec.testKey}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal("testVal"))
		g.By("40341 SUCCESS")
	})
})
