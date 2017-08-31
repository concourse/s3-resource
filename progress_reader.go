package s3resource

import (
	"io"

	"github.com/cheggaaa/pb"
)

type progressReader struct {
	reader io.Reader
	pb     *pb.ProgressBar
}

func (r progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		return n, err
	}

	r.pb.Add(n)

	return n, nil
}
