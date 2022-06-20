package operators

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type Packagemanifest struct {
	Name                    string
	SupportsOwnNamespace    bool
	SupportsSingleNamespace bool
	SupportsAllNamespaces   bool
	CsvVersion              string
	Namespace               string
	DefaultChannel          string
	CatalogSource           string
	CatalogSourceNamespace  string
}

var SkippedOperators = []string{"quay-bridge-operator", "kubevirt-hyperconverged", "cost-mgmt-operator", "ptp-operator", "cass-operator", "openshift-sriov-network-operator",
	"armory-operator", "cluster-logging", "k8s-triliovault", "quay-bridge-operator", "windows-machine-config-operator", "local-storage-operator", "prisma-cloud-compute-console-operator.v2.0.1",
	"citrix-adc-istio-ingress-gateway-operator", "citrix-cpx-with-ingress-controller-operator", "kubemq-operator-marketplace"}

//The ISV operators may from the "community-operators", "certified-operators", "redhat-operators", and "redhat-marketplace" CatalogSources,
//For the specific details, you can refer to https://docs.google.com/spreadsheets/d/1Y3Y4Xv_r_PkwjE69iA9WlQ9ups98zYO1pprtQlii7wk/edit#gid=349207498
var ISVOperators = []string{"3scale-community-operator", "amq-streams",
	"argocd-operator", "cert-utils-operator", "couchbase-enterprise-certified",
	"federatorai-certified", "jaeger-product", "keycloak-operator", "kiali-ossm", "mongodb-enterprise", "must-gather-operator",
	"percona-server-mongodb-operator-certified", "percona-xtradb-cluster-operator-certified", "planetscale",
	"portworx-certified", "postgresql", "presto-operator", "prometheus", "radanalytics-spark",
	"resource-locker-operator", "spark-gcp", "storageos2", "strimzi-kafka-operator",
	"syndesis", "tidb-operator"}

var CaseIDISVOperators = map[string]string{
	"3scale-community-operator":                 "26931",
	"amq-streams":                               "23955",
	"argocd-operator":                           "27312",
	"cert-utils-operator":                       "26058",
	"couchbase-enterprise-certified":            "25414",
	"federatorai-certified":                     "25444",
	"jaeger-product":                            "26057",
	"keycloak-operator":                         "26945",
	"kiali-ossm":                                "27301",
	"mongodb-enterprise":                        "24064",
	"must-gather-operator":                      "28699",
	"percona-server-mongodb-operator-certified": "26052",
	"percona-xtradb-cluster-operator-certified": "26053",
	"planetscale":                               "25413",
	"portworx-certified":                        "25880",
	"postgresql":                                "27782",
	"presto-operator":                           "26947",
	"prometheus":                                "36889",
	"radanalytics-spark":                        "27313",
	"resource-locker-operator":                  "27311",
	"spark-gcp":                                 "26944",
	"storageos2":                                "25885",
	"strimzi-kafka-operator":                    "26056",
	"syndesis":                                  "26055",
	"tidb-operator":                             "25412",
}
var CatalogLabels = []string{"certified-operators", "redhat-operators", "community-operators"}
var BasicPrefix = "[Basic]"

const INSTALLPLAN_AUTOMATIC_MODE = "Automatic"
const INSTALLPLAN_MANUAL_MODE = "Manual"

// var _ = g.Describe("[Suite:openshift/isv] ISV_Operators", func() {

// 	var oc = exutil.NewCLI("isv", exutil.KubeConfigPath())
// 	defer g.GinkgoRecover()

// 	buildPruningBaseDir := exutil.FixturePath("testdata", "olm")
// 	subTemplate := filepath.Join(buildPruningBaseDir, "olm-subscription.yaml")
// 	ogSingleTemplate := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")

// 	for i := range ISVOperators {
// 		operator := ISVOperators[i]
// 		g.It(fmt.Sprintf("ConnectedOnly-Author:bandrade-Medium-%s-[Basic] Operator %s should work properly", CaseIDISVOperators[operator], operator), func() {
// 			g.By("1) Constructing the subscription")
// 			dr := make(describerResrouce)
// 			itName := g.CurrentGinkgoTestDescription().TestText
// 			dr.addIr(itName)

