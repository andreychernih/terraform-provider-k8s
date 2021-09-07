# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

OS = $(shell uname | tr A-Z a-z)

BINARY_NAME = terraform-provider-k8s

# Build variables
BUILD_DIR ?= build
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git symbolic-ref -q --short HEAD)
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_DATE ?= $(shell date +%FT%T%z)
LDFLAGS += -X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH} -X main.buildDate=${BUILD_DATE}
export CGO_ENABLED ?= 0
ifeq (${VERBOSE}, 1)
ifeq ($(filter -v,${GOARGS}),)
	GOARGS += -v
endif
TEST_FORMAT = short-verbose
endif

export TF_CLI_CONFIG_FILE = ${PWD}/.terraformrc

# Dependency versions
GOTESTSUM_VERSION = 0.3.5
GOLANGCI_VERSION = 1.17.1
GORELEASER_VERSION = 0.113.0
TERRAFORM_VERSION = 1.0.6

GOLANG_VERSION = 1.13

# Add the ability to override some variables
# Use with care
-include override.mk

.PHONY: clean
clean: ## Clean builds
	rm -rf ${BUILD_DIR}/

.PHONY: goversion
goversion:
ifneq (${IGNORE_GOLANG_VERSION_REQ}, 1)
	@printf "${GOLANG_VERSION}\n$$(go version | awk '{sub(/^go/, "", $$3);print $$3}')" | sort -t '.' -k 1,1 -k 2,2 -k 3,3 -g | head -1 | grep -q -E "^${GOLANG_VERSION}$$" || (printf "Required Go version is ${GOLANG_VERSION}\nInstalled: `go version`" && exit 1)
endif

.PHONY: build
build: goversion ## Build a binary
ifeq (${VERBOSE}, 1)
	go env
endif

	go build ${GOARGS} -tags "${GOTAGS}" -ldflags "${LDFLAGS}" -o ${BUILD_DIR}/${BINARY_NAME} .

.PHONY: test-integration
test-integration: EXAMPLE_DIR ?= examples/0.12
test-integration: build bin/terraform .terraformrc ## Execute integration tests
ifneq (,$(findstring 0.12.,${TERRAFORM_VERSION}))
	cp build/terraform-provider-k8s .
	cp hack/versions012.tf ${EXAMPLE_DIR}/versions.tf
	bin/terraform init ${EXAMPLE_DIR}
	bin/terraform apply -auto-approve -input=false ${EXAMPLE_DIR}
else
	mkdir -p build/registry.terraform.io/banzaicloud/k8s/99.99.99/${OS}_amd64
	cp build/terraform-provider-k8s build/registry.terraform.io/banzaicloud/k8s/99.99.99/${OS}_amd64/
	cp hack/versions.tf ${EXAMPLE_DIR}
	bin/terraform -chdir=${EXAMPLE_DIR} init
	bin/terraform -chdir=${EXAMPLE_DIR} apply -auto-approve -input=false
endif
	${MAKE} test-integration-destroy EXAMPLE_DIR=${EXAMPLE_DIR}

.PHONY: test-integration-destroy
test-integration-destroy: EXAMPLE_DIR ?= examples/0.12
test-integration-destroy: bin/terraform .terraformrc
ifneq (,$(findstring 0.12.,${TERRAFORM_VERSION}))
	bin/terraform destroy -auto-approve ${EXAMPLE_DIR}
	rm terraform-provider-k8s
else
	bin/terraform -chdir=${EXAMPLE_DIR} destroy -auto-approve
	rm -rf ${EXAMPLE_DIR}/{.terraform,.terraform.lock.hcl}
endif

bin/terraform: bin/terraform-${TERRAFORM_VERSION}
	@ln -sf terraform-${TERRAFORM_VERSION} bin/terraform
bin/terraform-${TERRAFORM_VERSION}:
	@mkdir -p bin
ifeq (${OS}, darwin)
	curl -sfL https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_darwin_amd64.zip > bin/terraform.zip
endif
ifeq (${OS}, linux)
	curl -sfL https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip > bin/terraform.zip
endif
	unzip -d bin bin/terraform.zip
	@mv bin/terraform $@
	rm bin/terraform.zip

.terraformrc:
	sed "s|PATH|$$PWD/build|" hack/.terraformrc.tpl > .terraformrc

bin/goreleaser: bin/goreleaser-${GORELEASER_VERSION}
	@ln -sf goreleaser-${GORELEASER_VERSION} bin/goreleaser
bin/goreleaser-${GORELEASER_VERSION}:
	@mkdir -p bin
	curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | bash -s -- -b ./bin/ v${GORELEASER_VERSION}
	@mv bin/goreleaser $@

.PHONY: release
release: bin/goreleaser # Publish a release
	bin/goreleaser release

.PHONY: list
list: ## List all make targets
	@${MAKE} -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Variable outputting/exporting rules
var-%: ; @echo $($*)
varexport-%: ; @echo $*=$($*)
