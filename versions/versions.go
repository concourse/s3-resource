package versions

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/concourse/s3-resource"
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
	var commitsSinceVersionMatch string

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

		index = sliceIndex(names, "commits_since_version")
		if index > 0 {
			commitsSinceVersionMatch = matches[index]
		}
	}

	probablySemver := match
	segs := strings.SplitN(probablySemver, ".", 3)
	switch len(segs) {
	case 2:
		probablySemver += ".0"
	case 1:
		probablySemver += ".0.0"
	}

	version, err := semver.Parse(probablySemver)
	if err != nil {
		panic("version number was not valid: " + err.Error())
	}

	var commitsSinceVersion uint

	if len(commitsSinceVersionMatch) > 0 {
		sinceVersion, err := strconv.ParseUint(commitsSinceVersionMatch, 10, 32)
		if err != nil {
			panic("commits_since_version group was not valid: " + err.Error())
		}

		commitsSinceVersion = uint(sinceVersion)
	} else {
		commitsSinceVersion = 0
	}

	extraction := Extraction{
		Path:                path,
		Version:             version,
		CommitsSinceVersion: commitsSinceVersion,
		VersionNumber:       match,
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
	if e[i].Version.EQ(e[j].Version) {
		return e[i].CommitsSinceVersion < e[j].CommitsSinceVersion
	}

	return e[i].Version.LT(e[j].Version)
}

func (e Extractions) Swap(i int, j int) {
	e[i], e[j] = e[j], e[i]
}

type Extraction struct {
	// path to s3 object in bucket
	Path string

	// parsed semantic version
	Version semver.Version

	// from git-describe output
	CommitsSinceVersion uint

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
