package s3resource

type Source struct {
	AccessKeyID          string `json:"access_key_id"`
	SecretAccessKey      string `json:"secret_access_key"`
	SessionToken         string `json:"session_token"`
	Bucket               string `json:"bucket"`
	Regexp               string `json:"regexp"`
	VersionedFile        string `json:"versioned_file"`
	Private              bool   `json:"private"`
	RegionName           string `json:"region_name"`
	CloudfrontURL        string `json:"cloudfront_url"`
	Endpoint             string `json:"endpoint"`
	DisableSSL           bool   `json:"disable_ssl"`
	ServerSideEncryption string `json:"server_side_encryption"`
	SSEKMSKeyId          string `json:"sse_kms_key_id"`
	UseV2Signing         bool   `json:"use_v2_signing"`
	SkipSSLVerification  bool   `json:"skip_ssl_verification"`
	SkipDownload         bool   `json:"skip_download"`
	InitialVersion       string `json:"initial_version"`
	InitialPath          string `json:"initial_path"`
	InitialContentText   string `json:"initial_content_text"`
	InitialContentBinary string `json:"initial_content_binary"`
	DisableMultipart     bool   `json:"disable_multipart"`
	OnOverwrite          string `json:"on_overwrite"`
}

func (source Source) IsValid() (bool, string) {
	if source.Regexp != "" && source.VersionedFile != "" {
		return false, "please specify either regexp or versioned_file"
	}

	if source.Regexp != "" && source.InitialVersion != "" {
		return false, "please use initial_path when regexp is set"
	}

	if source.VersionedFile != "" && source.InitialPath != "" {
		return false, "please use initial_version when versioned_file is set"
	}

	if source.InitialContentText != "" && source.InitialContentBinary != "" {
		return false, "please use intial_content_text or initial_content_binary but not both"
	}

	hasInitialContent := source.InitialContentText != "" || source.InitialContentBinary != ""
	if hasInitialContent && source.InitialVersion == "" && source.InitialPath == "" {
		return false, "please specify initial_version or initial_path if initial content is set"
	}

	switch source.OnOverwrite {
	case "", "allow", "fail":
		// valid option, so ignore
	case "ignore":
		if source.VersionedFile != "" {
			return false, `on_overwrite cannot be set to "ignore" when using versioned_file`
		}
	default:
		return false, `on_overwrite must be one of "allow" (default), "fail" or "ignore"`
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
