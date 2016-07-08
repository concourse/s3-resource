package main

import (
	"encoding/json"
	"os"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/check"
)

func main() {
	var request check.CheckRequest
	inputRequest(&request)

	awsConfig := s3resource.NewAwsConfig(
		request.Source.AccessKeyID,
		request.Source.SecretAccessKey,
		request.Source.RegionName,
		request.Source.Endpoint,
		request.Source.DisableSSL,
	)

	client := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
	)

	command := check.NewCheckCommand(client)
	response, err := command.Run(request)
	if err != nil {
		s3resource.Fatal("running command", err)
	}

	outputResponse(response)
}

func inputRequest(request *check.CheckRequest) {
	if err := json.NewDecoder(os.Stdin).Decode(request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}
}

func outputResponse(response check.CheckResponse) {
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
