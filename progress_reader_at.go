package s3resource

import (
	"io"

	"github.com/cheggaaa/pb"
)

type SeekReaderAt interface {
	io.Seeker
	io.ReaderAt
}

type progressSeekReaderAt struct {
	SeekReaderAt
	*pb.ProgressBar
}

func (pra progressSeekReaderAt) Seek(offset int64, whence int) (int64, error) {
	n, err := pra.SeekReaderAt.Seek(offset, whence)
	if err != nil {
		return n, err
	}

	pra.ProgressBar.Add64(n)

	return n, err
}

func (pra progressSeekReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return pra.SeekReaderAt.ReadAt(p, off)
}
