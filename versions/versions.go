package versions

import (
	"regexp"
	"sort"

	"github.com/concourse/s3-resource"
	"github.com/hashicorp/go-version"
)

func Match(paths []string, pattern string) ([]string, error) {
	matched := []string{}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return matched, err
	}

	for _, path := range paths {
		match := regex.MatchString(path)

		if match {
			matched = append(matched, path)
		}
	}

	return matched, nil
}

var extractor = regexp.MustCompile("[\\d.]*\\d")

func Extract(path string) (Extraction, bool) {
	match := extractor.FindString(path)

	if len(match) == 0 {
		return Extraction{}, false
	}

	version, err := version.NewVersion(match)
	if err != nil {
		panic("version number was not valid: " + err.Error())
	}

	extraction := Extraction{
		Path:    path,
		Version: version,
	}

	return extraction, true
}

type Extractions []Extraction

func (e Extractions) Len() int {
	return len(e)
}

func (e Extractions) Less(i int, j int) bool {
	return e[i].Version.LessThan(e[j].Version)
}

func (e Extractions) Swap(i int, j int) {
	e[i], e[j] = e[j], e[i]
}

type Extraction struct {
	Path    string
	Version *version.Version
}

func GetBucketFileVersions(client s3resource.S3Client, source s3resource.Source) Extractions {
	paths, err := client.BucketFiles(source.Bucket)
	if err != nil {
		s3resource.Fatal("listing files", err)
	}

	matchingPaths, err := Match(paths, source.Glob)
	if err != nil {
		s3resource.Fatal("finding matches", err)
	}

	var extractions = make(Extractions, 0, len(matchingPaths))
	for _, path := range matchingPaths {
		extraction, ok := Extract(path)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)
	return extractions
}
