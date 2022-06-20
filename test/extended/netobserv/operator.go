package netobserv

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"
)

type version struct {
	Operator struct {
		Branch  string `yaml:"branch"`
		TagName string `yaml:"tagName"`
	} `yaml:"operator"`
	GoflowKube struct {
		Image string `yaml:"image"`
	} `yaml:"goflow-kube"`
	ConsolePlugin struct {
		Image string `yaml:"image"`
	} `yaml:"consolePlugin"`
}

// deploy/undeploys network-observability operator given action is true/false
func (versions *version) deployNetobservOperator(action bool, tempdir *string) error {

	var (
		deployCmd string
		err       error
	)

	if action {
		if err != nil {
			return err
		}
		err = versions.gitCheckout(tempdir)
		if err != nil {
			return err
		}
		defer os.RemoveAll(*tempdir)
		e2e.Logf("cloned git repo successfully at %s", *tempdir)
		deployCmd = "make deploy"
	} else {
		e2e.Logf("undeploying operator")
		deployCmd = "make undeploy"
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s && %s", *tempdir, deployCmd))
	err = cmd.Run()

	if err != nil {
		e2e.Logf("Failed action: %s for network-observability operator - err %s", deployCmd, err.Error())
		return err

	}
	return nil
}

// parses version.yaml and converts to version struct
func (vers *version) versionMap() error {
	componentVersions := "version.yaml"
	versionsFixture := exutil.FixturePath("testdata", "netobserv", componentVersions)
	versions, err := os.ReadFile(versionsFixture)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(versions, &vers)
	if err != nil {
		return err
	}

	e2e.Logf("versions in versionMap are %s", vers)

	return nil
}

// clones operator git repo and switches to tag if specified in version.yaml
func (versions *version) gitCheckout(tempdir *string) error {
	var err error
	*tempdir, err = ioutil.TempDir("", "netobserv")
	operatorDir := "network-observability-operator"
	operatorRepo := fmt.Sprintf("https://github.com/netobserv/%s.git", operatorDir)

	repo, err := git.PlainClone(*tempdir, false, &git.CloneOptions{
		URL:           operatorRepo,
		ReferenceName: "refs/heads/main",
		SingleBranch:  true,
	})

	if err != nil {
		e2e.Logf("failed to clone git repo %s: %s", operatorRepo, err)
		return err
	}

	e2e.Logf("cloned git repo for %s successfully at %s", operatorDir, *tempdir)

	tree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Checkout our tag
	if versions.Operator.TagName != "" {
		e2e.Logf("Deploying tag %s\n", versions.Operator.TagName)
		err = tree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName("refs/tags/v" + versions.Operator.TagName),
		})

		if err != nil {
			return err
		}
		os.Setenv("VERSION", versions.Operator.TagName)
	}
	return nil
}
