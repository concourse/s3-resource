package check

import (
	"fmt"
	
	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

type CheckCommand struct {
	s3client s3resource.S3Client
}

func NewCheckCommand(s3client s3resource.S3Client) *CheckCommand {
	return &CheckCommand{
		s3client: s3client,
	}
}

func (command *CheckCommand) Run(request CheckRequest) (CheckResponse, error) {
	extractions := versions.GetBucketFileVersions(command.s3client, request.Source)
	response := CheckResponse{}

	if request.Version.Path == "" {
		lastExtraction := extractions[len(extractions)-1]
		version := s3resource.Version{
			Path: lastExtraction.Path,
		}
		response = append(response, version)
	} else {
		lastVersion, ok := versions.Extract(request.Version.Path, request.Source.Regexp)
		if !ok {
			return response, fmt.Errorf("version number could not be found in: %s", request.Version.Path)
		}

		for _, extraction := range extractions {
			if extraction.Version.GreaterThan(lastVersion.Version) {
				version := s3resource.Version{
					Path: extraction.Path,
				}
				response = append(response, version)
			}
		}
	}
	
	return response, nil
}
