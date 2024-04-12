package database

import "github.com/uptrace/bun"

type SelectOption interface {
	Appply(query *bun.SelectQuery)
}

type ColumnOption struct {
	cols []string
}

// Appply implements SelectOption.
func (c *ColumnOption) Appply(q *bun.SelectQuery) {
	q.Column(c.cols...)
}

func Columns(columns ...string) SelectOption {
	return &ColumnOption{
		cols: columns,
	}
}
