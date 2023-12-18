package tagparser

import (
	"errors"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// MetaTags parse metadata of README file, return tags found
func MetaTags(readme string) (map[string][]string, error) {
	meta := metaText(readme)
	if len(meta) == 0 {
		return map[string][]string{}, nil
	}

	categoryMap := make(map[string]any)
	//parse yaml string
	err := yaml.Unmarshal([]byte(meta), categoryMap)
	if err != nil {
		slog.Error("error unmarshall meta for tags", slog.Any("error", err), slog.String("meta", meta))
		return nil, err
	}
	tagMap := make(map[string][]string)
	for category, tagStr := range categoryMap {
		slog.Debug("tag category map content", slog.String("category", category), slog.Any("tagStr", tagStr))
		if tagName, match := tagStr.(string); match {
			tagMap[category] = append(tagMap[category], strings.TrimSpace(tagName))
		} else if tagNames, match := tagStr.([]interface{}); match {
			for _, tagName := range tagNames {
				tagMap[category] = append(tagMap[category], strings.TrimSpace(tagName.(string)))
			}
		} else {
			slog.Error("Unknown meta format", slog.Any("tagStr", tagStr))
			return nil, errors.New("unknown meta format")
		}
	}

	return tagMap, nil
}

func metaText(readme string) string {
	splits := strings.Split(readme, "---")
	if len(splits) < 2 {
		return ""
	}

	return splits[1]
}
