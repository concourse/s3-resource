package s3resource

import (
	"io"

	"github.com/cheggaaa/pb"
)

type progressWriterAt struct {
	io.WriterAt
	*pb.ProgressBar
}

func (pwa progressWriterAt) WriteAt(p []byte, off int64) (int, error) {
	n, err := pwa.WriterAt.WriteAt(p, off)
	if err != nil {
		return n, err
	}

	pwa.ProgressBar.Add(len(p))

	return n, err
}
