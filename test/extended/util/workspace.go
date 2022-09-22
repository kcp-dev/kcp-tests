package util

import (
	"fmt"

	o "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// WorkSpace definition
type WorkSpace struct {
	Name             string   // WorkSpace Name                                                E.g. e2e-test-kcp-workspace-xxxxx
	CurrentNameSpace string   // The latest workSpace's namespace created by SetupNameSpace()  E.g. e2e-ns-kcp-workspace-xxxxx
	Namespaces       []string // WorkSpace's Namespaces created by SetupNameSpace()            E.g. e2e-ns-kcp-workspace-xxxxx
	ServerURL        string   // WorkSpace ServerURL                                           E.g. https://{{kcp-service-domain}}/clusters/root:orgID:e2e-test-kcp-workspace-xxxxx
	ParentServerURL  string   // WorkSpace ParentServerURL                                     E.g. https://{{kcp-service-domain}}/clusters/root:orgID
}

// SetNamespace creates a new namespace with
// name in the format of "e2e-ns-"" + basename + 5Bytes random string
func (ws *WorkSpace) SetNamespace(c *CLI) {
	newNamespace := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-ns-%s-", c.kubeFramework.BaseName))
	e2e.Logf("Creating namespace %q", newNamespace)
	output, errinfo := c.WithoutNamespace().WithoutKubeconf().Run("create").Args("namespace", newNamespace).Output()
	o.Expect(errinfo).NotTo(o.HaveOccurred())
	o.Expect(output).Should(o.ContainSubstring("created"))
	ws.CurrentNameSpace = newNamespace
	ws.Namespaces = append(c.currentWorkSpace.Namespaces, newNamespace)
}
