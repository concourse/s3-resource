package versions

import (
	"regexp"
	"sort"
	"strings"

	s3resource "github.com/concourse/s3-resource"
	"github.com/cppforlife/go-semi-semantic/version"
)

func sliceIndex(haystack []string, needle string) int {
	for i, element := range haystack {
		if element == needle {
			return i
		}
	}

	return -1
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

func GetMatch(path string, pattern string) (string, bool) {
	compiled := regexp.MustCompile(pattern)
	matches := compiled.FindStringSubmatch(path)

	var match string
	if len(matches) < 2 { // whole string and match
		return "", false
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
	return match, true
}

func Extract(path string, pattern string, order_by string) (Extraction, bool) {
	if order_by == "string" {
		return ExtractString(path, pattern)
	} else {
		return ExtractSemver(path, pattern)
	}
}
func ExtractSemver(path string, pattern string) (SemverExtraction, bool) {

	match, ok := GetMatch(path, pattern)

	if !ok {
		return SemverExtraction{}, false
	}

	ver, err := version.NewVersionFromString(match)
	if err != nil {
		panic("version number was not valid: " + err.Error())
	}

	extraction := SemverExtraction{
		Path:          path,
		Version:       ver,
		VersionNumber: match,
	}

	return extraction, ok
}
func ExtractString(path string, pattern string) (StringExtraction, bool) {

	match, ok := GetMatch(path, pattern)

	if !ok {
		return StringExtraction{}, false
	}

	extraction := StringExtraction{
		Path:          path,
		VersionNumber: match,
	}

	return extraction, ok
}

type Extraction interface {
	Compare(other Extraction) int
	GetPath() string
	GetVersion() version.Version
	GetVersionNumber() string
}

type Extractions []Extraction

func (e Extractions) Len() int {
	return len(e)
}

func (e Extractions) Less(i int, j int) bool {
	return e[i].Compare(e[j]) == -1
}

func (e Extractions) Swap(i int, j int) {
	e[i], e[j] = e[j], e[i]
}

type SemverExtraction struct {
	// path to s3 object in bucket
	Path string

	// parsed version
	Version version.Version

	// the raw version match
	VersionNumber string
}

func (s SemverExtraction) Compare(other Extraction) int {
	return s.Version.Compare(other.GetVersion())
}

func (s SemverExtraction) GetPath() string {
	return s.Path
}

func (s SemverExtraction) GetVersion() version.Version {
	return s.Version
}

func (s SemverExtraction) GetVersionNumber() string {
	return s.VersionNumber
}

type StringExtraction struct {
	// path to s3 object in bucket
	Path string

	// the raw version match
	VersionNumber string
}

func (s StringExtraction) Compare(other Extraction) int {
	return strings.Compare(s.VersionNumber, other.GetVersionNumber())
}

func (s StringExtraction) GetPath() string {
	return s.Path
}

func (s StringExtraction) GetVersion() version.Version {
	panic("StringExtraction does not have a parsed Version")
}

func (s StringExtraction) GetVersionNumber() string {
	return s.VersionNumber
}

// GetMatchingPathsFromBucket gets all the paths in the S3 bucket `bucketName` which match all the sections of `regex`
//
// `regex` is a forward-slash (`/`) delimited list of regular expressions that
// must match each corresponding sub-directories and file name for the path to
// be retained.
//
// The function walks the file tree stored in the S3 bucket `bucketName` and
// collects the full paths that matches `regex` along the way. It takes care of
// following only the branches (prefix in S3 terms) that matches with the
// corresponding section of `regex`.
func GetMatchingPathsFromBucket(client s3resource.S3Client, bucketName string, regex string) ([]string, error) {
	type work struct {
		prefix  string
		remains []string
	}

	specialCharsRE := regexp.MustCompile(`[\\\*\.\[\]\(\)\{\}\?\|\^\$\+]`)

	if strings.HasPrefix(regex, "^") {
		regex = regex[1:]
	}
	if strings.HasSuffix(regex, "$") {
		regex = regex[:len(regex)-1]
	}

	matchingPaths := []string{}
	queue := []work{{prefix: "", remains: strings.Split(regex, "/")}}
	for len(queue) != 0 {
		prefix := queue[0].prefix
		remains := queue[0].remains
		section := remains[0]
		remains = remains[1:]
		queue = queue[1:]
		if !specialCharsRE.MatchString(section) && len(remains) != 0 {
			// No special char so it can match a single string and we can just extend the prefix
			// but only if some remains exists, i.e. the section is not a leaf.
			prefix += section + "/"
			queue = append(queue, work{prefix: prefix, remains: remains})
			continue
		}
		// Let's list what's under the current prefix and see if that matches with the section
		var prefixRE *regexp.Regexp
		if len(remains) != 0 {
			// We need to look deeper so full prefix will end with a /
			prefixRE = regexp.MustCompile("^" + prefix + section + "/$")
		} else {
			prefixRE = regexp.MustCompile("^" + prefix + section + "$")
		}
		var (
			continuationToken *string
			truncated         bool
		)
		for continuationToken, truncated = nil, true; truncated; {
			s3ListChunk, err := client.ChunkedBucketList(bucketName, prefix, continuationToken)
			if err != nil {
				return []string{}, err
			}
			truncated = s3ListChunk.Truncated
			continuationToken = s3ListChunk.ContinuationToken

			if len(remains) != 0 {
				// We need to look deeper so full prefix will end with a /
				for _, commonPrefix := range s3ListChunk.CommonPrefixes {
					if prefixRE.MatchString(commonPrefix) {
						queue = append(queue, work{prefix: commonPrefix, remains: remains})
					}
				}
			} else {
				// We're looking for a leaf
				for _, path := range s3ListChunk.Paths {
					if prefixRE.MatchString(path) {
						matchingPaths = append(matchingPaths, path)
					}
				}
			}
		}
	}
	return matchingPaths, nil
}

func GetBucketFileVersions(client s3resource.S3Client, source s3resource.Source) Extractions {
	regex := source.Regexp

	matchingPaths, err := GetMatchingPathsFromBucket(client, source.Bucket, regex)
	if err != nil {
		s3resource.Fatal("listing files", err)
	}

	var extractions = make(Extractions, 0, len(matchingPaths))
	for _, path := range matchingPaths {
		extraction, ok := Extract(path, regex, source.OrderBy)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)

	return extractions
}
