package kcp

import (
        "encoding/json"
        "fmt"
        "io/ioutil"
        "os"
        "os/exec"
        "path/filepath"
        "regexp"
        "strings"
        "time"

        g "github.com/onsi/ginkgo"
        o "github.com/onsi/gomega"
        "k8s.io/apimachinery/pkg/util/wait"
        e2e "k8s.io/kubernetes/test/e2e/framework"

        exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[sig-kcp] Basics", func() {
        defer g.GinkgoRecover()

        var (
                oc = exutil.NewCLI("oc", exutil.KubeConfigPath())
        )

        g.It("Author:knarra-Medium-280071-Checking kubectl version", func() {
                out, err := oc.Run("version").Args("-o", "json").Output()
                o.Expect(err).NotTo(o.HaveOccurred())
                versionInfo := &VersionInfo{}
                if err := json.Unmarshal([]byte(out), &versionInfo); err != nil {
                        e2e.Failf("unable to decode version with error: %v", err)
                }
                if match, _ := regexp.MatchString("clean", versionInfo.ClientInfo.GitTreeState); !match {
                        e2e.Failf("varification GitTreeState with error: %v", err)
                }

        })
})
