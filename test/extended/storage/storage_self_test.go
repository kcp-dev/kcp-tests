package storage

import (
	"testing"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"github.com/stretchr/testify/assert"
)

// Test generate csi extra parameters funtions
func TestGererateCsiScExtraParametersAndValidRandomCapacityByVolType(t *testing.T) {
	var (
		oc = exutil.NewCLI("storage-self-test", exutil.KubeConfigPath())
		as = assert.New(t)
	)

	csiVolumeTypes := []string{"io1", "io2", "gp2", "gp3", "sc1", "st1", "standard"}
	for _, volumeType := range csiVolumeTypes {
		scParameters := gererateCsiScExtraParametersByVolType(oc, ebsCsiDriverPrivisioner, volumeType)
		validCapacity := getValidRandomCapacityByCsiVolType(ebsCsiDriverPrivisioner, volumeType)
		as.Contains(interfaceToString(scParameters), volumeType)
		debugLogf("*csiProvisioner:\"%s\"*volType:\"%s\"*Parameters:\"%+v\"*Capacty:\"%s\"",
			ebsCsiDriverPrivisioner, volumeType, scParameters, validCapacity)
	}
}

// Test version Compare function
func TestVersionIsAbove(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	var as = assert.New(t)

	aboveVersions := []string{"4.11", "5.11", "4.21"}
	belowVersions := []string{"4.10.0", "4.9.33", "3.11.12"}
	// Test the 4.10 above versions
	for _, aboveVersion := range aboveVersions {
		as.Equal(versionIsAbove(aboveVersion, "4.10"), true)
	}
	// Test the 4.10.1 below versions
	for _, belowVersion := range belowVersions {
		as.Equal(versionIsAbove(belowVersion, "4.10.1"), false)
	}
}
