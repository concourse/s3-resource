package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/concourse/s3-resource/in"
	"github.com/mitchellh/colorstring"
	"github.com/rlmcpherson/s3gof3r"
)

func main() {
	if len(os.Args) < 2 {
		sayf("usage: %s <dest directory>\n", os.Args[0])
		os.Exit(1)
	}

	var request in.InRequest

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("reading request from stdin", err)
	}

	keys := s3gof3r.Keys{
		AccessKey: request.Source.AccessKeyID,
		SecretKey: request.Source.SecretAccessKey,
	}

	client := s3gof3r.New("", keys)
	bucket := client.Bucket(request.Source.Bucket)

	reader, _, err := bucket.GetReader(request.Version.Path, nil)
	if err != nil {
		fatal("getting reader", err)
	}

	filename := path.Base(request.Version.Path)
	dest := filepath.Join(os.Args[1], filename)
	file, err := os.Create(dest)
	if err != nil {
		fatal("opening destination file", err)
	}

	_, err = io.Copy(file, reader)
	if err != nil {
		fatal("writing output file", err)
	}

	response := in.InResponse{
		Version: request.Version,
		Metadata: []in.MetadataPair{
			in.MetadataPair{
				Name:  "filename",
				Value: filename,
			},
		},
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("writing response to stdout", err)
	}
}

func sayf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
}

func fatal(doing string, err error) {
	sayf(colorstring.Color("[red]error %s: %s\n"), doing, err)
	os.Exit(1)
}
