################################################################################
##                               BUILD ARGS                                   ##
################################################################################
# This build arg allows the specification of a custom Golang image.
ARG GOLANG_IMAGE=golang:1.15.3

# The distroless image on which the CPI manager image is built.
#
# Please do not use "latest". Explicit tags should be used to provide
# deterministic builds. This image doesn't have semantic version tags, but
# the fully-qualified image can be obtained by entering
# "gcr.io/distroless/static:latest" in a browser and then copying the
# fully-qualified image from the web page.
ARG DISTROLESS_IMAGE=alpine:3.9

# docker build command:
# docker build \
#		--build-arg VERSION=$(VERSION) \
#		--build-arg GOOS=$(GOOS) \
#		--build-arg GOPROXY=$(GOPROXY) \
#		--tag $(IMAGE) .

################################################################################
##                              BUILD STAGE                                   ##
################################################################################
# Build the manager as a statically compiled binary so it has no dependencies
# libc, muscl, etc.
FROM ${GOLANG_IMAGE} as builder

ARG VERSION=v1.0
ARG GOPROXY=https://goproxy.cn,direct
ARG GOOS=linux

WORKDIR /build
COPY go.mod go.sum ./
COPY cmd/ cmd/
COPY pkg/ pkg/
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=${GOOS} GOPROXY=${GOPROXY} go build \
		-ldflags="-w -s -X 'main.version=${VERSION}'" \
		-o=tencent-cloud-controller-manager \
		cmd/tencent-cloud-controller-manager/main.go

################################################################################
##                               MAIN STAGE                                   ##
################################################################################
# Copy the manager into the distroless image.
FROM ${DISTROLESS_IMAGE}
COPY --from=builder /build/tencent-cloud-controller-manager /bin/tencent-cloud-controller-manager
ENTRYPOINT [ "/bin/tencent-cloud-controller-manager" ]