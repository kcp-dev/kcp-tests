package router

import (
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-network-edge] Network_Edge should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("router-ipfailover", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		g.By("Check platforms")
		platformtype, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		platforms := map[string]bool{
			// 'None' for Baremetal
			"None":      true,
			"VSphere":   true,
			"OpenStack": true,
		}
		if !platforms[platformtype] {
			g.Skip("Skip for non-supported platform")
		}
	})

	// author: hongli@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:hongli-ConnectedOnly-Critical-41025-support to deploy ipfailover [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41025",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
	})

	// author: mjoseph@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:mjoseph-ConnectedOnly-Medium-41028-ipfailover configuration can be customized by ENV [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41028",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")

		g.By("set the HA virtual IP for the failover group")
		podName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		ipv4Address := getPodv4Address(oc, oc.Namespace(), podName[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)

		g.By("set other ipfailover env varibales")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_CONFIG_NAME=IPFailover")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VIP_GROUPS=4")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_NETWORK_INTERFACE=ens1")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_MONITOR_PORT=30061")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VRRP_ID_OFFSET=2")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_REPLICA_COUNT=3")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_USE_UNICAST=true`)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_IPTABLES_CHAIN=OUTPUT`)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_NOTIFY_SCRIPT=/etc/keepalive/mynotifyscript.sh`)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_CHECK_SCRIPT=/etc/keepalive/mycheckscript.sh`)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_PREEMPTION=preempt_delay 600`)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_CHECK_INTERVAL=3")

		g.By("verify the HA virtual ip ENV variable")
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err1, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		newPodName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		checkenv := readPodEnv(oc, newPodName[0], oc.Namespace(), "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))

		g.By("check the ipfailover configurations and verify the other ENV variables")
		result := describePod(oc, oc.Namespace(), newPodName[0])
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VIP_GROUPS:         4"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_CONFIG_NAME:        IPFailover"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_NETWORK_INTERFACE:  ens1"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_MONITOR_PORT:       30061"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VRRP_ID_OFFSET:     2"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_REPLICA_COUNT:      3"))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_USE_UNICAST:        true`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_IPTABLES_CHAIN:     OUTPUT`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_NOTIFY_SCRIPT:      /etc/keepalive/mynotifyscript.sh`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_CHECK_SCRIPT:       /etc/keepalive/mycheckscript.sh`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_PREEMPTION:         preempt_delay 600`))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_CHECK_INTERVAL:     3"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS:        " + virtualIP))
	})

	// author: mjoseph@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:mjoseph-ConnectedOnly-Medium-41029-ipfailover can support up to a maximum of 255 VIPs for the entire cluster [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41029",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")

		g.By("add some VIP configuration for the failover group")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VRRP_ID_OFFSET=0")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VIP_GROUPS=238")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_VIRTUAL_IPS=192.168.254.1-255`)

		g.By("verify from the ipfailover pod, the 255 VIPs are added")
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err1, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		newPodName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		checkenv := readPodEnv(oc, newPodName[0], oc.Namespace(), "OPENSHIFT_HA_VIP_GROUPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIP_GROUPS=238"))
	})

	// author: mjoseph@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:mjoseph-ConnectedOnly-Medium-41027-pod and service automatically switched over to standby when master fails [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41027",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)
		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")

		g.By("set the HA virtual IP for the failover group")
		podNames := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		ipv4Address := getPodv4Address(oc, oc.Namespace(), podNames[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)

		g.By("verify the HA virtual ip ENV variable")
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err1, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		newPodName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		checkenv := readPodEnv(oc, newPodName[0], oc.Namespace(), "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))

		g.By("find the primary and the secondary pod")
		primaryPod := getVipOwnerPod(oc, oc.Namespace(), newPodName, virtualIP)
		secondaryPod := slicingElement(primaryPod, newPodName)
		g.By("restarting the ipfailover primary pod")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", oc.Namespace(), "pod", primaryPod).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verify whether the other pod becomes master and it has the VIP")
		_ = getVipOwnerPod(oc, oc.Namespace(), secondaryPod, virtualIP)
	})

	// author: mjoseph@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:mjoseph-ConnectedOnly-High-41030-preemption strategy for keepalived ipfailover [Disruptive]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-41030",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)
		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")

		g.By("set the HA virtual IP for the failover group")
		podNames := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		ipv4Address := getPodv4Address(oc, oc.Namespace(), podNames[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_PREEMPTION=preempt_delay 60`)

		g.By("verify the HA virtual ip ENV variable")
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err1, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		newPodName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		checkenv := readPodEnv(oc, newPodName[0], oc.Namespace(), "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))
		checkenv1 := readPodEnv(oc, newPodName[0], oc.Namespace(), "OPENSHIFT_HA_PREEMPTION")
		o.Expect(checkenv1).To(o.ContainSubstring("preempt_delay 60"))

		g.By("find the primary and the secondary pod")
		primaryPod := getVipOwnerPod(oc, oc.Namespace(), newPodName, virtualIP)
		secondaryPod := slicingElement(primaryPod, newPodName)
		g.By("restarting the ipfailover primary pod")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", oc.Namespace(), "pod", primaryPod).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verify whether the other pod becomes primary and it has the VIP")
		_ = getVipOwnerPod(oc, oc.Namespace(), secondaryPod, virtualIP)

		g.By("verify the new pod preempts the exiting primary after the delay expires")
		latestpods := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		// Identifying the new pod from the other
		futurePrimaryPod := slicingElement(secondaryPod[0], latestpods)
		// Waiting till the preempt delay 60 seconds expires
		time.Sleep(60 * time.Second)
		waitForPreemptPod(oc, oc.Namespace(), futurePrimaryPod[0], virtualIP)
	})

	// author: mjoseph@redhat.com
	// might conflict with other ipfailover cases so set it as Serial
	g.It("Author:mjoseph-ConnectedOnly-Medium-49214-Excluding the existing VRRP cluster ID from ipfailover deployments [Serial]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "router")
		customTemp := filepath.Join(buildPruningBaseDir, "ipfailover.yaml")
		var (
			ipf = ipfailoverDescription{
				name:      "ipf-49214",
				namespace: "",
				image:     "",
				template:  customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		oc.SetupProject()
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, oc.Namespace())
		err := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err, "the pod with ipfailover=hello-openshift Ready status not met")

		g.By("add 254 VIPs for the failover group")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `OPENSHIFT_HA_VIRTUAL_IPS=192.168.254.1-254`)

		g.By("Exclude VIP '9' from the ipfailover group")
		setEnvVariable(oc, oc.Namespace(), "deploy/"+ipf.name, `HA_EXCLUDED_VRRP_IDS=9`)

		g.By("verify from the ipfailover pod, the excluded VRRP_ID is configured")
		err1 := waitForPodWithLabelReady(oc, oc.Namespace(), "ipfailover=hello-openshift")
		exutil.AssertWaitPollNoErr(err1, "the pod with ipfailover=hello-openshift Ready status not met")
		ensureIpfailoverEnterMaster(oc, oc.Namespace(), "ipfailover=hello-openshift")
		newPodName := getPodName(oc, oc.Namespace(), "ipfailover=hello-openshift")
		checkenv := readPodEnv(oc, newPodName[0], oc.Namespace(), "HA_EXCLUDED_VRRP_IDS")
		o.Expect(checkenv).To(o.ContainSubstring("HA_EXCLUDED_VRRP_IDS=9"))

		g.By("verify the excluded VIP is removed from the router_ids of ipfailover pods")
		routerIds := readPodData(oc, newPodName[0], oc.Namespace(), `cat /etc/keepalived/keepalived.conf`, `virtual_router_id`)
		o.Expect(routerIds).NotTo(o.ContainSubstring(`virtual_router_id 9`))
	})
})
