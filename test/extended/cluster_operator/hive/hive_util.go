package hive

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type hiveNameSpace struct {
	name     string
	template string
}

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
	currentCSV      string
	installedCSV    string
	template        string
}

type hiveconfig struct {
	logLevel        string
	targetNamespace string
	template        string
}

type clusterImageSet struct {
	name         string
	releaseImage string
	template     string
}

type clusterPool struct {
	name           string
	namespace      string
	fake           string
	baseDomain     string
	imageSetRef    string
	platformType   string
	credRef        string
	region         string
	pullSecretRef  string
	size           int
	maxSize        int
	runningCount   int
	maxConcurrent  int
	hibernateAfter string
	template       string
}

type clusterClaim struct {
	name            string
	namespace       string
	clusterPoolName string
	template        string
}

type installConfig struct {
	name1      string
	namespace  string
	baseDomain string
	name2      string
	region     string
	template   string
}

type clusterDeployment struct {
	fake                 string
	name                 string
	namespace            string
	baseDomain           string
	clusterName          string
	platformType         string
	credRef              string
	region               string
	imageSetRef          string
	installConfigSecret  string
	pullSecretRef        string
	installAttemptsLimit int
	template             string
}

type machinepool struct {
	clusterName string
	namespace   string
	template    string
}

type syncSetResource struct {
	name          string
	namespace     string
	namespace2    string
	cdrefname     string
	ramode        string
	applybehavior string
	cmname        string
	cmnamespace   string
	template      string
}

type syncSetPatch struct {
	name        string
	namespace   string
	cdrefname   string
	cmname      string
	cmnamespace string
	pcontent    string
	patchType   string
	template    string
}

type syncSetSecret struct {
	name       string
	namespace  string
	cdrefname  string
	sname      string
	snamespace string
	tname      string
	tnamespace string
	template   string
}

type objectTableRef struct {
	kind      string
	namespace string
	name      string
}

//Azure
type azureInstallConfig struct {
	name1      string
	namespace  string
	baseDomain string
	name2      string
	resGroup   string
	azureType  string
	region     string
	template   string
}

type azureClusterDeployment struct {
	fake                string
	name                string
	namespace           string
	baseDomain          string
	clusterName         string
	platformType        string
	credRef             string
	region              string
	resGroup            string
	azureType           string
	imageSetRef         string
	installConfigSecret string
	pullSecretRef       string
	template            string
}

type azureClusterPool struct {
	name           string
	namespace      string
	fake           string
	baseDomain     string
	imageSetRef    string
	platformType   string
	credRef        string
	region         string
	resGroup       string
	pullSecretRef  string
	size           int
	maxSize        int
	runningCount   int
	maxConcurrent  int
	hibernateAfter string
	template       string
}

//GCP
type gcpInstallConfig struct {
	name1      string
	namespace  string
	baseDomain string
	name2      string
	region     string
	projectid  string
	template   string
}

type gcpClusterDeployment struct {
	fake                string
	name                string
	namespace           string
	baseDomain          string
	clusterName         string
	platformType        string
	credRef             string
	region              string
	imageSetRef         string
	installConfigSecret string
	pullSecretRef       string
	template            string
}

type gcpClusterPool struct {
	name           string
	namespace      string
	fake           string
	baseDomain     string
	imageSetRef    string
	platformType   string
	credRef        string
	region         string
	pullSecretRef  string
	size           int
	maxSize        int
	runningCount   int
	maxConcurrent  int
	hibernateAfter string
	template       string
}

//Hive Configurations
const (
	HiveNamespace           = "hive" //Hive Namespace
	OCP49ReleaseImage       = "quay.io/openshift-release-dev/ocp-release:4.9.0-rc.6-x86_64"
	OCP410ReleaseImage      = "quay.io/openshift-release-dev/ocp-release:4.10.14-x86_64"
	PullSecret              = "pull-secret"
	ClusterInstallTimeout   = 3600
	DefaultTimeout          = 120
	ClusterResumeTimeout    = 600
	ClusterUninstallTimeout = 1800
)

