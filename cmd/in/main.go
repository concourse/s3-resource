package main

import (
	"encoding/json"
	"os"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/in"
)

func main() {
	if len(os.Args) < 2 {
		s3resource.Sayf("usage: %s <dest directory>\n", os.Args[0])
		os.Exit(1)
	}

	destinationDir := os.Args[1]

	var request in.Request
	inputRequest(&request)

	awsConfig, err := s3resource.NewAwsConfig(
		request.Source.AccessKeyID,
		request.Source.SecretAccessKey,
		request.Source.SessionToken,
		request.Source.AwsRoleARN,
		request.Source.RegionName,
		request.Source.SkipSSLVerification,
		request.Source.CABundle,
		request.Source.UseAwsCredsProvider,
	)
	if err != nil {
		s3resource.Fatal("error creating aws config", err)
	}

	endpoint := request.Source.Endpoint
	if len(request.Source.CloudfrontURL) != 0 {
		s3resource.Sayf("'cloudfront_url' is deprecated and no longer used. You only need to specify 'endpoint' now.")
	}

	client, err := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		endpoint,
		request.Source.DisableSSL,
		request.Source.UsePathStyle,
	)
	if err != nil {
		s3resource.Fatal("error creating s3 client", err)
	}

	command := in.NewCommand(client)

	response, err := command.Run(destinationDir, request)
	if err != nil {
		s3resource.Fatal("running command", err)
	}

	outputResponse(response)
}

func inputRequest(request *in.Request) {
	if err := json.NewDecoder(os.Stdin).Decode(request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}
}

func outputResponse(response in.Response) {
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
