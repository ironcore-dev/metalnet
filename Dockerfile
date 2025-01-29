# Build the manager binary
FROM golang:1.23 as builder

ARG GOARCH=''

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY hack hack

ENV GOPRIVATE=github.com/onmetal/*,github.com/ironcore-dev/*

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY client/ client/
COPY control/ control/
COPY controllers/ controllers/
COPY internal/ internal/
COPY encoding/ encoding/
COPY metalbond/ metalbond/
COPY netfns/ netfns/
COPY sysfs/ sysfs/
# Needed for version extraction by go build
COPY .git/ .git/

ARG TARGETOS
ARG TARGETARCH

# Build
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w -X main.buildVersion=$(git describe --tags)" -a -o manager main.go

FROM debian:bullseye-slim
WORKDIR /

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN update-ca-certificates

COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]
