package main

import (
	"encoding/json"
	"os"

	"fmt"
	"net/url"
	"strings"

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
	)
	if err != nil {
		s3resource.Fatal("error creating aws config", err)
	}

	s3PathStyle := true
	endpoint := request.Source.Endpoint
	if len(request.Source.CloudfrontURL) != 0 {
		cloudfrontUrl, err := url.ParseRequestURI(request.Source.CloudfrontURL)
		if err != nil {
			s3resource.Fatal("parsing 'cloudfront_url'", err)
		}
		s3PathStyle = false

		splitResult := strings.Split(cloudfrontUrl.Host, ".")
		if len(splitResult) < 2 {
			s3resource.Fatal("verifying 'cloudfront_url'", fmt.Errorf("'%s' doesn't have enough dots ('.'), a typical format is 'https://d111111abcdef8.cloudfront.net'", request.Source.CloudfrontURL))
		}
		request.Source.Bucket = strings.Split(cloudfrontUrl.Host, ".")[0]
		fqdn := strings.SplitAfterN(cloudfrontUrl.Host, ".", 2)[1]
		endpoint = fmt.Sprintf("%s://%s", cloudfrontUrl.Scheme, fqdn)
	}

	client, err := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		endpoint,
		request.Source.DisableSSL,
		s3PathStyle,
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
