package securityandcompliance

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime/debug"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CLI provides function to call the CLI
type CLI struct {
	execPath        string
	ExecCommandPath string
	verb            string
	username        string
	globalArgs      []string
	commandArgs     []string
	finalArgs       []string
	stdin           *bytes.Buffer
	stdout          io.Writer
	stderr          io.Writer
	verbose         bool
	showInfo        bool
	skipTLS         bool
}

// OcComplianceCLI is to initialize the OC-Compliance framework
func OcComplianceCLI() *CLI {
	ocPlug := &CLI{}
	ocPlug.execPath = "oc-compliance"
	ocPlug.showInfo = true
	return ocPlug
}

// Run executes given oc-compliance command
func (c *CLI) Run(commands ...string) *CLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ocPlug := &CLI{
		execPath:        c.execPath,
		ExecCommandPath: c.ExecCommandPath,
	}
	ocPlug.globalArgs = commands
	ocPlug.stdin, ocPlug.stdout, ocPlug.stderr = in, out, errout
	return ocPlug.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *CLI) setOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the oc-compliance CLI command
func (c *CLI) Args(args ...string) *CLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *CLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// ExitError returns the error info
type ExitError struct {
	Cmd    string
	StdErr string
	*exec.ExitError
}

// FatalErr exits the test in case a fatal error has occurred.
func FatalErr(msg interface{}) {
	// the path that leads to this being called isn't always clear...
	fmt.Fprintln(g.GinkgoWriter, string(debug.Stack()))
	e2e.Failf("%v", msg)
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *CLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: %s %s\n", c.execPath, c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	if c.ExecCommandPath != "" {
		e2e.Logf("set exec command path is %s\n", c.ExecCommandPath)
		cmd.Dir = c.ExecCommandPath
	}
	cmd.Stdin = c.stdin
	if c.showInfo {
		e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))
	}
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(out)
		return trimmed, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\n%s", cmd, trimmed)
		return trimmed, &ExitError{ExitError: err.(*exec.ExitError), Cmd: c.execPath + " " + strings.Join(c.finalArgs, " "), StdErr: trimmed}
	default:
		FatalErr(fmt.Errorf("unable to execute %q: %v", c.execPath, err))
		// unreachable code
		return "", nil
	}
}

func assertCheckProfileControls(oc *exutil.CLI, profl string, keyword [2]string) {
	var kw string
	var flag = true
	proControl, err := OcComplianceCLI().Run("controls").Args("profile", profl, "-n", oc.Namespace()).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, v := range keyword {
		kw = fmt.Sprintf("%s", v)
		if !strings.Contains(proControl, kw) {
			e2e.Failf("The keyword %v not exist!", v)
			flag = false
			break
		} else {
			e2e.Logf("keyword matches '%v' with profile '%v' standards and controls", v, profl)
		}
	}
	if flag == false {
		e2e.Failf("The keyword not exist!")
	}
}

func assertRuleResult(oc *exutil.CLI, rule string, namespace string, keyword [2]string) {
	var kw string
	var flag = true
	viewResult, err := OcComplianceCLI().Run("view-result").Args(rule, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, v := range keyword {
		kw = fmt.Sprintf("%s", v)
		if !strings.Contains(viewResult, kw) {
			e2e.Failf("The keyword '%v' not exist!", v)
			flag = false
			break
		} else {
			e2e.Logf("keyword matches '%v' with view-result report output", v)
		}
	}
	if flag == false {
		e2e.Failf("The keyword not exist!")
	}
}

func assertDryRunBind(oc *exutil.CLI, profile string, namespace string, keyword string) {
	cisPrfl, err := OcComplianceCLI().Run("bind").Args("--dry-run", "-N", "my-binding", profile, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if !strings.Contains(cisPrfl, keyword) {
		e2e.Failf("The keyword '%v' not exist!", keyword)
	} else {
		e2e.Logf("keyword matches '%v' with bind dry run command output", keyword)
	}
}

func assertfetchRawResult(oc *exutil.CLI, ssb string, namespace string) {
	tmpOcComlianceDir := "/tmp/oc-compliance-resultsdir-" + getRandomString()
	exec.Command("bash", "-c", "mkdir -p "+tmpOcComlianceDir).Output()
	e2e.Logf("The " + tmpOcComlianceDir + " created successfully...!!\n")
	defer exec.Command("bash", "-c", "rm -rf "+tmpOcComlianceDir).Output()
	_, err := OcComplianceCLI().Run("fetch-raw").Args("scansettingbinding", ssb, "-o", tmpOcComlianceDir, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Fetched raw result and store in " + tmpOcComlianceDir + "...!!\n")
	result, err1 := exec.Command("bash", "-c", "ls "+tmpOcComlianceDir).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	results := fmt.Sprintf("%s", result)
	e2e.Logf("Listed the "+tmpOcComlianceDir+" contents: %v", results)
	if strings.Contains(results, "ocp4-cis") {
		e2e.Logf("List the files from " + tmpOcComlianceDir + "/ocp4-cis directory...!!\n")
		lists, _ := exec.Command("bash", "-c", "ls "+tmpOcComlianceDir+"/ocp4-cis/").Output()
		list := fmt.Sprintf("%s", lists)
		o.Expect(list).To(o.ContainSubstring("ocp4-cis-api-checks-pod.xml.bzip2"))
		e2e.Logf("The raw result file %v fetched successfully.. \n", list)
	} else {
		e2e.Failf("The scan directory %v does not exist.. \n", results)
	}
}

func assertfetchFixes(oc *exutil.CLI, object string, profile string, namespace string) {
	tmpOcComlianceDir := "/tmp/oc-compliance-resultsdir-" + getRandomString()
	exec.Command("bash", "-c", "mkdir -p "+tmpOcComlianceDir).Output()
	e2e.Logf("The " + tmpOcComlianceDir + " created successfully...!!\n")
	defer exec.Command("bash", "-c", "rm -rf "+tmpOcComlianceDir).Output()
	_, err := OcComplianceCLI().Run("fetch-fixes").Args(object, profile, "-o", tmpOcComlianceDir, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Fetched fixes and store in " + tmpOcComlianceDir + "...!!\n")
	result, err1 := exec.Command("bash", "-c", "ls "+tmpOcComlianceDir).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	results := fmt.Sprintf("%s", result)
	e2e.Logf("The fixes fetched successfully:  %v", results)
	if strings.Contains(results, "ocp4-api-server-encryption-provider-cipher.yaml") {
		e2e.Logf("List the file contents from " + tmpOcComlianceDir + "...!!\n")
		lists, _ := exec.Command("bash", "-c", "cat "+tmpOcComlianceDir+"/ocp4-api-server-encryption-provider-cipher.yaml | egrep 'name: cluster'").Output()
		list := fmt.Sprintf("%s", lists)
		e2e.Logf("%v", list)
		if strings.Contains(list, "name: cluster") {
			e2e.Logf("The fetched file content does match with keyword: %v \n", list)
		} else {
			e2e.Failf("The fetched file content does not match with keyword: %v \n", list)
		}
	} else {
		e2e.Failf("The fetched fixes %v does not exist.. \n", results)
	}
}
