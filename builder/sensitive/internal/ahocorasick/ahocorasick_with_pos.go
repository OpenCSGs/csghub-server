package ahocorasick

import (
	"sync/atomic"
)

// MatchResultWithPos represents a match result with both hint and position
type MatchResultWithPos struct {
	Hint     int // Index of the matched word in the dictionary
	StartPos int // Start position of the matched word in the input text
	EndPos   int // End position of the matched word in the input text
}

// matchWithPos is a core of matching logic with position tracking.
// Accepts input byte slice, starting node and a func to check whether should we include result into response or not
func matchWithPos(in []byte, n *node, unique func(f *node) bool) []MatchResultWithPos {
	var hits []MatchResultWithPos

	for i, b := range in {
		c := int(b)

		if !n.root && n.child[c] == nil {
			n = n.fails[c]
		}

		if n.child[c] != nil {
			f := n.child[c]
			n = f

			if f.output {
				if unique(f) {
					wordLength := len(f.b)
					startPos := i - wordLength + 1
					if startPos < 0 {
						startPos = 0
					}
					hits = append(hits, MatchResultWithPos{
						Hint:     f.index,
						StartPos: startPos,
						EndPos:   i,
					})
				}
			}

			currentSuffix := f.suffix
			for !currentSuffix.root {
				if unique(currentSuffix) {
					wordLength := len(currentSuffix.b)
					startPos := i - wordLength + 1
					if startPos < 0 {
						startPos = 0
					}
					hits = append(hits, MatchResultWithPos{
						Hint:     currentSuffix.index,
						StartPos: startPos,
						EndPos:   i,
					})
				} else {
					break
				}
				currentSuffix = currentSuffix.suffix
			}
		}
	}

	return hits
}

// MatchThreadSafeWithPos provides the same result as MatchThreadSafe() but also returns positions
// of the matched words in the input text. Uses a sync.Pool of haystacks to track the uniqueness of
// the result items.
func (m *Matcher) MatchThreadSafeWithPos(in []byte) []MatchResultWithPos {
	var (
		heap map[int]uint64
	)

	generation := atomic.AddUint64(&m.counter, 1)
	n := m.root
	// read the matcher's heap
	item := m.heap.Get()
	if item == nil {
		heap = make(map[int]uint64, len(m.trie))
	} else {
		heap = item.(map[int]uint64)
	}

	hits := matchWithPos(in, n, func(f *node) bool {
		g := heap[f.index]
		if g != generation {
			heap[f.index] = generation
			return true
		}
		return false
	})

	m.heap.Put(heap)
	return hits
}
