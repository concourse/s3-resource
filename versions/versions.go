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

func Extract(path string, pattern string) (Extraction, bool) {
	compiled := regexp.MustCompile(pattern)
	matches := compiled.FindStringSubmatch(path)

	var match string
	if len(matches) < 2 { // whole string and match
		return Extraction{}, false
	} else if len(matches) == 2 {
		match = matches[1]
	} else if len(matches) > 2 { // many matches
		names := compiled.SubexpNames()
		index := sliceIndex(names, "version")

		if index > 0 {
			match = matches[index]
		} else {
			match = matches[1]
		}
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

func sliceIndex(haystack []string, needle string) int {
	for i, element := range haystack {
		if element == needle {
			return i
		}
	}

	return -1
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

	matchingPaths, err := Match(paths, source.Regexp)
	if err != nil {
		s3resource.Fatal("finding matches", err)
	}

	var extractions = make(Extractions, 0, len(matchingPaths))
	for _, path := range matchingPaths {
		extraction, ok := Extract(path, source.Regexp)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)
	return extractions
}
