package main

import (
	"encoding/json"
	"os"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/check"
)

func main() {
	var request check.Request
	inputRequest(&request)

	b := s3resource.AwsConfigBuilder{
		AccessKey: request.Source.AccessKeyID,
		SecretKey: request.Source.SecretAccessKey,
		SessionToken: request.Source.SessionToken,
		RegionName: request.Source.RegionName,
		Endpoint: request.Source.Endpoint,
		DisableSSL: request.Source.DisableSSL,
		SkipSSLVerification: request.Source.SkipSSLVerification,
		AssumeRoleArn: request.Source.AssumeRoleArn,
	}

	awsConfig := b.Build()

	client := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		request.Source.UseV2Signing,
	)

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
