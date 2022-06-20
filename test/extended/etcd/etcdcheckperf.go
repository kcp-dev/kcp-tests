package etcd

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd] ETCD", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("openshift-etcd")
	// author: jgeorge@redhat.com
	g.It("Author:jgeorge-High-44199-run etcd benchmark [Exclusive]", func() {
		var platform = exutil.CheckPlatform(oc)
		rtt_th := map[string]string{
			"aws": "0.03",
			"gcp": "0.06",
		}
		wall_fsync_th := map[string]string{
			"aws": "0.04",
			"gcp": "0.06",
		}

		if _, exists := rtt_th[platform]; !exists {
			g.Skip(fmt.Sprintf("Skip for non-supported platform: %s", platform))
		}

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

		cmd := "/root/etcd/tools/benchmark/benchmark put " +
			"--cacert /etc/kubernetes/static-pod-certs/configmaps/etcd-peer-client-ca/ca-bundle.crt " +
			"--cert /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-peer-$(hostname).*crt " +
			"--key /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-peer-$(hostname).*key " +
			"--conns=100 --clients=200 --key-size=32 --sequential-keys --rate=4000 --total=240000 " +
			"--val-size=1024 --target-leader"

		start := time.Now()
		output, err := exutil.RemoteShPodWithBash(oc, "openshift-etcd", etcdPodList[0], cmd)
		duration := time.Since(start)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Benchmark result:\n%s", output))

		// Check benchmark did not take too long
		expected := 90
		o.Expect(duration.Seconds()).Should(o.BeNumerically("<", expected), "Failed to run benchmark in under %d seconds", expected)

		// Check prometheus metrics
		prometheus_url := "https://prometheus-k8s.openshift-monitoring.svc:9091/api/v1/query?query="

		// Get the monitoring token
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", "prometheus-k8s", "-n",
			"openshift-monitoring").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Network RTT metric
		query := fmt.Sprintf("histogram_quantile(0.99,(irate(etcd_network_peer_round_trip_time_seconds_bucket[1m])))>%s", rtt_th[platform])
		data := doPrometheusQuery(oc, token, prometheus_url, query)
		o.Expect(len(data.Data.Result)).To(o.Equal(0))

		// Disk commit duration
		query = "histogram_quantile(0.99, irate(etcd_disk_backend_commit_duration_seconds_bucket[1m]))>0.03"
		data = doPrometheusQuery(oc, token, prometheus_url, query)
		o.Expect(len(data.Data.Result)).To(o.Equal(0))

		// WAL fsync duration
		query = fmt.Sprintf("histogram_quantile(0.999,(irate(etcd_disk_wal_fsync_duration_seconds_bucket[1m])))>%s", wall_fsync_th[platform])
		data = doPrometheusQuery(oc, token, prometheus_url, query)
		o.Expect(len(data.Data.Result)).To(o.Equal(0))
	})
})
