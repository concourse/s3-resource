package versions

import (
	"regexp"
	"sort"
	"strings"

	"github.com/concourse/s3-resource"
	"github.com/cppforlife/go-semi-semantic/version"
)

func Match(paths []string, pattern string) ([]string, error) {
	return MatchUnanchored(paths, "^"+pattern+"$")
}

func MatchUnanchored(paths []string, pattern string) ([]string, error) {
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

	ver, err := version.NewVersionFromString(match)
	if err != nil {
		panic("version number was not valid: " + err.Error())
	}

	extraction := Extraction{
		Path:          path,
		Version:       ver,
		VersionNumber: match,
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
	return e[i].Version.IsLt(e[j].Version)
}

func (e Extractions) Swap(i int, j int) {
	e[i], e[j] = e[j], e[i]
}

type Extraction struct {
	// path to s3 object in bucket
	Path string

	// parsed version
	Version version.Version

	// the raw version match
	VersionNumber string
}

const regexpSpecialChars = `\\\*\.\[\]\(\)\{\}\?\|\^\$\+`

func PrefixHint(regex string) string {
	nonRE := regexp.MustCompile(`\\(?P<chr>[` + regexpSpecialChars + `])|(?P<chr>[^` + regexpSpecialChars + `])`)
	re := regexp.MustCompile(`^(` + nonRE.String() + `)*$`)

	validSections := []string{}

	sections := strings.Split(regex, "/")

	for _, section := range sections {
		if re.MatchString(section) {
			validSections = append(validSections, nonRE.ReplaceAllString(section, "${chr}"))
		} else {
			break
		}
	}

	if len(validSections) == 0 {
		return ""
	}

	return strings.Join(validSections, "/") + "/"
}

func GetBucketFileVersions(client s3resource.S3Client, source s3resource.Source) Extractions {
	regexp := source.Regexp
	hint := PrefixHint(regexp)

	paths, err := client.BucketFiles(source.Bucket, hint)
	if err != nil {
		s3resource.Fatal("listing files", err)
	}

	matchingPaths, err := Match(paths, source.Regexp)
	if err != nil {
		s3resource.Fatal("finding matches", err)
	}

	var extractions = make(Extractions, 0, len(matchingPaths))
	for _, path := range matchingPaths {
		extraction, ok := Extract(path, regexp)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)

	return extractions
}
