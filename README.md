# KCP Tests
This repository holds the kcp tests that tests against the publicly available  interfaces (APIs, code, CLIs)

## Prerequisites
* Git installed. See [Installing Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)
* Golang installed. See [Installing Golang](https://golang.org/doc/install),the newer the better.
        * Ensure you install Golang from a binary release [found here](https://golang.org/dl/), not with a package manager such as `dnf`
* golint installed. See [Installing golint](https://github.com/golang/lint#installation)
* Have the environment variable `KUBECONFIG` set pointing to your cluster

### Include new test folder
If you create a new folder for your test case, **add the path** to the [include.go](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/include.go)

### Create go-bindata for new YAML files
If you have some **new YAML files** used in your code, you have to generate the bindata first.
Run `make update` to update the bindata. For example, you can see the bindata has been updated after running the `make update` as follows:
```console
$ git status
	modified:   test/extended/testdata/bindata.go
	new file:   test/extended/testdata/olm/etcd-subscription-manual.yaml
```

### Compile the executable binary
Note that we use the `go module` for package management, the previous `go path` is deprecated.
```console
$ git clone git@github.com:openshift/openshift-tests-private.git
$ cd openshift-tests-private/
$ make build
mkdir -p "bin"
export GO111MODULE="on" && export GOFLAGS="" && go build -o "bin" "./cmd/extended-platform-tests"
$ ls -hl ./bin/extended-platform-tests 
-rwxrwxr-x. 1 cloud-user cloud-user 165M Jun 24 22:17 ./bin/extended-platform-tests
```

## Contribution 
Below are the general steps for submitting a PR to master branch. First, you should **Fork** this repo to your own Github account.
```console
$ git remote add <Your Name> git@github.com:<Your Github Account>/openshift-tests-private.git
$ git pull origin master
$ git checkout -b <Branch Name>
$ git add xxx
$ git diff master --name-only |grep ".go$"| grep -v "bindata.go$" | xargs -n1 golint
  Please fix all golint error
$ git diff master --name-only |grep ".go$"| grep -v "bindata.go$" | xargs gofmt -s -l
  Please fix all gofmt error, running 'gofmt -s -d [file_path]' or autocorrect with 'gofmt -s -w [file_path]'
$ git add xxx
$ make build
$ ./bin/extended-platform-tests run all --dry-run |grep <Test Case ID>|./bin/extended-platform-tests run -f -
$ git commit -m "xxx"
$ git push <Your Name> <Branch Name>:<Branch Name>
```
And then there will be a prompt in your Github repo console to open a PR, click it to do so.

### Run the automation test case
The binary finds the test case via searching for the test case title. It searches the test case titles by RE (`Regular Expression`). So, you can filter your test cases by using `grep`. Such as, if I want to run all [OLM test cases](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/operators/olm.go#L21), and all of them contain the `OLM` letter, I can use the `grep OLM` to filter them, as follows: 
```console
$ ./bin/extended-platform-tests run all --dry-run | grep "OLM" | ./bin/extended-platform-tests run -f -
I0624 22:48:36.599578 2404223 test_context.go:419] Tolerating taints "node-role.kubernetes.io/master" when considering if nodes are ready
"[sig-operators] OLM for an end user handle common object Author:kuiwang-Medium-22259-marketplace operator CR status on a running cluster [Exclusive] [Serial]"
...
```
You can save the above output to a file and run it:
```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```
If you want to run a test case, such as `g.It("Author:jiazha-Critical-23440-can subscribe to the etcd operator  [Serial]"`, since the `TestCaseID` is unique, you can do:
```console
$ ./bin/extended-platform-tests run all --dry-run|grep "23440"|./bin/extended-platform-tests run --junit-dir=./ -f -
```

### Debugging
#### Keep generated temporary project
Sometime, we want to **keep the generated namespace for debugging**. Just add the Env Var: `export DELETE_NAMESPACE=false`. These random namespaces will be kept, like below:
```console
...
Dec 18 09:39:33.448: INFO: Running AfterSuite actions on all nodes
Dec 18 09:39:33.448: INFO: Waiting up to 7m0s for all (but 100) nodes to be ready
Dec 18 09:39:33.511: INFO: Found DeleteNamespace=false, skipping namespace deletion!
Dec 18 09:39:33.511: INFO: Running AfterSuite actions on node 1
...
1 pass, 0 skip (2m50s)
[root@preserve-olm-env openshift-tests-private]# oc get ns
NAME                                               STATUS   AGE
default                                            Active   4h46m
e2e-test-olm-a-a92jyymd-lmgj6                      Active   4m28s
e2e-test-olm-a-a92jyymd-pr8hx                      Active   4m29s
...
```
#### Print cluster event on Terminal
When you execute cases, there are some cluster event which is printed to the terminal, like
```console
Timeline:

Mar 30 03:57:36.435 I ns/openshift-kube-controller-manager pod/kube-controller-manager-ip-10-0-190-60.ec2.internal created SCC ranges for e2e-test-olm-common-l21c9cfo-g6xwx namespace
Mar 30 03:57:47.894 W ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg node/ip-10-0-247-215.ec2.internal graceful deletion within 30s
Mar 30 03:57:48.097 I ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg Stopping container marketplace-operator
...
```
Someone does not want it on the terminal, but someone wants it for debugging.

So, we add environment variable ENABLE_PRINT_EVENT_STDOUT to enable it.

In default, it does not print the cluster event on the terminal when you execute the case on your terminal.

### Compile the executable binary
Note that we use the `go module` for package management, the previous `go path` is deprecated.
```console
$ git clone git@github.com:openshift/openshift-tests-private.git
$ cd openshift-tests-private/
$ make build
mkdir -p "bin"
export GO111MODULE="on" && export GOFLAGS="" && go build -o "bin" "./cmd/extended-platform-tests"
$ ls -hl ./bin/extended-platform-tests 
-rwxrwxr-x. 1 cloud-user cloud-user 165M Jun 24 22:17 ./bin/extended-platform-tests
```

### Run the automation test case
The binary finds the test case via searching for the test case title. It searches the test case titles by RE (`Regular Expression`). So, you can filter your test cases by using `grep`. Such as, if I want to run all [OLM test cases](https://github.com/openshift/openshift-tests-private/blob/master/test/extended/operators/olm.go#L21), and all of them contain the `OLM` letter, I can use the `grep OLM` to filter them, as follows: 
```console
$ ./bin/extended-platform-tests run all --dry-run | grep "OLM" | ./bin/extended-platform-tests run -f -
I0624 22:48:36.599578 2404223 test_context.go:419] Tolerating taints "node-role.kubernetes.io/master" when considering if nodes are ready
"[sig-operators] OLM for an end user handle common object Author:kuiwang-Medium-22259-marketplace operator CR status on a running cluster [Exclusive] [Serial]"
...
```
You can save the above output to a file and run it:
```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```
If you want to run a test case, such as `g.It("Author:jiazha-Critical-23440-can subscribe to the etcd operator  [Serial]"`, since the `TestCaseID` is unique, you can do:
```console
$ ./bin/extended-platform-tests run all --dry-run|grep "23440"|./bin/extended-platform-tests run --junit-dir=./ -f -
```

### Debugging
#### Keep generated temporary project
Sometime, we want to **keep the generated namespace for debugging**. Just add the Env Var: `export DELETE_NAMESPACE=false`. These random namespaces will be kept, like below:
```console
...
Dec 18 09:39:33.448: INFO: Running AfterSuite actions on all nodes
Dec 18 09:39:33.448: INFO: Waiting up to 7m0s for all (but 100) nodes to be ready
Dec 18 09:39:33.511: INFO: Found DeleteNamespace=false, skipping namespace deletion!
Dec 18 09:39:33.511: INFO: Running AfterSuite actions on node 1
...
1 pass, 0 skip (2m50s)
[root@preserve-olm-env openshift-tests-private]# oc get ns
NAME                                               STATUS   AGE
default                                            Active   4h46m
e2e-test-olm-a-a92jyymd-lmgj6                      Active   4m28s
e2e-test-olm-a-a92jyymd-pr8hx                      Active   4m29s
...
```
#### Print cluster event on Terminal
When you execute cases, there are some cluster event which is printed to the terminal, like
```console
Timeline:

Mar 30 03:57:36.435 I ns/openshift-kube-controller-manager pod/kube-controller-manager-ip-10-0-190-60.ec2.internal created SCC ranges for e2e-test-olm-common-l21c9cfo-g6xwx namespace
Mar 30 03:57:47.894 W ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg node/ip-10-0-247-215.ec2.internal graceful deletion within 30s
Mar 30 03:57:48.097 I ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg Stopping container marketplace-operator
...
```
Someone does not want it on the terminal, but someone wants it for debugging.

So, we add environment variable ENABLE_PRINT_EVENT_STDOUT to enable it.

In default, it does not print the cluster event on the terminal when you execute the case on your terminal.

if you want it for debugging, **please set `export ENABLE_PRINT_EVENT_STDOUT=true` before executing the case.**
if you want it for debugging, **please set `export ENABLE_PRINT_EVENT_STDOUT=true` before executing the case.**
