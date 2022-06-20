package opm

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
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
	podmanAuthfile  string
}

// NewOpmCLI initialize the OPM framework
func NewOpmCLI() *CLI {
	client := &CLI{}
	client.username = "admin"
	client.execPath = "opm"
	client.showInfo = true
	return client
}

func NewInitializer() *CLI {
	client := &CLI{}
	client.username = "admin"
	client.execPath = "initializer"
	client.showInfo = true
	return client
}

// Run executes given command verb
func (c *CLI) Run(commands ...string) *CLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	client := &CLI{
		execPath:        c.execPath,
		verb:            commands[0],
		username:        c.username,
		ExecCommandPath: c.ExecCommandPath,
		podmanAuthfile:  c.podmanAuthfile,
	}
	if c.skipTLS {
		client.globalArgs = append([]string{"--skip-tls=true"}, commands...)
	} else {
		client.globalArgs = commands
	}
	client.stdin, client.stdout, client.stderr = in, out, errout
	return client.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *CLI) setOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the OpenShift CLI command
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

func (c *CLI) SetAuthFile(authfile string) *CLI {
	c.podmanAuthfile = authfile
	return c
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *CLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: %s %s\n", c.execPath, c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	if c.podmanAuthfile != "" {
		cmd.Env = append(os.Environ(), "REGISTRY_AUTH_FILE="+c.podmanAuthfile)
	}
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

func GetDirPath(filePathStr string, filePre string) string {
	if !strings.Contains(filePathStr, "/") || filePathStr == "/" {
		return ""
	}
	dir, file := filepath.Split(filePathStr)
	if strings.HasPrefix(file, filePre) {
		return filePathStr
	} else {
		return GetDirPath(filepath.Dir(dir), filePre)
	}
}

func DeleteDir(filePathStr string, filePre string) bool {
	filePathToDelete := GetDirPath(filePathStr, filePre)
	if filePathToDelete == "" || !strings.Contains(filePathToDelete, filePre) {
		e2e.Logf("there is no such dir %s", filePre)
		return false
	} else {
		e2e.Logf("remove dir %s", filePathToDelete)
		os.RemoveAll(filePathToDelete)
		if _, err := os.Stat(filePathToDelete); err == nil {
			e2e.Logf("delele dir %s failed", filePathToDelete)
			return false
		}
		return true
	}
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}
