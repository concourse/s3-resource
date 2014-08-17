package out

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/s3-resource"
)

type OutCommand struct {
	s3client s3resource.S3Client
}

func NewOutCommand(s3client s3resource.S3Client) *OutCommand {
	return &OutCommand{
		s3client: s3client,
	}
}

func (command *OutCommand) Run(sourceDir string, request OutRequest) (OutResponse, error) {
	sourceGlob := filepath.Join(sourceDir, request.Params.From)
	matches, err := filepath.Glob(sourceGlob)
	if err != nil {
		return OutResponse{}, err
	}

	if len(matches) == 0 {
		return OutResponse{}, fmt.Errorf("no matches found for pattern: %s", sourceGlob)
	}

	if len(matches) > 1 {
		return OutResponse{}, fmt.Errorf("more than one match found for pattern: %s", sourceGlob)
	}

	match := matches[0]

	remotePath := filepath.Join(request.Params.To, filepath.Base(match))
	remoteFilename := filepath.Base(remotePath)

	err = command.s3client.UploadFile(
		request.Source.Bucket,
		remotePath,
		match,
	)
	if err != nil {
		return OutResponse{}, err
	}

	return OutResponse{
		Version: s3resource.Version{
			Path: remotePath,
		},
		Metadata: []s3resource.MetadataPair{
			s3resource.MetadataPair{
				Name:  "filename",
				Value: remoteFilename,
			},
		},
	}, nil
}
