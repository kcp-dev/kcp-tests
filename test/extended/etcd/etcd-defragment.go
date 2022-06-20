package etcd

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd] ETCD", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: mifiedle@redhat.com
	g.It("Author:mifiedle-High-43335-etcd data store will defragment and recover unused space [Slow] [Flaky]", func() {

		g.By("Discover all the etcd pods")
		etcdPodList := getPodListByLabel(oc, "etcd=true")

		g.By("Install and run etcd benchmark")

		_, err := exutil.RemoteShPod(oc, "openshift-etcd", etcdPodList[0], "dnf", "install", "-y", "git", "golang")
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = exutil.RemoteShPodWithBash(oc, "openshift-etcd", etcdPodList[0], "cd /root && git clone --single-branch --branch release-3.5 https://github.com/etcd-io/etcd.git")
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = exutil.RemoteShPodWithBash(oc, "openshift-etcd", etcdPodList[0], "cd /root/etcd/tools/benchmark && go build")
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			_, err := exutil.RemoteShPod(oc, "openshift-etcd", etcdPodList[0], "dnf", "remove", "-y", "git", "golang")
			if err != nil {
				e2e.Logf("Could not remove git and golang packages")
			}
			_, err = exutil.RemoteShPod(oc, "openshift-etcd", etcdPodList[0], "rm", "-rf", "/root/go", "/root/etcd")
			if err != nil {
				e2e.Logf("Could not remove test directories")
			}
		}()

		//push data store over 400MBytes disk size
		cmd := "/root/etcd/tools/benchmark/benchmark put " +
			"--cacert /etc/kubernetes/static-pod-certs/configmaps/etcd-peer-client-ca/ca-bundle.crt " +
			"--cert /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-peer-$(hostname).*crt " +
			"--key /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-peer-$(hostname).*key " +
			"--conns=100 --clients=200 --key-size=32 --sequential-keys --rate=8000 --total=250000 " +
			"--val-size=4096 --target-leader"

		start := time.Now()
		output, err := exutil.RemoteShPodWithBash(oc, "openshift-etcd", etcdPodList[0], cmd)
		duration := time.Since(start)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Benchmark result:\n%s", output))

		// Check benchmark did not take too long
		expected := 120
		o.Expect(duration.Seconds()).Should(o.BeNumerically("<", expected), "Failed to run benchmark in under %d seconds", expected)

		// Check prometheus metrics
		prometheus_url := "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query="

		// Get the monitoring token
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n",
			"openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Allow etcd datastore to reach full size of ~1GB
		g.By("Query etcd datastore size to ensure it grows over 1GB")
		query := "avg(etcd_mvcc_db_total_size_in_use_in_bytes)<1000000000"
		err = wait.Poll(60*time.Second, 120*time.Second, func() (bool, error) {
			data := doPrometheusQuery(oc, token, prometheus_url, query)
			if len(data.Data.Result) == 0 {
				return true, nil
			} else {
				return false, nil
			}
		})
		exutil.AssertWaitPollNoErr(err, "etcd datastore did not grow over 1GB in 2 minutes")

		g.By("Poll for etcd compaction every 60 seconds for up to 20 minutes")
		//Check for datastore to go below 100MB in size
		query = "avg(etcd_mvcc_db_total_size_in_use_in_bytes)>100000000"
		err = wait.Poll(60*time.Second, 1200*time.Second, func() (bool, error) {
			data := doPrometheusQuery(oc, token, prometheus_url, query)
			if len(data.Data.Result) == 0 {
				return true, nil
			} else {
				return false, nil
			}
		})

		exutil.AssertWaitPollNoErr(err, "Compaction did not occur within 20 minutes")

	})
})
