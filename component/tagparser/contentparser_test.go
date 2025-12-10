package tagparser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const readme = `
---
task_categories:
- text-generation
language:
- zh
tags:
- text-generation
- 'llm '
- casual-lm
- language-modeling
pretty_name: SkyPile-150B
size_categories:
- 100B<n<1T
---
# SkyPile-150B

## Dataset Summary
SkyPile-150B is a comprehensive, large-scale Chinese dataset specifically designed for the pre-training of large language models. It is derived from a broad array of publicly accessible Chinese Internet web pages. Rigorous filtering, extensive deduplication, and thorough sensitive data filtering have been employed to ensure its quality. Furthermore, we have utilized advanced tools such as fastText and BERT to filter out low-quality data.

The publicly accessible portion of the SkyPile-150B dataset encompasses approximately 233 million unique web pages, each containing an average of over 1,000 Chinese characters. In total, the dataset includes approximately 150 billion tokens and 620 gigabytes of plain text data.


## Language
The SkyPile-150B dataset is exclusively composed of Chinese data.
`

const actualMeta = `
task_categories:
- text-generation
language:
- zh
tags:
- text-generation
- 'llm '
- casual-lm
- language-modeling
pretty_name: SkyPile-150B
size_categories:
- 100B<n<1T
`

func TestTagParser_MetaText(t *testing.T) {
	testMeta := metaText(readme)
	if testMeta != actualMeta {
		t.Errorf("expected %s, got %s", actualMeta, testMeta)
		t.Fail()
	}
}

func TestTagParser_MetaTags(t *testing.T) {
	metaTags, err := MetaTags(readme)
	require.Nil(t, err)
	require.Equal(t, 4, len(metaTags["task"]))
	require.ElementsMatch(
		t, []string{"text-generation", "llm", "casual-lm", "language-modeling"}, metaTags["task"],
	)
	require.Equal(t, 1, len(metaTags["language"]))
	require.ElementsMatch(
		t, []string{"zh"}, metaTags["language"],
	)
	require.Equal(t, 0, len(metaTags["tags"]))
	require.Equal(t, 1, len(metaTags["pretty_name"]))
	require.ElementsMatch(
		t, []string{"SkyPile-150B"}, metaTags["pretty_name"],
	)
	require.Equal(t, 1, len(metaTags["size"]))
	require.ElementsMatch(
		t, []string{"100B<n<1T"}, metaTags["size"],
	)
}

func TestTagParser_MetaTags_LongTag(t *testing.T) {
	longTag := strings.Repeat("a", 129)
	readmeWithLongTag := `
---
tags:
- a-short-tag
- ` + longTag + `
- another-short-tag
---
# Title
`
	metaTags, err := MetaTags(readmeWithLongTag)
	require.NoError(t, err)
	require.Contains(t, metaTags, "task")
	tags := metaTags["task"]
	require.Len(t, tags, 2)
	require.ElementsMatch(t, []string{"a-short-tag", "another-short-tag"}, tags)
	require.NotContains(t, tags, longTag)
}
