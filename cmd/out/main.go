package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/out"
	"github.com/rlmcpherson/s3gof3r"
)

func main() {
	if len(os.Args) < 2 {
		s3resource.Sayf("usage: %s <sources directory>\n", os.Args[0])
		os.Exit(1)
	}

	var request out.OutRequest

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}

	sourceDir := os.Args[1]

	auth := s3gof3r.Keys{
		AccessKey: request.Source.AccessKeyID,
		SecretKey: request.Source.SecretAccessKey,
	}

	client := s3gof3r.New("", auth)
	bucket := client.Bucket(request.Source.Bucket)

	sourceGlob := filepath.Join(sourceDir, request.Params.From)
	matches, err := filepath.Glob(sourceGlob)
	if err != nil {
		s3resource.Fatal("getting matches", err)
	}

	if len(matches) == 0 {
		s3resource.Fatal("counting matches", errors.New("there were no files matching the given pattern"))
	}

	if len(matches) > 1 {
		s3resource.Fatal("counting matches", errors.New("there was more than one file found matching that pattern"))
	}

	match := matches[0]

	s3resource.Sayf("uploading file: %s\n", match)

	file, err := os.Open(match)
	if err != nil {
		s3resource.Fatal("opening local file", err)
	}

	destinationName := filepath.Base(match)
	destinationPath := filepath.Join(request.Params.To, destinationName)
	writer, err := bucket.PutWriter(destinationPath, nil, nil)
	if err != nil {
		s3resource.Fatal("getting remote writer", err)
	}

	if _, err = io.Copy(writer, file); err != nil {
		s3resource.Fatal("writing file to remote", err)
	}

	if err = writer.Close(); err != nil {
		s3resource.Fatal("closing remote writer", err)
	}

	if err = file.Close(); err != nil {
		s3resource.Fatal("closing local file", err)
	}

	response := out.OutResponse{
		Version: s3resource.Version{
			Path: destinationPath,
		},
		Metadata: []s3resource.MetadataPair{
			s3resource.MetadataPair{
				Name:  "filename",
				Value: destinationName,
			},
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
