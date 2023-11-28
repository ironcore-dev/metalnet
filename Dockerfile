# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.21 as builder

ARG GOARCH=''
ARG GITHUB_PAT=''

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY hack hack

ENV GOPRIVATE='github.com/onmetal/*'

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=ssh --mount=type=secret,id=github_pat \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GITHUB_PAT_PATH=/run/secrets/github_pat ./hack/setup-git-redirect.sh \
    && mkdir -p -m 0600 ~/.ssh \
    && ssh-keyscan github.com >> ~/.ssh/known_hosts \
    && go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY client/ client/
COPY controllers/ controllers/
COPY internal/ internal/
COPY encoding/ encoding/
COPY metalbond/ metalbond/
COPY netfns/ netfns/
COPY sysfs/ sysfs/
# Needed for version extraction by go build
COPY .git/ .git/

ARG TARGETOS TARGETARCH

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w -X main.buildVersion=$(git describe --tags)" -a -o manager main.go

FROM debian:bullseye-slim
WORKDIR /

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN update-ca-certificates

COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]
