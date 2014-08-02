package main

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/in"
	"github.com/concourse/s3-resource/versions"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

func main() {
	if len(os.Args) < 2 {
		s3resource.Sayf("usage: %s <dest directory>\n", os.Args[0])
		os.Exit(1)
	}

	destinationDir := os.Args[1]
	os.MkdirAll(destinationDir, 0777)

	var request in.InRequest

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}

	auth := aws.Auth{
		AccessKey: request.Source.AccessKeyID,
		SecretKey: request.Source.SecretAccessKey,
	}

	client := s3.New(auth, aws.USEast)
	bucket := client.Bucket(request.Source.Bucket)

	var filePath string
	if request.Version.Path == "" {
		extractions := versions.GetBucketFileVersions(request.Source)
		lastExtraction := extractions[len(extractions)-1]
		filePath = lastExtraction.Path
	} else {
		filePath = request.Version.Path
	}

	reader, err := bucket.GetReader(filePath)
	if err != nil {
		s3resource.Fatal("getting reader", err)
	}
	defer reader.Close()

	filename := path.Base(filePath)
	dest := filepath.Join(destinationDir, filename)
	file, err := os.Create(dest)
	if err != nil {
		s3resource.Fatal("opening destination file", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		s3resource.Fatal("writing output file", err)
	}

	response := in.InResponse{
		Version: s3resource.Version{
			Path: filePath,
		},
		Metadata: []in.MetadataPair{
			in.MetadataPair{
				Name:  "filename",
				Value: filename,
			},
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
