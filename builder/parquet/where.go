package parquet

import (
	"fmt"
	"strings"
)

const (
	// maxWhereClauseLength bounds the total length of a user-provided
	// WHERE clause.
	maxWhereClauseLength = 1024
	// maxStringLiteralLength bounds each string literal inside a WHERE
	// clause.
	maxStringLiteralLength = 256
	// maxWhereExpressionDepth bounds parenthesis / NOT nesting so that a
	// deeply nested clause cannot exhaust the stack during validation.
	maxWhereExpressionDepth = 32
)

// ValidateWhereClause validates a user-provided SQL WHERE clause against a
// strict allowlist grammar before it is embedded into a DuckDB query.
//
// Only these constructs are accepted:
//   - column references (bare or double-quoted identifiers)
//   - number, single-quoted string, TRUE/FALSE/NULL literals
//   - comparison operators =, !=, <>, <, <=, >, >=
//   - AND, OR, NOT and parentheses
//   - IS [NOT] NULL, [NOT] LIKE, [NOT] IN (literal list),
//     [NOT] BETWEEN ... AND ...
//   - basic arithmetic (+, -, *, /, %) over the above
//
// Function calls, table functions, subqueries, casts, string concatenation
// and any other SQL syntax are rejected, which blocks DuckDB SQL injection
// through denial-of-service keywords, file-reading table functions such as
// read_csv_auto, and error-based exfiltration helpers such as error().
// The whole clause must be consumed by the grammar; trailing tokens are
// rejected. An empty (or whitespace-only) clause is valid.
func ValidateWhereClause(clause string) error {
	if strings.TrimSpace(clause) == "" {
		return nil
	}
	if len(clause) > maxWhereClauseLength {
		return fmt.Errorf("where clause is too long, must be at most %d characters", maxWhereClauseLength)
	}
	tokens, err := tokenizeWhereClause(clause)
	if err != nil {
		return err
	}
	p := &whereParser{tokens: tokens}
	return p.parse()
}

// reservedWhereKeywords are SQL keywords that must never be interpreted as
// column references. Quoted identifiers can still reference columns whose
// names collide with these keywords.
var reservedWhereKeywords = map[string]struct{}{
	"and":       {},
	"as":        {},
	"between":   {},
	"by":        {},
	"case":      {},
	"cast":      {},
	"else":      {},
	"end":       {},
	"exists":    {},
	"from":      {},
	"group":     {},
	"having":    {},
	"in":        {},
	"is":        {},
	"join":      {},
	"like":      {},
	"limit":     {},
	"not":       {},
	"offset":    {},
	"on":        {},
	"or":        {},
	"order":     {},
	"select":    {},
	"then":      {},
	"union":     {},
	"values":    {},
	"when":      {},
	"where":     {},
	"distinct":  {},
	"intersect": {},
}

type whereTokenKind int

const (
	whereTokenEOF whereTokenKind = iota
	whereTokenIdent
	whereTokenQuotedIdent
	whereTokenNumber
	whereTokenString
	whereTokenOper
)

type whereToken struct {
	kind  whereTokenKind
	value string // operators verbatim; identifiers lowercased; string literals decoded
	pos   int
}

func tokenizeWhereClause(clause string) ([]whereToken, error) {
	var tokens []whereToken
	i := 0
	for i < len(clause) {
		c := clause[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'':
			token, next, err := scanWhereQuoted(clause, i, '\'', whereTokenString)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
		case c == '"':
			token, next, err := scanWhereQuoted(clause, i, '"', whereTokenQuotedIdent)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			start := i
			for i < len(clause) && (clause[i] == '_' || clause[i] >= 'a' && clause[i] <= 'z' ||
				clause[i] >= '0' && clause[i] <= '9' || clause[i] >= 'A' && clause[i] <= 'Z') {
				i++
			}
			tokens = append(tokens, whereToken{
				kind:  whereTokenIdent,
				value: strings.ToLower(clause[start:i]),
				pos:   start,
			})
		case c >= '0' && c <= '9':
			start := i
			for i < len(clause) && clause[i] >= '0' && clause[i] <= '9' {
				i++
			}
			if i < len(clause) && clause[i] == '.' {
				i++
				for i < len(clause) && clause[i] >= '0' && clause[i] <= '9' {
					i++
				}
			}
			tokens = append(tokens, whereToken{kind: whereTokenNumber, value: clause[start:i], pos: start})
		default:
			if strings.HasPrefix(clause[i:], "--") || strings.HasPrefix(clause[i:], "/*") {
				return nil, fmt.Errorf("comment operator at position %d is not allowed in where clause", i)
			}
			opLen := 0
			switch {
			case strings.HasPrefix(clause[i:], "<="), strings.HasPrefix(clause[i:], ">="),
				strings.HasPrefix(clause[i:], "<>"), strings.HasPrefix(clause[i:], "!="):
				opLen = 2
			case c == '=' || c == '<' || c == '>' || c == '+' || c == '-' ||
				c == '*' || c == '/' || c == '%' || c == '(' || c == ')' || c == ',':
				opLen = 1
			default:
				return nil, fmt.Errorf("invalid character %q at position %d in where clause", c, i)
			}
			tokens = append(tokens, whereToken{kind: whereTokenOper, value: clause[i : i+opLen], pos: i})
			i += opLen
		}
	}
	tokens = append(tokens, whereToken{kind: whereTokenEOF, pos: len(clause)})
	return tokens, nil
}

