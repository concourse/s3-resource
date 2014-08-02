package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/concourse/s3-resource/check"
	"github.com/mitchellh/colorstring"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

func main() {
	var request check.CheckRequest

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("reading request from stdin", err)
	}

	auth, err := aws.GetAuth(
		request.Source.AccessKeyID,
		request.Source.SecretAccessKey,
	)
	if err != nil {
		fatal("setting up aws auth", err)
	}

	// TODO: more regions
	client := s3.New(auth, aws.USEast)
	bucket := client.Bucket(request.Source.Bucket)
	entries, err := bucket.GetBucketContents()
	if err != nil {
		fatal("listing buckets contents", err)
	}

	paths := make([]string, 0, len(*entries))
	for entry := range *entries {
		paths = append(paths, entry)
	}

	matchingPaths, err := check.Match(paths, request.Source.Glob)
	if err != nil {
		fatal("finding matches", err)
	}

	var extractions = make(check.Extractions, 0, len(matchingPaths))
	for _, path := range matchingPaths {
		extraction, ok := check.Extract(path)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)

	response := check.CheckResponse{}

	if request.Version.Path == "" {
		lastExtraction := extractions[len(extractions)-1]
		version := check.Version{
			Path: lastExtraction.Path,
		}
		response = append(response, version)
	} else {
		lastVersion, ok := check.Extract(request.Version.Path)

		if !ok {
			fatal("extracting version from last successful check", err)
		}

		for _, extraction := range extractions {
			if extraction.Version > lastVersion.Version {
				version := check.Version{
					Path: extraction.Path,
				}
				response = append(response, version)
			}
		}
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
