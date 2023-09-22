# Build the manager binary
FROM docker.io/library/golang:1.20-alpine as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/onlyfyio/aws-iam-operator"
LABEL org.opencontainers.image.url="https://github.com/onlyfyio/aws-iam-operator"
LABEL org.opencontainers.image.authors="moulickaggarwal@gmail.com"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.title="AWS IAM Operator"
LABEL org.opencontainers.image.base.name="662412797287.dkr.ecr.eu-central-1.amazonaws.com/aws-iam-operator:latest"

WORKDIR /
COPY --from=builder /workspace/manager /
ENTRYPOINT ["/manager"]
