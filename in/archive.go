package in

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/h2non/filetype"
)

var archiveMimetypes = []string{
	"application/x-gzip",
	"application/gzip",
	"application/x-tar",
	"application/zip",
}

func mimetype(filename string) (string, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	kind, err := filetype.Match(buf)
	if err != nil {
		return "", err
	}

	return kind.MIME.Value, nil
}

func archiveMimetype(filename string) string {
	mime, err := mimetype(filename)
	if err != nil {
		return ""
	}

	for i := range archiveMimetypes {
		if strings.HasPrefix(mime, archiveMimetypes[i]) {
			return archiveMimetypes[i]
		}
	}

	return ""
}

func inflate(mime, path, destination string) error {
	var cmd *exec.Cmd

	switch mime {
	case "application/zip":
		cmd = exec.Command("unzip", "-P", "", "-d", destination, path)
		defer os.Remove(path)

	case "application/x-tar":
		cmd = exec.Command("tar", "xf", path, "-C", destination)
		defer os.Remove(path)

	case "application/gzip", "application/x-gzip":
		cmd = exec.Command("gunzip", path)

	default:
		return fmt.Errorf("don't know how to extract %s", mime)
	}

	return cmd.Run()
}
