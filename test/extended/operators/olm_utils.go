package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"io/ioutil"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	asUser           = false
	withoutNamespace = true
	withNamespace    = false
	compare          = true
	contain          = false
	requireNS        = true
	notRequireNS     = false
	present          = true
	notPresent       = false
	ok               = true
	nok              = false
)

type csvDescription struct {
	name      string
	namespace string
}

// the method is to delete csv.
func (csv csvDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("remove %s, ns %s", csv.name, csv.namespace)
	dr.getIr(itName).remove(csv.name, "csv", csv.namespace)
}

type subscriptionDescription struct {
	subName                string `json:"name"`
	namespace              string `json:"namespace"`
	channel                string `json:"channel"`
	ipApproval             string `json:"installPlanApproval"`
	operatorPackage        string `json:"spec.name"`
	catalogSourceName      string `json:"source"`
	catalogSourceNamespace string `json:"sourceNamespace"`
	startingCSV            string `json:"startingCSV,omitempty"`
	currentCSV             string
	installedCSV           string
	template               string
	singleNamespace        bool
	ipCsv                  string
}

// PrometheusQueryResult the prometheus query result
type PrometheusQueryResult struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name      string `json:"__name__"`
				Approval  string `json:"approval"`
				Channel   string `json:"channel"`
				Container string `json:"container"`
				Endpoint  string `json:"endpoint"`
				Installed string `json:"installed"`
				Instance  string `json:"instance"`
				Job       string `json:"job"`
				SrcName   string `json:"name"`
				Namespace string `json:"namespace"`
				Package   string `json:"package"`
				Pod       string `json:"pod"`
				Service   string `json:"service"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

//the method is to create sub, and save the sub resrouce into dr. and more create csv possible depending on sub.ipApproval
//if sub.ipApproval is Automatic, it will wait the sub's state become AtLatestKnown and get installed csv as sub.installedCSV, and save csv into dr
//if sub.ipApproval is not Automatic, it will just wait sub's state become UpgradePending
func (sub *subscriptionDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	// for most operator subscription failure, the reason is that there is a left cluster-scoped CSV.
	// I'd like to print all CSV before create it.
	// allCSVs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "--all-namespaces").Output()
	// if err != nil {
	// 	e2e.Failf("!!! Couldn't get all CSVs:%v\n", err)
	// }
	// e2e.Logf("!!! Get all CSVs in this cluster:\n%s\n", allCSVs)

	sub.createWithoutCheck(oc, itName, dr)
	if strings.Compare(sub.ipApproval, "Automatic") == 0 {
		sub.findInstalledCSV(oc, itName, dr)
	} else {
		newCheck("expect", asAdmin, withoutNamespace, compare, "UpgradePending", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	}
}

// It's for the manual subscription to get its latest status, such as, the installedCSV.
func (sub *subscriptionDescription) update(oc *exutil.CLI, itName string, dr describerResrouce) {
	installedCSV := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
	o.Expect(installedCSV).NotTo(o.BeEmpty())
	if strings.Compare(sub.installedCSV, installedCSV) != 0 {
		sub.installedCSV = installedCSV
		dr.getIr(itName).add(newResource(oc, "csv", sub.installedCSV, requireNS, sub.namespace))
	}
	e2e.Logf("updating the subscription to get the latest installedCSV: %s", sub.installedCSV)
}

//the method is to just create sub, and save it to dr, do not check its state.
func (sub *subscriptionDescription) createWithoutCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	//isAutomatic := strings.Compare(sub.ipApproval, "Automatic") == 0

	//startingCSV is not necessary. And, if there are multi same package from different CatalogSource, it will lead to error.
	//if strings.Compare(sub.currentCSV, "") == 0 {
	//	sub.currentCSV = getResource(oc, asAdmin, withoutNamespace, "packagemanifest", sub.operatorPackage, fmt.Sprintf("-o=jsonpath={.status.channels[?(@.name==\"%s\")].currentCSV}", sub.channel))
	//	o.Expect(sub.currentCSV).NotTo(o.BeEmpty())
	//}

	//if isAutomatic {
	//	sub.startingCSV = sub.currentCSV
	//} else {
	//	o.Expect(sub.startingCSV).NotTo(o.BeEmpty())
	//}

	// for most operator subscription failure, the reason is that there is a left cluster-scoped CSV.
	// I'd like to print all CSV before create it.
	// It prints many lines which descrease the exact match for RP, and increase log size.
	// So, change it to one line with neccessary information csv name and namespace.
	allCSVs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "--all-namespaces", "-o=jsonpath={range .items[*]}{@.metadata.name}{\",\"}{@.metadata.namespace}{\":\"}{end}").Output()
	if err != nil {
		e2e.Failf("!!! Couldn't get all CSVs:%v\n", err)
	}
	csvMap := make(map[string][]string)
	csvList := strings.Split(allCSVs, ":")
	for _, csv := range csvList {
		if strings.Compare(csv, "") == 0 {
			continue
		}
		name := strings.Split(csv, ",")[0]
		ns := strings.Split(csv, ",")[1]
		val, ok := csvMap[name]
		if ok {
			if strings.HasPrefix(ns, "openshift-") {
				alreadyOpenshiftDefaultNS := false
				for _, v := range val {
					if strings.Contains(v, "openshift-") {
						alreadyOpenshiftDefaultNS = true // normally one default operator exists in all openshift- ns, like elasticsearch-operator
						// only add one openshift- ns to indicate. to save log size and line size. Or else one line
						// will be greater than 3k
						break
					}
				}
				if !alreadyOpenshiftDefaultNS {
					val = append(val, ns)
					csvMap[name] = val
				}
			} else {
				val = append(val, ns)
				csvMap[name] = val
			}
		} else {
			nsSlice := make([]string, 20)
			nsSlice[1] = ns
			csvMap[name] = nsSlice
		}
	}
	for name, ns := range csvMap {
		e2e.Logf("getting csv is %v, the related NS is %v", name, ns)
	}

	e2e.Logf("create sub %s", sub.subName)
	err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.subName, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"APPROVAL="+sub.ipApproval, "OPERATORNAME="+sub.operatorPackage, "SOURCENAME="+sub.catalogSourceName, "SOURCENAMESPACE="+sub.catalogSourceNamespace, "STARTINGCSV="+sub.startingCSV)

	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "sub", sub.subName, requireNS, sub.namespace))
}

//the method is to check if the sub's state is AtLatestKnown.
//if it is AtLatestKnown, get installed csv from sub and save it to dr.
//if it is not AtLatestKnown, raise error.
func (sub *subscriptionDescription) findInstalledCSV(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		state := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}")
		if strings.Compare(state, "AtLatestKnown") == 0 {
			return true, nil
		}
		e2e.Logf("sub %s state is %s, not AtLatestKnown", sub.subName, state)
		return false, nil
	})
	if err != nil {
		output := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o", "yaml")
		e2e.Logf(output)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown", sub.subName))

	installedCSV := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
	o.Expect(installedCSV).NotTo(o.BeEmpty())
	if strings.Compare(sub.installedCSV, installedCSV) != 0 {
		sub.installedCSV = installedCSV
		dr.getIr(itName).add(newResource(oc, "csv", sub.installedCSV, requireNS, sub.namespace))
	}
	e2e.Logf("the installed CSV name is %s", sub.installedCSV)
}

//the method is to check if the cv parameter is same to the installed csv.
//if not same, raise error.
//if same, nothong happen.
func (sub *subscriptionDescription) expectCSV(oc *exutil.CLI, itName string, dr describerResrouce, cv string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		sub.findInstalledCSV(oc, itName, dr)
		if strings.Compare(sub.installedCSV, cv) == 0 {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("expected csv %s not found", cv))
}

//the method is to approve the install plan when you create sub with sub.ipApproval != Automatic
//normally firstly call sub.create(), then call this method sub.approve. it is used to operator upgrade case.
func (sub *subscriptionDescription) approve(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := wait.Poll(6*time.Second, 360*time.Second, func() (bool, error) {
		for strings.Compare(sub.installedCSV, "") == 0 {
			state := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}")
			if strings.Compare(state, "AtLatestKnown") == 0 {
				sub.installedCSV = getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
				dr.getIr(itName).add(newResource(oc, "csv", sub.installedCSV, requireNS, sub.namespace))
				e2e.Logf("it is already done, and the installed CSV name is %s", sub.installedCSV)
				continue
			}

			ipCsv := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}{\" \"}{.status.currentCSV}")
			sub.ipCsv = ipCsv + "##" + sub.ipCsv
			installPlan := strings.Fields(ipCsv)[0]
			o.Expect(installPlan).NotTo(o.BeEmpty())
			e2e.Logf("try to approve installPlan %s", installPlan)
			patchResource(oc, asAdmin, withoutNamespace, "installplan", installPlan, "-n", sub.namespace, "--type", "merge", "-p", "{\"spec\": {\"approved\": true}}")
			err := wait.Poll(10*time.Second, 70*time.Second, func() (bool, error) {
				err := newCheck("expect", asAdmin, withoutNamespace, compare, "Complete", ok, []string{"installplan", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).checkWithoutAssert(oc)
				if err != nil {
					e2e.Logf("the get error is %v, and try next", err)
					return false, nil
				}
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("installPlan %s is not Complete", installPlan))
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("not found installed csv for %s", sub.subName))
}

// The user can approve the specific InstallPlan:
// NAME            CSV                   APPROVAL   APPROVED
// install-vmwlk   etcdoperator.v0.9.4   Manual     false
// install-xqgtx   etcdoperator.v0.9.2   Manual     true
// sub.approveSpecificIP(oc, itName, dr, "etcdoperator.v0.9.2", "Complete") approve this "etcdoperator.v0.9.2" InstallPlan only
func (sub *subscriptionDescription) approveSpecificIP(oc *exutil.CLI, itName string, dr describerResrouce, csvName string, phase string) {
	// fix https://github.com/openshift/openshift-tests-private/issues/735
	var state string
	wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		state = getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}")
		if strings.Compare(state, "UpgradePending") == 0 {
			return true, nil
		}
		return false, nil
	})
	if strings.Compare(state, "UpgradePending") == 0 {
		e2e.Logf(" and the expected CSV")
		ipCsv := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installplan.name}{\" \"}{.status.currentCSV}")
		if strings.Contains(ipCsv, csvName) {
			installPlan := strings.Fields(ipCsv)[0]
			o.Expect(installPlan).NotTo(o.BeEmpty())
			e2e.Logf("---> Get the pending InstallPlan %s", installPlan)
			patchResource(oc, asAdmin, withoutNamespace, "installplan", installPlan, "-n", sub.namespace, "--type", "merge", "-p", "{\"spec\": {\"approved\": true}}")
			err := wait.Poll(10*time.Second, 70*time.Second, func() (bool, error) {
				err := newCheck("expect", asAdmin, withoutNamespace, compare, phase, ok, []string{"installplan", installPlan, "-n", sub.namespace, "-o=jsonpath={.status.phase}"}).checkWithoutAssert(oc)
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			exutil.AssertWaitPollNoErr(err, fmt.Sprintf("installPlan %s is not %s", installPlan, phase))
		} else {
			e2e.Logf("--> Not found the specific InstallPlan, the current IP:%s", ipCsv)
		}
	} else {
		CSVs := getResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}{\" \"}{.status.currentCSV}")
		e2e.Logf("---> No need any apporval operation, the InstalledCSV and currentCSV are the same: %s", CSVs)
	}
}

//the method is to construct one csv object.
func (sub *subscriptionDescription) getCSV() csvDescription {
	e2e.Logf("csv is %s, namespace is %s", sub.installedCSV, sub.namespace)
	return csvDescription{sub.installedCSV, sub.namespace}
}

// get the reference InstallPlan
func (sub *subscriptionDescription) getIP(oc *exutil.CLI) string {
	var installPlan string
	waitErr := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		var err error
		installPlan, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.installPlanRef.name}").Output()
		if strings.Compare(installPlan, "") == 0 || err != nil {
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(waitErr, fmt.Sprintf("sub %s has no installplan", sub.subName))
	o.Expect(installPlan).NotTo(o.BeEmpty())
	return installPlan
}

//the method is to get the CR version from alm-examples of csv if it exists
func (sub *subscriptionDescription) getInstanceVersion(oc *exutil.CLI) string {
	version := ""
	output := strings.Split(getResource(oc, asUser, withoutNamespace, "csv", sub.installedCSV, "-n", sub.namespace, "-o=jsonpath={.metadata.annotations.alm-examples}"), "\n")
	for _, line := range output {
		if strings.Contains(line, "\"version\"") {
			version = strings.Trim(strings.Fields(strings.TrimSpace(line))[1], "\"")
			break
		}
	}
	o.Expect(version).NotTo(o.BeEmpty())
	e2e.Logf("sub cr version is %s", version)
	return version
}

//the method is obsolete
func (sub *subscriptionDescription) createInstance(oc *exutil.CLI, instance string) {
	path := filepath.Join(e2e.TestContext.OutputDir, sub.namespace+"-"+"instance.json")
	err := ioutil.WriteFile(path, []byte(instance), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-n", sub.namespace, "-f", path).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to delete sub which is saved when calling sub.create() or sub.createWithoutCheck()
func (sub *subscriptionDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("remove sub %s, ns is %s", sub.subName, sub.namespace)
	dr.getIr(itName).remove(sub.subName, "sub", sub.namespace)
}
func (sub *subscriptionDescription) deleteCSV(itName string, dr describerResrouce) {
	e2e.Logf("remove csv %s, ns is %s, the subscription is: %s", sub.installedCSV, sub.namespace, sub)
	dr.getIr(itName).remove(sub.installedCSV, "csv", sub.namespace)
}

//the method is to patch sub object
func (sub *subscriptionDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "sub", sub.subName, "-n", sub.namespace, "--type", "merge", "-p", patch)
}

type subscriptionDescriptionProxy struct {
	subscriptionDescription
	httpProxy  string
	httpsProxy string
	noProxy    string
}

//the method is to just create sub with proxy, and save it to dr, do not check its state.
func (sub *subscriptionDescriptionProxy) createWithoutCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	e2e.Logf("install subscriptionDescriptionProxy")
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "SUBNAME="+sub.subName, "SUBNAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"APPROVAL="+sub.ipApproval, "OPERATORNAME="+sub.operatorPackage, "SOURCENAME="+sub.catalogSourceName, "SOURCENAMESPACE="+sub.catalogSourceNamespace, "STARTINGCSV="+sub.startingCSV,
		"SUBHTTPPROXY="+sub.httpProxy, "SUBHTTPSPROXY="+sub.httpsProxy, "SUBNOPROXY="+sub.noProxy)

	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "sub", sub.subName, requireNS, sub.namespace))
	e2e.Logf("install subscriptionDescriptionProxy %s SUCCESS", sub.subName)
}

func (sub *subscriptionDescriptionProxy) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	sub.createWithoutCheck(oc, itName, dr)
	if strings.Compare(sub.ipApproval, "Automatic") == 0 {
		sub.findInstalledCSV(oc, itName, dr)
	} else {
		newCheck("expect", asAdmin, withoutNamespace, compare, "UpgradePending", ok, []string{"sub", sub.subName, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	}
}

type crdDescription struct {
	name     string
	template string
}

//the method is to create CRD with template and save it to dr.
func (crd *crdDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", crd.template, "-p", "NAME="+crd.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "crd", crd.name, notRequireNS, ""))
	e2e.Logf("create crd %s SUCCESS", crd.name)
}

//the method is to delete CRD.
func (crd *crdDescription) delete(oc *exutil.CLI) {
	e2e.Logf("remove crd %s, withoutNamespace is %v", crd.name, withoutNamespace)
	removeResource(oc, asAdmin, withoutNamespace, "crd", crd.name)
}

type configMapDescription struct {
	name      string
	namespace string
	template  string
}

//the method is to create cm with template and save it to dr
func (cm *configMapDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cm.template, "-p", "NAME="+cm.name, "NAMESPACE="+cm.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "cm", cm.name, requireNS, cm.namespace))
	e2e.Logf("create cm %s SUCCESS", cm.name)
}

//the method is to patch cm.
func (cm *configMapDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "cm", cm.name, "-n", cm.namespace, "--type", "merge", "-p", patch)
}

//the method is to delete cm.
func (cm *configMapDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("remove cm %s, ns is %v", cm.name, cm.namespace)
	dr.getIr(itName).remove(cm.name, "cm", cm.namespace)
}

type catalogSourceDescription struct {
	name          string
	namespace     string
	displayName   string
	publisher     string
	sourceType    string
	address       string
	template      string
	priority      int
	secret        string
	interval      string
	imageTemplate string
}

//the method is to create catalogsource with template, and save it to dr.
func (catsrc *catalogSourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	if strings.Compare(catsrc.interval, "") == 0 {
		catsrc.interval = "10m0s"
		e2e.Logf("set interval to be 10m0s")
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", catsrc.template,
		"-p", "NAME="+catsrc.name, "NAMESPACE="+catsrc.namespace, "ADDRESS="+catsrc.address, "SECRET="+catsrc.secret,
		"DISPLAYNAME="+"\""+catsrc.displayName+"\"", "PUBLISHER="+"\""+catsrc.publisher+"\"", "SOURCETYPE="+catsrc.sourceType,
		"INTERVAL="+catsrc.interval, "IMAGETEMPLATE="+catsrc.imageTemplate)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "catsrc", catsrc.name, requireNS, catsrc.namespace))
	e2e.Logf("create catsrc %s SUCCESS", catsrc.name)
}

func (catsrc *catalogSourceDescription) createWithCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	catsrc.create(oc, itName, dr)
	newCheck("expect", asAdmin, withoutNamespace, compare, "READY", ok, []string{"catsrc", catsrc.name, "-n", catsrc.namespace, "-o=jsonpath={.status..lastObservedState}"}).check(oc)
	e2e.Logf("catsrc %s lastObservedState is READY", catsrc.name)
}

//the method is to delete catalogsource.
func (catsrc *catalogSourceDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete carsrc %s, ns is %s", catsrc.name, catsrc.namespace)
	dr.getIr(itName).remove(catsrc.name, "catsrc", catsrc.namespace)
}

type customResourceDescription struct {
	name      string
	namespace string
	typename  string
	template  string
}

//the method is to create CR with template, and save it to dr.
func (crinstance *customResourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", crinstance.template,
		"-p", "NAME="+crinstance.name, "NAMESPACE="+crinstance.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, crinstance.typename, crinstance.name, requireNS, crinstance.namespace))
	e2e.Logf("create CR %s SUCCESS", crinstance.name)
}

//the method is to delete CR
func (crinstance *customResourceDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete crinstance %s, type is %s, ns is %s", crinstance.name, crinstance.typename, crinstance.namespace)
	dr.getIr(itName).remove(crinstance.name, crinstance.typename, crinstance.namespace)
}

type operatorGroupDescription struct {
	name               string
	namespace          string
	multinslabel       string
	template           string
	serviceAccountName string
	upgradeStrategy    string
}

//the method is to check if og exist. if not existing, create it with template and save it to dr.
//if existing, nothing happen.
func (og *operatorGroupDescription) createwithCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	output, err := doAction(oc, "get", asAdmin, false, "operatorgroup")
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(output, "No resources found") {
		e2e.Logf(fmt.Sprintf("No operatorgroup in project: %s, create one: %s", oc.Namespace(), og.name))
		og.create(oc, itName, dr)
	} else {
		e2e.Logf(fmt.Sprintf("Already exist operatorgroup in project: %s", oc.Namespace()))
	}

}

//the method is to create og and save it to dr
//if og.multinslabel is not set, it will create og with ownnamespace or allnamespace depending on template
//if og.multinslabel is set, it will create og with multinamespace.
func (og *operatorGroupDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	var err error
	if strings.Compare(og.multinslabel, "") != 0 && strings.Compare(og.serviceAccountName, "") != 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "MULTINSLABEL="+og.multinslabel, "SERVICE_ACCOUNT_NAME="+og.serviceAccountName)
	} else if strings.Compare(og.multinslabel, "") == 0 && strings.Compare(og.serviceAccountName, "") == 0 && strings.Compare(og.upgradeStrategy, "") == 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	} else if strings.Compare(og.multinslabel, "") != 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "MULTINSLABEL="+og.multinslabel)
	} else if strings.Compare(og.upgradeStrategy, "") != 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "UPGRADESTRATEGY="+og.upgradeStrategy)
	} else {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "SERVICE_ACCOUNT_NAME="+og.serviceAccountName)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "og", og.name, requireNS, og.namespace))
	e2e.Logf("create og %s success", og.name)
}

//the method is to delete og
func (og *operatorGroupDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete og %s, ns is %s", og.name, og.namespace)
	dr.getIr(itName).remove(og.name, "og", og.namespace)
}

//the struct and its method are obsolete because no operatorSource anymore.
type operatorSourceDescription struct {
	name              string
	namespace         string
	namelabel         string
	registrynamespace string
	displayname       string
	publisher         string
	template          string
	deploymentName    string
}

func (osrc *operatorSourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", osrc.template, "-p", "NAME="+osrc.name, "NAMESPACE="+osrc.namespace,
		"NAMELABEL="+osrc.namelabel, "REGISTRYNAMESPACE="+osrc.registrynamespace, "DISPLAYNAME="+osrc.displayname, "PUBLISHER="+osrc.publisher)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "opsrc", osrc.name, requireNS, osrc.namespace))
	e2e.Logf("create operatorSource %s success", osrc.name)
}

func (osrc *operatorSourceDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete operatorSource %s, ns is %s", osrc.name, osrc.namespace)
	dr.getIr(itName).remove(osrc.name, "opsrc", osrc.namespace)
}

func (osrc *operatorSourceDescription) getRunningNodes(oc *exutil.CLI) string {
	nodesNames := getResource(oc, asAdmin, withoutNamespace, "pod", fmt.Sprintf("--selector=marketplace.operatorSource=%s", osrc.name), "-n", osrc.namespace, "-o=jsonpath={.items[*]..nodeName}")
	o.Expect(nodesNames).NotTo(o.BeEmpty())
	e2e.Logf("getRunningNodes: nodesNames %s", nodesNames)
	return nodesNames
}
func (osrc *operatorSourceDescription) getDeployment(oc *exutil.CLI) {
	output := getResource(oc, asAdmin, withoutNamespace, "deployment", fmt.Sprintf("--selector=opsrc-owner-name=%s", osrc.name), "-n", osrc.namespace, "-o=jsonpath={.items[0].metadata.name}")
	o.Expect(output).NotTo(o.BeEmpty())
	e2e.Logf("getDeployment: deploymentName %s", output)
	osrc.deploymentName = output
}
func (osrc *operatorSourceDescription) patchDeployment(oc *exutil.CLI, content string) {
	if strings.Compare(osrc.deploymentName, "") == 0 {
		osrc.deploymentName = osrc.name
	}
	patchResource(oc, asAdmin, withoutNamespace, "deployment", osrc.deploymentName, "-n", osrc.namespace, "--type", "merge", "-p", content)
}
func (osrc *operatorSourceDescription) getTolerations(oc *exutil.CLI) string {
	if strings.Compare(osrc.deploymentName, "") == 0 {
		osrc.deploymentName = osrc.name
	}
	output := getResource(oc, asAdmin, withoutNamespace, "deployment", osrc.deploymentName, "-n", osrc.namespace, "-o=jsonpath={.spec.template.spec.tolerations}")
	if strings.Compare(output, "") == 0 {
		e2e.Logf("no tolerations %v", output)
		return "\"tolerations\": null"
	}
	tolerations := "\"tolerations\": " + convertLMtoJSON(output)
	e2e.Logf("the tolerations:===%v===", tolerations)
	return tolerations
}

////the struct and its method are obsolete because no csc anymore.
type catalogSourceConfigDescription struct {
	name            string
	namespace       string
	packages        string
	targetnamespace string
	source          string
	template        string
}

func (csc *catalogSourceConfigDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", csc.template, "-p", "NAME="+csc.name, "NAMESPACE="+csc.namespace,
		"PACKAGES="+csc.packages, "TARGETNAMESPACE="+csc.targetnamespace, "SOURCE="+csc.source)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "csc", csc.name, requireNS, csc.namespace))
	e2e.Logf("create catalogSourceConfig %s success", csc.name)
}

func (csc *catalogSourceConfigDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete catalogSourceConfig %s, ns is %s", csc.name, csc.namespace)
	dr.getIr(itName).remove(csc.name, "csc", csc.namespace)
}

type projectDescription struct {
	name            string
	targetNamespace string
}

//the method is to check if the project exists. if not, create it with name, and go to it.
//if existing, nothing happen.
func (p *projectDescription) createwithCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	output, err := doAction(oc, "get", asAdmin, withoutNamespace, "project", p.name)
	if err != nil {
		e2e.Logf(fmt.Sprintf("Output: %s, cannot find the %s project, create one", output, p.name))
		_, err := doAction(oc, "adm", asAdmin, withoutNamespace, "new-project", p.name)
		o.Expect(err).NotTo(o.HaveOccurred())
		dr.getIr(itName).add(newResource(oc, "project", p.name, notRequireNS, ""))
		_, err = doAction(oc, "project", asAdmin, withoutNamespace, p.name)
		o.Expect(err).NotTo(o.HaveOccurred())

	} else {
		e2e.Logf(fmt.Sprintf("project: %s already exist!", p.name))
	}
}

//the method is to delete project with name if exist. and then create it with name, and back to project with targetNamespace
func (p *projectDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	removeResource(oc, asAdmin, withoutNamespace, "project", p.name)
	_, err := doAction(oc, "new-project", asAdmin, withoutNamespace, p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "project", p.name, notRequireNS, ""))
	_, err = doAction(oc, "project", asAdmin, withoutNamespace, p.targetNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to label project
func (p *projectDescription) label(oc *exutil.CLI, label string) {
	_, err := doAction(oc, "label", asAdmin, withoutNamespace, "ns", p.name, "env="+label)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to delete project
func (p *projectDescription) delete(oc *exutil.CLI) {
	_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "project", p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type serviceAccountDescription struct {
	name           string
	namespace      string
	definitionfile string
}

//the method is to construct one sa.
func newSa(name, namespace string) *serviceAccountDescription {
	return &serviceAccountDescription{
		name:           name,
		namespace:      namespace,
		definitionfile: "",
	}
}

//the method is to get sa definition.
func (sa *serviceAccountDescription) getDefinition(oc *exutil.CLI) {
	parameters := []string{"sa", sa.name, "-n", sa.namespace, "-o=json"}
	definitionfile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(parameters...).OutputToFile("sa-config.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("getDefinition: definitionfile is %s", definitionfile)
	sa.definitionfile = definitionfile
}

//the method is to delete sa
func (sa *serviceAccountDescription) delete(oc *exutil.CLI) {
	e2e.Logf("delete sa %s, ns is %s", sa.name, sa.namespace)
	_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "sa", sa.name, "-n", sa.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to apply sa with its member definitionfile
func (sa *serviceAccountDescription) reapply(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", sa.definitionfile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to check if what sa can do is expected with expected paramter.
func (sa *serviceAccountDescription) checkAuth(oc *exutil.CLI, expected string, cr string) {
	err := wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, _ := doAction(oc, "auth", asAdmin, withNamespace, "--as", fmt.Sprintf("system:serviceaccount:%s:%s", sa.namespace, sa.name), "can-i", "create", cr)
		e2e.Logf("the result of checkAuth:%v", output)
		if strings.Contains(output, expected) {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sa %s expects %s permssion to create %s, but no", sa.name, expected, cr))
}

type roleDescription struct {
	name      string
	namespace string
	template  string
}

//the method is to create role with template
func (role *roleDescription) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", role.template,
		"-p", "NAME="+role.name, "NAMESPACE="+role.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to construct one Role object.
func newRole(name string, namespace string) *roleDescription {
	return &roleDescription{
		name:      name,
		namespace: namespace,
	}
}

//the method is to patch Role object.
func (role *roleDescription) patch(oc *exutil.CLI, patch string) {
	patchResource(oc, asAdmin, withoutNamespace, "role", role.name, "-n", role.namespace, "--type", "merge", "-p", patch)
}

//the method is to get rules from Role object.
func (role *roleDescription) getRules(oc *exutil.CLI) string {
	return role.getRulesWithDelete(oc, "nodelete")
}

//the method is to get new rule without delete parameter based on current role.
func (role *roleDescription) getRulesWithDelete(oc *exutil.CLI, delete string) string {
	var roleboday map[string]interface{}
	output := getResource(oc, asAdmin, withoutNamespace, "role", role.name, "-n", role.namespace, "-o=json")
	err := json.Unmarshal([]byte(output), &roleboday)
	o.Expect(err).NotTo(o.HaveOccurred())
	rules := roleboday["rules"].([]interface{})

	handleRuleAttribute := func(rc *strings.Builder, rt string, r map[string]interface{}) {
		rc.WriteString("\"" + rt + "\":[")
		items := r[rt].([]interface{})
		e2e.Logf("%s:%v, and the len:%v", rt, items, len(items))
		for i, v := range items {
			vc := v.(string)
			rc.WriteString("\"" + vc + "\"")
			if i != len(items)-1 {
				rc.WriteString(",")
			}
		}
		rc.WriteString("]")
		if strings.Compare(rt, "verbs") != 0 {
			rc.WriteString(",")
		}
	}

	var rc strings.Builder
	rc.WriteString("[")
	for _, rv := range rules {
		rule := rv.(map[string]interface{})
		if strings.Compare(delete, "nodelete") != 0 && strings.Compare(rule["apiGroups"].([]interface{})[0].(string), delete) == 0 {
			continue
		}

		rc.WriteString("{")
		handleRuleAttribute(&rc, "apiGroups", rule)
		handleRuleAttribute(&rc, "resources", rule)
		handleRuleAttribute(&rc, "verbs", rule)
		rc.WriteString("},")
	}
	result := strings.TrimSuffix(rc.String(), ",") + "]"
	e2e.Logf("rc:%v", result)
	return result
}

type rolebindingDescription struct {
	name      string
	namespace string
	rolename  string
	saname    string
	template  string
}

//the method is to create role with template
func (rolebinding *rolebindingDescription) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", rolebinding.template,
		"-p", "NAME="+rolebinding.name, "NAMESPACE="+rolebinding.namespace, "SA_NAME="+rolebinding.saname, "ROLE_NAME="+rolebinding.rolename)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type checkDescription struct {
	method          string
	executor        bool
	inlineNamespace bool
	expectAction    bool
	expectContent   string
	expect          bool
	resource        []string
}

//the method is to make newCheck object.
//the method paramter is expect, it will check something is expceted or not
//the method paramter is present, it will check something exists or not
//the executor is asAdmin, it will exectue oc with Admin
//the executor is asUser, it will exectue oc with User
//the inlineNamespace is withoutNamespace, it will execute oc with WithoutNamespace()
//the inlineNamespace is withNamespace, it will execute oc with WithNamespace()
//the expectAction take effective when method is expect, if it is contain, it will check if the strings contain substring with expectContent parameter
//                                                       if it is compare, it will check the strings is samme with expectContent parameter
//the expectContent is the content we expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
//the expect is ok, resource existing is OK for method == present, no error raise. if resource not existing, error raise
//the expect is nok, resource not existing is OK for method == present, no error raise. if resource existing, error raise
func newCheck(method string, executor bool, inlineNamespace bool, expectAction bool,
	expectContent string, expect bool, resource []string) checkDescription {
	return checkDescription{
		method:          method,
		executor:        executor,
		inlineNamespace: inlineNamespace,
		expectAction:    expectAction,
		expectContent:   expectContent,
		expect:          expect,
		resource:        resource,
	}
}

//the method is to check the resource per definition of the above described newCheck.
func (ck checkDescription) check(oc *exutil.CLI) {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		o.Expect(ok).To(o.BeTrue())
	case "expect":
		err := expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("expected content %s not found by %v", ck.expectContent, ck.resource))
	default:
		err := fmt.Errorf("unknown method")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//the method is to check the resource, but not assert it which is diffrence with the method check().
func (ck checkDescription) checkWithoutAssert(oc *exutil.CLI) error {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		if ok {
			return nil
		}
		return fmt.Errorf("it is not epxected")
	case "expect":
		return expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
	default:
		return fmt.Errorf("unknown method")
	}
}

//it is the check list so that all the check are done in parallel.
type checkList []checkDescription

//the method is to add one check
func (cl checkList) add(ck checkDescription) {
	cl = append(cl, ck)
}

//the method is to make check list empty.
func (cl checkList) empty() {
	cl = cl[0:0]
}

//the method is to execute all the check in parallel.
func (cl checkList) check(oc *exutil.CLI) {
	var wg sync.WaitGroup
	for _, ck := range cl {
		wg.Add(1)
		go func(ck checkDescription) {
			defer g.GinkgoRecover()
			defer wg.Done()
			ck.check(oc)
		}(ck)
	}
	wg.Wait()
}

type resourceDescription struct {
	oc               *exutil.CLI
	asAdmin          bool
	withoutNamespace bool
	kind             string
	name             string
	requireNS        bool
	namespace        string
}

//the method is to construc one resource so that it can be deleted with itResource and describerResrouce
//oc is the oc client
//asAdmin means when deleting resource, we take admin role
//withoutNamespace means when deleting resource, we take WithoutNamespace
//kind is the kind of resource
//name is the name of resource
//namespace is the namesapce of resoruce. it is "" for cluster level resource
//if requireNS is requireNS, need to add "-n" parameter. used for project level resource
//if requireNS is notRequireNS, no need to add "-n". used for cluster level resource
func newResource(oc *exutil.CLI, kind string, name string, nsflag bool, namespace string) resourceDescription {
	return resourceDescription{
		oc:               oc,
		asAdmin:          asAdmin,
		withoutNamespace: withoutNamespace,
		kind:             kind,
		name:             name,
		requireNS:        nsflag,
		namespace:        namespace,
	}
}

//the method is to delete resource.
func (r resourceDescription) delete() {
	if r.withoutNamespace && r.requireNS {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name, "-n", r.namespace)
	} else {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name)
	}
}

//the struct to save the resource created in g.It, and it take name+kind+namespace as key to save resoruce of g.It.
type itResource map[string]resourceDescription

func (ir itResource) add(r resourceDescription) {
	ir[r.name+r.kind+r.namespace] = r
}
func (ir itResource) get(name string, kind string, namespace string) resourceDescription {
	r, ok := ir[name+kind+namespace]
	o.Expect(ok).To(o.BeTrue())
	return r
}
func (ir itResource) remove(name string, kind string, namespace string) {
	rKey := name + kind + namespace
	if r, ok := ir[rKey]; ok {
		r.delete()
		delete(ir, rKey)
	}
}
func (ir itResource) cleanup() {
	for _, r := range ir {
		e2e.Logf("cleanup resource %s,   %s", r.kind, r.name)
		ir.remove(r.name, r.kind, r.namespace)
	}
}

//the struct is to save g.It in g.Describe, and map the g.It name to itResource so that it can get all resource of g.Describe per g.It.
type describerResrouce map[string]itResource

func (dr describerResrouce) addIr(itName string) {
	dr[itName] = itResource{}
}
func (dr describerResrouce) getIr(itName string) itResource {
	ir, ok := dr[itName]
	if !ok {
		e2e.Logf("!!! couldn't find the itName:%s, print the describerResrouce:%v", itName, dr)
	}
	o.Expect(ok).To(o.BeTrue())
	return ir
}
func (dr describerResrouce) rmIr(itName string) {
	delete(dr, itName)
}

//the method is to convert to json format from one map sting got with -jsonpath
func convertLMtoJSON(content string) string {
	var jb strings.Builder
	jb.WriteString("[")
	items := strings.Split(strings.TrimSuffix(strings.TrimPrefix(content, "["), "]"), "map")
	for _, item := range items {
		if strings.Compare(item, "") == 0 {
			continue
		}
		kvs := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(item, "["), "]"))
		jb.WriteString("{")
		for ki, kv := range kvs {
			p := strings.Split(kv, ":")
			jb.WriteString("\"" + p[0] + "\":")
			jb.WriteString("\"" + p[1] + "\"")
			if ki < len(kvs)-1 {
				jb.WriteString(", ")
			}
		}
		jb.WriteString("},")
	}
	return strings.TrimSuffix(jb.String(), ",") + "]"
}

//the method is to get random string with length 8.
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

//the method is to update z version of kube version of platform.
func generateUpdatedKubernatesVersion(oc *exutil.CLI) string {
	subKubeVersions := strings.Split(getKubernetesVersion(oc), ".")
	zVersion, _ := strconv.Atoi(subKubeVersions[1])
	subKubeVersions[1] = strconv.Itoa(zVersion + 1)
	return strings.Join(subKubeVersions[0:2], ".") + ".0"
}

//the method is to get kube versoin of the platform.
func getKubernetesVersion(oc *exutil.CLI) string {
	output, err := doAction(oc, "version", asAdmin, withoutNamespace, "-o=json")
	o.Expect(err).NotTo(o.HaveOccurred())

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	o.Expect(err).NotTo(o.HaveOccurred())

	gitVersion := result["serverVersion"].(map[string]interface{})["gitVersion"]
	e2e.Logf("gitVersion is %v", gitVersion)
	return strings.TrimPrefix(gitVersion.(string), "v")
}

//the method is to create one resource with template
func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "olm-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

//the method is to check the presence of the resource
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//present means if you expect the resource presence or not. if it is ok, expect presence. if it is nok, expect not present.
func isPresentResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, present bool, parameters ...string) bool {
	parameters = append(parameters, "--ignore-not-found")
	err := wait.Poll(3*time.Second, 70*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		if !present && strings.Compare(output, "") == 0 {
			return true, nil
		}
		if present && strings.Compare(output, "") != 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}
	return true
}

//the method is to patch one resource
//asAdmin means if taking admin to patch it
//withoutNamespace means if take WithoutNamespace() to patch it.
func patchResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	_, err := doAction(oc, "patch", asAdmin, withoutNamespace, parameters...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to execute something in pod to get output
//asAdmin means if taking admin to execute it
//withoutNamespace means if take WithoutNamespace() to execute it.
func execResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	err := wait.Poll(3*time.Second, 6*time.Second, func() (bool, error) {
		output, err := doAction(oc, "exec", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the exec error is %v, and try next", err)
			return false, nil
		}
		result = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not exec %v", parameters))
	e2e.Logf("the result of exec resource:%v", result)
	return result
}

//the method is to get something from resource. it is "oc get xxx" actaully
//asAdmin means if taking admin to get it
//withoutNamespace means if take WithoutNamespace() to get it.
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not get %v", parameters))
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

func getResourceNoEmpty(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil || len(strings.TrimSpace(result)) == 0 {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not get %v without empty", parameters))
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

//the method is to check one resource's attribution is expected or not.
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//isCompare means if containing or exactly comparing. if it is contain, it check result contain content. if it is compare, it compare the result with content exactly.
//content is the substing to be expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
func expectedResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, isCompare bool, content string, expect bool, parameters ...string) error {
	expectMap := map[bool]string{
		true:  "do",
		false: "do not",
	}

	cc := func(a, b string, ic bool) bool {
		bs := strings.Split(b, "+2+")
		ret := false
		for _, s := range bs {
			if (ic && strings.Compare(a, s) == 0) || (!ic && strings.Contains(a, s)) {
				ret = true
			}
		}
		return ret
	}
	e2e.Logf("Running: oc get asAdmin(%t) withoutNamespace(%t) %s", asAdmin, withoutNamespace, strings.Join(parameters, " "))
	return wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("---> we %v expect value: %s, in returned value: %s", expectMap[expect], content, output)
		if isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s matches one of the content %s, expected", output, content)
			return true, nil
		}
		if isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not matche the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s contains one of the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not contain the content %s, expected", output, content)
			return true, nil
		}
		e2e.Logf("---> Not as expected! Return false")
		return false, nil
	})
}

//the method is to remove resource
//asAdmin means if taking admin to remove it
//withoutNamespace means if take WithoutNamespace() to remove it.
func removeResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	output, err := doAction(oc, "delete", asAdmin, withoutNamespace, parameters...)
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
			e2e.Logf("the resource is delete successfully")
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not remove %v", parameters))
}

//the method is to do something with oc.
//asAdmin means if taking admin to do it
//withoutNamespace means if take WithoutNamespace() to do it.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func clusterPackageExists(oc *exutil.CLI, sub subscriptionDescription) (bool, error) {
	found := false
	var v []string
	msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-o=jsonpath={range .items[*]}{@.metadata.name}{\",\"}{@.metadata.labels.catalog}{\"\\n\"}{end}").Output()
	if err == nil {
		for _, s := range strings.Fields(msg) {
			v = strings.Split(s, ",")
			if v[0] == sub.operatorPackage && v[1] == sub.catalogSourceName {
				found = true
				e2e.Logf("%v matches: %v", s, sub.operatorPackage)
				break
			}
		}
	}
	// add logging on failures
	if !found {
		e2e.Logf("%v was not found in \n%v", sub.operatorPackage, msg)
	}
	return found, err
}

func clusterPackageExistsInNamespace(oc *exutil.CLI, sub subscriptionDescription, namespace string) (bool, error) {
	found := false
	var v []string
	var msg string
	var err error
	if namespace == "all" {
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "-o=jsonpath={range .items[*]}{@.metadata.name}{\",\"}{@.metadata.labels.catalog}{\"\\n\"}{end}").Output()
	} else {
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-n", namespace, "-o=jsonpath={range .items[*]}{@.metadata.name}{\",\"}{@.metadata.labels.catalog}{\"\\n\"}{end}").Output()
	}
	if err == nil {
		for _, s := range strings.Fields(msg) {
			v = strings.Split(s, ",")
			if v[0] == sub.operatorPackage && v[1] == sub.catalogSourceName {
				found = true
				e2e.Logf("%v matches: %v", s, sub.operatorPackage)
				break
			}
		}
	}
	if !found {
		e2e.Logf("%v was not found in \n%v", sub.operatorPackage, msg)
	}
	return found, err
}

// Return a github client
func githubClient() (context.Context, *http.Client) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	return ctx, tc
}

// GetDirPath return a string of dir path
func GetDirPath(filePathStr string, filePre string) string {
	if !strings.Contains(filePathStr, "/") || filePathStr == "/" {
		return ""
	}
	dir, file := filepath.Split(filePathStr)
	if strings.HasPrefix(file, filePre) {
		return filePathStr
	}
	return GetDirPath(filepath.Dir(dir), filePre)
}

// DeleteDir delete the dir
func DeleteDir(filePathStr string, filePre string) bool {
	filePathToDelete := GetDirPath(filePathStr, filePre)
	if filePathToDelete == "" || !strings.Contains(filePathToDelete, filePre) {
		e2e.Logf("there is no such dir %s", filePre)
		return false
	}
	e2e.Logf("remove dir %s", filePathToDelete)
	os.RemoveAll(filePathToDelete)
	if _, err := os.Stat(filePathToDelete); err == nil {
		e2e.Logf("delele dir %s failed", filePathToDelete)
		return false
	}
	return true
}

// CheckUpgradeStatus check upgrade status
func CheckUpgradeStatus(oc *exutil.CLI, expectedStatus string) {
	e2e.Logf("Check the Upgradeable status of the OLM, expected: %s", expectedStatus)
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		upgradeable, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "operator-lifecycle-manager", "-o=jsonpath={.status.conditions[?(@.type==\"Upgradeable\")].status}").Output()
		if err != nil {
			e2e.Failf("Fail to get the Upgradeable status of the OLM: %v", err)
		}
		if upgradeable != expectedStatus {
			return false, nil
		}
		e2e.Logf("The Upgraableable status should be %s, and get %s", expectedStatus, upgradeable)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Upgradeable status of the OLM %s is not expected", expectedStatus))
}

// SkipARM64 skip the test if cluster is arm64
func SkipARM64(oc *exutil.CLI) {
	e2e.Logf("get architecture")
	version := exutil.GetClusterArchitecture(oc)
	if version == "arm64" {
		g.Skip("Skip for arm64")
	}
}

func getSAToken(oc *exutil.CLI, sa, ns string) (string, error) {
	e2e.Logf("Getting a token assgined to specific serviceaccount from %s namespace...", ns)
	token, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("token", sa, "-n", ns).Output()
	if err != nil {
		if strings.Contains(token, "unknown command") { // oc client is old version, create token is not supported
			e2e.Logf("oc create token is not supported by current client, use oc sa get-token instead")
			token, err = oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", sa, "-n", ns).Output()
		} else {
			return "", err
		}
	}
	return token, err
}
