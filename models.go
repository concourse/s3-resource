package s3resource

type Source struct {
	AccessKeyID         string `json:"access_key_id"`
	SecretAccessKey     string `json:"secret_access_key"`
	Bucket              string `json:"bucket"`
	Regexp              string `json:"regexp"`
	VersionedFile       string `json:"versioned_file"`
	Private             bool   `json:"private"`
	RegionName          string `json:"region_name"`
	CloudfrontURL       string `json:"cloudfront_url"`
	Endpoint            string `json:"endpoint"`
	DisableMD5HashCheck bool   `json:"disable_md5_hash_check"`
}

type Version struct {
	Path      string `json:"path"`
	VersionID string `json:"version_id,omitempty"`
}

type MetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
