package database

import "testing"

func TestRepoListCountLimit(t *testing.T) {
	tests := []struct {
		name string
		per  int
		page int
		want int
	}{
		{name: "first page counts through page 101", per: 20, page: 1, want: 2020},
		{name: "page 100 keeps page 101 reachable", per: 20, page: 100, want: 2020},
		{name: "count advances after page 100", per: 20, page: 101, want: 2040},
		{name: "unbounded request", per: 0, page: 1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoListCountLimit(tt.per, tt.page)
			if got != tt.want {
				t.Errorf("repoListCountLimit(%d, %d) = %d, want %d", tt.per, tt.page, got, tt.want)
			}
		})
	}
}
