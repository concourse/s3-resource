ARG base_image=cgr.dev/chainguard/wolfi-base
ARG builder_image=cgr.dev/chainguard/go

ARG TARGETOS
ARG TARGETARCH
ARG GOAMD64=v3
ARG GOARM64=v8.2

FROM --platform=$BUILDPLATFORM ${builder_image} AS builder
WORKDIR /go/src/github.com/concourse/s3-resource
COPY . .
RUN go mod download
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOAMD64=${GOAMD64} GOARM64=${GOARM64} go build -o /assets/in ./cmd/in
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOAMD64=${GOAMD64} GOARM64=${GOARM64} go build -o /assets/out ./cmd/out
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOAMD64=${GOAMD64} GOARM64=${GOARM64} go build -o /assets/check ./cmd/check
RUN set -e; for pkg in $(go list ./...); do \
		go test -o "/tests/$(basename $pkg).test" -c $pkg; \
	done

FROM ${base_image} AS resource
RUN apk add --no-cache tzdata ca-certificates unzip zip
COPY --from=builder /assets/ /opt/resource/
RUN chmod +x /opt/resource/*

FROM resource AS tests
ARG S3_TESTING_ACCESS_KEY_ID
ARG S3_TESTING_SECRET_ACCESS_KEY
ARG S3_TESTING_SESSION_TOKEN
ARG S3_TESTING_AWS_ROLE_ARN
ARG S3_VERSIONED_TESTING_BUCKET
ARG S3_TESTING_BUCKET
ARG S3_TESTING_REGION
ARG S3_ENDPOINT
ARG S3_USE_PATH_STYLE
ARG TEST_SESSION_TOKEN
COPY --from=builder /tests /go-tests
WORKDIR /go-tests
RUN set -e; for test in /go-tests/*.test; do \
		$test; \
	done

FROM resource
