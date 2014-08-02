package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/check"
	"github.com/concourse/s3-resource/versions"
)

func main() {
	var request check.CheckRequest

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		s3resource.Fatal("reading request from stdin", err)
	}

	extractions := versions.GetBucketFileVersions(request.Source)
	response := check.CheckResponse{}

	if request.Version.Path == "" {
		lastExtraction := extractions[len(extractions)-1]
		version := s3resource.Version{
			Path: lastExtraction.Path,
		}
		response = append(response, version)
	} else {
		lastVersion, ok := versions.Extract(request.Version.Path)

		if !ok {
			s3resource.Fatal(
				"extracting version from last successful check",
				fmt.Errorf("version number could not be found in: %s", request.Version.Path),
			)
		}

		for _, extraction := range extractions {
			if extraction.Version > lastVersion.Version {
				version := s3resource.Version{
					Path: extraction.Path,
				}
				response = append(response, version)
			}
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		s3resource.Fatal("writing response to stdout", err)
	}
}
