package s3resource

type Source struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
	Bucket          string `json:"bucket"`
	Regexp          string `json:"regexp"`
	VersionedFile   string `json:"versioned_file"`
	Private         bool   `json:"private"`
	RegionName      string `json:"region_name"`
	CloudfrontURL   string `json:"cloudfront_url"`
	Endpoint        string `json:"endpoint"`
	DisableSSL      bool   `json:"disable_ssl"`
}

func (source Source) IsValid() (bool, string) {
	if source.Regexp != "" && source.VersionedFile != "" {
		return false, "please specify either regexp or versioned_file"
	}

	return true, ""
}

type Version struct {
	Path      string `json:"path,omitempty"`
	VersionID string `json:"version_id,omitempty"`
}

type MetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
