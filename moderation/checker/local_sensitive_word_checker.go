package checker

// GetLocalWordChecker returns the local word checker instance
//
// It's better to call this method everytime before using the local checker, as the checker may be changed asynchronously
func GetLocalWordChecker() *DFA {
	return localWordChecker
}

// DFA State
type State struct {
	transitions map[rune]*State
	isEnd       bool
}

// Deterministic Finite Automaton (DFA)
type DFA struct {
	root *State
}

// NewDFA initializes a new DFA
func NewDFA() *DFA {
	return &DFA{
		root: &State{transitions: make(map[rune]*State)},
	}
}

// BuildDFA constructs the DFA from a list of sensitive words
func (d *DFA) BuildDFA(words []string) {
	for _, word := range words {
		current := d.root
		for _, char := range word {
			if next, exists := current.transitions[char]; exists {
				current = next
			} else {
				newState := &State{transitions: make(map[rune]*State)}
				current.transitions[char] = newState
				current = newState
			}
		}
		current.isEnd = true // Mark the end of a sensitive word
	}
}

// ContainsSensitiveWord checks if the input text contains any sensitive words
func (d *DFA) ContainsSensitiveWord(text string) bool {
	runes := []rune(text)
	current := d.root
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		if isIgnoredCharacter(char) {
			continue
		}
		if next, exists := current.transitions[char]; exists {
			current = next
			if current.isEnd {
				return true
			}
		} else {
			current = d.root // Reset to the root state
			if i > 0 {
				// Check if the current character is the start of a new sensitive word
				if next, exists := current.transitions[char]; exists {
					current = next
				}
			}
		}
	}
	return false
}

// isIgnoredCharacter checks if a character is in the specified ignored set
func isIgnoredCharacter(c rune) bool {
	ignoredCharacters := []rune{' ', '\u3000', '\t', '&', '%', '$', '@', '*', '！', '!', '#', '^', '~', '_', '—', '｜', '\'', '"', ';', '.', '，', ',', '?', '<', '>', '《', '》', '：', ':'}
	for _, ignored := range ignoredCharacters {
		if c == ignored {
			return true
		}
	}
	return false
}
