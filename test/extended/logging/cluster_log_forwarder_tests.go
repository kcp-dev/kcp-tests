package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease", func() {
	var oc = exutil.NewCLI("logfwd-namespace", exutil.KubeConfigPath())
	defer g.GinkgoRecover()

	var (
		eo             = "elasticsearch-operator"
		clo            = "cluster-logging-operator"
		cloPackageName = "cluster-logging"
		eoPackageName  = "elasticsearch-operator"
	)

	g.Context("Log Forward with namespace selector in the CLF", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
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

		g.It("CPaasrunOnly-Author:kbharti-High-41598-forward logs only from specific pods via a label selector inside the Log Forwarding API[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)
			// Dev label - create a project and pod in the project to generate some logs
			g.By("create application for logs with dev label")
			appProjDev := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProjDev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// QA label - create a project and pod in the project to generate some logs
			g.By("create application for logs with qa label")
			oc.SetupProject()
			appProjQa := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProjQa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogForwarder instance
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41598.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate)
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			//check app index in ES
			g.By("check indices in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app-000")

			//Waiting for the app index to be populated
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, appProjQa, "app-000")

			// check data in ES for QA namespace
			g.By("check logs in ES pod for QA namespace in CLF")
			logs := searchDocByQuery(oc, cloNS, podList.Items[0].Name, "app", "{\"size\": 1, \"sort\": [{\"@timestamp\": {\"order\":\"desc\"}}], \"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \""+appProjQa+"\"}}}")
			o.Expect(logs.Hits.DataHits[0].Source.Kubernetes.NamespaceLabels.KubernetesIOMetadataName).Should(o.Equal(appProjQa))

			//check that no data exists for the other Dev namespace - Negative test
			g.By("check logs in ES pod for Dev namespace in CLF")
			count, _ := getDocCountByQuery(oc, cloNS, podList.Items[0].Name, "app-0000", "{\"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \""+appProjDev+"\"}}}")
			o.Expect(count).Should(o.Equal(0))

		})

		g.It("CPaasrunOnly-Author:kbharti-High-41599-Forward Logs from specified pods combining namespaces and label selectors[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)

			g.By("create application for logs with dev1 label")
			appProjDev := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProjDev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev-1", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-dev1", "-p", "CONFIGMAP=logtest-config-dev1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with dev2 label")
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProjDev, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-dev-2", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-dev2", "-p", "CONFIGMAP=logtest-config-dev2").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with qa1 label")
			oc.SetupProject()
			appProjQa := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProjQa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa-1", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-qa1", "-p", "CONFIGMAP=logtest-config-qa1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application for logs with qa2 label")
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProjQa, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest-qa-2", "-p", "REPLICATIONCONTROLLER=logging-centos-logtest-qa2", "-p", "CONFIGMAP=logtest-config-qa2").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogForwarder instance
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41599.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "APP_NAMESPACE_QA="+appProjQa, "-p", "APP_NAMESPACE_DEV="+appProjDev)
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			//check app index in ES
			g.By("check indices in ES pod")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app-00")

			//Waiting for the app index to be populated
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, appProjQa, "app-00")
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, appProjDev, "app-00")

			g.By("check doc count in ES pod for QA1 namespace in CLF")
			logCount, _ := getDocCountByQuery(oc, cloNS, podList.Items[0].Name, "app-00", "{\"query\": {\"terms\": {\"kubernetes.flat_labels\": [\"run=centos-logtest-qa-1\"]}}}")
			o.Expect(logCount).ShouldNot(o.Equal(0))

			g.By("check doc count in ES pod for QA2 namespace in CLF")
			logCount, _ = getDocCountByQuery(oc, cloNS, podList.Items[0].Name, "app-00", "{\"query\": {\"terms\": {\"kubernetes.flat_labels\": [\"run=centos-logtest-qa-2\"]}}}")
			o.Expect(logCount).Should(o.Equal(0))

			g.By("check doc count in ES pod for DEV1 namespace in CLF")
			logCount, _ = getDocCountByQuery(oc, cloNS, podList.Items[0].Name, "app-00", "{\"query\": {\"terms\": {\"kubernetes.flat_labels\": [\"run=centos-logtest-dev-1\"]}}}")
			o.Expect(logCount).ShouldNot(o.Equal(0))

			g.By("check doc count in ES pod for DEV2 namespace in CLF")
			logCount, _ = getDocCountByQuery(oc, cloNS, podList.Items[0].Name, "app-00", "{\"query\": {\"terms\": {\"kubernetes.flat_labels\": [\"run=centos-logtest-dev-2\"]}}}")
			o.Expect(logCount).Should(o.Equal(0))

		})
	})

	g.Context("test forward logs to external log stores", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
			jsonLogFile       = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
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

		g.It("CPaasrunOnly-Author:ikanse-High-42981-Collect OVN audit logs [Serial]", func() {

			g.By("Check the network type for the test")
			networkType := checkNetworkType(oc)
			if !strings.Contains(networkType, "ovnkubernetes") {
				g.Skip("Skip for non-supported network type, type is not OVNKubernetes!!!")
			}

			g.By("Create clusterlogforwarder instance to forward OVN audit logs to default Elasticsearch instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "42981.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err := clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Check audit index in ES pod")
			esPods, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, esPods.Items[0].Name, "audit-00")

			g.By("Create a test project, enable OVN network log collection on it, add the OVN log app and network policies for the project")
			oc.SetupProject()
			ovnProj := oc.Namespace()
			ovn := resource{"deployment", "ovn-app", ovnProj}
			esTemplate := exutil.FixturePath("testdata", "logging", "generatelog", "42981.yaml")
			err = ovn.applyFromTemplate(oc, "-n", ovn.namespace, "-f", esTemplate, "-p", "NAMESPACE="+ovn.namespace)
			o.Expect(err).NotTo(o.HaveOccurred())
			WaitForDeploymentPodsToBeReady(oc, ovnProj, ovn.name)

			g.By("Access the OVN app pod from another pod in the same project to generate OVN ACL messages")
			ovnPods, err := oc.AdminKubeClient().CoreV1().Pods(ovnProj).List(metav1.ListOptions{LabelSelector: "app=ovn-app"})
			o.Expect(err).NotTo(o.HaveOccurred())
			podIP := ovnPods.Items[0].Status.PodIP
			e2e.Logf("Pod IP is %s ", podIP)
			ovnCurl := "curl " + podIP + ":8080"
			_, err = e2e.RunHostCmdWithRetries(ovnProj, ovnPods.Items[1].Name, ovnCurl, 3*time.Second, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check for the generated OVN audit logs on the OpenShift cluster nodes")
			nodeLogs, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("-n", ovnProj, "node-logs", "-l", "beta.kubernetes.io/os=linux", "--path=/ovn/acl-audit-log.log").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nodeLogs).Should(o.ContainSubstring(ovnProj), "The OVN logs doesn't contain logs from project %s", ovnProj)

			g.By("Check for the generated OVN audit logs in Elasticsearch")
			err = wait.Poll(10*time.Second, 300*time.Second, func() (done bool, err error) {
				cmd := "es_util --query=audit*/_search?format=JSON -d '{\"query\":{\"query_string\":{\"query\":\"verdict=allow AND severity=alert AND tcp,vlan_tci AND tcp_flags=ack\",\"default_field\":\"message\"}}}'"
				stdout, err := e2e.RunHostCmdWithRetries(cloNS, esPods.Items[0].Name, cmd, 3*time.Second, 30*time.Second)
				if err != nil {
					return false, err
				}
				res := SearchResult{}
				json.Unmarshal([]byte(stdout), &res)
				if res.Hits.Total > 0 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprint("The ovn audit logs are not collected", ""))
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-Medium-41134-Forward Log under different namespaces to different external Elasticsearch[Serial][Slow]", func() {
			appProj1 := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj1, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			oc.SetupProject()
			appProj2 := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProj2, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			oc.SetupProject()
			appProj3 := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProj3, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy 2 external ES servers")
			oc.SetupProject()
			esProj1 := oc.Namespace()
			ees1 := externalES{esProj1, "6.8", "elasticsearch-server-1", false, false, false, "", "", "", cloNS}
			defer ees1.remove(oc)
			ees1.deploy(oc)

			oc.SetupProject()
			esProj2 := oc.Namespace()
			ees2 := externalES{esProj2, "7.16", "elasticsearch-server-2", false, false, false, "", "", "", cloNS}
			defer ees2.remove(oc)
			ees2.deploy(oc)

			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41134.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			qa := []string{appProj1, appProj2}
			qaProjects, _ := json.Marshal(qa)
			dev := []string{appProj1, appProj3}
			devProjects, _ := json.Marshal(dev)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "QA_NS="+string(qaProjects), "-p", "DEV_NS="+string(devProjects), "-p", "URL_QA=http://"+ees1.serverName+"."+esProj1+".svc:9200", "-p", "URL_DEV=http://"+ees2.serverName+"."+esProj2+".svc:9200")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in external ES")
			ees1.waitForIndexAppear(oc, "app")
			for _, proj := range qa {
				ees1.waitForProjectLogsAppear(oc, proj, "app")
			}
			count1, _ := ees1.getDocCount(oc, "app", "{\"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \""+appProj3+"\"}}}")
			o.Expect(count1 == 0).Should(o.BeTrue())

			ees2.waitForIndexAppear(oc, "app")
			for _, proj := range dev {
				ees2.waitForProjectLogsAppear(oc, proj, "app")
			}
			count2, _ := ees2.getDocCount(oc, "app", "{\"query\": {\"match_phrase\": {\"kubernetes.namespace_name\": \""+appProj2+"\"}}}")
			o.Expect(count2 == 0).Should(o.BeTrue())

		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-High-41240-BZ1905615 The application logs can be sent to the default ES when part of projects logs are sent to external aggregator[Serial][Slow]", func() {
			appProj1 := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj1, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			oc.SetupProject()
			appProj2 := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProj2, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy rsyslog server")
			oc.SetupProject()
			syslogProj := oc.Namespace()
			rsyslog := rsyslog{"rsyslog", syslogProj, false, "rsyslog", cloNS}
			defer rsyslog.remove(oc)
			rsyslog.deploy(oc)

			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41240.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "PROJ_NS="+appProj1, "-p", "URL=udp://"+rsyslog.serverName+"."+rsyslog.namespace+".svc:514")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("check logs in rsyslog server")
			rsyslog.checkData(oc, true, "app-container.log")

			g.By("check logs in internal ES")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "es-node-master=true"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "app")
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "infra")
			waitForIndexAppear(oc, cloNS, podList.Items[0].Name, "audit")

			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, appProj1, "app")
			waitForProjectLogsAppear(oc, cloNS, podList.Items[0].Name, appProj2, "app")
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-High-45419-ClusterLogForwarder Forward logs to remote syslog with tls[Serial][Slow]", func() {
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy rsyslog server")
			oc.SetupProject()
			syslogProj := oc.Namespace()
			rsyslog := rsyslog{"rsyslog", syslogProj, true, "rsyslog", cloNS}
			defer rsyslog.remove(oc)
			rsyslog.deploy(oc)

			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "45419.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "URL=tls://"+rsyslog.serverName+"."+rsyslog.namespace+".svc:6514", "-p", "OUTPUT_SECRET="+rsyslog.secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in rsyslog server")
			rsyslog.checkData(oc, true, "app-container.log")
			rsyslog.checkData(oc, true, "infra-container.log")
			rsyslog.checkData(oc, true, "audit.log")
			rsyslog.checkData(oc, true, "infra.log")
		})

		//Author: kbharti@redhat.com
		g.It("CPaasrunOnly-Author:kbharti-High-43745-Forward to Loki using default value via http[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)
			//create a project and app to generate some logs
			g.By("create project for app logs")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create Loki project and deploy Loki Server
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")

			//Create ClusterLogForwarder
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43745.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "LOKINAMESPACE="+lokiNS)
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
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
			infraLogs := searchLogsInLoki(oc, cloNS, lokiNS, podList.Items[0].Name, "infrastructure")
			o.Expect(infraLogs.Status).Should(o.Equal("success"))
			o.Expect(infraLogs.Data.Result[0].Stream.LogType).Should(o.Equal("infrastructure"))
			o.Expect(infraLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Infra Logs Query is a success")

			g.By("Searching for Application Logs in Loki")
			appLogs := searchAppLogsInLokiByNamespace(oc, cloNS, lokiNS, podList.Items[0].Name, appProj)
			o.Expect(appLogs.Status).Should(o.Equal("success"))
			o.Expect(appLogs.Data.Result[0].Stream.LogType).Should(o.Equal("application"))
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appLogs.Data.Result[0].Stream.KubernetesPodName).Should(o.Equal(appPodName.Items[0].Name))
			o.Expect(appLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Application Logs Query is a success")

		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-High-43250-Forward logs to fluentd enable mTLS with shared_key and tls_client_private_key_passphrase[Serial]", func() {
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy fluentd server")
			oc.SetupProject()
			fluentdProj := oc.Namespace()
			fluentd := fluentdServer{"fluentdtest", fluentdProj, true, true, "testOCP43250", "", "fluentd-43250", cloNS}
			defer fluentd.remove(oc)
			fluentd.deploy(oc)

			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43250.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "URL=tls://"+fluentd.serverName+"."+fluentd.namespace+".svc:24224", "-p", "OUTPUT_SECRET="+fluentd.secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in fluentd server")
			fluentd.checkData(oc, true, "app.log")
			fluentd.checkData(oc, true, "infra-container.log")
			fluentd.checkData(oc, true, "audit.log")
			fluentd.checkData(oc, true, "infra.log")
		})

		//Author: kbharti@redhat.com
		g.It("CPaasrunOnly-Author:kbharti-High-43746- Forward to Loki using loki.tenantkey[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)
			//create a project and app to generate some logs
			g.By("create project for app logs")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create Loki project and deploy Loki Server
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			tenantKey := "kubernetes_pod_name"

			//Create ClusterLogForwarder
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43746.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "LOKINAMESPACE="+lokiNS, "-p", "TENANTKEY=kubernetes.pod_name")
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Application Logs in Loki using tenantKey")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appLogs := searchAppLogsInLokiByTenantKey(oc, cloNS, lokiNS, podList.Items[0].Name, tenantKey, appPodName.Items[0].Name)
			o.Expect(appLogs.Status).Should(o.Equal("success"))
			o.Expect(appLogs.Data.Result[0].Stream.LogType).Should(o.Equal("application"))
			o.Expect(appLogs.Data.Result[0].Stream.KubernetesPodName).Should(o.Equal(appPodName.Items[0].Name))
			o.Expect(appLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Application Logs Query using tenantKey is a success")

		})

		g.It("CPaasrunOnly-Author:kbharti-High-43771- Forward to Loki using correct loki.tenantKey.kubernetes.namespace_name via http[Serial]", func() {

			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_non_json_log_template.json")
			)
			//create a project and app to generate some logs
			g.By("create project for app logs")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", loglabeltemplate, "-p", "LABELS=centos-logtest").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create Loki project and deploy Loki Server
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			tenantKey := "kubernetes_namespace_name"

			//Create ClusterLogForwarder
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43746.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "LOKINAMESPACE="+lokiNS, "-p", "TENANTKEY=kubernetes.namespace_name")
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			g.By("Searching for Application Logs in Loki using tenantKey")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appPodName, err := oc.AdminKubeClient().CoreV1().Pods(appProj).List(metav1.ListOptions{LabelSelector: "run=centos-logtest"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appLogs := searchAppLogsInLokiByTenantKey(oc, cloNS, lokiNS, podList.Items[0].Name, tenantKey, appProj)
			o.Expect(appLogs.Status).Should(o.Equal("success"))
			o.Expect(appLogs.Data.Result[0].Stream.LogType).Should(o.Equal("application"))
			o.Expect(appLogs.Data.Result[0].Stream.KubernetesPodName).Should(o.Equal(appPodName.Items[0].Name))
			o.Expect(appLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			e2e.Logf("Application Logs Query using namespace as tenantKey is a success")

		})
		g.It("CPaasrunOnly-Author:kbharti-Low-43770-Forward to Loki using loki.labelKeys which does not exist[Serial]", func() {

			//This case covers OCP-45697 and OCP-43770
			var (
				loglabeltemplate = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
			)
			//create a project and app to generate some logs
			g.By("create project1 for app logs")
			appProj1 := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj1, "-f", loglabeltemplate, "-p", "LABELS={\"negative\": \"centos-logtest\"}").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create project2 for app logs")
			oc.SetupProject()
			appProj2 := oc.Namespace()
			err = oc.WithoutNamespace().Run("new-app").Args("-n", appProj2, "-f", loglabeltemplate, "-p", "LABELS={\"positive\": \"centos-logtest\"}").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create Loki project and deploy Loki Server
			lokiNS := deployExternalLokiServer(oc, "loki-config", "loki-server")
			labelKeys := "kubernetes_labels_positive"
			podLabel := "centos-logtest"

			//Create ClusterLogForwarder
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43770.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "LOKINAMESPACE="+lokiNS, "-p", "LABELKEY=kubernetes.labels.positive")
			o.Expect(err).NotTo(o.HaveOccurred())

			//Create ClusterLogging instance
			g.By("deploy EFK pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "cl-template.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			g.By("waiting for the EFK pods to be ready...")
			WaitForECKPodsToBeReady(oc, cloNS)

			//Positive Scenario - Matching labelKeys
			g.By("Searching for Application Logs in Loki using LabelKey - Postive match")
			podList, err := oc.AdminKubeClient().CoreV1().Pods(cloNS).List(metav1.ListOptions{LabelSelector: "component=collector"})
			o.Expect(err).NotTo(o.HaveOccurred())
			appLogs := searchAppLogsInLokiByLabelKeys(oc, cloNS, lokiNS, podList.Items[0].Name, labelKeys, podLabel)
			o.Expect(appLogs.Status).Should(o.Equal("success"))
			o.Expect(appLogs.Data.Result).ShouldNot(o.BeEmpty())
			o.Expect(appLogs.Data.Stats.Summary.BytesProcessedPerSecond).ShouldNot(o.BeZero())
			o.Expect(appLogs.Data.Stats.Ingester.TotalLinesSent).ShouldNot((o.BeZero()))
			e2e.Logf("App logs found with matching LabelKey: " + labelKeys + " and pod Label: " + podLabel)

			// Negative Scenario - No labelKeys are matching
			g.By("Searching for Application Logs in Loki using LabelKey - Negative match")
			labelKeys = "kubernetes_labels_negative"
			appLogs = searchAppLogsInLokiByLabelKeys(oc, cloNS, lokiNS, podList.Items[0].Name, labelKeys, podLabel)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appLogs.Status).Should(o.Equal("success"))
			o.Expect(appLogs.Data.Result).Should(o.BeEmpty())
			o.Expect(appLogs.Data.Stats.Summary.BytesProcessedPerSecond).Should(o.BeZero())
			o.Expect(appLogs.Data.Stats.Store.TotalChunksDownloaded).Should((o.BeZero()))
			e2e.Logf("No App logs found with matching LabelKey: " + labelKeys + " and pod Label: " + podLabel)

		})

	})

	g.Context("Log Forward to Cloudwatch", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			jsonLogFile       = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		)
		cloNS := "openshift-logging"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		var cw cloudwatchSpec

		g.BeforeEach(func() {
			g.By("deploy CLO")
			CLO.SubscribeOperator(oc)
			oc.SetupProject()
			g.By("init Cloudwatch test spec")
			cw = cw.init(oc)
		})
		g.AfterEach(func() {
			cw.deleteGroups()
		})

		g.It("CPaasrunOnly-Author:anli-Critical-43443-Fluentd Forward logs to Cloudwatch by logtype [Serial][Slow]", func() {
			platform := exutil.CheckPlatform(oc)
			if platform != "aws" {
				g.Skip("Skip for non-supported platform, the support platform is AWS!!!")
			}
			cw.awsKeyID, cw.awsKey = getAWSKey(oc)

			g.By("create log producer")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create clusterlogforwarder/instance")
			s := resource{"secret", cw.secretName, cw.secretNamespace}
			defer s.clear(oc)
			cw.createClfSecret(oc)

			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "43443.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "SECRETNAME="+cw.secretName, "-p", "REGION="+cw.awsRegion)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in Cloudwatch")
			o.Expect(cw.logsFound()).To(o.BeTrue())
		})

		g.It("CPaasrunOnly-Author:anli-High-43839-Fluentd logs to Cloudwatch group by namespaceName and groupPrefix [Serial][Slow]", func() {
			platform := exutil.CheckPlatform(oc)
			if platform != "aws" {
				g.Skip("Skip for non-supported platform, the support platform is AWS!!!")
			}
			cw.awsKeyID, cw.awsKey = getAWSKey(oc)
			cw.groupPrefix = "qeauto" + getInfrastructureName(oc)
			cw.groupType = "namespaceName"
			// Disable audit, so the test be more stable
			cw.logTypes = []string{"infrastructure", "application"}

			g.By("create log producer")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create clusterlogforwarder/instance")
			s := resource{"secret", cw.secretName, cw.secretNamespace}
			defer s.clear(oc)
			cw.createClfSecret(oc)

			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "clf-cloudwatch.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "SECRETNAME="+cw.secretName, "-p", "REGION="+cw.awsRegion, "-p", "PREFIX="+cw.groupPrefix, "-p", "GROUPTYPE="+cw.groupType)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in Cloudwatch")
			o.Expect(cw.logsFound()).To(o.BeTrue())
		})

		g.It("CPaasrunOnly-Author:anli-High-43840-Forward logs to Cloudwatch group by namespaceUUID and groupPrefix [Serial][Slow]", func() {
			platform := exutil.CheckPlatform(oc)
			if platform != "aws" {
				g.Skip("Skip for non-supported platform, the support platform is AWS!!!")
			}
			cw.awsKeyID, cw.awsKey = getAWSKey(oc)
			cw.groupPrefix = "qeauto" + getInfrastructureName(oc)
			cw.groupType = "namespaceUUID"
			// Disable audit, so the test be more stable
			cw.logTypes = []string{"infrastructure", "application"}

			g.By("create log producer")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create clusterlogforwarder/instance")
			s := resource{"secret", cw.secretName, cw.secretNamespace}
			defer s.clear(oc)
			cw.createClfSecret(oc)

			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "clf-cloudwatch.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "SECRETNAME="+cw.secretName, "-p", "REGION="+cw.awsRegion, "-p", "PREFIX="+cw.groupPrefix, "-p", "GROUPTYPE="+cw.groupType)
			defer clf.clear(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("check logs in Cloudwatch")
			o.Expect(cw.logsFound()).To(o.BeTrue())
		})
	})

	g.Context("Log Forward to Kafka", func() {
		var (
			subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
			SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
			jsonLogFile       = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
		)
		cloNS := "openshift-logging"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		g.BeforeEach(func() {
			g.By("deploy CLO")
			CLO.SubscribeOperator(oc)
			oc.SetupProject()
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-Author:qitang-Medium-41726-Forward logs to different kafka brokers[Serial][Slow]", func() {
			g.By("create log producer")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("subscribe AMQ kafka into 2 different namespaces")
			// to avoid collecting kafka logs, deploy kafka in project openshift-*
			amqNs1 := "openshift-amq-1"
			amqNs2 := "openshift-amq-2"
			catsrc := CatalogSourceObjects{"stable", "redhat-operators", "openshift-marketplace"}
			amq1 := SubscriptionObjects{"amq-streams-cluster-operator", amqNs1, SingleNamespaceOG, subTemplate, "amq-streams", catsrc}
			amq2 := SubscriptionObjects{"amq-streams-cluster-operator", amqNs2, SingleNamespaceOG, subTemplate, "amq-streams", catsrc}
			topicName := "topic-logging-app"
			kafkaClusterName := "kafka-cluster"
			for _, amq := range []SubscriptionObjects{amq1, amq2} {
				defer deleteNamespace(oc, amq.Namespace)
				//defer amq.uninstallOperator(oc)
				amq.SubscribeOperator(oc)
				// before creating kafka, check the existence of crd kafkas.kafka.strimzi.io
				checkResource(oc, true, true, "kafka.strimzi.io", []string{"crd", "kafkas.kafka.strimzi.io", "-ojsonpath={.spec.group}"})
				kafka := resource{"kafka", kafkaClusterName, amq.Namespace}
				kafkaTemplate := exutil.FixturePath("testdata", "logging", "external-log-stores", "kafka", "amqstreams", "kafka-cluster-no-auth.yaml")
				//defer kafka.clear(oc)
				kafka.applyFromTemplate(oc, "-n", kafka.namespace, "-f", kafkaTemplate, "-p", "NAME="+kafka.name, "NAMESPACE="+kafka.namespace, "VERSION=3.0.0", "MESSAGE_VERSION=3.0")
				o.Expect(err).NotTo(o.HaveOccurred())
				// create topics
				topicTemplate := exutil.FixturePath("testdata", "logging", "external-log-stores", "kafka", "amqstreams", "kafka-topic.yaml")
				topic := resource{"Kafkatopic", topicName, amq.Namespace}
				//defer topic.clear(oc)
				err = topic.applyFromTemplate(oc, "-n", topic.namespace, "-f", topicTemplate, "-p", "NAME="+topic.name, "CLUSTER_NAME="+kafka.name, "NAMESPACE="+topic.namespace)
				o.Expect(err).NotTo(o.HaveOccurred())
				// wait for kafka cluster to be ready
				waitForPodReadyWithLabel(oc, kafka.namespace, "app.kubernetes.io/instance="+kafka.name)
			}
			g.By("forward logs to Kafkas")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "41726.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			brokers, _ := json.Marshal([]string{"tls://" + kafkaClusterName + "-kafka-bootstrap." + amqNs1 + ".svc:9092", "tls://" + kafkaClusterName + "-kafka-bootstrap." + amqNs2 + ".svc:9092"})
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "TOPIC="+topicName, "BROKERS="+string(brokers))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			//create consumer pod
			for _, ns := range []string{amqNs1, amqNs2} {
				consumerTemplate := exutil.FixturePath("testdata", "logging", "external-log-stores", "kafka", "amqstreams", "topic-consumer.yaml")
				consumer := resource{"job", topicName + "-consumer", ns}
				//defer consumer.clear(oc)
				err = consumer.applyFromTemplate(oc, "-n", consumer.namespace, "-f", consumerTemplate, "-p", "NAME="+consumer.name, "NAMESPACE="+consumer.namespace, "KAFKA_TOPIC="+topicName, "CLUSTER_NAME="+kafkaClusterName)
				o.Expect(err).NotTo(o.HaveOccurred())
				waitForPodReadyWithLabel(oc, consumer.namespace, "job-name="+consumer.name)
			}

			g.By("check data in kafka")
			for _, consumer := range []resource{{"job", topicName + "-consumer", amqNs1}, {"job", topicName + "-consumer", amqNs2}} {
				err = wait.Poll(10*time.Second, 180*time.Second, func() (done bool, err error) {
					logs, err := getDataFromKafkaConsumerPod(oc, consumer.namespace, consumer.name)
					if err != nil {
						return false, err
					}
					if strings.Contains(logs, appProj) {
						return true, nil
					}
					return false, nil
				})
				exutil.AssertWaitPollNoErr(err, fmt.Sprintf("App logs are not found in %s/%s", consumer.namespace, consumer.name))
			}
		})
		// author gkarager@redhat.com
		g.It("CPaasrunOnly-Author:gkarager-Medium-45368-Forward logs to kafka using sals_plaintext[Serial][Slow]", func() {
			g.By("create log producer")
			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Deploy kafka")
			kafka := kafka{cloNS, "kafka", "zookeeper", "sasl_plaintext"}
			defer kafka.removeZookeeper(oc)
			kafka.deployZookeeper(oc)
			defer kafka.removeKafka(oc)
			kafka.deployKafka(oc)

			g.By("Create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "45368.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", cloNS, "-f", clfTemplate)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace)
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")

			g.By("Check app logs in kafka consumer pod")
			consumerPodPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", cloNS, "-l", "component=kafka-consumer", "-o", "name").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(10*time.Second, 180*time.Second, func() (done bool, err error) {
				consumerPodLogs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(consumerPodPodName, "-n", cloNS).Output()
				if err != nil {
					return false, err
				}
				if strings.Contains(consumerPodLogs, appProj) {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("App logs are not found in %s/%s", cloNS, consumerPodPodName))
		})
	})
})