// 			subItems := constructSubscription(operator, oc, INSTALLPLAN_AUTOMATIC_MODE)
// 			// Note: don't create OperatorGroup for openshift-operators namespace
// 			if subItems.Namespace != "openshift-operators" {
// 				og := operatorGroupDescription{
// 					name:      fmt.Sprintf("og-%s", CaseIDISVOperators[operator]),
// 					namespace: subItems.Namespace,
// 					template:  ogSingleTemplate,
// 				}
// 				og.createwithCheck(oc, itName, dr)
// 			}
// 			// Create subscription
// 			sub := subscriptionDescription{
// 				subName:                fmt.Sprintf("sub-%s", CaseIDISVOperators[operator]),
// 				namespace:              subItems.Namespace,
// 				catalogSourceName:      subItems.CatalogSource,
// 				catalogSourceNamespace: subItems.CatalogSourceNamespace,
// 				channel:                subItems.DefaultChannel,
// 				ipApproval:             "Automatic",
// 				operatorPackage:        subItems.Name,
// 				startingCSV:            subItems.CsvVersion,
// 				singleNamespace:        subItems.SupportsSingleNamespace,
// 				template:               subTemplate,
// 			}
// 			defer sub.delete(itName, dr)
// 			g.By(fmt.Sprintf("2) Subscribe to %s", operator))
// 			e2e.Logf("--> The subscription:\n %v", sub)
// 			sub.create(oc, itName, dr)

// 			defer sub.deleteCSV(itName, dr)
// 			g.By(fmt.Sprintf("3) Check if %s works well", operator))
// 			newCheck("expect", asAdmin, withoutNamespace, compare, "Succeeded", ok, []string{"csv", sub.startingCSV, "-n", oc.Namespace(), "-o=jsonpath={.status.phase}"}).check(oc)
// 		})
// 	}
// })

func constructSubscription(operator string, oc *exutil.CLI, installPlanApprovalMode string) Packagemanifest {
	p := CreatePackageManifest(operator, oc)
	// Create Namespace
	oc.SetupProject()
	if p.SupportsSingleNamespace || p.SupportsOwnNamespace {
		p.Namespace = oc.Namespace()
	} else if p.SupportsAllNamespaces {
		p.Namespace = "openshift-operators"
	} else {
		g.Skip("Install Modes AllNamespaces and SingleNamespace are disabled for Operator: " + operator)
	}

	return p
}

func IsCertifiedOperator(operator string) bool {
	if contains(ISVOperators, operator) {
		return true
	}
	return false
}
func contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

