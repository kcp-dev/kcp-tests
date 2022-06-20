package nfd

import (
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] PSAP should", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("nfd-test", exutil.KubeConfigPath())
		apiNamespace = "openshift-machine-api"
		iaasPlatform string
	)

	g.BeforeEach(func() {
		// get IaaS platform
		iaasPlatform = exutil.CheckPlatform(oc)
	})

	// author: nweinber@redhat.com
	g.It("Author:nweinber-Medium-43461-Add a new worker node on an NFD-enabled OCP cluster [Slow] [Flaky]", func() {

		// currently test is only supported on AWS, GCP, and Azure
		if iaasPlatform != "aws" && iaasPlatform != "gcp" && iaasPlatform != "azure" {
			g.Skip("IAAS platform: " + iaasPlatform + " is not automated yet - skipping test ...")
		}

		// test requires NFD to be installed and an instance to be runnning
		g.By("Deploy NFD Operator and create instance on Openshift Container Platform")
		nfdInstalled := isPodInstalled(oc, nfdNamespace)
		isNodeLabeled := exutil.IsNodeLabeledByNFD(oc)
		if nfdInstalled && isNodeLabeled {
			e2e.Logf("NFD installation and node label found! Continuing with test ...")
		} else {
			exutil.InstallNFD(oc, nfdNamespace)
			exutil.CreateNFDInstance(oc, nfdNamespace)
		}

		g.By("Get existing machinesets in cluster")
		oc_get_machineset := exutil.ListWorkerMachineSetNames(oc)
		e2e.Logf("Existing machinesets:\n%v", oc_get_machineset)

		g.By("Get name of first machineset in existing machineset list")
		first_machineset_name := exutil.GetFirstLinuxMachineSets(oc)
		e2e.Logf("Got %v from machineset list", first_machineset_name)

		g.By("Generate name of new machineset that will be created")
		re1 := regexp.MustCompile(`-worker-`)
		new_machineset_name := re1.ReplaceAllString(first_machineset_name, "-worker-new-")
		e2e.Logf("Generated %v as new machineset name", new_machineset_name)

		defer func() {
			g.By("Destroy newly created machineset and node once check is complete")
			err := deleteMachineSet(oc, apiNamespace, new_machineset_name)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Create machineset-43461-old.yaml")
		machineset_yaml_old, err := createYAMLFromMachineSet(oc, apiNamespace, first_machineset_name, "machineset-43461-old.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Print machineset-old.yaml")
		raw_yaml_file, err := exec.Command("bash", "-c", "cat "+machineset_yaml_old+"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		stringified_yaml_file := strings.TrimSpace(string(raw_yaml_file))
		e2e.Logf("File:\n %v", stringified_yaml_file)

		g.By("Read machineset-old.yaml in as string, find and replace old machineset name with new machineset name, and write machineset-new.yaml")
		machineset_old_b, err := ioutil.ReadFile(machineset_yaml_old)
		o.Expect(err).NotTo(o.HaveOccurred())
		machineset_old_s := string(machineset_old_b)
		re2 := regexp.MustCompile(first_machineset_name)
		machineset_new_s := re2.ReplaceAllString(machineset_old_s, new_machineset_name)
		machineset_new_b := []byte(machineset_new_s)
		err = ioutil.WriteFile("machineset-43461-new.yaml", machineset_new_b, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Print machineset-new.yaml")
		raw_yaml_file, err = exec.Command("bash", "-c", "cat machineset-43461-new.yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		stringified_yaml_file = strings.TrimSpace(string(raw_yaml_file))
		e2e.Logf("File:\n %v", stringified_yaml_file)

		g.By("Create new machineset from machineset-new.yaml")
		err = createMachineSetFromYAML(oc, "machineset-43461-new.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verify new node was created")
		exutil.WaitForMachinesRunning(oc, 1, new_machineset_name)

		g.By("Check that the NFD labels are created")
		oc_describe_nodes, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oc_describe_nodes).To(o.ContainSubstring("feature"))

	})
})
