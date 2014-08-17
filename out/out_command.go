package out

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
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
	paths := []string{}
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		paths = append(paths, path)
		return nil
	})

	pattern := request.Params.From
	matches, err := versions.Match(paths, pattern)
	if err != nil {
		return OutResponse{}, err
	}

	if len(matches) == 0 {
		return OutResponse{}, fmt.Errorf("no matches found for pattern: %s", pattern)
	}

	if len(matches) > 1 {
		return OutResponse{}, fmt.Errorf("more than one match found for pattern: %s", pattern)
	}

	match := matches[0]

	folderDestination := strings.HasSuffix(request.Params.To, "/")

	var remotePath string
	var remoteFilename string

	if folderDestination || request.Params.To == "" {
		remotePath = filepath.Join(request.Params.To, filepath.Base(match))
		remoteFilename = filepath.Base(remotePath)
	} else {
		compiled := regexp.MustCompile(request.Params.From)
		fileName := strings.TrimPrefix(match, sourceDir+"/")
		remotePath = compiled.ReplaceAllString(fileName, request.Params.To)
		remoteFilename = filepath.Base(remotePath)
	}

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
