package hypershift

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type OcpClientVerb string

const (
	OcpGet      OcpClientVerb = "get"
	OcpPatch    OcpClientVerb = "patch"
	OcpWhoami   OcpClientVerb = "whoami"
	OcpDelete   OcpClientVerb = "delete"
	OcpAnnotate OcpClientVerb = "annotate"
	OcpDebug    OcpClientVerb = "debug"
	OcpExec     OcpClientVerb = "exec"
)

func doOcpReq(oc *exutil.CLI, verb OcpClientVerb, notEmpty bool, args []string) string {
	e2e.Logf("running command : oc %s %s \n", string(verb), strings.Join(args, " "))
	res, err := oc.AsAdmin().WithoutNamespace().Run(string(verb)).Args(args...).Output()
	o.Expect(err).ShouldNot(o.HaveOccurred())
	if notEmpty {
		o.Expect(res).ShouldNot(o.BeEmpty())
	}
	return res
}

func checkSubstring(src string, expect []string) {
	if expect == nil || len(expect) <= 0 {
		o.Expect(expect).ShouldNot(o.BeEmpty())
	}

	for i := 0; i < len(expect); i++ {
		o.Expect(src).To(o.ContainSubstring(expect[i]))
	}
}

type workload struct {
	name      string
	namespace string
	template  string
}

func (wl *workload) create(oc *exutil.CLI, kubeconfig, parsedTemplate string) {
	err := wl.applyResourceFromTemplate(oc, kubeconfig, parsedTemplate, "--ignore-unknown-parameters=true", "-f", wl.template, "-p", "NAME="+wl.name, "NAMESPACE="+wl.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (wl *workload) delete(oc *exutil.CLI, kubeconfig, parsedTemplate string) {
	defer func() {
		path := filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-"+parsedTemplate)
		os.Remove(path)
	}()
	args := []string{"job", wl.name, "-n", wl.namespace}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeconfig)
	}
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(args...).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (wl *workload) applyResourceFromTemplate(oc *exutil.CLI, kubeconfig, parsedTemplate string, parameters ...string) error {
	var configFile string

	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(parsedTemplate)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("the file of resource is %s", configFile)

	var args = []string{"-f", configFile}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeconfig)
	}
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args(args...).Execute()
}
