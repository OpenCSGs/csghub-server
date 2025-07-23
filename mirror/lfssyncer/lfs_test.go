package lfssyncer

import (
	"reflect"
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func TestSplitPointersBySizeAndCount(t *testing.T) {
	type args struct {
		pointers []*types.Pointer
	}
	tests := []struct {
		name string
		args args
		want [][]*types.Pointer
	}{
		{
			name: "test with 1 pointer",
			args: args{
				pointers: []*types.Pointer{
					{
						Oid:  "1",
						Size: 100,
					},
				},
			},
			want: [][]*types.Pointer{
				{
					{
						Oid:  "1",
						Size: 100,
					},
				},
			},
		},
		{
			name: "test with pointers over 10G",
			args: args{
				pointers: []*types.Pointer{
					{
						Oid:  "1",
						Size: 5 * 1024 * 1024 * 1024,
					},
					{
						Oid:  "2",
						Size: 5 * 1024 * 1024 * 1024,
					},
					{
						Oid:  "3",
						Size: 5 * 1024 * 1024 * 1024,
					},
				},
			},
			want: [][]*types.Pointer{
				{
					{
						Oid:  "1",
						Size: 5 * 1024 * 1024 * 1024,
					},
					{
						Oid:  "2",
						Size: 5 * 1024 * 1024 * 1024,
					},
				},
				{
					{
						Oid:  "3",
						Size: 5 * 1024 * 1024 * 1024,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitPointersBySizeAndCount(tt.args.pointers); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitPointersBySizeAndCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
