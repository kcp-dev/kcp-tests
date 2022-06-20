package storage

import (
	g "github.com/onsi/ginkgo"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage] STORAGE", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("storage-storageclass", exutil.KubeConfigPath())

	// author: wduan@redhat.com
	// OCP-22019-The cluster-storage-operator should manage pre-defined storage class
	g.It("Author:wduan-High-22019-The cluster-storage-operator should manage pre-defined storage class [Disruptive]", func() {

		// Get pre-defined storageclass and default storageclass from testdata/storage/pre-defined-storageclass.json
		g.By("Get pre-defined storage class and default storage class")
		cloudProvider = getCloudProvider(oc)
		preDefinedStorageclassCheck(cloudProvider)
		defaultsc := getClusterDefaultStorageclassByPlatform(cloudProvider)

		preDefinedStorageclassList := getClusterPreDefinedStorageclassByPlatform(cloudProvider)
		e2e.Logf("The pre-defined storageclass list is: %v", preDefinedStorageclassList)

		// Check the default storageclass is expected, otherwise skip
		checkStorageclassExists(oc, defaultsc)
		if !checkDefaultStorageclass(oc, defaultsc) {
			g.Skip("Skip for unexpected default storageclass! The *" + defaultsc + "* is the expected default storageclass for test.")
		}

		// Delete all storageclass and check
		for _, sc := range preDefinedStorageclassList {
			checkStorageclassExists(oc, sc)
			e2e.Logf("Delete pre-defined storageclass %s ...", sc)
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("sc", sc).Execute()
			checkStorageclassExists(oc, sc)
			e2e.Logf("Check pre-defined storageclass %s restored.", sc)
		}
		if !checkDefaultStorageclass(oc, defaultsc) {
			g.Fail("Failed due to the previous default storageclass is not restored!")
		}
	})
})