//AWS Configurations
const (
	AWSBaseDomain = "qe.devcluster.openshift.com" //AWS BaseDomain
	AWSRegion     = "us-east-2"
	AWSCreds      = "aws-creds"
)

//Azure Configurations
const (
	AzureBaseDomain = "qe.azure.devcluster.openshift.com" //Azure BaseDomain
	AzureRegion     = "centralus"
	AzureCreds      = "azure-credentials"
	AzureRESGroup   = "os4-common"
	AzurePublic     = "AzurePublicCloud"
)

//GCP Configurations
const (
	GCPBaseDomain = "qe.gcp.devcluster.openshift.com" //GCP BaseDomain
	GCPRegion     = "us-central1"
	GCPCreds      = "gcp-credentials"
)

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var cfgFileJSON string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "-hive-resource-cfg.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		cfgFileJSON = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, "fail to create config file")

	e2e.Logf("the file of resource is %s", cfgFileJSON)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", cfgFileJSON).Execute()
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

//Create hive namespace if not exist
func (ns *hiveNameSpace) createIfNotExist(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ns.template, "-p", "NAME="+ns.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Create operatorGroup for Hive if not exist
func (og *operatorGroup) createIfNotExist(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (sub *subscription) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
		"APPROVAL="+sub.approval, "OPERATORNAME="+sub.operatorName, "SOURCENAME="+sub.sourceName, "SOURCENAMESPACE="+sub.sourceNamespace, "STARTINGCSV="+sub.startingCSV)
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Compare(sub.approval, "Automatic") == 0 {
		sub.findInstalledCSV(oc)
	} else {
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "UpgradePending", ok, DefaultTimeout, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	}
}

//Create subscription for Hive if not exist and wait for resource is ready
func (sub *subscription) createIfNotExist(oc *exutil.CLI) {

	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "-n", sub.namespace).Output()
	if strings.Contains(output, "NotFound") || strings.Contains(output, "No resources") || err != nil {
		e2e.Logf("No hive subscription, Create it.")
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace, "CHANNEL="+sub.channel,
			"APPROVAL="+sub.approval, "OPERATORNAME="+sub.operatorName, "SOURCENAME="+sub.sourceName, "SOURCENAMESPACE="+sub.sourceNamespace, "STARTINGCSV="+sub.startingCSV)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(sub.approval, "Automatic") == 0 {
			sub.findInstalledCSV(oc)
		} else {
			newCheck("expect", "get", asAdmin, withoutNamespace, compare, "UpgradePending", ok, DefaultTimeout, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
		}
		//wait for pod running
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=hive-operator", "-n",
			sub.namespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
	} else {
		e2e.Logf("hive subscription already exists.")
	}

}

func (sub *subscription) findInstalledCSV(oc *exutil.CLI) {
	newCheck("expect", "get", asAdmin, withoutNamespace, compare, "AtLatestKnown", ok, DefaultTimeout, []string{"sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.state}"}).check(oc)
	installedCSV := getResource(oc, asAdmin, withoutNamespace, "sub", sub.name, "-n", sub.namespace, "-o=jsonpath={.status.installedCSV}")
	o.Expect(installedCSV).NotTo(o.BeEmpty())
	if strings.Compare(sub.installedCSV, installedCSV) != 0 {
		sub.installedCSV = installedCSV
	}
	e2e.Logf("the installed CSV name is %s", sub.installedCSV)
}

