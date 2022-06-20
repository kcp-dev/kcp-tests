package osus

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type operatorGroup struct {
	name      string
	namespace string
	template  string
}

type subscription struct {
	name            string
	namespace       string
	channel         string
	approval        string
	operatorName    string
	sourceName      string
	sourceNamespace string
	startingCSV     string
	template        string
}

type resource struct {
	oc               *exutil.CLI
	asAdmin          bool
	withoutNamespace bool
	kind             string
	name             string
	requireNS        bool
	namespace        string
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var cfgFileJson string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "osus-resource-cfg.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		cfgFileJson = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", cfgFileJson)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", cfgFileJson).Execute()
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

func (og *operatorGroup) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscription) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"APPROVAL="+sub.approval, "OPERATORNAME="+sub.operatorName, "SOURCENAME="+sub.sourceName, "SOURCENAMESPACE="+sub.sourceNamespace, "STARTINGCSV="+sub.startingCSV)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func removeResource(oc *exutil.CLI, parameters ...string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(parameters...).Output()
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("No resource found!")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (og *operatorGroup) delete(oc *exutil.CLI) {
	removeResource(oc, "-n", og.namespace, "operatorgroup", og.name)
}

func (sub *subscription) delete(oc *exutil.CLI) {
	removeResource(oc, "-n", sub.namespace, "subscription", sub.name)
}
