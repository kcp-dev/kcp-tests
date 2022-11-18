package syncer

import (
	"fmt"
	"os"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// SyncTarget struct definition
type SyncTarget struct {
	Name            string
	SyncerImage     string
	OutputFilePath  string
	WorkSpaceServer string
}

// SyncTargetOption uses function option mode to change the default values of SyncTarget attributes
type SyncTargetOption func(*SyncTarget)

// SetSyncTargetName sets the SyncTarget's name
func SetSyncTargetName(name string) SyncTargetOption {
	return func(s *SyncTarget) {
		s.Name = name
	}
}

// SetSyncTargetOutputFilePath sets the SyncTarget's OutputFilePath
func SetSyncTargetOutputFilePath(path string) SyncTargetOption {
	return func(s *SyncTarget) {
		s.OutputFilePath = path
	}
}

// NewSyncTarget create a new customized SyncTarget
func NewSyncTarget(opts ...SyncTargetOption) SyncTarget {
	defaultSyncTarget := SyncTarget{
		Name:            "mysyncer-" + exutil.GetRandomString(),
		WorkSpaceServer: "",
	}
	for _, o := range opts {
		o(&defaultSyncTarget)
	}
	return defaultSyncTarget
}

// Create SyncTarget
func (s *SyncTarget) Create(k *exutil.CLI) {
	if s.WorkSpaceServer == "" {
		s.WorkSpaceServer = k.WorkSpace().ServerURL
	}
	s.CreateAsExpectedResult(k, true, "Wrote physical cluster manifest to")

}

// GetSynerImageTag gets the syncer image tag by kcp server version/gitCommit
// returns kcp server version tag for kcp release versions test environments
// returns kcp server gitCommit tag for kcp dev versions test environments
func (s *SyncTarget) GetSynerImageTag(k *exutil.CLI) string {
	var (
		syncerImageTag string
		err            error
	)
	if os.Getenv("E2E_TEST_CONTEXT") == "kcp-unstable-root" {
		syncerImageTag, err = exutil.GetKcpServerGitCommit(k)
		o.Expect(err).ShouldNot(o.HaveOccurred())
		o.Expect(syncerImageTag).Should(o.Not(o.BeEmpty()))
		syncerImageTag = syncerImageTag[:len(syncerImageTag)-1]
	} else {
		syncerImageTag, err = exutil.GetKcpServerVersion(k)
		o.Expect(err).ShouldNot(o.HaveOccurred())
		o.Expect(syncerImageTag).Should(o.MatchRegexp(`v\d+(.\d+){0,2}`))
	}
	return syncerImageTag
}

// CreateAsExpectedResult creates SyncTarget CR and checks the created result is as expected
func (s *SyncTarget) CreateAsExpectedResult(k *exutil.CLI, successFlag bool, containsMsg string) {
	msg, createError := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("kcp").Args("--server="+s.WorkSpaceServer, "workload", "sync", s.Name,
		"--syncer-image=ghcr.io/kcp-dev/kcp/syncer:"+s.GetSynerImageTag(k), "--output-file="+s.OutputFilePath).Output()
	if successFlag {
		o.Expect(createError).ShouldNot(o.HaveOccurred())
		o.Expect(msg).Should(o.ContainSubstring(containsMsg))
	} else {
		o.Expect(createError).Should(o.HaveOccurred())
		o.Expect(fmt.Sprint(msg)).Should(o.ContainSubstring(containsMsg))
	}
}

// GetFieldByJSONPath gets specific field value of the SyncTarget by jsonpath
func (s *SyncTarget) GetFieldByJSONPath(k *exutil.CLI, JSONPath string) (string, error) {
	return k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "syncTarget", s.Name, "-o", "jsonpath="+JSONPath).Output()
}

// CheckReady checks whether the SyncTarget is ready
func (s *SyncTarget) CheckReady(k *exutil.CLI) (bool, error) {
	readyFlag, err := s.GetFieldByJSONPath(k, `{.status.conditions[?(@.type=="Ready")].status}`)
	if err != nil {
		e2e.Logf(`Getting SyncTarget/%s status failed: "%v"`, s.Name, err)
		return false, err
	}
	e2e.Logf("SyncTarget/%s ready condition is %s", s.Name, readyFlag)
	return strings.EqualFold(readyFlag, "True"), nil
}

// WaitUntilReady waits the SyncTarget become ready
func (s *SyncTarget) WaitUntilReady(k *exutil.CLI) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		SyncTargetReady, err := s.CheckReady(k)
		return SyncTargetReady, err
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting for SyncTarget/%s to become ready times out", s.Name))
}

// WaitDeploymentsAPISynced waits the deployments api resource synced
func (s *SyncTarget) WaitDeploymentsAPISynced(k *exutil.CLI) {
	exutil.WaitSpecificAPISyncedInSpecificWorkSpace(k, "deployments", s.WorkSpaceServer)
	e2e.Logf("The syntarget/%s's deployments api resource synced succeed", s.Name)
}

// WaitUntilReadyAndDeploymentsAPISynced waits the SyncTarget become ready and the deployments api resource synced
func (s *SyncTarget) WaitUntilReadyAndDeploymentsAPISynced(k *exutil.CLI) {
	s.WaitUntilReady(k)
	s.WaitDeploymentsAPISynced(k)
}

// CheckDisplayColumns checks the SyncTarget info showing the expected columns
func (s *SyncTarget) CheckDisplayColumns(k *exutil.CLI) {
	// Check the display columns
	syncTargetInfo, err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(syncTargetInfo).Should(o.And(
		o.ContainSubstring("NAME"),
		o.ContainSubstring("AGE"),
	))
	// Check the display attributes with "-o wide" option
	syncTargetInfo, err = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name, "-o", "wide").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(syncTargetInfo).Should(o.And(
		o.ContainSubstring("NAME"),
		o.ContainSubstring("LOCATION"),
		o.ContainSubstring("READY"),
		o.ContainSubstring("SYNCED API RESOURCES"),
		o.ContainSubstring("KEY"),
		o.ContainSubstring("AGE"),
	))
	displayLines := strings.Split(string(syncTargetInfo), "\n")
	schemaAttributes := strings.Fields(strings.TrimSpace(displayLines[0]))
	// Check all the display attributes not be empty
	// "SYNCED API RESOURCES" will be recognized to 3 different columns
	// while its value only displays in one column
	// "len(schemaAttributes)-2" should be equal to "len(attributesValues)"
	o.Eventually(func() int {
		syncTargetInfo, _ = k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name, "-o", "wide").Output()
		displayLines = strings.Split(string(syncTargetInfo), "\n")
		return len(strings.Fields(strings.TrimSpace(displayLines[1])))
	}, 120*time.Second, 5*time.Second).Should(o.Equal(len(schemaAttributes) - 2))
}

// Delete the SyncTarget
func (s *SyncTarget) Delete(k *exutil.CLI) {
	o.Expect(s.Clean(k)).NotTo(o.HaveOccurred())
}

// Clean the SyncTarget resource
func (s *SyncTarget) Clean(k *exutil.CLI) error {
	// TODO: Temp solution for known limitation: https://github.com/kcp-dev/kcp/issues/999
	// Need to remove after the issue solved
	mySyncerKey, _ := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name, `-o=jsonpath={.metadata.labels.internal\.workload\.kcp\.dev/key}`).Output()
	k.AsPClusterKubeconf().WithoutNamespace().WithoutWorkSpaceServer().Run("delete").Args("ns", "-l", "internal.workload.kcp.dev/cluster="+mySyncerKey).Execute()
	return k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("delete").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name).Execute()
}
