package main

import (
	"encoding/json"
	"os"

	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/concourse/s3-resource"
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

	if len(request.Source.CloudfrontURL) != 0 {
		cloudfrontUrl, err := url.ParseRequestURI(request.Source.CloudfrontURL)
		if err != nil {
			s3resource.Fatal("parsing 'cloudfront_url'", err)
		}
		awsConfig.S3ForcePathStyle = aws.Bool(false)

		splitResult := strings.Split(cloudfrontUrl.Host, ".")
		if len(splitResult) < 2 {
			s3resource.Fatal("verifying 'cloudfront_url'", fmt.Errorf("'%s' doesn't have enough dots ('.'), a typical format is 'https://d111111abcdef8.cloudfront.net'", request.Source.CloudfrontURL))
		}
		request.Source.Bucket = strings.Split(cloudfrontUrl.Host, ".")[0]
		fqdn := strings.SplitAfterN(cloudfrontUrl.Host, ".", 2)[1]
		awsConfig.Endpoint = aws.String(fmt.Sprintf("%s://%s", cloudfrontUrl.Scheme, fqdn))
	}

	client := s3resource.NewS3Client(
		os.Stderr,
		awsConfig,
		request.Source.UseV2Signing,
	)

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