// scanWhereQuoted scans a single-quoted string literal or a double-quoted
// identifier starting at the opening quote. A doubled quote escapes the
// quote character. Backslashes are rejected so that the decoded value can
// never change how the enclosing SQL statement is tokenized, even if the
// SQL engine enabled backslash escapes.
func scanWhereQuoted(clause string, start int, quote byte, kind whereTokenKind) (whereToken, int, error) {
	i := start + 1
	var sb strings.Builder
	for i < len(clause) {
		if clause[i] == quote {
			if i+1 < len(clause) && clause[i+1] == quote {
				sb.WriteByte(quote)
				i += 2
				continue
			}
			if sb.Len() > maxStringLiteralLength {
				return whereToken{}, 0, fmt.Errorf("quoted literal at position %d is too long in where clause", start)
			}
			return whereToken{kind: kind, value: sb.String(), pos: start}, i + 1, nil
		}
		if clause[i] == '\\' || clause[i] == 0 || clause[i] < 0x20 || clause[i] == 0x7f {
			return whereToken{}, 0, fmt.Errorf("invalid character at position %d in where clause", i)
		}
		sb.WriteByte(clause[i])
		i++
	}
	return whereToken{}, 0, fmt.Errorf("unterminated quoted literal at position %d in where clause", start)
}

type whereParser struct {
	tokens []whereToken
	pos    int
	depth  int
}

func (p *whereParser) peek() whereToken {
	return p.tokens[p.pos]
}

func (p *whereParser) next() whereToken {
	t := p.tokens[p.pos]
	if t.kind != whereTokenEOF {
		p.pos++
	}
	return t
}

func (p *whereParser) acceptKeyword(keyword string) bool {
	t := p.peek()
	if t.kind == whereTokenIdent && t.value == keyword {
		p.pos++
		return true
	}
	return false
}

func (p *whereParser) acceptOper(op string) bool {
	t := p.peek()
	if t.kind == whereTokenOper && t.value == op {
		p.pos++
		return true
	}
	return false
}

func (p *whereParser) expectOper(op string) error {
	if !p.acceptOper(op) {
		t := p.peek()
		return fmt.Errorf("expected %q but found %s at position %d in where clause", op, whereTokenDescription(t), t.pos)
	}
	return nil
}

// parse consumes the whole token stream as one boolean expression.
func (p *whereParser) parse() error {
	if err := p.parseOr(); err != nil {
		return err
	}
	if t := p.peek(); t.kind != whereTokenEOF {
		return fmt.Errorf("unexpected %s at position %d in where clause", whereTokenDescription(t), t.pos)
	}
	return nil
}

func (p *whereParser) parseOr() error {
	if err := p.parseAnd(); err != nil {
		return err
	}
	for p.acceptKeyword("or") {
		if err := p.parseAnd(); err != nil {
			return err
		}
	}
	return nil
}

func (p *whereParser) parseAnd() error {
	if err := p.parseNot(); err != nil {
		return err
	}
	for p.acceptKeyword("and") {
		if err := p.parseNot(); err != nil {
			return err
		}
	}
	return nil
}

func (p *whereParser) parseNot() error {
	if err := p.enter(); err != nil {
		return err
	}
	defer p.leave()
	if p.acceptKeyword("not") {
		return p.parseNot()
	}
	return p.parseComparison()
}

func (p *whereParser) parseComparison() error {
	if err := p.parseOperand(); err != nil {
		return err
	}
	switch {
	case p.acceptOper("="), p.acceptOper("!="), p.acceptOper("<>"),
		p.acceptOper("<"), p.acceptOper("<="), p.acceptOper(">"), p.acceptOper(">="):
		return p.parseOperand()
	case p.acceptKeyword("is"):
		p.acceptKeyword("not")
		return p.expectKeyword("null")
	case p.acceptKeyword("like"):
		return p.parseLikePattern()
	case p.acceptKeyword("in"):
		return p.parseInList()
	case p.acceptKeyword("between"):
		return p.parseBetween()
	case p.acceptKeyword("not"):
		switch {
		case p.acceptKeyword("like"):
			return p.parseLikePattern()
		case p.acceptKeyword("in"):
			return p.parseInList()
		case p.acceptKeyword("between"):
			return p.parseBetween()
		default:
			t := p.peek()
			return fmt.Errorf("expected LIKE, IN or BETWEEN after NOT at position %d in where clause", t.pos)
		}
	}
	// A bare operand is a valid boolean expression (column reference or
	// literal predicate target).
	return nil
}

