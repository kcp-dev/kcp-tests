package monitoring

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"path/filepath"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-monitoring] Cluster_Observability parallel User workload monitoring", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLI("monitor-"+getRandomString(), exutil.KubeConfigPath())
		monitoringCM      monitoringConfig
		monitoringBaseDir string
	)

	g.BeforeEach(func() {
		monitoringBaseDir = exutil.FixturePath("testdata", "monitoring")
		monitoringCMTemplate := filepath.Join(monitoringBaseDir, "cluster-monitoring-cm.yaml")
		//enable user workload monitoring
		monitoringCM = monitoringConfig{
			name:               "cluster-monitoring-config",
			namespace:          "openshift-monitoring",
			enableUserWorkload: "true",
			template:           monitoringCMTemplate,
		}
		monitoringCM.create(oc)
	})

	// author: hongyli@redhat.com
	g.It("Author:hongyli-Critical-43341-Exclude namespaces from user workload monitoring based on label", func() {
		var err error
		var (
			exampleAppRule = filepath.Join(monitoringBaseDir, "example-app-rule.yaml")
		)

		//create project
		oc.SetupProject()
		ns := oc.Namespace()

		g.By("label project not being monitored")
		labelNameSpace(oc, ns, "openshift.io/user-monitoring=false")

		//create example app and alert rule under the project
		g.By("Create example app and alert rule!")
		err = oc.AsAdmin().Run("apply").Args("-n", ns, "-f", exampleAppRule).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check metrics")
		err = checkMetric(oc, "https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query?query=version", "\"result\":[]", 60)
		exutil.AssertWaitPollNoErr(err, "metrics does not contain \"result\":[]")

		g.By("check alerts")
		err = checkMetric(oc, "https://thanos-ruler.openshift-user-workload-monitoring.svc:9091/api/v1/alerts", "null", 60)

		g.By("label project being monitored")
		labelNameSpace(oc, ns, "openshift.io/user-monitoring=true")

		g.By("check metrics")
		err = checkMetric(oc, "https://thanos-querier.openshift-monitoring.svc:9091/api/v1/query?query=version", "prometheus-example-app", 120)

		g.By("check alerts")
		err = checkMetric(oc, "https://thanos-ruler.openshift-user-workload-monitoring.svc:9091/api/v1/alerts", "TestAlert", 60)
	})

	// author: hongyli@redhat.com
	g.It("Author:hongyli-Critical-44032-Restore cluster monitoring stack default configuration [Serial]", func() {
		var err error
		g.By("Delete config map cluster-monitoring-config")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ConfigMap", monitoringCM.name, "-n", monitoringCM.namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