func (hc *hiveconfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", hc.template, "-p", "LOGLEVEL="+hc.logLevel, "TARGETNAMESPACE="+hc.targetNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Create hivconfig if not exist and wait for resource is ready
func (hc *hiveconfig) createIfNotExist(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HiveConfig", "hive").Output()
	if strings.Contains(output, "have a resource type") || err != nil {
		e2e.Logf("No hivconfig, Create it.")
		err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", hc.template, "-p", "LOGLEVEL="+hc.logLevel, "TARGETNAMESPACE="+hc.targetNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		//wait for pods running
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hive-clustersync", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=clustersync",
			"-n", HiveNamespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=clustersync", "-n",
			HiveNamespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hive-controllers", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=controller-manager",
			"-n", HiveNamespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running", ok, DefaultTimeout, []string{"pod", "--selector=control-plane=controller-manager", "-n",
			HiveNamespace, "-o=jsonpath={.items[0].status.phase}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, contain, "hiveadmission", ok, DefaultTimeout, []string{"pod", "--selector=app=hiveadmission",
			"-n", HiveNamespace, "-o=jsonpath={.items[*].metadata.name}"}).check(oc)
		newCheck("expect", "get", asAdmin, withoutNamespace, compare, "Running Running", ok, DefaultTimeout, []string{"pod", "--selector=app=hiveadmission", "-n",
			HiveNamespace, "-o=jsonpath={.items[*].status.phase}"}).check(oc)
	} else {
		e2e.Logf("hivconfig already exists.")
	}

}

