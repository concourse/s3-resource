package in

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

var ErrMissingPath = errors.New("missing path in request")

type RequestURLProvider struct {
	s3Client s3resource.S3Client
}

func (up *RequestURLProvider) GetURL(request InRequest, remotePath string) string {
	return up.s3URL(request, remotePath)
}

func (up *RequestURLProvider) s3URL(request InRequest, remotePath string) string {
	return up.s3Client.URL(request.Source.Bucket, remotePath, request.Source.Private, request.Version.VersionID)
}

type InCommand struct {
	s3client    s3resource.S3Client
	urlProvider RequestURLProvider
}

func NewInCommand(s3client s3resource.S3Client) *InCommand {
	return &InCommand{
		s3client: s3client,
		urlProvider: RequestURLProvider{
			s3Client: s3client,
		},
	}
}

func (command *InCommand) Run(destinationDir string, request InRequest) (InResponse, error) {
	if ok, message := request.Source.IsValid(); !ok {
		return InResponse{}, errors.New(message)
	}

	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return InResponse{}, err
	}

	var remotePath string
	var versionNumber string
	var versionID string

	if request.Source.Regexp != "" {
		if request.Version.Path == "" {
			return InResponse{}, ErrMissingPath
		}

		remotePath = request.Version.Path

		extraction, ok := versions.Extract(remotePath, request.Source.Regexp)
		if !ok {
			return InResponse{}, fmt.Errorf("regex does not match provided version: %#v", request.Version)
		}

		versionNumber = extraction.VersionNumber
	} else {
		remotePath = request.Source.VersionedFile
		versionNumber = request.Version.VersionID
		versionID = request.Version.VersionID
	}

	err = command.downloadFile(
		request.Source.Bucket,
		remotePath,
		versionID,
		destinationDir,
		path.Base(remotePath),
	)

	if err != nil {
		return InResponse{}, err
	}

	if request.Params.Unpack {
		destinationPath := filepath.Join(destinationDir, path.Base(remotePath))
		mime := archiveMimetype(destinationPath)
		if mime == "" {
			return InResponse{}, fmt.Errorf("not an archive: %s", destinationPath)
		}

		err = extractArchive(mime, destinationPath)
		if err != nil {
			return InResponse{}, err
		}
	}

	url := command.urlProvider.GetURL(request, remotePath)
	if err = command.writeURLFile(destinationDir, url); err != nil {
		return InResponse{}, err
	}

	err = command.writeVersionFile(versionNumber, destinationDir)
	if err != nil {
		return InResponse{}, err
	}

	metadata := command.metadata(remotePath, request.Source.Private, url)

	if versionID == "" {
		return InResponse{
			Version: s3resource.Version{
				Path: remotePath,
			},
			Metadata: metadata,
		}, nil
	}

	return InResponse{
		Version: s3resource.Version{
			VersionID: versionID,
		},
		Metadata: metadata,
	}, nil
}

func (command *InCommand) writeURLFile(destDir string, url string) error {
	return ioutil.WriteFile(filepath.Join(destDir, "url"), []byte(url), 0644)
}

func (command *InCommand) writeVersionFile(versionNumber string, destDir string) error {
	return ioutil.WriteFile(filepath.Join(destDir, "version"), []byte(versionNumber), 0644)
}

func (command *InCommand) downloadFile(bucketName string, remotePath string, versionID string, destinationDir string, destinationFile string) error {
	localPath := filepath.Join(destinationDir, destinationFile)

	return command.s3client.DownloadFile(
		bucketName,
		remotePath,
		versionID,
		localPath,
	)
}

func (command *InCommand) metadata(remotePath string, private bool, url string) []s3resource.MetadataPair {
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
			Value: url,
		})
	}

	return metadata
}

func extractArchive(mime, filename string) error {
	destDir := filepath.Dir(filename)

	err := inflate(mime, filename, destDir)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %s", err)
	}

	if mime == "application/gzip" || mime == "application/x-gzip" {
		fileInfos, err := ioutil.ReadDir(destDir)
		if err != nil {
			return fmt.Errorf("failed to read dir: %s", err)
		}

		if len(fileInfos) != 1 {
			return fmt.Errorf("%d files found after gunzip; expected 1", len(fileInfos))
		}

		filename = filepath.Join(destDir, fileInfos[0].Name())
		mime = archiveMimetype(filename)
		if mime == "application/x-tar" {
			err = inflate(mime, filename, destDir)
			if err != nil {
				return fmt.Errorf("failed to extract archive: %s", err)
			}
		}
	}

	return nil
}
