package out

import (
	"github.com/concourse/s3-resource"
)

type OutRequest struct {
	Source s3resource.Source `json:"source"`
	Params Params            `json:"params"`
}

type Params struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type OutResponse struct {
	Version  s3resource.Version        `json:"version"`
	Metadata []s3resource.MetadataPair `json:"metadata"`
}
