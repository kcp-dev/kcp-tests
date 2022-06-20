package networking

import (
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-networking] SDN", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("networking-sctp", exutil.KubeConfigPath())

	// author: weliang@redhat.com
	g.It("Longduration-Author:weliang-Medium-28757-Establish pod to pod SCTP connections. [Disruptive]", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "networking/sctp")
			sctpClientPod       = filepath.Join(buildPruningBaseDir, "sctpclient.yaml")
			sctpServerPod       = filepath.Join(buildPruningBaseDir, "sctpserver.yaml")
			sctpModule          = filepath.Join(buildPruningBaseDir, "load-sctp-module.yaml")
			sctpServerPodName   = "sctpserver"
			sctpClientPodname   = "sctpclient"
		)

		g.By("install load-sctp-module in all workers")
		installSctpModule(oc, sctpModule)

		g.By("check load-sctp-module in all workers")
		workerNodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			g.Skip("Can not find any woker nodes in the cluster")
		}
		for i := range workerNodeList.Items {
			checkSctpModule(oc, workerNodeList.Items[i].Name)
		}

		g.By("create new namespace")
		oc.SetupProject()

		g.By("create sctpClientPod")
		createResourceFromFile(oc, oc.Namespace(), sctpClientPod)
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpclient")
		exutil.AssertWaitPollNoErr(err1, "sctpClientPod is not running")

		g.By("create sctpServerPod")
		createResourceFromFile(oc, oc.Namespace(), sctpServerPod)
		err2 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpserver")
		exutil.AssertWaitPollNoErr(err2, "sctpServerPod is not running")

		ipStackType := checkIPStackType(oc)

		g.By("test ipv4 in ipv4 cluster or dualstack cluster")
		if ipStackType == "ipv4single" || ipStackType == "dualstack" {
			g.By("get ipv4 address from the sctpServerPod")
			sctpServerPodIP := getPodIPv4(oc, oc.Namespace(), sctpServerPodName)

			g.By("sctpserver pod start to wait for sctp traffic")
			_, _, _, err := oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			o.Expect(err).NotTo(o.HaveOccurred())
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			_, err1 := e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServerPodIP+" 30102 --sctp; }")
			o.Expect(err1).NotTo(o.HaveOccurred())

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err1 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err1).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))
		}

		g.By("test ipv6 in ipv6 cluster or dualstack cluster")
		if ipStackType == "ipv6single" || ipStackType == "dualstack" {
			g.By("get ipv6 address from the sctpServerPod")
			sctpServerPodIP := getPodIPv6(oc, oc.Namespace(), sctpServerPodName, ipStackType)

			g.By("sctpserver pod start to wait for sctp traffic")
			oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServerPodIP+" 30102 --sctp; }")

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err1 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err1).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))
		}
	})

	// author: weliang@redhat.com
	g.It("Longduration-NonPreRelease-Author:weliang-Medium-28758-Expose SCTP ClusterIP Services. [Disruptive]", func() {
		var (
			buildPruningBaseDir  = exutil.FixturePath("testdata", "networking/sctp")
			sctpClientPod        = filepath.Join(buildPruningBaseDir, "sctpclient.yaml")
			sctpServerPod        = filepath.Join(buildPruningBaseDir, "sctpserver.yaml")
			sctpModule           = filepath.Join(buildPruningBaseDir, "load-sctp-module.yaml")
			sctpServerPodName    = "sctpserver"
			sctpClientPodname    = "sctpclient"
			sctpServicev4        = filepath.Join(buildPruningBaseDir, "sctpservicev4.yaml")
			sctpServicev6        = filepath.Join(buildPruningBaseDir, "sctpservicev6.yaml")
			sctpServiceDualstack = filepath.Join(buildPruningBaseDir, "sctpservicedualstack.yaml")
		)

		g.By("install load-sctp-module in all workers")
		installSctpModule(oc, sctpModule)

		g.By("check load-sctp-module in all workers")
		workerNodeList, err := e2enode.GetReadySchedulableNodes(oc.KubeFramework().ClientSet)
		if err != nil {
			g.Skip("Can not find any woker nodes in the cluster")
		}
		for i := range workerNodeList.Items {
			checkSctpModule(oc, workerNodeList.Items[i].Name)
		}

		g.By("create new namespace")
		oc.SetupProject()

		g.By("create sctpClientPod")
		createResourceFromFile(oc, oc.Namespace(), sctpClientPod)
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpclient")
		exutil.AssertWaitPollNoErr(err1, "sctpClientPod is not running")

		g.By("create sctpServerPod")
		createResourceFromFile(oc, oc.Namespace(), sctpServerPod)
		err2 := waitForPodWithLabelReady(oc, oc.Namespace(), "name=sctpserver")
		exutil.AssertWaitPollNoErr(err2, "sctpServerPod is not running")

		ipStackType := checkIPStackType(oc)

		g.By("test ipv4 singlestack cluster")
		if ipStackType == "ipv4single" {
			g.By("create sctpServiceIPv4")
			createResourceFromFile(oc, oc.Namespace(), sctpServicev4)
			output, err := oc.WithoutNamespace().Run("get").Args("service").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("sctpservice-v4"))

			g.By("get service ipv4 address")
			sctpServiceIPv4 := getSvcIPv4(oc, oc.Namespace(), "sctpservice-v4")

			g.By("sctpserver pod start to wait for sctp traffic")
			_, _, _, err1 := oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			o.Expect(err1).NotTo(o.HaveOccurred())
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err2 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err2).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			_, err3 := e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServiceIPv4+" 30102 --sctp; }")
			o.Expect(err3).NotTo(o.HaveOccurred())

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err4 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err4).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))
		}

		g.By("test ipv6 singlestack cluster")
		if ipStackType == "ipv6single" {
			g.By("create sctpServiceIPv4")
			createResourceFromFile(oc, oc.Namespace(), sctpServicev6)
			output, err := oc.WithoutNamespace().Run("get").Args("service").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("sctpservice-v6"))

			g.By("get service ipv6 address")
			sctpServiceIPv6 := getSvcIPv6(oc, oc.Namespace(), "sctpservice-v6")

			g.By("sctpserver pod start to wait for sctp traffic")
			_, _, _, err1 := oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			o.Expect(err1).NotTo(o.HaveOccurred())
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err2 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err2).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			_, err3 := e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServiceIPv6+" 30102 --sctp; }")
			o.Expect(err3).NotTo(o.HaveOccurred())

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err4 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err4).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))
		}

		g.By("test ip dualstack cluster")
		if ipStackType == "dualstack" {
			g.By("create sctpservicedualstack")
			createResourceFromFile(oc, oc.Namespace(), sctpServiceDualstack)
			output, err := oc.WithoutNamespace().Run("get").Args("service").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("sctpservice-dualstack"))

			g.By("get service ipv4 and ipv6 address")
			sctpServiceIPv4, sctpServiceIPv6 := getSvcIPdualstack(oc, oc.Namespace(), "sctpservice-dualstack")

			g.By("test ipv4 in dualstack cluster")
			g.By("sctpserver pod start to wait for sctp traffic")
			_, _, _, err1 := oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			o.Expect(err1).NotTo(o.HaveOccurred())
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err2 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err2).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			_, err3 := e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServiceIPv4+" 30102 --sctp; }")
			o.Expect(err3).NotTo(o.HaveOccurred())

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err4 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err4).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))

			g.By("test ipv6 in dualstack cluster")
			g.By("sctpserver pod start to wait for sctp traffic")
			oc.Run("exec").Args("-n", oc.Namespace(), sctpServerPodName, "--", "/usr/bin/ncat", "-l", "30102", "--sctp").Background()
			time.Sleep(5 * time.Second)

			g.By("check sctp process enabled in the sctp server pod")
			msg, err5 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err5).NotTo(o.HaveOccurred())
			o.Expect(strings.Contains(msg, "/usr/bin/ncat -l 30102 --sctp")).To(o.BeTrue())

			g.By("sctpclient pod start to send sctp traffic")
			e2e.RunHostCmd(oc.Namespace(), sctpClientPodname, "echo 'Test traffic using sctp port from sctpclient to sctpserver' | { ncat -v "+sctpServiceIPv6+" 30102 --sctp; }")

			g.By("server sctp process will end after get sctp traffic from sctp client")
			time.Sleep(5 * time.Second)
			msg1, err6 := e2e.RunHostCmd(oc.Namespace(), sctpServerPodName, "ps aux | grep sctp")
			o.Expect(err6).NotTo(o.HaveOccurred())
			o.Expect(msg1).NotTo(o.ContainSubstring("/usr/bin/ncat -l 30102 --sctp"))
		}
	})
})
