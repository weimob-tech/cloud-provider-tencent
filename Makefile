REGISTRY := ccr.ccs.tencentyun.com/weimob-public
BIN := tencent-cloud-controller-manager
VERSION := v1.2.3
IMAGE := $(REGISTRY)/$(BIN)
GOOS ?= linux
ARCH ?= amd64
SRC_DIRS := cmd pkg # directories which hold app source (not vendored)

.PHONY: all
all: build

.PHONY: build
build: build
	docker build --build-arg VERSION=${VERSION} -t ${IMAGE}:${VERSION} .
	docker push ${IMAGE}:${VERSION}

.PHONY: deploy
deploy:
	kubectl -n kube-system set image deployments/${BIN} ${BIN}=${IMAGE}:${VERSION}

.PHONY: version
version:
	@echo ${VERSION}