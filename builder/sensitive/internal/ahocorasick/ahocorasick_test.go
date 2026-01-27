// ahocorasick_test.go: test suite for ahocorasick
//
// Copyright (c) 2013 CloudFlare, Inc.

package ahocorasick

import (
	"regexp"
	"strings"
	"sync"
	"testing"
)

func assert(t *testing.T, b bool) {
	if !b {
		t.Fail()
	}
}

func TestNoPatterns(t *testing.T) {
	m := NewStringMatcher([]string{})
	hits := m.Match([]byte("foo bar baz"))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte("foo bar baz"))
	assert(t, len(hits) == 0)
}

func TestNoData(t *testing.T) {
	m := NewStringMatcher([]string{"foo", "baz", "bar"})
	hits := m.Match([]byte(""))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte(""))
	assert(t, len(hits) == 0)
}

func TestSuffixes(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "uperman", "perman", "erman"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)
}

func TestPrefixes(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "Superma", "Superm", "Super"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 3)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 1)
	assert(t, hits[3] == 0)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 3)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 1)
	assert(t, hits[3] == 0)
}

func TestInterior(t *testing.T) {
	m := NewStringMatcher([]string{"Steel", "tee", "e"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[2] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[0] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[2] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[0] == 2)
}

func TestMatchAtStart(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Th", "he"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
	assert(t, hits[2] == 2)
}

func TestMatchAtEnd(t *testing.T) {
	m := NewStringMatcher([]string{"teel", "eel", "el"})
	hits := m.Match([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
}

func TestOverlappingPatterns(t *testing.T) {
	m := NewStringMatcher([]string{"Man ", "n Of", "Of S"})
	hits := m.Match([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
}

func TestMultipleMatches(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Man", "an"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 0)
}

func TestSingleCharacterMatches(t *testing.T) {
	m := NewStringMatcher([]string{"a", "M", "z"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
}

func TestNothingMatches(t *testing.T) {
	m := NewStringMatcher([]string{"baz", "bar", "foo"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 0)
}

func TestWikipedia(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})
	hits := m.Match([]byte("abccab"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 4)

	hits = m.Match([]byte("bccab"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 2)
	assert(t, hits[1] == 4)
	assert(t, hits[2] == 0)
	assert(t, hits[3] == 1)

	hits = m.Match([]byte("bccb"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 2)
	assert(t, hits[1] == 4)
}

func TestWikipediaConcurrently(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("abccab"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 2)
		assert(t, hits[3] == 4)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("bccab"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 2)
		assert(t, hits[1] == 4)
		assert(t, hits[2] == 0)
		assert(t, hits[3] == 1)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("bccb"))
		assert(t, len(hits) == 2)
		assert(t, hits[0] == 2)
		assert(t, hits[1] == 4)
	}()

	wg.Wait()
}

func TestMatch(t *testing.T) {
	m := NewStringMatcher(dictionary)
	hits := m.Match([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, len(hits) == 1)
	assert(t, hits[0] == 0)

	hits = m.Match([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, len(hits) == 0)
}

func TestMatchThreadSafe(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})

	wg := sync.WaitGroup{}
	wg.Add(5)
	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 2)
		assert(t, hits[3] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 3)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 2)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 1)
		assert(t, hits[0] == 0)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 0)
	}()

	wg.Wait()
}

func TestLargeDictionaryMatchThreadSafeWorks(t *testing.T) {
	/**
	 * we have 105 unique words extracted from dictionary, therefore the result
	 * is supposed to show 105 hits
	 */
	hits := precomputed6.MatchThreadSafe(bytes2)
	assert(t, len(hits) == 105)

}

func TestContains(t *testing.T) {
	m := NewStringMatcher(dictionary)
	contains := m.Contains([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, contains)

	contains = m.Contains([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, !contains)

	m = NewStringMatcher([]string{"SupermanX", "per"})
	contains = m.Contains([]byte("The Man Of Steel: Superman"))
	assert(t, contains == true)
}

var bytes = []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
var sbytes = string(bytes)
var dictionary = []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"}
var precomputed = NewStringMatcher(dictionary)

func BenchmarkMatchWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed.Match(bytes)
	}
}

func BenchmarkMatchThreadSafeWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed.MatchThreadSafe(bytes)
	}
}

func BenchmarkContainsWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary {
			if strings.Contains(sbytes, s) {
				_ = append(hits, i)
			}
		}
	}
}

var re = regexp.MustCompile("(" + strings.Join(dictionary, "|") + ")")

func BenchmarkRegexpWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re.FindAllIndex(bytes, -1)
	}
}

var dictionary2 = []string{"Googlebot", "bingbot", "msnbot", "Yandex", "Baiduspider"}
var precomputed2 = NewStringMatcher(dictionary2)

func BenchmarkMatchFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed2.Match(bytes)
	}
}

func BenchmarkContainsFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary2 {
			if strings.Contains(sbytes, s) {
				_ = append(hits, i)
			}
		}
	}
}

var re2 = regexp.MustCompile("(" + strings.Join(dictionary2, "|") + ")")

func BenchmarkRegexpFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re2.FindAllIndex(bytes, -1)
	}
}

