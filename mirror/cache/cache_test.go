package cache

import (
	"testing"
)

func Test_lfsProgressCacheKey(t *testing.T) {
	type args struct {
		repoPath string
		oid      string
		partSize string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoPath: "test",
				oid:      "oid",
				partSize: "partSize",
			},
			want: "lfs-sync-progress-test-oid-partSize",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lfsProgressCacheKey(tt.args.repoPath, tt.args.oid, tt.args.partSize); got != tt.want {
				t.Errorf("lfsProgressCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lfsPartCacheKey(t *testing.T) {
	type args struct {
		repoPath string
		oid      string
		partSize string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoPath: "test",
				oid:      "oid",
				partSize: "partSize",
			},
			want: "lfs-sync-test-oid-partSize",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lfsPartCacheKey(tt.args.repoPath, tt.args.oid, tt.args.partSize); got != tt.want {
				t.Errorf("lfsPartCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_uploadIDCacheKey(t *testing.T) {
	type args struct {
		repoPath string
		oid      string
		partSize string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				repoPath: "test",
				oid:      "oid",
				partSize: "partSize",
			},
			want: "upload-id-test-oid-partSize",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadIDCacheKey(tt.args.repoPath, tt.args.oid, tt.args.partSize); got != tt.want {
				t.Errorf("uploadIDCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
