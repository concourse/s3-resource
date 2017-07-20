package in

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"net/http"
)

var archiveMimetypes = []string{
	"application/x-gzip",
	"application/gzip",
	"application/x-tar",
	"application/zip",
}

func mimetype(r *bufio.Reader) (string, error) {
	bs, err := r.Peek(512)
	if err != nil && err != io.EOF {
		return "", err
	}

	if len(bs) == 0 {
		return "", errors.New("cannot determine mimetype from empty bytes")
	}

	return http.DetectContentType(bs), nil
}

func archiveMimetype(filename string) string {
	f, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer f.Close()

	mime, err := mimetype(bufio.NewReader(f))
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
