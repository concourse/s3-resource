package check

import (
	"regexp"
	"strconv"
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
