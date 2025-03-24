package token

type Tokenizer interface {
	Encode(string) (int64, error)
}

var _ Tokenizer = (*DumyTokenizer)(nil)

// dumyy tokenizer for testing only
type DumyTokenizer struct{}

// Encode implements Tokenizer.
func (d *DumyTokenizer) Encode(s string) (int64, error) {
	return int64(len(s)), nil
}
