package logging

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-openshift-logging] Logging NonPreRelease", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLI("loki-stack", exutil.KubeConfigPath())
		lo                = "loki-operator-controller-manager"
		clo               = "cluster-logging-operator"
		cloPackageName    = "cluster-logging"
		loPackageName     = "loki-operator"
		subTemplate       = exutil.FixturePath("testdata", "logging", "subscription", "sub-template.yaml")
		SingleNamespaceOG = exutil.FixturePath("testdata", "logging", "subscription", "singlenamespace-og.yaml")
		AllNamespaceOG    = exutil.FixturePath("testdata", "logging", "subscription", "allnamespace-og.yaml")
		jsonLogFile       = exutil.FixturePath("testdata", "logging", "generatelog", "container_json_log_template.json")
	)

	g.Context("Loki Stack testing", func() {
		cloNS := "openshift-logging"
		loNS := "openshift-operators-redhat"
		CLO := SubscriptionObjects{clo, cloNS, SingleNamespaceOG, subTemplate, cloPackageName, CatalogSourceObjects{}}
		LO := SubscriptionObjects{lo, loNS, AllNamespaceOG, subTemplate, loPackageName, CatalogSourceObjects{"candidate", "", ""}}
		g.BeforeEach(func() {
			g.By("deploy CLO and LO")
			CLO.SubscribeOperator(oc)
			LO.SubscribeOperator(oc)
			oc.SetupProject()

		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-ConnectedOnly-Author:qitang-Critical-49168-Deploy lokistack on s3[Serial]", func() {
			if !validateInfraAndResourcesForLoki(oc, []string{"aws"}, "10Gi", "6") {
				g.Skip("Current platform not supported/resources not available for this test!")
			}

			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			// deploy loki
			g.By("deploy loki stack")
			lokiStackTemplate := exutil.FixturePath("testdata", "logging", "lokistack", "lokistack-simple.yaml")
			ls := lokiStack{"my-loki", cloNS, "1x.extra-small", "s3", "s3-secret", "1", sc, "logging-loki-49168-" + getInfrastructureName(oc), lokiStackTemplate}
			defer ls.removeLokiStack(oc)
			err = ls.prepareResourcesForLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = ls.deployLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			ls.waitForLokiStackToBeReady(oc)
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-ConnectedOnly-Author:qitang-Critical-49169-Deploy lokistack on GCS[Serial]", func() {
			if !validateInfraAndResourcesForLoki(oc, []string{"gcp"}, "10Gi", "6") {
				g.Skip("Current platform not supported/resources not available for this test!")
			}

			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			// deploy loki
			g.By("deploy loki stack")
			lokiStackTemplate := exutil.FixturePath("testdata", "logging", "lokistack", "lokistack-simple.yaml")
			ls := lokiStack{"my-loki", cloNS, "1x.extra-small", "gcs", "gcs-secret", "1", sc, "logging-loki-49169-" + getInfrastructureName(oc), lokiStackTemplate}
			defer ls.removeLokiStack(oc)
			err = ls.prepareResourcesForLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = ls.deployLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			ls.waitForLokiStackToBeReady(oc)
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-ConnectedOnly-Author:qitang-Critical-49171-Deploy lokistack on azure[Serial]", func() {
			if !validateInfraAndResourcesForLoki(oc, []string{"azure"}, "10Gi", "6") {
				g.Skip("Current platform not supported/resources not available for this test!")
			}

			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			// deploy loki
			g.By("deploy loki stack")
			lokiStackTemplate := exutil.FixturePath("testdata", "logging", "lokistack", "lokistack-simple.yaml")
			ls := lokiStack{"my-loki", cloNS, "1x.extra-small", "azure", "azure-secret", "1", sc, "logging-loki-49171-" + getInfrastructureName(oc), lokiStackTemplate}
			defer ls.removeLokiStack(oc)
			err = ls.prepareResourcesForLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = ls.deployLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			ls.waitForLokiStackToBeReady(oc)
		})

		// author qitang@redhat.com
		g.It("CPaasrunOnly-ConnectedOnly-Author:qitang-Critical-49364-Forward logs to LokiStack with gateway using fluentd as the collector[Serial]", func() {
			if !validateInfraAndResourcesForLoki(oc, []string{"aws", "gcp", "azure"}, "10Gi", "6") {
				g.Skip("Current platform not supported/resources not available for this test!")
			}

			appProj := oc.Namespace()
			err := oc.WithoutNamespace().Run("new-app").Args("-n", appProj, "-f", jsonLogFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err := getStorageClassName(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("deploy loki stack")
			lokiStackTemplate := exutil.FixturePath("testdata", "logging", "lokistack", "lokistack-simple.yaml")
			ls := lokiStack{"my-loki", cloNS, "1x.extra-small", getStorageType(oc), "storage-secret", "1", sc, "logging-loki-49364-" + getInfrastructureName(oc), lokiStackTemplate}
			defer ls.removeLokiStack(oc)
			err = ls.prepareResourcesForLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = ls.deployLokiStack(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			ls.waitForLokiStackToBeReady(oc)

			g.By("create CLF to forward logs to loki")
			lokiSecret := resource{"secret", "lokistack-gateway", cloNS}
			defer lokiSecret.clear(oc)
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", lokiSecret.namespace, "secret", "generic", lokiSecret.name, "--from-literal=token=/var/run/secrets/kubernetes.io/serviceaccount/token").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			lokiGatewaySVC := ls.name + "-gateway-http." + ls.namespace + ".svc:8080"
			g.By("create clusterlogforwarder/instance")
			clfTemplate := exutil.FixturePath("testdata", "logging", "clusterlogforwarder", "49364.yaml")
			clf := resource{"clusterlogforwarder", "instance", cloNS}
			defer clf.clear(oc)
			err = clf.applyFromTemplate(oc, "-n", clf.namespace, "-f", clfTemplate, "-p", "SECRET="+lokiSecret.name, "-p", "GATEWAY_SVC="+lokiGatewaySVC)
			o.Expect(err).NotTo(o.HaveOccurred())

			// deploy collector pods
			g.By("deploy collector pods")
			instance := exutil.FixturePath("testdata", "logging", "clusterlogging", "collector_only.yaml")
			cl := resource{"clusterlogging", "instance", cloNS}
			defer cl.deleteClusterLogging(oc)
			cl.createClusterLogging(oc, "-n", cl.namespace, "-f", instance, "-p", "NAMESPACE="+cl.namespace, "COLLECTOR=fluentd")
			WaitForDaemonsetPodsToBeReady(oc, cloNS, "collector")
			defer removeLokiStackPermissionFromSA(oc, "lokistack-dev-tenant-logs")
			grantLokiPermissionsToSA(oc, "lokistack-dev-tenant-logs", "logcollector", cloNS)

			//check logs in loki stack
			g.By("check logs in loki")
			bearerToken := getSAToken(oc, "logcollector", cloNS)
			lc := lokiClient{"", "", "http://" + getRouteAddress(oc, ls.namespace, ls.name), "", bearerToken, "", 5, "", true}
			for _, logType := range []string{"application", "infrastructure"} {
				err = wait.Poll(30*time.Second, 180*time.Second, func() (done bool, err error) {
					res, err := lc.queryRange(logType, "{log_type=\""+logType+"\"}", 5, time.Now().Add(time.Duration(-1)*time.Hour), time.Now(), false)
					if err != nil {
						fmt.Printf("\n\n\ngot err when getting %s logs: %v\n\n\n", logType, err)
						return false, err
					}
					if len(res.Data.Result) > 0 {
						return true, nil
					}
					return false, nil
				})
				exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s logs are not founded", logType))
			}

			//sa/logcollector can't view audit logs
			//create a new sa, and check audit logs
			sa := resource{"serviceaccount", "loki-viewer-" + getRandomString(), cloNS}
			defer sa.clear(oc)
			_ = oc.AsAdmin().WithoutNamespace().Run("create").Args("sa", sa.name, "-n", sa.namespace).Execute()
			defer removeLokiStackPermissionFromSA(oc, sa.name)
			grantLokiPermissionsToSA(oc, sa.name, sa.name, sa.namespace)
			token := getSAToken(oc, sa.name, sa.namespace)

			lcAudit := lokiClient{"", "", "http://" + getRouteAddress(oc, ls.namespace, ls.name), "", token, "", 5, "", true}
			err = wait.Poll(30*time.Second, 180*time.Second, func() (done bool, err error) {
				res, err := lcAudit.queryRange("audit", "{log_type=\"audit\"}", 5, time.Now().Add(time.Duration(-1)*time.Hour), time.Now(), false)
				if err != nil {
					fmt.Printf("\n\n\ngot err when getting audit logs: %v\n\n\n", err)
					return false, err
				}
				if len(res.Data.Result) > 0 {
					return true, nil
				}
				return false, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("%s logs are not founded", "audit"))

			appLog, err := lc.queryRange("application", "{kubernetes_namespace_name=\""+appProj+"\"}", 5, time.Now().Add(time.Duration(-1)*time.Hour), time.Now(), false)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(appLog.Data.Result) > 0).Should(o.BeTrue())
		})

	})

})
