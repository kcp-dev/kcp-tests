package storage

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// networking Service related functions
type service struct {
	name          string
	namespace     string
	port          string
	protocol      string
	targetPort    string
	nodePort      string
	selectorLable string
	clusterIP     string
	template      string
}

// function option mode to change the default values of service Object attributes
type serviceOption func(*service)

// Replace the default value of service name
func setServiceName(name string) serviceOption {
	return func(svc *service) {
		svc.name = name
	}
}

// Replace the default value of service namespace
func setServiceNamespace(namespace string) serviceOption {
	return func(svc *service) {
		svc.namespace = namespace
	}
}

// Replace the default value of service port
func setServicePort(port string) serviceOption {
	return func(svc *service) {
		svc.port = port
	}
}

// Replace the default value of service targetPort
func setServiceTargetPort(targetPort string) serviceOption {
	return func(svc *service) {
		svc.targetPort = targetPort
	}
}

// Replace the default value of service nodePort
func setServiceNodePort(nodePort string) serviceOption {
	return func(svc *service) {
		svc.nodePort = nodePort
	}
}

// Replace the default value of service protocol
func setServiceProtocol(protocol string) serviceOption {
	return func(svc *service) {
		svc.protocol = protocol
	}
}

// Replace the default value of service selectorLable
func setServiceSelectorLable(selectorLable string) serviceOption {
	return func(svc *service) {
		svc.selectorLable = selectorLable
	}
}

//  Create a new customized service object
func newService(opts ...serviceOption) service {
	defaultService := service{
		name:          "storage-svc-" + getRandomString(),
		namespace:     "",
		protocol:      "TCP",
		port:          "2049",
		targetPort:    "2049",
		nodePort:      "0",
		selectorLable: "",
	}
	for _, o := range opts {
		o(&defaultService)
	}
	return defaultService
}

// Create a specified service
func (svc *service) create(oc *exutil.CLI) {
	if svc.namespace == "" {
		svc.namespace = oc.Namespace()
	}
	err := applyResourceFromTemplateAsAdmin(oc, "--ignore-unknown-parameters=true", "-f", svc.template, "-p", "NAME="+svc.name, "NAMESPACE="+svc.namespace,
		"PROTOCOL="+svc.protocol, "PORT="+svc.port, "TARGETPORT="+svc.targetPort, "NODEPORT="+svc.nodePort, "SELECTORLABEL="+svc.selectorLable)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete a specified service with kubeadmin user
func (svc *service) deleteAsAdmin(oc *exutil.CLI) {
	oc.WithoutNamespace().AsAdmin().Run("delete").Args("-n", svc.namespace, "service", svc.name).Execute()
}

// Get ClusterIP type service IP address
func (svc *service) getClusterIP(oc *exutil.CLI) string {
	svcClusterIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("service", "-n", svc.namespace, svc.name, "-o=jsonpath={.spec.clusterIP}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	svc.clusterIP = svcClusterIP
	e2e.Logf("The service %s in namespace %s ClusterIP is %q", svc.name, svc.namespace, svc.clusterIP)
	return svc.clusterIP
}
