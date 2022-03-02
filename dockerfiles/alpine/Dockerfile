ARG base_image=alpine:latest
ARG builder_image=concourse/golang-builder

FROM ${builder_image} as builder
COPY . /go/src/github.com/concourse/s3-resource
WORKDIR /go/src/github.com/concourse/s3-resource
ENV CGO_ENABLED 0
RUN go mod download
RUN go build -o /assets/in ./cmd/in
RUN go build -o /assets/out ./cmd/out
RUN go build -o /assets/check ./cmd/check
RUN set -e; for pkg in $(go list ./...); do \
		go test -o "/tests/$(basename $pkg).test" -c $pkg; \
	done

FROM ${base_image} AS resource
RUN apk update && apk upgrade
RUN apk add --no-cache bash tzdata ca-certificates unzip zip gzip tar
COPY --from=builder assets/ /opt/resource/
RUN chmod +x /opt/resource/*

FROM resource AS tests
ARG S3_TESTING_ACCESS_KEY_ID
ARG S3_TESTING_SECRET_ACCESS_KEY
ARG S3_TESTING_SESSION_TOKEN
ARG S3_TESTING_AWS_ROLE_ARN
ARG S3_VERSIONED_TESTING_BUCKET
ARG S3_TESTING_BUCKET
ARG S3_TESTING_REGION
ARG S3_TESTING_NO_LARGE_UPLOAD
ARG S3_ENDPOINT
ARG TEST_SESSION_TOKEN
COPY --from=builder /tests /go-tests
WORKDIR /go-tests
RUN set -e; for test in /go-tests/*.test; do \
		$test; \
	done

FROM resource
