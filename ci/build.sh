#!/bin/bash

set -e -u -x

export GOPATH=$PWD/gopath
export PATH=$GOPATH/bin:$PATH

BUILD_DIR=$PWD/built-resource

cd $GOPATH/src/github.com/concourse/s3-resource

mkdir -p assets
go build -o assets/in ./cmd/in
go build -o assets/out ./cmd/out
go build -o assets/check ./cmd/check

cp -a assets/ Dockerfile $BUILD_DIR
