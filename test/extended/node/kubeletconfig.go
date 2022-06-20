package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
)

var _ = g.Describe("[sig-node] NODE kubeletconfig feature", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("node-"+getRandomString(), exutil.KubeConfigPath())

	// author: minmli@redhat.com
	g.It("Author:minmli-Medium-39142-kubeletconfig should not prompt duplicate error message", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "node")
		kubeletConfigT := filepath.Join(buildPruningBaseDir, "kubeletconfig-maxpod.yaml")
		g.By("Test for case OCP-39142")

		labelKey := "custom-kubelet-" + getRandomString()
		labelValue := "maxpods-" + getRandomString()

		kubeletcfg39142 := kubeletCfgMaxpods{
			name:       "custom-kubelet-39142",
			labelkey:   labelKey,
			labelvalue: labelValue,
			maxpods:    239,
			template:   kubeletConfigT,
		}

		g.By("Create a kubeletconfig without matching machineConfigPool label")
		kubeletcfg39142.createKubeletConfigMaxpods(oc)
		defer kubeletcfg39142.deleteKubeletConfigMaxpods(oc)

		g.By("Check kubeletconfig should not prompt duplicate error message")
		keyword := "Error: could not find any MachineConfigPool set for KubeletConfig"
		err := kubeletNotPromptDupErr(oc, keyword, kubeletcfg39142.name)
		exutil.AssertWaitPollNoErr(err, "kubeletconfig prompt duplicate error message")
	})
})
