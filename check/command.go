package check

import (
	"errors"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

type Command struct {
	s3client s3resource.S3Client
}

func NewCommand(s3client s3resource.S3Client) *Command {
	return &Command{
		s3client: s3client,
	}
}

func (command *Command) Run(request Request) (Response, error) {
	if ok, message := request.Source.IsValid(); !ok {
		return Response{}, errors.New(message)
	}

	if request.Source.Regexp != "" {
		return command.checkByRegex(request), nil
	} else {
		return command.checkByVersionedFile(request), nil
	}
}

func (command *Command) checkByRegex(request Request) Response {
	extractions := versions.GetBucketFileVersions(command.s3client, request.Source)

	if request.Source.InitialPath != "" {
		extraction, ok := versions.Extract(request.Source.InitialPath, request.Source.Regexp)
		if ok {
			extractions = append([]versions.Extraction{extraction}, extractions...)
		}
	}

	if len(extractions) == 0 {
		return nil
	}

	lastVersion, matched := versions.Extract(request.Version.Path, request.Source.Regexp)
	if !matched {
		return latestVersion(extractions)
	} else {
		return newVersions(lastVersion, extractions)
	}
}

func (command *Command) checkByVersionedFile(request Request) Response {
	response := Response{}

	bucketVersions, err := command.s3client.BucketFileVersions(request.Source.Bucket, request.Source.VersionedFile)

	if err != nil {
		s3resource.Fatal("finding versions", err)
	}

	if request.Source.InitialVersion != "" {
		bucketVersions = append(bucketVersions, request.Source.InitialVersion)
	}

	if len(bucketVersions) == 0 {
		return response
	}

	requestVersionIndex := -1

	if request.Version.VersionID != "" {
		for i, bucketVersion := range bucketVersions {
			if bucketVersion == request.Version.VersionID {
				requestVersionIndex = i
				break
			}
		}
	}

	if requestVersionIndex == -1 {
		version := s3resource.Version{
			VersionID: bucketVersions[0],
		}
		response = append(response, version)
	} else {
		for i := requestVersionIndex; i >= 0; i-- {
			version := s3resource.Version{
				VersionID: bucketVersions[i],
			}
			response = append(response, version)
		}
	}

	return response
}

func latestVersion(extractions versions.Extractions) Response {
	lastExtraction := extractions[len(extractions)-1]
	return []s3resource.Version{{Path: lastExtraction.Path}}
}

func newVersions(lastVersion versions.Extraction, extractions versions.Extractions) Response {
	response := Response{}

	for _, extraction := range extractions {
		if extraction.Version.Compare(lastVersion.Version) >= 0 {
			version := s3resource.Version{
				Path: extraction.Path,
			}
			response = append(response, version)
		}
	}

	return response
}
