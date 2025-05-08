package main

import (
	"encoding/json"
	"os"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/check"
)

func main() {
	var request check.Request
	inputRequest(&request)

	awsConfig, err := s3resource.NewAwsConfig(
		request.Source.AccessKeyID,
		request.Source.SecretAccessKey,
		request.Source.SessionToken,
		request.Source.AwsRoleARN,
		request.Source.RegionName,
		request.Source.SkipSSLVerification,
		request.Source.UseAwsCredsProvider,
	)
	if err != nil {
		s3resource.Fatal("error creating aws config", err)
	}

	client, err := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		request.Source.Endpoint,
		request.Source.DisableSSL,
		request.Source.UsePathStyle,
	)
	if err != nil {
		s3resource.Fatal("error creating s3 client", err)
	}

	command := check.NewCommand(client)
	response, err := command.Run(request)
	if err != nil {
		s3resource.Fatal("running command", err)
	}

	outputResponse(response)
}

func inputRequest(request *check.Request) {
	if err := json.NewDecoder(os.Stdin).Decode(request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}
}

func outputResponse(response check.Response) {
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