func (imageset *clusterImageSet) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", imageset.template, "-p", "NAME="+imageset.name, "RELEASEIMAGE="+imageset.releaseImage)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pool *clusterPool) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pool.template, "-p", "NAME="+pool.name, "NAMESPACE="+pool.namespace, "FAKE="+pool.fake, "BASEDOMAIN="+pool.baseDomain, "IMAGESETREF="+pool.imageSetRef, "PLATFORMTYPE="+pool.platformType, "CREDREF="+pool.credRef, "REGION="+pool.region, "PULLSECRETREF="+pool.pullSecretRef, "SIZE="+strconv.Itoa(pool.size), "MAXSIZE="+strconv.Itoa(pool.maxSize), "RUNNINGCOUNT="+strconv.Itoa(pool.runningCount), "MAXCONCURRENT="+strconv.Itoa(pool.maxConcurrent), "HIBERNATEAFTER="+pool.hibernateAfter)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (claim *clusterClaim) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", claim.template, "-p", "NAME="+claim.name, "NAMESPACE="+claim.namespace, "CLUSTERPOOLNAME="+claim.clusterPoolName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (config *installConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", config.template, "-p", "NAME1="+config.name1, "NAMESPACE="+config.namespace, "BASEDOMAIN="+config.baseDomain, "NAME2="+config.name2, "REGION="+config.region)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (cluster *clusterDeployment) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cluster.template, "-p", "FAKE="+cluster.fake, "NAME="+cluster.name, "NAMESPACE="+cluster.namespace, "BASEDOMAIN="+cluster.baseDomain, "CLUSTERNAME="+cluster.clusterName, "PLATFORMTYPE="+cluster.platformType, "CREDREF="+cluster.credRef, "REGION="+cluster.region, "IMAGESETREF="+cluster.imageSetRef, "INSTALLCONFIGSECRET="+cluster.installConfigSecret, "PULLSECRETREF="+cluster.pullSecretRef, "INSTALLATTEMPTSLIMIT="+strconv.Itoa(cluster.installAttemptsLimit))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (machine *machinepool) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", machine.template, "-p", "CLUSTERNAME="+machine.clusterName, "NAMESPACE="+machine.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (syncresource *syncSetResource) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", syncresource.template, "-p", "NAME="+syncresource.name, "NAMESPACE="+syncresource.namespace, "CDREFNAME="+syncresource.cdrefname, "NAMESPACE2="+syncresource.namespace2, "RAMODE="+syncresource.ramode, "APPLYBEHAVIOR="+syncresource.applybehavior, "CMNAME="+syncresource.cmname, "CMNAMESPACE="+syncresource.cmnamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (syncpatch *syncSetPatch) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", syncpatch.template, "-p", "NAME="+syncpatch.name, "NAMESPACE="+syncpatch.namespace, "CDREFNAME="+syncpatch.cdrefname, "CMNAME="+syncpatch.cmname, "CMNAMESPACE="+syncpatch.cmnamespace, "PCONTENT="+syncpatch.pcontent, "PATCHTYPE="+syncpatch.patchType)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (syncsecret *syncSetSecret) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", syncsecret.template, "-p", "NAME="+syncsecret.name, "NAMESPACE="+syncsecret.namespace, "CDREFNAME="+syncsecret.cdrefname, "SNAME="+syncsecret.sname, "SNAMESPACE="+syncsecret.snamespace, "TNAME="+syncsecret.tname, "TNAMESPACE="+syncsecret.tnamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Azure
func (config *azureInstallConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", config.template, "-p", "NAME1="+config.name1, "NAMESPACE="+config.namespace, "BASEDOMAIN="+config.baseDomain, "NAME2="+config.name2, "RESGROUP="+config.resGroup, "AZURETYPE="+config.azureType, "REGION="+config.region)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (cluster *azureClusterDeployment) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cluster.template, "-p", "FAKE="+cluster.fake, "NAME="+cluster.name, "NAMESPACE="+cluster.namespace, "BASEDOMAIN="+cluster.baseDomain, "CLUSTERNAME="+cluster.clusterName, "PLATFORMTYPE="+cluster.platformType, "CREDREF="+cluster.credRef, "REGION="+cluster.region, "RESGROUP="+cluster.resGroup, "AZURETYPE="+cluster.azureType, "IMAGESETREF="+cluster.imageSetRef, "INSTALLCONFIGSECRET="+cluster.installConfigSecret, "PULLSECRETREF="+cluster.pullSecretRef)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pool *azureClusterPool) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pool.template, "-p", "NAME="+pool.name, "NAMESPACE="+pool.namespace, "FAKE="+pool.fake, "BASEDOMAIN="+pool.baseDomain, "IMAGESETREF="+pool.imageSetRef, "PLATFORMTYPE="+pool.platformType, "CREDREF="+pool.credRef, "REGION="+pool.region, "RESGROUP="+pool.resGroup, "PULLSECRETREF="+pool.pullSecretRef, "SIZE="+strconv.Itoa(pool.size), "MAXSIZE="+strconv.Itoa(pool.maxSize), "RUNNINGCOUNT="+strconv.Itoa(pool.runningCount), "MAXCONCURRENT="+strconv.Itoa(pool.maxConcurrent), "HIBERNATEAFTER="+pool.hibernateAfter)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//GCP
func (config *gcpInstallConfig) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", config.template, "-p", "NAME1="+config.name1, "NAMESPACE="+config.namespace, "BASEDOMAIN="+config.baseDomain, "NAME2="+config.name2, "REGION="+config.region, "PROJECTID="+config.projectid)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (cluster *gcpClusterDeployment) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cluster.template, "-p", "FAKE="+cluster.fake, "NAME="+cluster.name, "NAMESPACE="+cluster.namespace, "BASEDOMAIN="+cluster.baseDomain, "CLUSTERNAME="+cluster.clusterName, "PLATFORMTYPE="+cluster.platformType, "CREDREF="+cluster.credRef, "REGION="+cluster.region, "IMAGESETREF="+cluster.imageSetRef, "INSTALLCONFIGSECRET="+cluster.installConfigSecret, "PULLSECRETREF="+cluster.pullSecretRef)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pool *gcpClusterPool) create(oc *exutil.CLI) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pool.template, "-p", "NAME="+pool.name, "NAMESPACE="+pool.namespace, "FAKE="+pool.fake, "BASEDOMAIN="+pool.baseDomain, "IMAGESETREF="+pool.imageSetRef, "PLATFORMTYPE="+pool.platformType, "CREDREF="+pool.credRef, "REGION="+pool.region, "PULLSECRETREF="+pool.pullSecretRef, "SIZE="+strconv.Itoa(pool.size), "MAXSIZE="+strconv.Itoa(pool.maxSize), "RUNNINGCOUNT="+strconv.Itoa(pool.runningCount), "MAXCONCURRENT="+strconv.Itoa(pool.maxConcurrent), "HIBERNATEAFTER="+pool.hibernateAfter)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	err := wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		result = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("cat not get %v without empty", parameters))
	e2e.Logf("the result of queried resource:%v", result)
	return result
}

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

