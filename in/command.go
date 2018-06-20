package in

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

var ErrMissingPath = errors.New("missing path in request")

type RequestURLProvider struct {
	s3Client s3resource.S3Client
}

func (up *RequestURLProvider) GetURL(request Request, remotePath string) string {
	return up.s3URL(request, remotePath)
}

func (up *RequestURLProvider) s3URL(request Request, remotePath string) string {
	return up.s3Client.URL(request.Source.Bucket, remotePath, request.Source.Private, request.Version.VersionID)
}

type Command struct {
	s3client    s3resource.S3Client
	urlProvider RequestURLProvider
}

func NewCommand(s3client s3resource.S3Client) *Command {
	return &Command{
		s3client: s3client,
		urlProvider: RequestURLProvider{
			s3Client: s3client,
		},
	}
}

func (command *Command) Run(destinationDir string, request Request) (Response, error) {
	if ok, message := request.Source.IsValid(); !ok {
		return Response{}, errors.New(message)
	}

	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return Response{}, err
	}

	var remotePath string
	var versionNumber string
	var versionID string
	var url string
	var isInitialVersion bool
	var skipDownload bool

	if request.Source.Regexp != "" {
		if request.Version.Path == "" {
			return Response{}, ErrMissingPath
		}

		remotePath = request.Version.Path

		extraction, ok := versions.Extract(remotePath, request.Source.Regexp)
		if !ok {
			return Response{}, fmt.Errorf("regex does not match provided version: %#v", request.Version)
		}

		versionNumber = extraction.VersionNumber

		isInitialVersion = request.Source.InitialPath != "" && request.Version.Path == request.Source.InitialPath
	} else {
		remotePath = request.Source.VersionedFile
		versionNumber = request.Version.VersionID
		versionID = request.Version.VersionID

		isInitialVersion = request.Source.InitialVersion != "" && request.Version.VersionID == request.Source.InitialVersion
	}

	if isInitialVersion {
		if request.Source.InitialContentText != "" || request.Source.InitialContentBinary == "" {
			err = command.createInitialFile(destinationDir, path.Base(remotePath), []byte(request.Source.InitialContentText))
			if err != nil {
				return Response{}, err
			}
		}
		if request.Source.InitialContentBinary != "" {
			b, err := base64.StdEncoding.DecodeString(request.Source.InitialContentBinary)
			if err != nil {
				return Response{}, errors.New("failed to decode initial_content_binary, make sure it's base64 encoded")
			}
			err = command.createInitialFile(destinationDir, path.Base(remotePath), b)
			if err != nil {
				return Response{}, err
			}
		}
	} else {

		if request.Params.SkipDownload != "" {
			skipDownload, err = strconv.ParseBool(request.Params.SkipDownload)
			if err != nil {
				return Response{}, fmt.Errorf("skip_download defined but invalid value: %s", request.Params.SkipDownload)
			}
		} else {
			skipDownload = request.Source.SkipDownload
		}

		if !skipDownload {
			err = command.downloadFile(
				request.Source.Bucket,
				remotePath,
				versionID,
				destinationDir,
				path.Base(remotePath),
			)
			if err != nil {
				return Response{}, err
			}

			if request.Params.Unpack {
				destinationPath := filepath.Join(destinationDir, path.Base(remotePath))
				mime := archiveMimetype(destinationPath)
				if mime == "" {
					return Response{}, fmt.Errorf("not an archive: %s", destinationPath)
				}

				err = extractArchive(mime, destinationPath)
				if err != nil {
					return Response{}, err
				}
			}
		}
		url = command.urlProvider.GetURL(request, remotePath)
		if err = command.writeURLFile(destinationDir, url); err != nil {
			return Response{}, err
		}
	}

	err = command.writeVersionFile(versionNumber, destinationDir)
	if err != nil {
		return Response{}, err
	}

	metadata := command.metadata(remotePath, request.Source.Private, url)

	if versionID == "" {
		return Response{
			Version: s3resource.Version{
				Path: remotePath,
			},
			Metadata: metadata,
		}, nil
	}

	return Response{
		Version: s3resource.Version{
			VersionID: versionID,
		},
		Metadata: metadata,
	}, nil
}

func (command *Command) writeURLFile(destDir string, url string) error {
	return ioutil.WriteFile(filepath.Join(destDir, "url"), []byte(url), 0644)
}

func (command *Command) writeVersionFile(versionNumber string, destDir string) error {
	return ioutil.WriteFile(filepath.Join(destDir, "version"), []byte(versionNumber), 0644)
}

func (command *Command) downloadFile(bucketName string, remotePath string, versionID string, destinationDir string, destinationFile string) error {
	localPath := filepath.Join(destinationDir, destinationFile)

	return command.s3client.DownloadFile(
		bucketName,
		remotePath,
		versionID,
		localPath,
	)
}

func (command *Command) createInitialFile(destDir string, destFile string, data []byte) error {
	return ioutil.WriteFile(filepath.Join(destDir, destFile), []byte(data), 0644)
}

func (command *Command) metadata(remotePath string, private bool, url string) []s3resource.MetadataPair {
	remoteFilename := filepath.Base(remotePath)

	metadata := []s3resource.MetadataPair{
		s3resource.MetadataPair{
			Name:  "filename",
			Value: remoteFilename,
		},
	}

	if url != "" && !private {
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