var bytes2 = []byte("Firefox is a web browser, and is Mozilla's flagship software product. It is available in both desktop and mobile versions. Firefox uses the Gecko layout engine to render web pages, which implements current and anticipated web standards. As of April 2013, Firefox has approximately 20% of worldwide usage share of web browsers, making it the third most-used web browser. Firefox began as an experimental branch of the Mozilla codebase by Dave Hyatt, Joe Hewitt and Blake Ross. They believed the commercial requirements of Netscape's sponsorship and developer-driven feature creep compromised the utility of the Mozilla browser. To combat what they saw as the Mozilla Suite's software bloat, they created a stand-alone browser, with which they intended to replace the Mozilla Suite. Firefox was originally named Phoenix but the name was changed so as to avoid trademark conflicts with Phoenix Technologies. The initially-announced replacement, Firebird, provoked objections from the Firebird project community. The current name, Firefox, was chosen on February 9, 2004.")
var sbytes2 = string(bytes2)

var dictionary3 = []string{"Mozilla", "Mac", "Macintosh", "Safari", "Phoenix"}
var precomputed3 = NewStringMatcher(dictionary3)

func BenchmarkLongMatchWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed3.Match(bytes2)
	}
}
func BenchmarkLongMatchThreadSafeWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed3.MatchThreadSafe(bytes2)
	}
}

func BenchmarkLongContainsWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary3 {
			if strings.Contains(sbytes2, s) {
				_ = append(hits, i)
			}
		}
	}
}

var re3 = regexp.MustCompile("(" + strings.Join(dictionary3, "|") + ")")

func BenchmarkLongRegexpWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re3.FindAllIndex(bytes2, -1)
	}
}

var dictionary4 = []string{"12343453", "34353", "234234523", "324234", "33333"}
var precomputed4 = NewStringMatcher(dictionary4)

func BenchmarkLongMatchFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed4.Match(bytes2)
	}
}

func BenchmarkLongContainsFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary4 {
			if strings.Contains(sbytes2, s) {
				_ = append(hits, i)
			}
		}
	}
}

var re4 = regexp.MustCompile("(" + strings.Join(dictionary4, "|") + ")")

func BenchmarkLongRegexpFails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re4.FindAllIndex(bytes2, -1)
	}
}

var dictionary5 = []string{"12343453", "34353", "234234523", "324234", "33333", "experimental", "branch", "of", "the", "Mozilla", "codebase", "by", "Dave", "Hyatt", "Joe", "Hewitt", "and", "Blake", "Ross", "mother", "frequently", "performed", "in", "concerts", "around", "the", "village", "uses", "the", "Gecko", "layout", "engine"}
var precomputed5 = NewStringMatcher(dictionary5)

func BenchmarkMatchMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed5.Match(bytes)
	}
}

func BenchmarkMatchThreadSafeMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed5.MatchThreadSafe(bytes)
	}
}

func BenchmarkContainsMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary4 {
			if strings.Contains(sbytes, s) {
				_ = append(hits, i)
			}
		}
	}
}

var re5 = regexp.MustCompile("(" + strings.Join(dictionary5, "|") + ")")

func BenchmarkRegexpMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re5.FindAllIndex(bytes, -1)
	}
}

func BenchmarkLongMatchMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed5.Match(bytes2)
	}
}

func BenchmarkLongMatchThreadSafeMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed5.MatchThreadSafe(bytes2)
	}
}

func BenchmarkLongContainsMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hits := make([]int, 0)
		for i, s := range dictionary4 {
			if strings.Contains(sbytes2, s) {
				_ = append(hits, i)
			}
		}
	}
}

func BenchmarkLongRegexpMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		re5.FindAllIndex(bytes2, -1)
	}
}

var dictionary6 = []string{"2004", "2013", "9", "a", "an", "and", "anticipated", "approximately", "April", "as", "available", "avoid", "began", "believed", "Blake", "bloat", "both", "branch", "browser", "browsers", "but", "by", "changed", "chosen", "codebase", "combat", "commercial", "community", "compromised", "conflicts", "created", "creep", "current", "Dave", "desktop", "developer-driven", "engine", "experimental", "feature", "February", "Firebird", "Firefox", "flagship", "from", "Gecko", "has", "Hewitt", "Hyatt", "implements", "in", "initially-announced", "intended", "is", "it", "Joe", "layout", "making", "mobile", "most-used", "Mozilla", "Mozilla's", "name", "named", "Netscape's", "objections", "of", "on", "originally", "pages", "Phoenix", "product", "project", "provoked", "render", "replace", "replacement", "requirements", "Ross", "saw", "share", "so", "software", "sponsorship", "stand-alone", "standards", "Suite", "Suite's", "Technologies", "the", "The", "they", "They", "third", "to", "trademark", "usage", "uses", "utility", "versions", "was", "web", "what", "which", "with", "worldwide"}
var precomputed6 = NewStringMatcher(dictionary6)

func BenchmarkLargeMatchWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed6.Match(bytes2)
	}
}

func BenchmarkLargeMatchThreadSafeWorks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		precomputed6.MatchThreadSafe(bytes2)
	}
}
