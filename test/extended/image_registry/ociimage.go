package imageregistry

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	container "github.com/openshift/openshift-tests-private/test/extended/util/container"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-imageregistry] Image_Registry", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLI("default-image-oci", exutil.KubeConfigPath())
		manifestType = "application/vnd.oci.image.manifest.v1+json"
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-VMonly-ConnectedOnly-High-36291-OCI image is supported by API server and image registry", func() {
		oc.SetupProject()
		g.By("Import an OCI image to internal registry")
		err := oc.Run("import-image").Args("myimage", "--from", "docker.io/wzheng/busyboxoci", "--confirm", "--reference-policy=local").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "myimage", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("create").Args("serviceaccount", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "admin", "-z", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-cluster-role-from-user", "admin", "-z", "registry", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Get internal registry token")
		token, err := getSAToken(oc, "registry", oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get worker nodes")
		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: `node-role.kubernetes.io/worker`})
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Discovered %d worker nodes.", len(workerNodes.Items))
		o.Expect(workerNodes.Items).NotTo(o.HaveLen(0))
		worker := workerNodes.Items[0]
		g.By("Login registry in the node and inspect image")
		commandsOnNode := fmt.Sprintf("podman login image-registry.openshift-image-registry.svc:5000 -u registry -p %q ;podman pull image-registry.openshift-image-registry.svc:5000/%q/myimage;podman inspect image-registry.openshift-image-registry.svc:5000/%q/myimage", token, oc.Namespace(), oc.Namespace())
		out, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+worker.Name, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c", fmt.Sprintf("%s", commandsOnNode)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Display oci image info")
		e2e.Logf(out)
		o.Expect(out).To(o.ContainSubstring(manifestType))
	})
})

var _ = g.Describe("[sig-imageregistry] Image_Registry Vmonly", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("default-image-oci-vm", exutil.KubeConfigPath())
	)
	// author: wewang@redhat.com
	g.It("Author:wewang-ConnectedOnly-VMonly-High-37498-Push image with OCI format directly to the internal registry", func() {
		var podmanCLI = container.NewPodmanCLI()
		containerCLI := podmanCLI
		//quay.io does not support oci image, so using docker image temporarily, https://issues.redhat.com/browse/PROJQUAY-2300
		// ociImage := "quay.io/openshifttest/ociimage"
		ociImage := "docker.io/wzheng/ociimage"

		g.By("Expose default route of internal registry")
		err := oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":true}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":false}}`, "--type=merge").Execute()
		oc.SetupProject()
		g.By("Log into the default route")
		time.Sleep(time.Second * 5)
		defroute, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route/default-route", "-n", "openshift-image-registry", "-o=jsonpath={.spec.host}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		loginRegistryDefaultRoute(oc, defroute, oc.Namespace())
		defer func() {
			g.By("Logout registry route")
			if output, err := containerCLI.Run("logout").Args(defroute).Output(); err != nil {
				e2e.Logf(output)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("podman tag an image")
		output, err := containerCLI.Run("pull").Args(ociImage).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}
		defer containerCLI.RemoveImage(ociImage)
		output, err = containerCLI.Run("tag").Args(ociImage, defroute+"/"+oc.Namespace()+"/myimage:latest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}

		g.By(" Push it with oci format")
		out := defroute + "/" + oc.Namespace() + "/myimage:latest"
		output, err = containerCLI.Run("push").Args("--format=oci", out).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}

		g.By("Check the manifest type")
		output, err = containerCLI.Run("inspect").Args(out).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}
		defer containerCLI.RemoveImage(out)
		o.Expect(output).To(o.ContainSubstring("application/vnd.oci.image.manifest.v1+json"))
	})

	// author: wewang@redhat.com
	g.It("Author:wewang-ConnectedOnly-VMonly-Critical-35998-OCI images layers configs can be pruned completely [Exclusive]", func() {
		var podmanCLI = container.NewPodmanCLI()
		containerCLI := podmanCLI
		ociImage := "docker.io/wzheng/ociimage"

		g.By("Tag the image to internal registry")
		oc.SetupProject()
		err := oc.Run("tag").Args("--source=docker", ociImage, "35998-image:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Check if new imagestreamtag created")
		out := getResource(oc, true, withoutNamespace, "istag", "-n", oc.Namespace())
		o.Expect(out).To(o.ContainSubstring("35998-image:latest"))

		g.By("Log into the default route")
		err = oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":true}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("patch").Args("configs.imageregistry/cluster", "-p", `{"spec":{"defaultRoute":false}}`, "--type=merge").Execute()
		time.Sleep(time.Second * 5)
		defroute, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route/default-route", "-n", "openshift-image-registry", "-o=jsonpath={.spec.host}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		loginRegistryDefaultRoute(oc, defroute, oc.Namespace())
		defer func() {
			g.By("Logout registry route")
			if output, err := containerCLI.Run("logout").Args(defroute).Output(); err != nil {
				e2e.Logf(output)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		g.By("Pull internal image locally")
		imageInfo := defroute + "/" + oc.Namespace() + "/35998-image:latest"
		output, err := containerCLI.Run("pull").Args(imageInfo).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}
		defer containerCLI.RemoveImage(imageInfo)

		g.By("Mark down the config/layer info of oci image")
		output, err = containerCLI.Run("run").Args("--rm", "quay.io/rh-obulatov/boater", "boater", "get-manifest", "-a", ociImage).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if err != nil {
			e2e.Logf(output)
		}
		defer containerCLI.RemoveImage("quay.io/rh-obulatov/boater")
		o.Expect(output).To(o.ContainSubstring("schemaVersion\":2,\"config"))
		o.Expect(output).To(o.ContainSubstring("layers"))
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", oc.Namespace(), "all", "--all").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Prune image")
		token, err := oc.Run("whoami").Args("-t").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.WithoutNamespace().AsAdmin().Run("adm").Args("prune", "images", "--keep-tag-revisions=0", "--keep-younger-than=0m", "--token="+token).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("Deleting layer link"))
		o.Expect(output).To(o.ContainSubstring("Deleting blob"))
		o.Expect(output).To(o.ContainSubstring("Deleting image"))
	})
})