func (p *whereParser) expectKeyword(keyword string) error {
	if !p.acceptKeyword(keyword) {
		t := p.peek()
		return fmt.Errorf("expected %q but found %s at position %d in where clause", strings.ToUpper(keyword), whereTokenDescription(t), t.pos)
	}
	return nil
}

// parseLikePattern accepts only string literal LIKE patterns. This keeps
// pattern matching usable while denying parenthesized expressions on the
// right-hand side of LIKE.
func (p *whereParser) parseLikePattern() error {
	t := p.next()
	if t.kind != whereTokenString {
		return fmt.Errorf("LIKE pattern must be a string literal, found %s at position %d in where clause", whereTokenDescription(t), t.pos)
	}
	return nil
}

func (p *whereParser) parseInList() error {
	if err := p.expectOper("("); err != nil {
		return err
	}
	for {
		t := p.next()
		switch t.kind {
		case whereTokenNumber, whereTokenString:
		case whereTokenIdent:
			if t.value != "true" && t.value != "false" && t.value != "null" {
				return fmt.Errorf("only literals are allowed in an IN list, found %s at position %d in where clause", whereTokenDescription(t), t.pos)
			}
		default:
			return fmt.Errorf("only literals are allowed in an IN list, found %s at position %d in where clause", whereTokenDescription(t), t.pos)
		}
		if p.acceptOper(",") {
			continue
		}
		return p.expectOper(")")
	}
}

func (p *whereParser) parseBetween() error {
	if err := p.parseOperand(); err != nil {
		return err
	}
	if !p.acceptKeyword("and") {
		t := p.peek()
		return fmt.Errorf("expected AND in BETWEEN predicate but found %s at position %d in where clause", whereTokenDescription(t), t.pos)
	}
	return p.parseOperand()
}

// parseOperand parses arithmetic expressions over atoms. Function calls are
// impossible by construction: an atom is only ever a literal, an
// identifier, or a parenthesized expression, and an identifier can never
// be followed directly by an opening parenthesis.
func (p *whereParser) parseOperand() error {
	if err := p.parseAdditive(); err != nil {
		return err
	}
	return nil
}

func (p *whereParser) parseAdditive() error {
	if err := p.parseMultiplicative(); err != nil {
		return err
	}
	for {
		if !p.acceptOper("+") && !p.acceptOper("-") {
			return nil
		}
		if err := p.parseMultiplicative(); err != nil {
			return err
		}
	}
}

func (p *whereParser) parseMultiplicative() error {
	if err := p.parseUnary(); err != nil {
		return err
	}
	for {
		if !p.acceptOper("*") && !p.acceptOper("/") && !p.acceptOper("%") {
			return nil
		}
		if err := p.parseUnary(); err != nil {
			return err
		}
	}
}

func (p *whereParser) parseUnary() error {
	if err := p.enter(); err != nil {
		return err
	}
	defer p.leave()
	if p.acceptOper("-") || p.acceptOper("+") {
		return p.parseUnary()
	}
	return p.parseAtom()
}

func (p *whereParser) parseAtom() error {
	t := p.next()
	switch t.kind {
	case whereTokenNumber, whereTokenString, whereTokenQuotedIdent:
		return nil
	case whereTokenIdent:
		switch t.value {
		case "true", "false", "null":
			return nil
		}
		if _, reserved := reservedWhereKeywords[t.value]; reserved {
			return fmt.Errorf("unexpected keyword %q at position %d in where clause", strings.ToUpper(t.value), t.pos)
		}
		return nil
	case whereTokenOper:
		if t.value == "(" {
			if err := p.parseOr(); err != nil {
				return err
			}
			return p.expectOper(")")
		}
		return fmt.Errorf("unexpected %s at position %d in where clause", whereTokenDescription(t), t.pos)
	default:
		return fmt.Errorf("unexpected end of where clause")
	}
}

func (p *whereParser) enter() error {
	p.depth++
	if p.depth > maxWhereExpressionDepth {
		return fmt.Errorf("where clause is nested too deeply, at most %d levels are allowed", maxWhereExpressionDepth)
	}
	return nil
}

func (p *whereParser) leave() {
	p.depth--
}

func whereTokenDescription(t whereToken) string {
	switch t.kind {
	case whereTokenEOF:
		return "end of clause"
	case whereTokenIdent:
		return fmt.Sprintf("keyword or identifier %q", t.value)
	case whereTokenQuotedIdent:
		return fmt.Sprintf("quoted identifier %q", t.value)
	case whereTokenNumber:
		return fmt.Sprintf("number %q", t.value)
	case whereTokenString:
		return "string literal"
	default:
		return fmt.Sprintf("operator %q", t.value)
	}
}
