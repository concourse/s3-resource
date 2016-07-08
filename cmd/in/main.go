package main

import (
	"encoding/json"
	"os"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/in"
)

func main() {
	if len(os.Args) < 2 {
		s3resource.Sayf("usage: %s <dest directory>\n", os.Args[0])
		os.Exit(1)
	}

	destinationDir := os.Args[1]

	var request in.InRequest
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

	command := in.NewInCommand(client)

	response, err := command.Run(destinationDir, request)
	if err != nil {
		s3resource.Fatal("running command", err)
	}

	outputResponse(response)
}

func inputRequest(request *in.InRequest) {
	if err := json.NewDecoder(os.Stdin).Decode(request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}
}

func outputResponse(response in.InResponse) {
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
