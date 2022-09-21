package workspacetype

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

var _ = g.Describe("[area/workspaces]", func() {
	defer g.GinkgoRecover()

	var (
		k = exutil.NewCLIWithWorkSpace("kcp-workspace")
	)

	g.It("Author:pewang-Medium-[Smoke] Multi levels workspaces lifecycle should work", func() {
		g.By("# Create a test workspace as parent workspace")
		k.SetupWorkSpace()
		parentWorkSpace := k.WorkSpace()

		g.By("# Create five child workspaces under the parent workspace")
		childWorkSpaces := []exutil.WorkSpace{}
		for i := 1; i <= 5; i++ {
			g.By(fmt.Sprintf("# Creating child workspace no.%v", i))
			k.SetupWorkSpaceWithSpecificPath(parentWorkSpace.ServerURL)
			childWorkSpace := k.WorkSpace()
			childWorkSpaces = append(childWorkSpaces, childWorkSpace)
		}

		g.By(`# Check list workspaces command returns all columns`)
		output, err := k.WithoutWorkSpaceServer().Run("get").Args("--server="+parentWorkSpace.ParentServerURL, "workspace").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.And(
			o.ContainSubstring("NAME"),
			o.ContainSubstring("TYPE"),
			o.ContainSubstring("PHASE"),
			o.ContainSubstring("URL"),
		))

		g.By(`# From the home workspace, I can list only the test "parent workspace" but cannot list the five "child workspaces"`)
		homeSubWorkSpaces := k.ListWorkSpacesWithSpecificPath(parentWorkSpace.ParentServerURL)
		// NOTE: use ContainSubstring as a workaround to WorkSpace name inequality bug: https://url.corp.redhat.com/1878a60
		homeSubWorkSpacesString := strings.Join(homeSubWorkSpaces, ",")
		o.Expect(homeSubWorkSpacesString).Should(o.ContainSubstring(parentWorkSpace.Name))
		for _, childWorkSpace := range childWorkSpaces {
			o.Expect(homeSubWorkSpacesString).ShouldNot(o.ContainSubstring(childWorkSpace.Name))
		}

		g.By("# From parent workspace, I can list its child workspaces")
		parentSubWorkSpaces := k.ListWorkSpacesWithSpecificPath(parentWorkSpace.ServerURL)
		// NOTE: use ContainSubstring as a workaround to WorkSpace name inequality bug: https://url.corp.redhat.com/1878a60
		parentSubWorkSpacesString := strings.Join(parentSubWorkSpaces, ",")
		for _, childWorkSpace := range childWorkSpaces {
			o.Expect(parentSubWorkSpacesString).Should(o.ContainSubstring(childWorkSpace.Name))
		}

		g.By("# Delete child workspaces")
		for _, childWorkSpace := range childWorkSpaces {
			output, err := k.WithoutWorkSpaceServer().Run("delete").Args("--server="+parentWorkSpace.ServerURL, "workspace", childWorkSpace.Name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).Should(o.ContainSubstring("deleted"))
			o.Eventually(func() string {
				workSpaces, _ := k.WithoutWorkSpaceServer().Run("get").Args("--server="+parentWorkSpace.ServerURL, "workspace").Output()
				return workSpaces
			}, 60*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(childWorkSpace.Name))
		}

		g.By("# Delete parent workspace")
		output, err = k.WithoutWorkSpaceServer().Run("delete").Args("--server="+parentWorkSpace.ParentServerURL, "workspace", parentWorkSpace.Name).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("deleted"))
		o.Eventually(func() string {
			workSpaces, _ := k.WithoutWorkSpaceServer().Run("get").Args("--server="+parentWorkSpace.ParentServerURL, "workspace").Output()
			return workSpaces
		}, 60*time.Second, 5*time.Second).ShouldNot(o.ContainSubstring(parentWorkSpace.Name))
	})

	// author: zxiao@redhat.com
	g.It("Author:zxiao-Medium-[Serial] I can create context for a specific workspace and use it", func() {
		g.By("# Create a test workspace")
		k.SetupWorkSpace()
		workSpace := k.WorkSpace()

		g.By("# Get context before test run")
		contextBeforeTestRun, err := k.Run("config").Args("current-context").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Create context under this workspace")
		err = k.WithoutWorkSpaceServer().Run("kcp").Args("workspace", "create-context", workSpace.Name, "--server="+workSpace.ParentServerURL).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Switch to the newly created context")
		// NOTE: not doing assertion as to avoid double assertion failures, use DeferCleanup in ginkgo v2.0 in the future
		defer func() {
			err = k.Run("config").Args("use-context", contextBeforeTestRun).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			k.Run("config").Args("delete-context", workSpace.Name).Execute()
		}()

		err = k.Run("config").Args("use-context", workSpace.Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("# Check current context name")
		currentContext, err := k.Run("config").Args("current-context").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(currentContext).To(o.Equal(workSpace.Name))

		g.By("# Recreate context, expect to show conflict")
		output, err := k.WithoutWorkSpaceServer().Run("kcp").Args("workspace", "create-context", workSpace.Name, "--server="+workSpace.ParentServerURL).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("already exists in kubeconfig"))
	})
})
