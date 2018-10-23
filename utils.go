package s3resource

import (
	"fmt"
	"os"

	"github.com/mitchellh/colorstring"
)

const redacted = "redacted"

func Fatal(doing string, err error) {
	Sayf(colorstring.Color("[red]error %s: %s\n"), doing, err)
	os.Exit(1)
}

func Sayf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
}

func RedactSource(src Source) Source {
	redactedSource := src

	redactedSource.AccessKeyID = redacted
	redactedSource.SecretAccessKey = redacted
	redactedSource.SSEKMSKeyId = redacted

	return redactedSource
}