//Check the resource meets the expect
//parameter method: expect or present
//parameter action: get, patch, delete, ...
//parameter executor: asAdmin or not
//parameter inlineNamespace: withoutNamespace or not
//parameter expectAction: Compare or not
//parameter expectContent: expected string
//parameter expect: ok, expected to have expectContent; nok, not expected to have expectContent
//parameter timeout: use CLUSTER_INSTALL_TIMEOUT de default, and CLUSTER_INSTALL_TIMEOUT, CLUSTER_RESUME_TIMEOUT etc in different scenarios
//parameter resource: resource
func newCheck(method string, action string, executor bool, inlineNamespace bool, expectAction bool,
	expectContent string, expect bool, timeout int, resource []string) checkDescription {
	return checkDescription{
		method:          method,
		action:          action,
		executor:        executor,
		inlineNamespace: inlineNamespace,
		expectAction:    expectAction,
		expectContent:   expectContent,
		expect:          expect,
		timeout:         timeout,
		resource:        resource,
	}
}

type checkDescription struct {
	method          string
	action          string
	executor        bool
	inlineNamespace bool
	expectAction    bool
	expectContent   string
	expect          bool
	timeout         int
	resource        []string
}

const (
	asAdmin          = true
	withoutNamespace = true
	requireNS        = true
	compare          = true
	contain          = false
	present          = true
	notPresent       = false
	ok               = true
	nok              = false
)

