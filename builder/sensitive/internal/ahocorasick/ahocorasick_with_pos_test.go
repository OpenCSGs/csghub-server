// ahocorasick_with_pos_test.go: test suite for ahocorasick_with_pos
//
// Copyright (c) 2013 CloudFlare, Inc.

package ahocorasick

import (
	"sync"
	"testing"
)

// Test cases for MatchThreadSafeWithPos

func TestNoPatternsWithPos(t *testing.T) {
	m := NewStringMatcher([]string{})
	hits := m.MatchThreadSafeWithPos([]byte("foo bar baz"))
	assert(t, len(hits) == 0)
}

func TestNoDataWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"foo", "baz", "bar"})
	hits := m.MatchThreadSafeWithPos([]byte(""))
	assert(t, len(hits) == 0)
}

func TestSuffixesWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "uperman", "perman", "erman"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0 || hits[3].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1 || hits[3].Hint == 1)
	assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2 || hits[3].Hint == 2)
	assert(t, hits[0].Hint == 3 || hits[1].Hint == 3 || hits[2].Hint == 3 || hits[3].Hint == 3)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("The Man Of Steel: Superman"))
	}
}

func TestPrefixesWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "Superma", "Superm", "Super"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0 || hits[3].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1 || hits[3].Hint == 1)
	assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2 || hits[3].Hint == 2)
	assert(t, hits[0].Hint == 3 || hits[1].Hint == 3 || hits[2].Hint == 3 || hits[3].Hint == 3)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("The Man Of Steel: Superman"))
	}
}

func TestInteriorWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"Steel", "tee", "e"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1)
	assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("The Man Of Steel: Superman"))
	}
}

func TestMatchAtStartWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Th", "he"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[0].Hint == 1)
	assert(t, hits[0].StartPos == 0)
	assert(t, hits[0].EndPos == 1)
	assert(t, hits[1].Hint == 0)
	assert(t, hits[1].StartPos == 0)
	assert(t, hits[1].EndPos == 2)
	assert(t, hits[2].Hint == 2)
	assert(t, hits[2].StartPos == 1)
	assert(t, hits[2].EndPos == 2)
}

func TestMatchAtEndWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"teel", "eel", "el"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1)
	assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("The Man Of Steel"))
	}
}

func TestOverlappingPatternsWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"Man ", "n Of", "Of S"})
	hits := m.MatchThreadSafeWithPos([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0].Hint == 0)
	assert(t, hits[0].StartPos == 4)
	assert(t, hits[0].EndPos == 7)
	assert(t, hits[1].Hint == 1)
	assert(t, hits[1].StartPos == 6)
	assert(t, hits[1].EndPos == 9)
	assert(t, hits[2].Hint == 2)
	assert(t, hits[2].StartPos == 8)
	assert(t, hits[2].EndPos == 11)
}

func TestMultipleMatchesWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Man", "an"})
	hits := m.MatchThreadSafeWithPos([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 3)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1)
	assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	}
}

func TestSingleCharacterMatchesWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"a", "M", "z"})
	hits := m.MatchThreadSafeWithPos([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 2)
	// 验证返回的hint正确
	assert(t, hits[0].Hint == 0 || hits[1].Hint == 0)
	assert(t, hits[0].Hint == 1 || hits[1].Hint == 1)
	// 验证位置计算合理
	for _, hit := range hits {
		assert(t, hit.StartPos >= 0)
		assert(t, hit.EndPos >= hit.StartPos)
		assert(t, hit.EndPos < len("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	}
}

func TestNothingMatchesWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"baz", "bar", "foo"})
	hits := m.MatchThreadSafeWithPos([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 0)
}

func TestWikipediaWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})
	hits := m.MatchThreadSafeWithPos([]byte("abccab"))
	assert(t, len(hits) == 4)
	assert(t, hits[0].Hint == 0)
	assert(t, hits[0].StartPos == 0)
	assert(t, hits[0].EndPos == 0)
	assert(t, hits[1].Hint == 1)
	assert(t, hits[1].StartPos == 0)
	assert(t, hits[1].EndPos == 1)
	assert(t, hits[2].Hint == 2)
	assert(t, hits[2].StartPos == 1)
	assert(t, hits[2].EndPos == 2)
	assert(t, hits[3].Hint == 4)
	assert(t, hits[3].StartPos == 2)
	assert(t, hits[3].EndPos == 2)
}

func TestWikipediaConcurrentlyWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafeWithPos([]byte("abccab"))
		assert(t, len(hits) == 4)
		assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0 || hits[3].Hint == 0)
		assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1 || hits[3].Hint == 1)
		assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2 || hits[3].Hint == 2)
		assert(t, hits[0].Hint == 4 || hits[1].Hint == 4 || hits[2].Hint == 4 || hits[3].Hint == 4)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafeWithPos([]byte("bccab"))
		assert(t, len(hits) == 4)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafeWithPos([]byte("bccb"))
		assert(t, len(hits) == 2)
	}()

	wg.Wait()
}

func TestMatchThreadSafeWithPos(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})

	wg := sync.WaitGroup{}
	wg.Add(5)
	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafeWithPos([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 4)
		// 验证返回的hint正确
		assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0 || hits[3].Hint == 0)
		assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1 || hits[3].Hint == 1)
		assert(t, hits[0].Hint == 2 || hits[1].Hint == 2 || hits[2].Hint == 2 || hits[3].Hint == 2)
		assert(t, hits[0].Hint == 3 || hits[1].Hint == 3 || hits[2].Hint == 3 || hits[3].Hint == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafeWithPos([]byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 3)
		// 验证返回的hint正确
		assert(t, hits[0].Hint == 0 || hits[1].Hint == 0 || hits[2].Hint == 0)
		assert(t, hits[0].Hint == 1 || hits[1].Hint == 1 || hits[2].Hint == 1)
		assert(t, hits[0].Hint == 3 || hits[1].Hint == 3 || hits[2].Hint == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafeWithPos([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 2)
		// 验证返回的hint正确
		assert(t, hits[0].Hint == 0 || hits[1].Hint == 0)
		assert(t, hits[0].Hint == 3 || hits[1].Hint == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafeWithPos([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 1)
		assert(t, hits[0].Hint == 0)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafeWithPos([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 0)
	}()

	wg.Wait()
}

// 基准测试
func BenchmarkMatchThreadSafeWithPosWorks(b *testing.B) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})
	input := []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
	for i := 0; i < b.N; i++ {
		m.MatchThreadSafeWithPos(input)
	}
}

func BenchmarkLongMatchThreadSafeWithPosWorks(b *testing.B) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Phoenix"})
	input := []byte("Firefox is a web browser, and is Mozilla's flagship software product. It is available in both desktop and mobile versions. Firefox uses the Gecko layout engine to render web pages, which implements current and anticipated web standards. As of April 2013, Firefox has approximately 20% of worldwide usage share of web browsers, making it the third most-used web browser. Firefox began as an experimental branch of the Mozilla codebase by Dave Hyatt, Joe Hewitt and Blake Ross. They believed the commercial requirements of Netscape's sponsorship and developer-driven feature creep compromised the utility of the Mozilla browser. To combat what they saw as the Mozilla Suite's software bloat, they created a stand-alone browser, with which they intended to replace the Mozilla Suite. Firefox was originally named Phoenix but the name was changed so as to avoid trademark conflicts with Phoenix Technologies. The initially-announced replacement, Firebird, provoked objections from the Firebird project community. The current name, Firefox, was chosen on February 9, 2004.")
	for i := 0; i < b.N; i++ {
		m.MatchThreadSafeWithPos(input)
	}
}
