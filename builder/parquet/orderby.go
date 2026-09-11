package parquet

import (
	"fmt"
	"strings"
)

// ValidateOrderByClause validates a user-provided ORDER BY clause against a
// strict allowlist: comma-separated column references with an optional
// case-insensitive asc/desc direction, e.g. "id, score desc". The allowed
// character set (letters, digits, underscore, comma, space) excludes
// quotes, parentheses, and operators, so function calls, subqueries, and
// multi-statement input are impossible by construction; reserved SQL
// keywords are rejected as column references so keyword soup cannot reach
// the query. Tabs and newlines are rejected like any other character. An
// empty (or whitespace-only) clause is valid.
func ValidateOrderByClause(clause string) error {
	if strings.TrimSpace(clause) == "" {
		return nil
	}
	if len(clause) > maxWhereClauseLength {
		return fmt.Errorf("order by clause is too long, must be at most %d characters", maxWhereClauseLength)
	}
	for i := 0; i < len(clause); i++ {
		if !isOrderByChar(clause[i]) {
			return fmt.Errorf("invalid character %q at position %d in order by clause", clause[i], i)
		}
	}
	for _, part := range strings.Split(clause, ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 || len(fields) > 2 {
			return fmt.Errorf("order by clause must be a comma-separated list of column names with an optional asc/desc direction")
		}
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc", "desc":
			default:
				return fmt.Errorf("invalid sort direction %q in order by clause, must be asc or desc", fields[1])
			}
		}
		column := fields[0]
		if !isOrderByIdentStart(column[0]) || !isOrderByIdentifier(column) {
			return fmt.Errorf("invalid column reference %q in order by clause", column)
		}
		if err := checkOrderByColumnName(column); err != nil {
			return err
		}
	}
	return nil
}

func isOrderByChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == ',' || c == ' '
}

func isOrderByIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isOrderByIdentifier(column string) bool {
	for i := 1; i < len(column); i++ {
		c := column[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func checkOrderByColumnName(column string) error {
	switch strings.ToLower(column) {
	case "asc", "desc":
		return fmt.Errorf("sort direction %q cannot be used as a column name in order by clause", column)
	}
	if _, reserved := reservedWhereKeywords[strings.ToLower(column)]; reserved {
		return fmt.Errorf("unexpected keyword %q in order by clause", strings.ToUpper(column))
	}
	return nil
}
