package logging

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease", func() {
	defer g.GinkgoRecover()

	var (
		oc             = exutil.NewCLI("vector-es-namespace", exutil.KubeConfigPath())
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("Vector collector tests", func() {
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

		g.It("CPaasrunOnly-Author:ikanse-Critical-49368-Low-46880-Vector Deploy Cluster Logging with Vector as collector using CLI and exclude Vector logs from collection[Serial]", func() {

			g.By("Create ClusterLogging instance with Vector as collector")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Create project for app logs and deploy the log generator app")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Make sure the Elasticsearch cluster is healthy")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.status}", "green")

			g.By("Check Vector status")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			pl := resource{"pods", podList.Items[0].Name, cloNS}
			pl.checkLogsFromRs(oc, "Healthcheck: Passed", "collector")
			pl.checkLogsFromRs(oc, "Vector has started", "collector")

			g.By("Check app indices in ES pod")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app-000")

			g.By("Check infra indices in ES pod")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "infra-000")

			g.By("Check for Vector logs in Elasticsearch")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			checkLog := "{\"size\": 1, \"sort\": [{\"@timestamp\": {\"order\":\"desc\"}}], \"query\": {\"match\": {\"kubernetes.container_name\": \"collector\"}}}"
			logs := searchDocByQuery(oc, cloNS, podList.Items[0].Name, "*", checkLog)
			o.Expect(logs.Hits.Total).Should(o.Equal(0), "Vector logs should not be collected")
		})

		g.It("CPaasrunOnly-Author:ikanse-Medium-49390-Vector Collecting Kubernetes events using event router[Serial][Slow]", func() {

			eventrouterTemplate := exutil.FixturePath("testdata", "logging", "eventrouter", "eventrouter.yaml")

			g.By("Create ClusterLogging instance with Vector as collector")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "COLLECTOR=vector", "-p", "NAMESPACE="+cl.namespace)
			g.By("Waiting for the Logging pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Create project for app logs and deploy the log generator app")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Make sure the Elasticsearch cluster is healthy")
			cl.assertResourceStatus(oc, "jsonpath={.status.logStore.elasticsearchStatus[0].cluster.status}", "green")

			g.By("Deploy the Event Router")
			evt := resource{"deployment", "eventrouter", cloNS}
			defer deleteEventRouter(oc, cloNS)
			evt.createEventRouter(oc, "-f", eventrouterTemplate)

			g.By("Check event logs in the Event Router pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=eventrouter"})
			o.Expect(err).NotTo(o.HaveOccurred())
			pl := resource{"pods", podList.Items[0].Name, cloNS}
			pl.checkLogsFromRs(oc, "ADDED", "kube-eventrouter")
			pl.checkLogsFromRs(oc, "Update", "kube-eventrouter")

			g.By("Check for Event Router logs in Elasticsearch")
			podList, err = oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			checkLog := "{\"size\": 1, \"sort\": [{\"@timestamp\": {\"order\":\"desc\"}}], \"query\": {\"match\": {\"kubernetes.flat_labels\": \"component=eventrouter\"}}}"
			err = wait.Poll(10*time.Second, 60*time.Second, func() (done bool, err error) {
				logs := searchDocByQuery(oc, cloNS, podList.Items[0].Name, "infra", checkLog)
				if logs.Hits.Total > 0 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("No Event Router logs found when using %s as log collector.", "vector"))
		})

	})

})
