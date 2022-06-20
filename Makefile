all: update update-public build
.PHONY: all
OUT_DIR=bin

update-public:
	export GOFLAGS="" && go get -d github.com/openshift/openshift-tests@master

build:
	mkdir -p "${OUT_DIR}"
	export GO111MODULE="on" && export GOFLAGS="" && go build  -ldflags="-s -w" -mod=mod -o "${OUT_DIR}" "./cmd/kcp-tests"

nightly-test: 
	./hack/nightly_test.sh

name-check:
	python ./hack/rule.py 

check-code:
	./hack/check-code.sh master

pr-test:
	python3 ./hack/pr.py

# Include the library makefile
include $(addprefix ./, bindata.mk)


IMAGE_REGISTRY :=registry.svc.ci.openshift.org

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
$(call build-image,ocp-extended-platform-tests,$(IMAGE_REGISTRY)/ocp/4.3:extended-platform-tests, ./Dockerfile.rhel7,.)

# This will call a macro called "add-bindata" which will generate bindata specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - input dirs
# $3 - prefix
# $4 - pkg
# $5 - output
# It will generate targets {update,verify}-bindata-$(1) logically grouping them in unsuffixed versions of these targets
# and also hooked into {update,verify}-generated for broader integration.
$(call add-bindata,testdata,test/extended/testdata/...,testextended,testdata,./test/extended/testdata/bindata.go)

test-e2e: GO_TEST_PACKAGES :=./test/e2e/...
test-e2e: GO_TEST_FLAGS += -v
test-e2e: GO_TEST_FLAGS += -timeout 1h
test-e2e: GO_TEST_FLAGS += -p 1
test-e2e: test-unit
.PHONY: test-e2e

clean:
	$(RM) ./bin/kcp-tests
.PHONY: clean
