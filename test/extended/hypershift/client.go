package hypershift

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime/debug"
	"strings"

	g "github.com/onsi/ginkgo"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type CLI struct {
	verb     string
	username string
	bashparm []string
	stdin    *bytes.Buffer
	stdout   io.Writer
	stderr   io.Writer
	verbose  bool
	showInfo bool
	skipTLS  bool
}

func NewCmdClient() *CLI {
	client := &CLI{}
	client.username = "admin"
	client.showInfo = false
	return client
}

func (c *CLI) WithShowInfo(showInfo bool) *CLI {
	c.showInfo = showInfo
	return c
}

func (c *CLI) WithOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

func (c *CLI) WithStdErr(err io.Writer) *CLI {
	c.stderr = err
	return c
}

// Run executes given Hypershift command verb
func (c *CLI) Run(args ...string) *CLI {
	c.stdin = &bytes.Buffer{}
	if c.stdout == nil {
		c.stdout = &bytes.Buffer{}
	}
	if c.stderr == nil {
		c.stdout = &bytes.Buffer{}
	}

	if c.skipTLS {
		c.bashparm = append(c.bashparm, "--skip-tls=true")
	} else {
		c.bashparm = args
	}
	return c
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
	parms := strings.Join(c.bashparm, " ")
	cmd := exec.Command("bash", "-c", parms)
	cmd.Stdin = c.stdin
	if c.showInfo {
		e2e.Logf("Running '%s'", parms)
	}

	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(out)
		return trimmed, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\n%s", cmd, trimmed)
		return trimmed, &ExitError{ExitError: err.(*exec.ExitError), Cmd: parms, StdErr: trimmed}
	default:
		FatalErr(fmt.Errorf("unable to execute %s: %v", parms, err))
		// unreachable code
		return "", nil
	}
}
