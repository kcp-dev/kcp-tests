package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-node] NODE Probe feature", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("node-"+getRandomString(), exutil.KubeConfigPath())

	// author: minmli@redhat.com
	g.It("Author:minmli-High-41579-Liveness probe failures should terminate the pod immediately", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "node")
		podProbeT := filepath.Join(buildPruningBaseDir, "pod-liveness-probe.yaml")
		g.By("Test for case OCP-41579")

		g.By("create new namespace")
		oc.SetupProject()

		pod41579 := podLivenessProbe{
			name:                  "probe-pod-41579",
			namespace:             oc.Namespace(),
			overridelivenessgrace: "10",
			terminationgrace:      300,
			failurethreshold:      1,
			periodseconds:         60,
			template:              podProbeT,
		}

		g.By("Create a pod with liveness probe")
		pod41579.createPodLivenessProbe(oc)
		defer pod41579.deletePodLivenessProbe(oc)

		g.By("check pod status")
		err := podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")

		g.By("check pod events") // create function
		timeout := 90
		keyword := "Container test failed liveness probe, will be restarted"
		err = podEvent(oc, timeout, keyword)
		exutil.AssertWaitPollNoErr(err, "event check failed: "+keyword)

		g.By("check pod restart in override termination grace period")
		err = podStatus(oc)
		exutil.AssertWaitPollNoErr(err, "pod is not running")
	})
})
