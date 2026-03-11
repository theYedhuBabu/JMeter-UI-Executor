package parser

import (
	"os"
	"regexp"
)

// Pre-compiled regexes — compiled once at package init, not on every call.
var (
	// csvDataSetRegex matches <CSVDataSet> ... </CSVDataSet> blocks
	csvDataSetRegex = regexp.MustCompile(`(?s)<CSVDataSet.*?>.*?</CSVDataSet>`)

	// filenamePropRegex extracts the variable name from a JMeter ${__P(VARIABLE_NAME)} expression
	// inside a <stringProp name="filename"> element.
	filenamePropRegex = regexp.MustCompile(`<stringProp name="filename">\s*\$\{\s*__P\(\s*([^,)}]+)\s*(?:,[^}]*)?\)\s*\}\s*</stringProp>`)
)

// ParseRequiredCSVs reads a JMX file and extracts CSV filenames defined within
// <CSVDataSet> blocks that use JMeter property variables like ${__P(VARIABLE_NAME)}.
func ParseRequiredCSVs(jmxFilePath string) ([]string, error) {
	content, err := os.ReadFile(jmxFilePath)
	if err != nil {
		return nil, err
	}

	text := string(content)

	csvDatasets := csvDataSetRegex.FindAllString(text, -1)

	uniqueVars := make(map[string]bool)
	var result []string

	for _, datasetBlock := range csvDatasets {
		matches := filenamePropRegex.FindAllStringSubmatch(datasetBlock, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := match[1]
				if !uniqueVars[varName] {
					uniqueVars[varName] = true
					result = append(result, varName)
				}
			}
		}
	}

	return result, nil
}
