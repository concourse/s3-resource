package in

import (
	"os"
	"path"
	"path/filepath"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

type InCommand struct {
	s3client s3resource.S3Client
}

func NewInCommand(s3client s3resource.S3Client) *InCommand {
	return &InCommand{
		s3client: s3client,
	}
}

func (command *InCommand) Run(destinationDir string, request InRequest) (InResponse, error) {
	err := command.createDirectory(destinationDir)
	if err != nil {
		return InResponse{}, err
	}

	remotePath := command.pathToDownload(request)
	remoteFilename := path.Base(remotePath)
	err = command.downloadFile(
		request.Source.Bucket,
		remotePath,
		destinationDir,
	)
	if err != nil {
		return InResponse{}, err
	}

	return InResponse{
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

func (command *InCommand) pathToDownload(request InRequest) string {
	if request.Version.Path == "" {
		extractions := versions.GetBucketFileVersions(command.s3client, request.Source)
		lastExtraction := extractions[len(extractions)-1]
		return lastExtraction.Path
	}

	return request.Version.Path
}

func (command *InCommand) createDirectory(destDir string) error {
	return os.MkdirAll(destDir, 0755)
}

func (command *InCommand) downloadFile(bucketName string, remotePath string, destinationDir string) error {
	remoteFilename := path.Base(remotePath)
	localPath := filepath.Join(destinationDir, remoteFilename)

	return command.s3client.DownloadFile(
		bucketName,
		remotePath,
		localPath,
	)
}