func (ck checkDescription) check(oc *exutil.CLI) {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.action, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		o.Expect(ok).To(o.BeTrue())
	case "expect":
		err := expectedResource(oc, ck.action, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.timeout, ck.resource...)
		exutil.AssertWaitPollNoErr(err, "can not get expected result")
	default:
		err := fmt.Errorf("unknown method")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func isPresentResource(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, present bool, parameters ...string) bool {
	parameters = append(parameters, "--ignore-not-found")
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		output, err := doAction(oc, action, asAdmin, withoutNamespace, parameters...)
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
	return err == nil
}

func expectedResource(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, isCompare bool, content string, expect bool, timeout int, parameters ...string) error {
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
	var interval, inputTimeout time.Duration
	if timeout >= ClusterInstallTimeout {
		inputTimeout = time.Duration(timeout/60) * time.Minute
		interval = 6 * time.Minute
	} else {
		inputTimeout = time.Duration(timeout) * time.Second
		interval = time.Duration(timeout/60) * time.Second
	}
	return wait.Poll(interval, inputTimeout, func() (bool, error) {
		output, err := doAction(oc, action, asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("the queried resource:%s", output)
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
		return false, nil
	})
}

//clean up the object resource
func cleanupObjects(oc *exutil.CLI, objs ...objectTableRef) {
	for _, v := range objs {
		e2e.Logf("Start to remove: %v", v)
		if v.namespace != "" {
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, "-n", v.namespace, v.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

		} else {
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, v.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		//For ClusterPool or ClusterDeployment, need to wait ClusterDeployment delete done
		if v.kind == "ClusterPool" || v.kind == "ClusterDeployment" {
			e2e.Logf("Wait ClusterDeployment delete done for %s", v.name)
			newCheck("expect", "get", asAdmin, withoutNamespace, contain, v.name, nok, ClusterUninstallTimeout, []string{"ClusterDeployment", "-A"}).check(oc)
		}
	}
}

func removeResource(oc *exutil.CLI, parameters ...string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args(parameters...).Output()
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("No resource found!")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (hc *hiveconfig) delete(oc *exutil.CLI) {
	removeResource(oc, "hiveconfig", "hive")
}

//Create pull-secret in current project namespace
func createPullSecret(oc *exutil.CLI, namespace string) {
	dirname := "/tmp/" + oc.Namespace() + "-pull"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)

	err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--to="+dirname, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("create").Args("secret", "generic", "pull-secret", "--from-file="+dirname+"/.dockerconfigjson", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Create AWS credentials in current project namespace
func createAWSCreds(oc *exutil.CLI, namespace string) {
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)

	err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/aws-creds", "-n", "kube-system", "--to="+dirname, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("create").Args("secret", "generic", "aws-creds", "--from-file="+dirname+"/aws_access_key_id", "--from-file="+dirname+"/aws_secret_access_key", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Create Azure credentials in current project namespace
func createAzureCreds(oc *exutil.CLI, namespace string) {
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)

	var azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID string
	azureClientID, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "--template='{{.data.azure_client_id | base64decode}}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	azureClientSecret, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "--template='{{.data.azure_client_secret | base64decode}}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	azureSubscriptionID, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "--template='{{.data.azure_subscription_id | base64decode}}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	azureTenantID, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "--template='{{.data.azure_tenant_id | base64decode}}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	//Convert credentials to osServicePrincipal.json format
	output := fmt.Sprintf("{\"subscriptionId\":\"%s\",\"clientId\":\"%s\",\"clientSecret\":\"%s\",\"tenantId\":\"%s\"}", azureSubscriptionID[1:len(azureSubscriptionID)-1], azureClientID[1:len(azureClientID)-1], azureClientSecret[1:len(azureClientSecret)-1], azureTenantID[1:len(azureTenantID)-1])
	outputFile, outputErr := os.OpenFile(dirname+"/osServicePrincipal.json", os.O_CREATE|os.O_WRONLY, 0666)
	o.Expect(outputErr).NotTo(o.HaveOccurred())
	defer outputFile.Close()
	outputWriter := bufio.NewWriter(outputFile)
	writeByte, writeError := outputWriter.WriteString(output)
	o.Expect(writeError).NotTo(o.HaveOccurred())
	writeError = outputWriter.Flush()
	o.Expect(writeError).NotTo(o.HaveOccurred())
	e2e.Logf("%d byte written to osServicePrincipal.json", writeByte)
	err = oc.Run("create").Args("secret", "generic", AzureCreds, "--from-file="+dirname+"/osServicePrincipal.json", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Create GCP credentials in current project namespace
func createGCPCreds(oc *exutil.CLI, namespace string) {
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)

	err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/gcp-credentials", "-n", "kube-system", "--to="+dirname, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("create").Args("secret", "generic", GCPCreds, "--from-file=osServiceAccount.json="+dirname+"/service_account.json", "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Reutrn Rlease version from Image
func extractRelfromImg(image string) string {
	index := strings.Index(image, ":")
	if index != -1 {
		tempStr := image[index+1:]
		index = strings.Index(tempStr, "-")
		if index != -1 {
			e2e.Logf("Extracted OCP release: %s", tempStr[:index])
			return tempStr[:index]
		}
	}
	e2e.Logf("Failed to extract OCP release from Image.")
	return ""
}

//Get CD list from Pool
//Return string CD list such as "pool-44945-2bbln5m47s\n pool-44945-f8xlv6m6s"
func getCDlistfromPool(oc *exutil.CLI, pool string) string {
	fileName := "cd_output_" + getRandomString() + ".txt"
	cdOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cd", "-A").OutputToFile(fileName)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.Remove(cdOutput)
	poolCdList, err := exec.Command("bash", "-c", "cat "+cdOutput+" | grep "+pool+" | awk '{print $1}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("CD list is %s for pool %s", poolCdList, pool)
	return string(poolCdList)
}

//Get cluster kubeconfig file
func getClusterKubeconfig(oc *exutil.CLI, clustername, namespace, dir string) {
	kubeconfigsecretname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cd", clustername, "-n", namespace, "-o=jsonpath={.spec.clusterMetadata.adminKubeconfigSecretRef.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Extract cluster %s kubeconfig to %s", clustername, dir)
	err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/"+kubeconfigsecretname, "-n", namespace, "--to="+dir, "--confirm").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//Check resource number after filtering
func checkResourceNumber(oc *exutil.CLI, resourceType string, filterName string) int {
	resourceOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(resourceType, "-A").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Count(resourceOutput, filterName)
}
