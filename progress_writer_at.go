package s3resource

import (
	"io"
)

type progressWriterAt struct {
	io.WriterAt
	io.Writer
}

func (pwa progressWriterAt) WriteAt(p []byte, off int64) (int, error) {
	n, err := pwa.WriterAt.WriteAt(p, off)
	if err != nil {
		return n, err
	}

	pwa.Write(p)
	return n, err
}
