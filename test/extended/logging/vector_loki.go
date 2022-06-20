package logging

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease", func() {
	defer g.GinkgoRecover()

	var (
		oc             = exutil.NewCLI("vector-loki-namespace", exutil.KubeConfigPath())
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("test forward logs to loki log store", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
			loglabeltemplate  = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		)
		cloNS := "openshift-logging"
		eoNS := "openshift-operators-redhat"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		EO := SubscriptionObjects{eo, eoNS, AllNamespaceOG, subTemplate, eoPackageName, CatalogSourceObjects{}}
		g.BeforeEach(func() {
			g.By("deploy CLO and EO")
			CLO.SubscribeOperator(oc)
			EO.SubscribeOperator(oc)
			oc.SetupProject()
		})

		g.It("CPaasrunOnly-Author:ikanse-High-47760-Vector Forward logs to Loki using default value via HTTP[Serial]", func() {

			g.By("Create project for app logs and deploy the log generator app")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create Loki project and deploy Loki Server")
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")

			g.By("Create ClusterLogForwarder instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "47760.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "URL=http://loki-server."+lokiNS+".svc:3100")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create ClusterLogging instance")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Audit Logs in Loki")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			auditLogs := searchLogsInLoki(oc, cloNS, lokiNS, podList.Items[0].Name, "audit")
			o.Expect(auditLogs.Status).Should(o.Equal("success"))
			o.Expect(auditLogs.Data.Result[0].Stream.LogType).Should(o.Equal("audit"))
			o.Expect(auditLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Audit Logs Query is a success")

			g.By("Searching for Infra Logs in Loki")
			infraLogs := searchLogsInLoki(oc, cloNS, lokiNS, podList.Items[0].Name, "infra")
			o.Expect(infraLogs.Status).Should(o.Equal("success"))
			o.Expect(infraLogs.Data.Result[0].Stream.LogType).Should(o.Equal("infrastructure"))
			o.Expect(infraLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Infra Logs Query is a success")

			g.By("Searching for Application Logs in Loki")
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				appLogs := searchAppLogsInLokiByNamespace(oc, cloNS, lokiNS, podList.Items[0].Name, appProj)
				if appLogs.Status == "success" && appLogs.Data.Stats.Summary.BytesProcessedPerSecond != 0 && appLogs.Data.Result[0].Stream.LogType == "application" && appLogs.Data.Result[0].Stream.KubernetesPodName == appPodName.Items[0].Name {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("failed searching for %s logs in Loki", "application"))
			e2e.Logf("Application Logs Query is a success")

		})

		g.It("CPaasrunOnly-Author:ikanse-Medium-48922-Vector Forward logs to Loki using correct loki.tenantKey.kubernetes.namespace_name via HTTP[Serial]", func() {

			g.By("Create project for app logs and deploy the log generator app")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create Loki project and deploy Loki Server")
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			tenantKey := "kubernetes_namespace_name"

			g.By("Create ClusterLogForwarder instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "48922.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "TENANTKEY=kubernetes.namespace_name", "-p", "URL=http://loki-server."+lokiNS+".svc:3100")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create ClusterLogging instance")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Application Logs in Loki using tenantKey")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				appLogs := searchAppLogsInLokiByTenantKey(oc, cloNS, lokiNS, podList.Items[0].Name, tenantKey, appProj)
				if appLogs.Status == "success" && appLogs.Data.Stats.Summary.BytesProcessedPerSecond != 0 && appLogs.Data.Result[0].Stream.LogType == "application" && appLogs.Data.Result[0].Stream.KubernetesPodName == appPodName.Items[0].Name {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed searching for %s logs in Loki", "application"))
			e2e.Logf("Application Logs Query using namespace as tenantKey is a success")

		})

		g.It("CPaasrunOnly-Author:ikanse-Medium-48060-Medium-47801-Vector Forward logs to Loki using loki.labelKeys [Serial]", func() {

			g.By("Create project1 for app logs")
			appProj1 := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj1, "-f", loglabeltemplate, "-p", "LABELS={\"negative\": \"centos-logtest\"}").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create project for app logs and deploy the log generator app")
			oc.SetupProject()
			appProj2 := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProj2, "-f", loglabeltemplate, "-p", "LABELS={\"positive\": \"centos-logtest\"}").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create Loki project and deploy Loki Server")
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			labelKeys := "kubernetes_pod_labels_positive"
			podLabel := "centos-logtest"

			g.By("Create ClusterLogForwarder instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "47801.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "LABELKEY=kubernetes.pod_labels.positive", "-p", "URL=http://loki-server."+lokiNS+".svc:3100")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create ClusterLogging instance")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Application Logs in Loki using LabelKey - Postive match")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				appLogs := searchAppLogsInLokiByLabelKeys(oc, cloNS, lokiNS, podList.Items[0].Name, labelKeys, podLabel)
				if appLogs.Status == "success" && appLogs.Data.Stats.Summary.BytesProcessedPerSecond != 0 && appLogs.Data.Stats.Ingester.TotalLinesSent != 0 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed searching for %s logs in Loki", "application"))
			e2e.Logf("App logs found with matching LabelKey: " + labelKeys + " and pod Label: " + podLabel)

			g.By("Searching for Application Logs in Loki using LabelKey - Negative match")
			labelKeys = "kubernetes_pod_labels_negative"
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				appLogs := searchAppLogsInLokiByLabelKeys(oc, cloNS, lokiNS, podList.Items[0].Name, labelKeys, podLabel)
				if appLogs.Status == "success" && appLogs.Data.Stats.Store.TotalChunksDownloaded == 0 && appLogs.Data.Stats.Summary.BytesProcessedPerSecond == 0 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed searching for %s logs in Loki", "application"))
			e2e.Logf("No App logs found with matching LabelKey: " + labelKeys + " and pod Label: " + podLabel)

		})

		g.It("CPaasrunOnly-Author:ikanse-High-48925-Vector Forward logs to Loki using correct loki.tenantKey.kubernetes.container_name via HTTP[Serial]", func() {

			g.By("Create project for app logs and deploy the log generator app")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create Loki project and deploy Loki Server")
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			tenantKey := "kubernetes_container_name"

			g.By("Create ClusterLogForwarder instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "48922.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "TENANTKEY=kubernetes.container_name", "-p", "URL=http://loki-server."+lokiNS+".svc:3100")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create ClusterLogging instance")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Application Logs in Loki using tenantKey")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			tenantKeyID := "logging-centos-logtest"
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				appLogs := searchAppLogsInLokiByTenantKey(oc, cloNS, lokiNS, podList.Items[0].Name, tenantKey, tenantKeyID)
				if appLogs.Status == "success" && appLogs.Data.Stats.Summary.BytesProcessedPerSecond != 0 && appLogs.Data.Result[0].Stream.LogType == "application" && appLogs.Data.Result[0].Stream.KubernetesPodName == appPodName.Items[0].Name {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Failed searching for %s logs in Loki", "application"))
			e2e.Logf("Application Logs Query using kubernetes.container_name as tenantKey is a success")
		})

	})

})
