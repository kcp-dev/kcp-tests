# Contribution 
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
