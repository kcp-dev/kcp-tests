package storage

import (
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Define the NFS Server related functions
type nfsServer struct {
	deploy deployment
	svc    service
}

// function option mode to change the default values of nfsServer Object attributes
type nfsServerOption func(*nfsServer)

// Replace the default value of nfsServer deployment
func setNfsServerDeployment(deploy deployment) nfsServerOption {
	return func(nfs *nfsServer) {
		nfs.deploy = deploy
	}
}

// Replace the default value of nfsServer service
func setNfsServerSvc(svc service) nfsServerOption {
	return func(nfs *nfsServer) {
		nfs.svc = svc
	}
}

//  Create a new customized nfsServer object
func newNfsServer(opts ...nfsServerOption) nfsServer {
	serverName := "nfs-" + getRandomString()
	defaultNfsServer := nfsServer{
		deploy: newDeployment(setDeploymentName(serverName), setDeploymentApplabel(serverName), setDeploymentMountpath("/mnt/data")),
		svc:    newService(setServiceSelectorLable(serverName)),
	}
	for _, o := range opts {
		o(&defaultNfsServer)
	}
	return defaultNfsServer
}

// Install the specified NFS Server on cluster
func (nfs *nfsServer) install(oc *exutil.CLI) {
	nfs.deploy.create(oc)
	nfs.deploy.waitReady(oc)
	nfs.svc.name = "nfs-service"
	nfs.svc.create(oc)
	nfs.svc.getClusterIP(oc)
	e2e.Logf("Install NFS server successful, serverIP is %s", nfs.svc.clusterIP)
}

// Uninstall the specified NFS Server from cluster
func (nfs *nfsServer) uninstall(oc *exutil.CLI) {
	nfs.svc.deleteAsAdmin(oc)
	nfs.deploy.deleteAsAdmin(oc)
}
