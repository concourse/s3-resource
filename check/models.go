package check

import "github.com/concourse/s3-resource"

type CheckRequest struct {
	Source  s3resource.Source  `json:"source"`
	Version s3resource.Version `json:"version"`
}

type CheckResponse []s3resource.Version
