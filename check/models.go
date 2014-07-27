package check

type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version"`
}

type Source struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Bucket          string `json:"bucket"`
	Glob            string `json:"glob"`
}

type OutResponse []Version

type Version struct {
	Bucket string `json:"bucket"`
	Path   string `json:"path"`
}
