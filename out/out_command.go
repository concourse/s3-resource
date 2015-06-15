package out

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

var ErrObjectVersioningNotEnabled = errors.New("object versioning not enabled")

type OutCommand struct {
	s3client s3resource.S3Client
}

func NewOutCommand(s3client s3resource.S3Client) *OutCommand {
	return &OutCommand{
		s3client: s3client,
	}
}

func (command *OutCommand) Run(sourceDir string, request OutRequest) (OutResponse, error) {
	if ok, message := request.Source.IsValid(); !ok {
		return OutResponse{}, errors.New(message)
	}

	localPath, err := command.match(sourceDir, request.Params.From)
	if err != nil {
		return OutResponse{}, err
	}

	remotePath := command.remotePath(request, localPath, sourceDir)

	bucketName := request.Source.Bucket
	versionID, err := command.s3client.UploadFile(
		bucketName,
		remotePath,
		localPath,
	)
	if err != nil {
		return OutResponse{}, err
	}

	version := s3resource.Version{}

	if request.Source.VersionedFile != "" {
		if versionID == "" {
			return OutResponse{}, ErrObjectVersioningNotEnabled
		}

		version.VersionID = versionID
	} else {
		version.Path = remotePath
	}

	return OutResponse{
		Version:  version,
		Metadata: command.metadata(bucketName, remotePath, request.Source.Private, versionID),
	}, nil
}

func (command *OutCommand) remotePath(request OutRequest, localPath string, sourceDir string) string {
	if request.Source.VersionedFile != "" {
		return request.Source.VersionedFile
	}

	folderDestination := strings.HasSuffix(request.Params.To, "/")
	if folderDestination || request.Params.To == "" {
		return filepath.Join(request.Params.To, filepath.Base(localPath))
	}

	compiled := regexp.MustCompile(request.Params.From)
	fileName := strings.TrimPrefix(localPath, sourceDir+"/")
	return compiled.ReplaceAllString(fileName, request.Params.To)
}

func (command *OutCommand) match(sourceDir, pattern string) (string, error) {
	paths := []string{}
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		paths = append(paths, path)
		return nil
	})

	matches, err := versions.MatchUnanchored(paths, pattern)
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no matches found for pattern: %s", pattern)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("more than one match found for pattern: %s\n%v", pattern, matches)
	}

	return matches[0], nil
}

func (command *OutCommand) metadata(bucketName, remotePath string, private bool, versionID string) []s3resource.MetadataPair {
	remoteFilename := filepath.Base(remotePath)

	metadata := []s3resource.MetadataPair{
		s3resource.MetadataPair{
			Name:  "filename",
			Value: remoteFilename,
		},
	}

	if !private {
		metadata = append(metadata, s3resource.MetadataPair{
			Name:  "url",
			Value: command.s3client.URL(bucketName, remotePath, false, versionID),
		})
	}

	return metadata
}
