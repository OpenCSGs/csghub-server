package parquet

import (
	"strings"
	"testing"
)

func TestValidateWhereClause(t *testing.T) {
	valid := []string{
		"",
		"   ",
		"id = 1",
		"label = 'positive'",
		"label = 'it''s ok'",
		"name = '数据集'",
		"a != 1",
		"a <> 1",
		"a < 1",
		"a <= 1",
		"a > 1",
		"a >= 1",
		"a = 1 AND b = 2",
		"a = 1 OR b = 2",
		"a = 1 AND b = 2 OR c = 3",
		"a = 1 and b = 2 or not c = 3",
		"NOT a = 1",
		"NOT NOT a = 1",
		"(a = 1 OR b = 2) AND c = 3",
		"a IS NULL",
		"a IS NOT NULL",
		"a LIKE 'x%'",
		"a LIKE '%middle%'",
		"a NOT LIKE 'x%'",
		"a IN (1, 2, 3)",
		"a IN ('x', 'y')",
		"a NOT IN (1, 2)",
		"a IN (TRUE, FALSE, NULL)",
		"a BETWEEN 1 AND 10",
		"a NOT BETWEEN 'a' AND 'b'",
		"a BETWEEN 1 AND 10 AND b = 2",
		"a = -1",
		"a = +1",
		"a > 1 + 2 * 3",
		"a > 1 + 2 - 3 / 4 % 5",
		"a = (1 + 2)",
		"(a = 1)",
		"((a = 1) AND (b = 2)) OR c = 3",
		"score > 0.5",
		"score > 100.25",
		"a = TRUE",
		"a = false",
		"a = null",
		"a1 = 1 AND _b = 2",
		`"col name" = 1`,
		`"select" = 1`,
		"a = 1 OR label = 'hello world'",
		"a=1 AND b LIKE 'x%' OR NOT (c IS NULL)",
	}

	invalid := []string{
		// Advisory PoC: error-based file read via table function.
		"error((FROM read_csv_auto(chr(47)||chr(101), header=false) LIMIT 1))",
		// Advisory PoC: blind boolean oracle via table function.
		"(FROM read_csv_auto('/etc/passwd', header=false) LIMIT 1) LIKE 'aws%'",
		// Table function calls in general.
		"read_csv('/etc/passwd')",
		"a = read_text('/etc/passwd')",
		"a = chr(65)",
		"chr(65) = 'A'",
		"upper(a) = 'X'",
		// String concatenation primitive.
		"a = 'x' || 'y'",
		"a || b = 'ab'",
		// Subqueries and keywords used as structure.
		"a = (SELECT 1)",
		"a = 1 UNION SELECT 1",
		"1 = 1 FROM x",
		"SELECT * FROM t",
		"a IN (SELECT 1)",
		"a = 1 GROUP BY a",
		"a = 1 ORDER BY a",
		"a = 1 LIMIT 1",
		"a = 1 OFFSET 1",
		"a = 1; DROP TABLE users",
		"1=1); DROP TABLE users; --",
		// Comments and casts.
		"a = 1 --comment",
		"a = 1 /*comment*/",
		"a::text = '1'",
		"a = 1 @> 2",
		"a = 1 # 2",
		// Malformed literals.
		"a = 'unterminated",
		`a = 'back\slash'`,
		`a = 'line\nbreak'`,
		`a = "unclosed`,
		// Structural errors.
		"a =",
		"= 1",
		"a = 1 AND",
		"AND a = 1",
		"(a = 1",
		"a = 1)",
		"a IN ()",
		"a IN (1, b)",
		"a BETWEEN 1",
		"a IS TRUE",
		"a NOT 1",
		"a b",
		"a = 1 b = 2",
		"a LIKE 1",
		"a LIKE (1)",
		// Keyword used as bare column reference.
		"select",
		"from = 1",
	}

	for _, clause := range valid {
		if err := ValidateWhereClause(clause); err != nil {
			t.Errorf("ValidateWhereClause(%q) = %v, want nil", clause, err)
		}
	}

	for _, clause := range invalid {
		if err := ValidateWhereClause(clause); err == nil {
			t.Errorf("ValidateWhereClause(%q) = nil, want error", clause)
		}
	}
}

func TestValidateWhereClauseLimits(t *testing.T) {
	long := "a = " + strings.Repeat("1 OR ", 400) + "1"
	if err := ValidateWhereClause(long); err == nil {
		t.Error("ValidateWhereClause(clause over length limit) = nil, want error")
	}

	longString := "a = '" + strings.Repeat("x", 300) + "'"
	if err := ValidateWhereClause(longString); err == nil {
		t.Error("ValidateWhereClause(string literal over length limit) = nil, want error")
	}

	deep := strings.Repeat("(", 40) + "a = 1" + strings.Repeat(")", 40)
	if err := ValidateWhereClause(deep); err == nil {
		t.Error("ValidateWhereClause(deeply nested clause) = nil, want error")
	}

	notTooDeep := strings.Repeat("(", 10) + "a = 1" + strings.Repeat(")", 10)
	if err := ValidateWhereClause(notTooDeep); err != nil {
		t.Errorf("ValidateWhereClause(%d nested parens) = %v, want nil", 10, err)
	}
}
