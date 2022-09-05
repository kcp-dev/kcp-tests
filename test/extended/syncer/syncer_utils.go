package syncer

import (
	"fmt"
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

// CreateAsExpectedResult creates SyncTarget CR and checks the created result is as expected
func (s *SyncTarget) CreateAsExpectedResult(k *exutil.CLI, successFlag bool, containsMsg string) {
	kcpServerVersion, getError := exutil.GetKcpServerVersion(k)
	o.Expect(getError).ShouldNot(o.HaveOccurred())
	o.Expect(kcpServerVersion).Should(o.MatchRegexp(`v\d+(.\d+){0,2}`))
	msg, createError := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("kcp").Args("--server="+s.WorkSpaceServer, "workload", "sync", s.Name,
		"--syncer-image=ghcr.io/kcp-dev/kcp/syncer:"+kcpServerVersion, "--output-file="+s.OutputFilePath).Output()
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
		e2e.Logf(`Get SyncTarget/%s status faied: "%v"`, s.Name, err)
		return false, err
	}
	e2e.Logf("SyncTarget/%s ready condition is %s", s.Name, readyFlag)
	return strings.EqualFold(readyFlag, "True"), nil
}

// WaitUntilReady waits the SyncTarget become ready
func (s *SyncTarget) WaitUntilReady(k *exutil.CLI) {
	err := wait.Poll(5*time.Second, 180*time.Second, func() (bool, error) {
		SyncTargetReady, err := s.CheckReady(k)
		if err != nil {
			return SyncTargetReady, err
		}
		return SyncTargetReady, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Waiting SyncTarget/%s become ready timeout", s.Name))
}

// CheckDisplayAttributes checks the SyncTarget info showing the expected columns
func (s *SyncTarget) CheckDisplayAttributes(k *exutil.CLI) {
	syncTargetInfo, err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("get").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(syncTargetInfo).Should(o.And(
		o.ContainSubstring("NAME"),
		o.ContainSubstring("AGE"),
	))
	syncTargetInfo, err = k.WithoutNamespace().WithoutKubeconf().Run("get").Args("synctarget", s.Name, "-o", "wide").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(syncTargetInfo).Should(o.And(
		o.ContainSubstring("NAME"),
		o.ContainSubstring("LOCATION"),
		o.ContainSubstring("READY"),
		o.ContainSubstring("SYNCED"),
		o.ContainSubstring("API"),
		o.ContainSubstring("RESOURCES"),
		o.ContainSubstring("KEY"),
		o.ContainSubstring("AGE"),
	))
}

// Delete the SyncTarget
func (s *SyncTarget) Delete(k *exutil.CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("delete").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Clean the SyncTarget resource
func (s *SyncTarget) Clean(k *exutil.CLI) {
	k.WithoutNamespace().WithoutKubeconf().WithoutWorkSpaceServer().Run("delete").Args("--server="+s.WorkSpaceServer, "synctarget", s.Name).Execute()
}
