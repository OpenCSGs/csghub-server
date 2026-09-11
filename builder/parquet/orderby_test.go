package parquet

import (
	"strings"
	"testing"
)

func TestValidateOrderByClause(t *testing.T) {
	valid := []string{
		"",
		"   ",
		"id",
		"id asc",
		"id ASC",
		"id Desc",
		"id  asc",
		" id",
		"id ",
		"a,b",
		"a, b",
		"a asc, b desc",
		"a asc,b desc",
		"a asc, b",
		"created_at desc, id asc",
		"_col",
		"col_1",
	}

	invalid := []string{
		// SQL structure is impossible without these characters, but every
		// known bypass attempt must still be rejected.
		"a; drop table t",
		"a,(select 1)",
		"upper(a)",
		"a = 1",
		"a asc desc",
		"a b",
		"a, b c",
		"'a'",
		`"a"`,
		"a\tasc",
		"a\nasc",
		"a--b",
		"a/*b*/",
		"a||b",
		"case when 1=1 then id else name end",
		"1",
		"1a",
		"a,",
		",a",
		"a,,b",
		"asc",
		"desc",
		"select",
		"select desc",
		"a desc, asc",
		"from asc",
		"randomkeywordthatlookslikeacolumn asc asc",
		strings.Repeat("a", 2000),
	}

	for _, clause := range valid {
		if err := ValidateOrderByClause(clause); err != nil {
			t.Errorf("ValidateOrderByClause(%q) = %v, want nil", clause, err)
		}
	}

	for _, clause := range invalid {
		if err := ValidateOrderByClause(clause); err == nil {
			t.Errorf("ValidateOrderByClause(%q) = nil, want error", clause)
		}
	}
}
