# KCP Tests
This repository holds the kcp tests that tests against the publicly available  interfaces (APIs, code, CLIs)

## Prerequisites
* Git installed. See [Installing Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)
* Golang installed. See [Installing Golang](https://golang.org/doc/install), the newer the better ( Ensure you install Golang from a binary release [found here](https://golang.org/dl/), not with a package manager such as `dnf` ）
* golint installed. See [Installing golint](https://github.com/golang/lint#installation)
* Install [kubelogin](https://github.com/int128/kubelogin.git) and [KCP kubectl plugin](https://github.com/kcp-dev/kcp.git) ( Ensure your installed KCP kubectl plugin version is the same with the kcp service version )
  - If you would like to download the suitable version packages directly go to [kcp release assets](https://github.com/kcp-dev/kcp/releases), download the zip file suitable for your OS, extract and copy the binaries to a place which is in your PATH and once copied run `kubectl kcp --version` to verify whether your client matches with the server 
  - If you would like to build the plugin by your self you could follow the steps like below:
  ```console
  $ git clone https://github.com/kcp-dev/kcp.git
  Check the same version with your kcp server in https://github.com/kcp-dev/kcp/tags and figure out its commit ID to checkout.
  $ git checkout <commit ID>
  $ make install WHAT=./cmd/kubectl-kcp && make install WHAT=./cmd/kubectl-workspaces && make install WHAT=./cmd/kubectl-ws
  ```

* Have the environment variable `KUBECONFIG` set pointing to your kcp service
  - If your `KUBECONFIG` has multiple contexts please specific target test environment by using environment variable `E2E_TEST_CONTEXT` ( In addition, if the specified test context doesn't exist, it will try to use `kcp-stable-root` context, if `kcp-stable-root` doesn't exist either, it'll use the `current-context` in `KUBECONFIG` instead). 
    ```console
    $ export E2E_TEST_CONTEXT="<specific test context>"
    ```
  
* Log in to the kcp service via SSO (Single Sign On)
  ```console
  $ kubectl oidc-login get-token --oidc-issuer-url=<oidc issuer url> --oidc-client-id=<oidc client ID> --oidc-redirect-url-hostname=127.0.0.1
  ```
* If you would like to test BYO cases please make sure the `PCLUSTER_KUBECONFIG` environment variable is exported otherwise else all the tests related to BYO will be skipped. 
## Contribution 
Below are the general steps for submitting a PR to main branch. First, you should **Fork** this repo to your own Github account.
```console
$ git remote add <Your Name> git@github.com:<Your Github Account>/kcp-tests.git
$ git pull origin main
$ git checkout -b <Branch Name>
$ git add xxx
$ git diff main --name-only |grep ".go$"| grep -v "bindata.go$" | xargs -n1 golint
  Please fix all golint error
$ git diff main --name-only |grep ".go$"| grep -v "bindata.go$" | xargs gofmt -s -l
  Please fix all gofmt error, running 'gofmt -s -d [file_path]' or autocorrect with 'gofmt -s -w [file_path]'
$ git add xxx
$ make build
$ ./bin/kcp-tests run all --dry-run |grep <Test Case Name>|./bin/kcp-tests run -f -
$ git commit -m "xxx"
$ git push <Your Name> <Branch Name>:<Branch Name>
```
And then there will be a prompt in your Github repo console to open a PR, click it to do so.
### Include new test folder
If you create a new folder for your test cases, **add the path** to the [include.go](https://github.com/kcp-dev/kcp-tests/blob/main/test/extended/include.go)

### Create go-bindata for new YAML files
If you have some **new YAML files** used in your code, you have to generate the bindata first.
Run `make update` to update the bindata. For example, you can see the bindata has been updated after running the `make update` as follows:
```console
$ git status
	modified:   test/extended/testdata/bindata.go
	new file:   test/extended/testdata/kcp/xxxx.yaml
```

### Compile the executable binary
Note that we use the `go module` for package management, the previous `go path` is deprecated.
```console
$ git clone git@github.com:kcp-dev/kcp-tests.git
$ cd kcp-tests/
$ make build
mkdir -p "bin"
export GO111MODULE="on" && export GOFLAGS="" && go build  -ldflags="-s -w" -mod=mod -o "bin" "./cmd/kcp-tests"
$ ls -hl ./bin/kcp-tests 
-rwxrwxr-x. 1 cloud-user cloud-user 106M Sep 13 14:41 ./bin/kcp-tests
```

### Run the automation test case
The binary finds the test case via searching for the test case title. It searches the test case titles by RE (`Regular Expression`). So, you can filter your test cases by using `grep`. 
##### Run automation test cases related to an area
If I want to run all [workspaces test cases](https://github.com/kcp-dev/kcp-tests/blob/main/test/extended/workspacetype/workspace.go#L14), and all of them contain the `area/workspaces` key word, I can use the `grep "area/workspaces"` to filter them, as follows: 
```console
$ ./bin/kcp-tests run all --dry-run | grep "area/workspaces" | ./bin/kcp-tests run -f -
"[area/workspaces] Author:pewang-Medium-[Smoke] Multi levels workspaces lifecycle should work [Suite:kcp/smoke/parallel/minimal]"
"[area/workspaces] Author:zxiao-Medium-[Serial] I can create context for a specific workspace and use it [Suite:kcp/smoke/serial]"
...
```
You can save the above output to a file and run it:
```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```
##### Run all smoke automation test cases
If you want to run all smoke test cases which has the `[Smoke]` label, you can do:
```console
$ ./bin/kcp-tests run smoke
```
##### Run a single automation test case
If you want to run a single test case, such as `g.It("Author:pewang-Medium-[Smoke] Multi levels workspaces lifecycle should work"`, you can do:
```console
$ ./bin/kcp-tests run all --dry-run|grep "Multi levels workspaces lifecycle should work"|./bin/kcp-tests run --junit-dir=./ -f -
```

### Debugging
#### Keep generated temporary workspaces
Sometime, we want to **keep the generated workspaces for debugging**, we could just set **`export DELETE_WORKSPACE=false`**, then these temporary workspaces will be kept. 

```console
...
Dec 18 09:39:33.448: INFO: Running AfterSuite actions on all nodes
...
1 pass, 0 skip (1m50s)
$ kubectl get ws
NAME                        TYPE        PHASE   URL
e2e-test-kcp-syncer-s6ktv   universal   Ready   https://<kcp-test-env-domain>/clusters/root:users:rp:pv:rh-sso-xxxx:e2e-test-kcp-syncer-s6ktv
e2e-test-kcp-syncer-a5spq   universal   Ready   https://<kcp-test-env-domain>/clusters/root:users:rp:pv:rh-sso-xxxx:e2e-test-kcp-syncer-a5spq
...
```
<!-- TODO: Retreive events from kcp server by test framework-->
<!-- #### Print cluster event on Terminal
When you execute cases, there are some events which is printed to the terminal (**`currently we cannot retreive events from kcp`)**, like
```console
Timeline:

Mar 30 03:57:36.435 I ns/openshift-kube-controller-manager pod/kube-controller-manager-ip-10-0-190-60.ec2.internal created SCC ranges for e2e-test-olm-common-l21c9cfo-g6xwx namespace
Mar 30 03:57:47.894 W ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg node/ip-10-0-247-215.ec2.internal graceful deletion within 30s
Mar 30 03:57:48.097 I ns/openshift-marketplace pod/marketplace-operator-5cf7b79dd4-xsffg Stopping container marketplace-operator
...
```
Someone does not want it on the terminal, but someone wants it for debugging.

So, we add environment variable ENABLE_PRINT_EVENT_STDOUT to enable it.

It does not print the kcp service events on the terminal by default when you execute the case on your terminal.

If you would like to enable it to help debug, **please set `export ENABLE_PRINT_EVENT_STDOUT=true` before executing the case.** -->
