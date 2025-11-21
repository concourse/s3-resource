package in

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
)

var ErrMissingPath = errors.New("missing path in request")

type Command struct {
	s3client s3resource.S3Client
}

func NewCommand(s3client s3resource.S3Client) *Command {
	return &Command{
		s3client: s3client,
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
	var s3_uri string
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

		if request.Params.DownloadTags {
			err = command.downloadTags(
				request.Source.Bucket,
				remotePath,
				versionID,
				destinationDir,
			)
			if err != nil {
				return Response{}, err
			}
		}

		url, err = command.getURL(request, remotePath)
		if err != nil {
			return Response{}, err
		}
		err = command.writeURLFile(destinationDir, url)
		if err != nil {
			return Response{}, err
		}
		s3_uri = command.gets3URI(request, remotePath)
		if err = command.writeS3URIFile(destinationDir, s3_uri); err != nil {
			return Response{}, err
		}
	}

	err = command.writeVersionFile(destinationDir, versionNumber)
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
	return os.WriteFile(filepath.Join(destDir, "url"), []byte(url), 0644)
}

func (command *Command) writeS3URIFile(destDir string, s3_uri string) error {
	return os.WriteFile(filepath.Join(destDir, "s3_uri"), []byte(s3_uri), 0644)
}

func (command *Command) writeVersionFile(destDir string, versionNumber string) error {
	return os.WriteFile(filepath.Join(destDir, "version"), []byte(versionNumber), 0644)
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

func (command *Command) downloadTags(bucketName string, remotePath string, versionID string, destinationDir string) error {
	localPath := filepath.Join(destinationDir, "tags.json")

	return command.s3client.DownloadTags(
		bucketName,
		remotePath,
		versionID,
		localPath,
	)
}

func (command *Command) createInitialFile(destDir string, destFile string, data []byte) error {
	return os.WriteFile(filepath.Join(destDir, destFile), []byte(data), 0644)
}

func (command *Command) metadata(remotePath string, private bool, url string) []s3resource.MetadataPair {
	remoteFilename := filepath.Base(remotePath)

	metadata := []s3resource.MetadataPair{
		{
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

func (command *Command) getURL(request Request, remotePath string) (string, error) {
	return command.s3client.URL(request.Source.Bucket, remotePath, request.Source.Private, request.Version.VersionID)
}

func (command *Command) gets3URI(request Request, remotePath string) string {
	return "s3://" + request.Source.Bucket + "/" + remotePath
}

func extractArchive(mime, filename string) error {
	destDir := filepath.Dir(filename)

	err := inflate(mime, filename, destDir)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %s", err)
	}

	// Special handling for gzip and bzip2: check if there's a tar inside
	if mime == "application/gzip" || mime == "application/x-gzip" ||
		mime == "application/x-bzip2" || mime == "application/x-bzip" {

		compressionType := "gzip"
		if mime == "application/x-bzip2" || mime == "application/x-bzip" {
			compressionType = "bzip2"
		}

		fileInfos, err := os.ReadDir(destDir)
		if err != nil {
			return fmt.Errorf("failed to read dir: %s", err)
		}

		if len(fileInfos) != 1 {
			return fmt.Errorf("%d files found after %s decompression; expected 1", len(fileInfos), compressionType)
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
