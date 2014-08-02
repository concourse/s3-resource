package versions

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/concourse/s3-resource"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
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

var extractor = regexp.MustCompile("\\d+")

func Extract(path string) (Extraction, bool) {
	match := extractor.FindString(path)

	if len(match) == 0 {
		return Extraction{}, false
	}

	version, err := strconv.Atoi(match)
	if err != nil {
		panic("regex that should only be numbers was not numbers: " + err.Error())
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
	return e[i].Version < e[j].Version
}

func (e Extractions) Swap(i int, j int) {
	e[i], e[j] = e[j], e[i]
}

type Extraction struct {
	Path    string
	Version int
}

func GetBucketFileVersions(source s3resource.Source) Extractions {
	auth, err := aws.GetAuth(
		source.AccessKeyID,
		source.SecretAccessKey,
	)
	if err != nil {
		s3resource.Fatal("setting up aws auth", err)
	}

	// TODO: more regions
	client := s3.New(auth, aws.USEast)
	bucket := client.Bucket(source.Bucket)
	entries, err := bucket.GetBucketContents()
	if err != nil {
		s3resource.Fatal("listing buckets contents", err)
	}

	paths := make([]string, 0, len(*entries))
	for entry := range *entries {
		paths = append(paths, entry)
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
