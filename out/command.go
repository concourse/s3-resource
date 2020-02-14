package out

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
	"github.com/fatih/color"
)

var ErrObjectVersioningNotEnabled = errors.New("object versioning not enabled")
var ErrorColor = color.New(color.FgWhite, color.BgRed, color.Bold)
var BlinkingErrorColor = color.New(color.BlinkSlow, color.FgWhite, color.BgRed, color.Bold)

func init() {
	ErrorColor.EnableColor()
}

type Command struct {
	stderr   io.Writer
	s3client s3resource.S3Client
}

func NewCommand(stderr io.Writer, s3client s3resource.S3Client) *Command {
	return &Command{
		stderr:   stderr,
		s3client: s3client,
	}
}

func (command *Command) Run(sourceDir string, request Request) (Response, error) {
	if request.Params.From != "" || request.Params.To != "" {
		command.printDeprecationWarning()
	}

	if ok, message := request.Source.IsValid(); !ok {
		return Response{}, errors.New(message)
	}
	if request.Params.File != "" && request.Params.From != "" {
		return Response{}, errors.New("contains both file and from")
	}

	localPath, err := command.match(request.Params, sourceDir)
	if err != nil {
		return Response{}, err
	}

	remotePath := command.remotePath(request, localPath, sourceDir)

	bucketName := request.Source.Bucket

	response, done, err := command.handleOvewriteProtection(request, bucketName, remotePath)
	if done {
		return response, err
	}

	options := s3resource.NewUploadFileOptions()

	if request.Params.Acl != "" {
		options.Acl = request.Params.Acl
	}

	options.ContentType = request.Params.ContentType
	options.ServerSideEncryption = request.Source.ServerSideEncryption
	options.KmsKeyId = request.Source.SSEKMSKeyId
	options.DisableMultipart = request.Source.DisableMultipart

	versionID, err := command.s3client.UploadFile(
		bucketName,
		remotePath,
		localPath,
		options,
	)
	if err != nil {
		return Response{}, err
	}

	version := s3resource.Version{}

	if request.Source.VersionedFile != "" {
		if versionID == "" {
			return Response{}, ErrObjectVersioningNotEnabled
		}

		version.VersionID = versionID
	} else {
		version.Path = remotePath
	}

	return Response{
		Version:  version,
		Metadata: command.metadata(bucketName, remotePath, request.Source.Private, versionID),
	}, nil
}

func (command *Command) handleOvewriteProtection(request Request, bucketName string, remotePath string) (Response, bool, error) {
	onOverwrite := request.Source.OnOverwrite
	if onOverwrite == "" || onOverwrite == "allow" {
		return Response{}, false, nil
	}

	exists, err := command.s3client.FileExists(bucketName, remotePath)
	if err != nil {
		return Response{}, true, err
	}
	if !exists {
		return Response{}, false, nil
	}

	if onOverwrite == "ignore" {
		command.stderr.Write([]byte(fmt.Sprintf(
			"Nothing was written to s3. File %q already exists in bucket %q and on_overwrite is set to %q.",
			remotePath, bucketName, onOverwrite,
		)))
		return Response{
			Version:  s3resource.Version{Path: remotePath},
			Metadata: command.metadata(bucketName, remotePath, request.Source.Private, ""),
		}, true, nil
	}

	return Response{}, true, fmt.Errorf("file %q already exists in bucket %q and on_overwrite is set to %q", remotePath, bucketName, onOverwrite)
}

func (command *Command) remotePath(request Request, localPath string, sourceDir string) string {
	if request.Source.VersionedFile != "" {
		return request.Source.VersionedFile
	}

	if request.Params.To == "" && request.Params.From == "" && request.Source.Regexp != "" {
		return filepath.Join(parentDir(request.Source.Regexp), filepath.Base(localPath))
	}

	folderDestination := strings.HasSuffix(request.Params.To, "/")
	if folderDestination || request.Params.To == "" {
		return filepath.Join(request.Params.To, filepath.Base(localPath))
	}

	compiled := regexp.MustCompile(request.Params.From)
	fileName := strings.TrimPrefix(localPath, sourceDir+"/")
	return compiled.ReplaceAllString(fileName, request.Params.To)
}

func parentDir(regexp string) string {
	return regexp[:strings.LastIndex(regexp, "/")+1]
}

func (command *Command) match(params Params, sourceDir string) (string, error) {
	var matches []string
	var err error
	var pattern string

	if params.File != "" {
		pattern = params.File
		matches, err = filepath.Glob(filepath.Join(sourceDir, pattern))
	} else {
		paths := []string{}
		filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			paths = append(paths, path)
			return nil
		})
		pattern = params.From
		matches, err = versions.MatchUnanchored(paths, pattern)
	}

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

func (command *Command) metadata(bucketName, remotePath string, private bool, versionID string) []s3resource.MetadataPair {
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

func (command *Command) printDeprecationWarning() {
	errorColor := ErrorColor.SprintFunc()
	blinkColor := BlinkingErrorColor.SprintFunc()
	command.stderr.Write([]byte(blinkColor("WARNING:")))
	command.stderr.Write([]byte("\n"))
	command.stderr.Write([]byte(errorColor("Parameters 'from/to' are deprecated, use 'file' instead")))
	command.stderr.Write([]byte("\n\n"))
}
