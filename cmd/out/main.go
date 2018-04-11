package main

import (
	"encoding/json"
	"os"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/out"
)

func main() {
	if len(os.Args) < 2 {
		s3resource.Sayf("usage: %s <sources directory>\n", os.Args[0])
		os.Exit(1)
	}

	var request out.Request
	inputRequest(&request)

	sourceDir := os.Args[1]

	awsConfig := s3resource.NewAwsConfig(
		request.Source.AccessKeyID,
		request.Source.SecretAccessKey,
		request.Source.SessionToken,
		request.Source.RegionName,
		request.Source.Endpoint,
		request.Source.DisableSSL,
		request.Source.SkipSSLVerification,
	)

	client := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		request.Source.UseV2Signing,
	)

	command := out.NewCommand(os.Stderr, client)
	response, err := command.Run(sourceDir, request)
	if err != nil {
		s3resource.Fatal("running command", err)
	}

	outputResponse(response)
}

func inputRequest(request *out.Request) {
	if err := json.NewDecoder(os.Stdin).Decode(request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}
}

func outputResponse(response out.Response) {
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
