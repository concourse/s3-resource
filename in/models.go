package in

import "github.com/concourse/s3-resource"

type InRequest struct {
	Source  s3resource.Source  `json:"source"`
	Version s3resource.Version `json:"version"`
}

type MetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type InResponse struct {
	Version  s3resource.Version `json:"version"`
	Metadata []MetadataPair     `json:"metadata"`
}
