package pgutil

import (
	"fmt"
	"regexp"
	"strings"
)

type Definition struct {
	ID            int
	Name          string
	UpQuery       Q
	DownQuery     Q
	IndexMetadata *IndexMetadata
}

type IndexMetadata struct {
	TableName string
	IndexName string
}

type MigrationReader interface {
	ReadAll() ([]RawDefinition, error)
}

type RawDefinition struct {
	ID           int
	Name         string
	RawUpQuery   string
	RawDownQuery string
}

var (
	identifierPattern = `[a-zA-Z0-9$_]+|"(?:[^"]+)"`

	cicPatternParts = strings.Join([]string{
		`CREATE`,
		`(?:UNIQUE)?`,
		`INDEX`,
		`CONCURRENTLY`,
		`(?:IF\s+NOT\s+EXISTS)?`,
		`(` + identifierPattern + `)`, // capture index name
		`ON`,
		`(?:ONLY)?`,
		`(` + identifierPattern + `)`, // capture table name
	}, `\s+`)

	createIndexConcurrentlyPattern    = regexp.MustCompile(cicPatternParts)
	createIndexConcurrentlyPatternAll = regexp.MustCompile(cicPatternParts + "[^;]+;")
)

func ReadMigrations(reader MigrationReader) (definitions []Definition, _ error) {
	rawDefinitions, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	ids := map[int]struct{}{}
	for _, rawDefinition := range rawDefinitions {
		if _, ok := ids[rawDefinition.ID]; ok {
			return nil, fmt.Errorf("duplicate migration identifier %d", rawDefinition.ID)
		}
		ids[rawDefinition.ID] = struct{}{}

		var indexMetadata *IndexMetadata
		prunedUp := removeComments(rawDefinition.RawUpQuery)
		prunedDown := removeComments(rawDefinition.RawDownQuery)

		if matches := createIndexConcurrentlyPattern.FindStringSubmatch(prunedUp); len(matches) > 0 {
			if strings.TrimSpace(createIndexConcurrentlyPatternAll.ReplaceAllString(prunedUp, "")) != "" {
				return nil, fmt.Errorf("CIC is not the only statement in the up migration")
			}

			indexMetadata = &IndexMetadata{
				TableName: matches[2],
				IndexName: matches[1],
			}
		}

		if len(createIndexConcurrentlyPatternAll.FindAllString(prunedDown, 1)) > 0 {
			return nil, fmt.Errorf("CIC is not allowed in down migrations")
		}

		definitions = append(definitions, Definition{
			ID:            rawDefinition.ID,
			Name:          rawDefinition.Name,
			UpQuery:       RawQuery(rawDefinition.RawUpQuery),
			DownQuery:     RawQuery(rawDefinition.RawDownQuery),
			IndexMetadata: indexMetadata,
		})
	}

	return definitions, nil
}

func removeComments(query string) string {
	var filtered []string
	for _, line := range strings.Split(query, "\n") {
		if line := strings.TrimSpace(strings.Split(line, "--")[0]); line != "" {
			filtered = append(filtered, line)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}
