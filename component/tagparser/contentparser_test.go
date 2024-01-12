package tagparser

import (
	"slices"
	"testing"
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

func TestMetaText(t *testing.T) {
	testMeta := metaText(readme)
	if testMeta != actualMeta {
		t.Errorf("expected %s, got %s", actualMeta, testMeta)
		t.Fail()
	}
}

func TestMetaTags(t *testing.T) {
	metaTags, err := MetaTags(readme)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if len(metaTags) != 4 {
		t.Errorf("expected 4 tags, got %d", len(metaTags))
		t.Fail()
	}
	if len(metaTags["task"]) != 4 || !slices.Equal(metaTags["task"], []string{"text-generation", "llm", "casual-lm", "language-modeling"}) {
		t.Error("wrong task_categories", len(metaTags["task"]), metaTags["task"])
		t.Fail()
	}
	if len(metaTags["language"]) != 1 || metaTags["language"][0] != "zh" {
		t.Error("wrong language")
		t.Fail()
	}
	if len(metaTags["tags"]) != 0 {
		t.Errorf("wrong tags, got:%v", metaTags["tags"])
		t.Fail()
	}
	if len(metaTags["pretty_name"]) != 1 || metaTags["pretty_name"][0] != "SkyPile-150B" {
		t.Error("wrong pretty_name")
		t.Fail()
	}
	if len(metaTags["size"]) != 1 || metaTags["size"][0] != "100B<n<1T" {
		t.Error("wrong size_categories")
		t.Fail()
	}
}
