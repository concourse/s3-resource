FROM ubuntu:14.04

RUN apt-get update && apt-get -y install wget git

ENV PATH /usr/local/go/bin:$PATH
ENV GOPATH /tmp/go/src/github.com/concourse/s3-resource/Godeps/_workspace:/tmp/go

ADD . /tmp/go/src/github.com/concourse/s3-resource

RUN wget -qO- https://storage.googleapis.com/golang/go1.3.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
      go build -o /opt/resource/check github.com/concourse/s3-resource/cmd/check && \
      go build -o /opt/resource/in github.com/concourse/s3-resource/cmd/in && \
      rm -rf /tmp/go /usr/local/go