//the method is to get the flag for each supportted model and return it with struct Packagemanifest
func checkOperatorInstallModes(p Packagemanifest, oc *exutil.CLI) Packagemanifest {
	supportsAllNamespaces, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", p.Name, "-o=jsonpath={.status.channels[?(.name=='"+p.DefaultChannel+"')].currentCSVDesc.installModes[?(.type=='AllNamespaces')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsAllNamespacesAsBool, _ := strconv.ParseBool(supportsAllNamespaces)

	supportsSingleNamespace, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", p.Name, "-o=jsonpath={.status.channels[?(.name=='"+p.DefaultChannel+"')].currentCSVDesc.installModes[?(.type=='SingleNamespace')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsSingleNamespaceAsBool, _ := strconv.ParseBool(supportsSingleNamespace)

	supportsOwnNamespace, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", p.Name, "-o=jsonpath={.status.channels[?(.name=='"+p.DefaultChannel+"')].currentCSVDesc.installModes[?(.type=='OwnNamespace')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsOwnNamespaceAsBool, _ := strconv.ParseBool(supportsOwnNamespace)

	p.SupportsAllNamespaces = supportsAllNamespacesAsBool
	p.SupportsSingleNamespace = supportsSingleNamespaceAsBool
	p.SupportsOwnNamespace = supportsOwnNamespaceAsBool
	return p
}

//the method is to construct the Packagemanifest object with operator name.
func CreatePackageManifest(operator string, oc *exutil.CLI) Packagemanifest {
	msg, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", operator, "-o=jsonpath={.status.catalogSource}:{.status.catalogSourceNamespace}:{.status.defaultChannel}", "-n", "openshift-marketplace").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	packageData := strings.Split(msg, ":")
	p := Packagemanifest{CatalogSource: packageData[0], CatalogSourceNamespace: packageData[1], DefaultChannel: packageData[2], Name: operator}

	csvVersion, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", p.Name, "-o=jsonpath={.status.channels[?(.name=='"+p.DefaultChannel+"')].currentCSV}", "-n", "openshift-marketplace").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	p.CsvVersion = strings.ReplaceAll(csvVersion, "\"", "")

	p = checkOperatorInstallModes(p, oc)
	return p
}

//the method is to create sub, but do not check the sub status and if csv is installed.
//if the operator support singlenamespace or ownnamespace, it will take priority to create ns with test-operators- prefix
//and then create og in that ns.
//if the operator does not support both singlenamespace and ownnamespace, it will create it with allnamepsace if it is supported.
//it is created in ns openshift-operators
//or else, it is skipped to create sub.
func CreateSubscription(operator string, oc *exutil.CLI, installPlanApprovalMode string) Packagemanifest {
	p := CreatePackageManifest(operator, oc)
	if p.SupportsSingleNamespace || p.SupportsOwnNamespace {
		p = CreateNamespace(p, oc)
		CreateOperatorGroup(p, oc)
	} else if p.SupportsAllNamespaces {
		p.Namespace = "openshift-operators"

	} else {
		g.Skip("Install Modes AllNamespaces and SingleNamespace are disabled for Operator: " + operator)
	}

	templateSubscriptionYAML := writeSubscription(p, installPlanApprovalMode)
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", templateSubscriptionYAML, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return p
}

//the method is to create sub with singlenamespace or ownnamespace in ns namespace parameter
//it will create ns or og depending on namespaceCreate or operatorGroupCreate
func CreateSubscriptionSpecificNamespace(operator string, oc *exutil.CLI, namespaceCreate bool, operatorGroupCreate bool, namespace string, installPlanApprovalMode string) Packagemanifest {
	p := CreatePackageManifest(operator, oc)
	p.Namespace = namespace
	if namespaceCreate {
		CreateNamespace(p, oc)
	}
	if operatorGroupCreate {
		CreateOperatorGroup(p, oc)
	}
	templateSubscriptionYAML := writeSubscription(p, installPlanApprovalMode)
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", templateSubscriptionYAML, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return p
}

//the method is to create ns without prefix
func CreateNamespaceWithoutPrefix(namespace string, oc *exutil.CLI) {
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("ns", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to create ns with prefix test-operators-
func CreateNamespace(p Packagemanifest, oc *exutil.CLI) Packagemanifest {
	if p.Namespace == "" {
		p.Namespace = names.SimpleNameGenerator.GenerateName("test-operators-")
	}
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("ns", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return p
}

//the method is to delete ns with namespace parameter if it exists
func RemoveNamespace(namespace string, oc *exutil.CLI) {
	_, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("ns", namespace).Output()

	if err == nil {
		_, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//the method is to create og with name test-operators in ns identified by p.
func CreateOperatorGroup(p Packagemanifest, oc *exutil.CLI) {

	templateOperatorGroupYAML := writeOperatorGroup(p.Namespace)
	_, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("-f", templateOperatorGroupYAML, "-n", p.Namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to generate operator_group_<namespace>_.yaml used to create og.
func writeOperatorGroup(namespace string) (templateOperatorYAML string) {
	operatorBaseDir := exutil.FixturePath("testdata", "operators")
	operatorGroupYAML := filepath.Join(operatorBaseDir, "operator_group.yaml")
	fileOperatorGroup, _ := os.Open(operatorGroupYAML)
	operatorGroup, _ := ioutil.ReadAll(fileOperatorGroup)
	operatorGroupTemplate := string(operatorGroup)
	templateOperatorYAML = strings.ReplaceAll(operatorGroupYAML, "operator_group.yaml", "operator_group_"+namespace+"_.yaml")
	operatorGroupString := strings.ReplaceAll(operatorGroupTemplate, "$OPERATOR_NAMESPACE", namespace)
	ioutil.WriteFile(templateOperatorYAML, []byte(operatorGroupString), 0644)
	return
}

//the method is to generate subscription_<CsvVersion>_.yaml used to create sub.
func writeSubscription(p Packagemanifest, installPlanApprovalMode string) (templateSubscriptionYAML string) {
	operatorBaseDir := exutil.FixturePath("testdata", "operators")
	subscriptionYAML := filepath.Join(operatorBaseDir, "subscription.yaml")
	fileSubscription, _ := os.Open(subscriptionYAML)
	subscription, _ := ioutil.ReadAll(fileSubscription)
	subscriptionTemplate := string(subscription)

	templateSubscriptionYAML = strings.ReplaceAll(subscriptionYAML, "subscription.yaml", "subscription_"+p.CsvVersion+"_.yaml")
	operatorSubscription := strings.ReplaceAll(subscriptionTemplate, "$OPERATOR_PACKAGE_NAME", p.Name)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CHANNEL", "\""+p.DefaultChannel+"\"")
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_NAMESPACE", p.Namespace)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_SOURCE", p.CatalogSource)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CATALOG_NAMESPACE", p.CatalogSourceNamespace)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CURRENT_CSV_VERSION", p.CsvVersion)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_INSTALLPLAN_APPROVAL", installPlanApprovalMode)
	ioutil.WriteFile(templateSubscriptionYAML, []byte(operatorSubscription), 0644)
	e2e.Logf("Subscription: %s", operatorSubscription)
	return
}

//the method is to check if the csv is installed successfully
//if ok, nothing happen
//if nok, it will delete sub, csv. and it also delete ns if it is singlenamespace or ownnamespace.
func CheckDeployment(p Packagemanifest, oc *exutil.CLI) {
	poolErr := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		msg, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("csv", p.CsvVersion, "-o=jsonpath={.status.phase}", "-n", p.Namespace).Output()
		if strings.Contains(msg, "Succeeded") {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		RemoveOperatorDependencies(p, oc, false)
		g.Fail("Could not obtain CSV:" + p.CsvVersion)
	}
}

//the method is to delete all related resource of operator: sub, csv, ns.
//if checkDeletion is true, it will check the result of deletion.
//if checkDeletion is false, it will not check the result of deletion.
func RemoveOperatorDependencies(p Packagemanifest, oc *exutil.CLI, checkDeletion bool) {
	ip, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("sub", p.Name, "-o=jsonpath={.status.installplan.name}", "-n", p.Namespace).Output()
	e2e.Logf("IP: %s", ip)
	if len(strings.TrimSpace(ip)) > 0 {
		msg, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("installplan", ip, "-o=jsonpath={.spec.clusterServiceVersionNames}", "-n", p.Namespace).Output()
		msg = strings.ReplaceAll(msg, "[", "")
		msg = strings.ReplaceAll(msg, "]", "")
		e2e.Logf("CSVS: %s", msg)
		csvs := strings.Split(msg, " ")
		for i := range csvs {
			e2e.Logf("CSV_: %s", csvs[i])
			msg, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args("csv", strings.ReplaceAll(csvs[i], "\"", ""), "-n", p.Namespace).Output()
			if checkDeletion {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(msg).To(o.ContainSubstring("deleted"))
			}
		}

		subsOutput, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("subs", "-o=jsonpath={range .items[?(.status.installplan.name=='"+ip+"')].metadata}{.name}{' '}", "-n", p.Namespace).Output()
		e2e.Logf("SUBS OUTPUT: %s", subsOutput)
		if len(strings.TrimSpace(subsOutput)) > 0 {
			subs := strings.Split(subsOutput, " ")
			e2e.Logf("SUBS: %s", subs)
			for i := range subs {
				e2e.Logf("SUB_: %s", subs[i])
				msg, err := oc.WithoutNamespace().AsAdmin().Run("delete").Args("subs", subs[i], "-n", p.Namespace).Output()
				if checkDeletion {
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(msg).To(o.ContainSubstring("deleted"))
				}
			}
		}
	}
	if p.SupportsSingleNamespace || p.SupportsOwnNamespace {
		RemoveNamespace(p.Namespace, oc)
	}

}

func itemExists(arrayType interface{}, item interface{}) bool {
	arr := reflect.ValueOf(arrayType)

	if arr.Kind() != reflect.Array {
		panic("Invalid data-type")
	}

	for i := 0; i < arr.Len(); i++ {
		if arr.Index(i).Interface() == item {
			return true
		}
	}

	return false
}
