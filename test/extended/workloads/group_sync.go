package workloads

import (
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-apps] Workloads", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("default-"+getRandomString(), exutil.KubeConfigPath())

	// author: yinzhou@redhat.com
	g.It("Author:yinzhou-High-40053-Syncing the groups with valid format [Flaky]", func() {
		buildPruningBaseDir := exutil.FixturePath("testdata", "workloads")
		initGroup := filepath.Join(buildPruningBaseDir, "init.ldif")
		syncConfig := filepath.Join(buildPruningBaseDir, "sync-config-user-defined.yaml")

		g.By("Test for case OCP-40053 Syncing the groups with valid format")
		g.By("create new namespace")
		oc.SetupProject()

		g.By("create The LDAP server")
		createLdapService(oc, oc.Namespace(), "ldapserver", initGroup)
		defer oc.Run("delete").Args("pod/ldapserver", "-n", oc.Namespace()).Execute()
		g.By("start to port-forward")
		cmd, _, _ := oc.Run("port-forward").Args("ldapserver", "59738:389", "-n", oc.Namespace()).BackgroundRC()
		defer cmd.Process.Kill()

		g.By("dump group info from the LDAP server")
		groupFile := getSyncGroup(oc, syncConfig)

		g.By("run the apply command to valid the group info")
		output, _ := oc.AsAdmin().Run("apply").Args("-f", groupFile, "--dry-run=server").Output()
		if matched, _ := regexp.MatchString("tc509128group1 created", output); matched {
			e2e.Logf("groups are created by dry-run:\n%s", output)
		}

	})

})
