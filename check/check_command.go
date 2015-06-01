package check

import (
	"errors"
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
	if ok, message := request.Source.IsValid(); !ok {
		return CheckResponse{}, errors.New(message)
	}

	if request.Source.Regexp != "" {
		return command.checkByRegex(request)
	} else {
		return command.checkByVersionedFile(request)
	}
}

func (command *CheckCommand) checkByRegex(request CheckRequest) (CheckResponse, error) {
	extractions := versions.GetBucketFileVersions(command.s3client, request.Source)
	response := CheckResponse{}

	if len(extractions) == 0 {
		return response, nil
	}

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

func (command *CheckCommand) checkByVersionedFile(request CheckRequest) (CheckResponse, error) {
	response := CheckResponse{}

	bucketVersions, err := command.s3client.BucketFileVersions(request.Source.Bucket, request.Source.VersionedFile)

	if err != nil {
		s3resource.Fatal("finding versions", err)
	}

	if len(bucketVersions) == 0 {
		return response, nil
	}

	if request.Version.VersionID == "" {
		version := s3resource.Version{
			VersionID: bucketVersions[0],
		}

		response = append(response, version)
	} else {
		track := false

		for i := len(bucketVersions) - 1; i >= 0; i-- {
			bucketVersion := bucketVersions[i]

			if track {
				version := s3resource.Version{
					VersionID: bucketVersion,
				}

				response = append(response, version)
			}

			if bucketVersion == request.Version.VersionID {
				track = true
			}
		}
	}

	return response, nil
}
