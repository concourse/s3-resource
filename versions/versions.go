package versions

import (
	"regexp"
	"sort"
	"strings"

	s3resource "github.com/concourse/s3-resource"
	"github.com/cppforlife/go-semi-semantic/version"
)

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

/* Get all the paths in the S3 bucket `bucketName` which match all the sections of `regex`
 *
 * `regex` is a forward-slash (`/`) delimited list of regular expressions that
 * must match each corresponding sub-directories and file name for the path to
 * be retained.
 *
 * The function walk the file tree stored in the S3 bucket `bucketName` and
 * collect the full paths that match `regex` along the way. It takes care of
 * following only the branches (prefix in S3 terms) that match with the
 * corresponding section of `regex`.
 */
func GetMatchingPathsFromBucket(client s3resource.S3Client, bucketName string, regex string) ([]string, error) {
	type work struct {
		prefix  string
		remains []string
	}

	specialCharsRE := regexp.MustCompile(`[\\\*\.\[\]\(\)\{\}\?\|\^\$\+]`)

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
			prefixRE = regexp.MustCompile(prefix + section + "/")
		} else {
			prefixRE = regexp.MustCompile(prefix + section)
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
		extraction, ok := Extract(path, regex)

		if ok {
			extractions = append(extractions, extraction)
		}
	}

	sort.Sort(extractions)

	return extractions
}
