package out

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/versions"
	"github.com/fatih/color"
)

var ErrObjectVersioningNotEnabled = errors.New("object versioning not enabled")
var ErrorColor = color.New(color.FgWhite, color.BgRed, color.Bold)
var BlinkingErrorColor = color.New(color.BlinkSlow, color.FgWhite, color.BgRed, color.Bold)

func init() {
	ErrorColor.EnableColor()
}

type OutCommand struct {
	stderr   io.Writer
	s3client s3resource.S3Client
}

func NewOutCommand(stderr io.Writer, s3client s3resource.S3Client) *OutCommand {
	return &OutCommand{
		stderr:   stderr,
		s3client: s3client,
	}
}

func (command *OutCommand) Run(sourceDir string, request OutRequest) (OutResponse, error) {
	if request.Params.From != "" || request.Params.To != "" {
		command.printDeprecationWarning()
	}

	if ok, message := request.Source.IsValid(); !ok {
		return OutResponse{}, errors.New(message)
	}
	if request.Params.File != "" && request.Params.From != "" {
		return OutResponse{}, errors.New("contains both file and from")
	}

	localPath, err := command.match(request.Params, sourceDir)
	if err != nil {
		return OutResponse{}, err
	}

	remotePath := command.remotePath(request, localPath, sourceDir)

	bucketName := request.Source.Bucket

	acl := "private"
	if request.Params.Acl != "" {
		acl = request.Params.Acl
	}

	versionID, err := command.s3client.UploadFile(
		bucketName,
		remotePath,
		localPath,
		acl,
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

func (command *OutCommand) match(params Params, sourceDir string) (string, error) {
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

func (command *OutCommand) printDeprecationWarning() {
	errorColor := ErrorColor.SprintFunc()
	blinkColor := BlinkingErrorColor.SprintFunc()
	command.stderr.Write([]byte(blinkColor("WARNING:")))
	command.stderr.Write([]byte("\n"))
	command.stderr.Write([]byte(errorColor("Parameters 'from/to' are deprecated, use 'file' instead")))
	command.stderr.Write([]byte("\n\n"))
}
