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

type CheckResponse []Version

type Version struct {
	Path string `json:"path"`
}
