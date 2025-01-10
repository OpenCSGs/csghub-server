package common

import (
	"reflect"
	"testing"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestWithPrefix(t *testing.T) {
	type args struct {
		name   string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "Test WithPrefix", args: args{name: "test", prefix: "prefix_"}, want: "prefix_test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithPrefix(tt.args.name, tt.args.prefix); got != tt.want {
				t.Errorf("WithPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithoutPrefix(t *testing.T) {
	type args struct {
		name   string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "Test WithoutPrefix when string has the prefix", args: args{name: "prefix_test", prefix: "prefix_"}, want: "test"},
		{name: "Test WithoutPrefix when string not has the prefix", args: args{name: "test", prefix: "prefix_"}, want: "test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithoutPrefix(tt.args.name, tt.args.prefix); got != tt.want {
				t.Errorf("WithoutPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertDotToSlash(t *testing.T) {
	type args struct {
		d string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "Test ConvertDotToSlash when string is dot", args: args{d: "."}, want: "/"},
		{name: "Test ConvertDotToSlash when string is dot dot", args: args{d: "a"}, want: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertDotToSlash(tt.args.d); got != tt.want {
				t.Errorf("ConvertDotToSlash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPortalCloneUrl(t *testing.T) {
	type args struct {
		url          string
		repoType     types.RepositoryType
		gitDomain    string
		portalDomain string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test PortalCloneUrl when git domain config right",
			args: args{
				url:          "https://gitea.com/models/abc.git",
				repoType:     types.ModelRepo,
				gitDomain:    "https://gitea.com",
				portalDomain: "https://portal.com",
			},
			want: "https://portal.com/models/abc.git",
		},
		{
			name: "Test PortalCloneUrl when git domain config wrong",
			args: args{
				url:          "https://gitea.com/models/abc.git",
				repoType:     types.ModelRepo,
				gitDomain:    "https://gitea.com:80",
				portalDomain: "https://portal.com",
			},
			want: "https://gitea.com/models/abc.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PortalCloneUrl(tt.args.url, tt.args.repoType, tt.args.gitDomain, tt.args.portalDomain); got != tt.want {
				t.Errorf("PortalCloneUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCloneInfo(t *testing.T) {
	type args struct {
		config     *config.Config
		repository *database.Repository
	}
	tests := []struct {
		name string
		args args
		want types.Repository
	}{
		{
			name: "Test BuildCloneInfo when SSHDomain has ssh:// prefix",
			args: args{
				config: &config.Config{
					APIServer: struct {
						Port         int    `env:"STARHUB_SERVER_SERVER_PORT, default=8080"`
						PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN, default=http://localhost:8080"`
						SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN, default=git@localhost:2222"`
					}{
						Port:         8080,
						PublicDomain: "https://opencsg.com",
						SSHDomain:    "ssh://git@opencsg.com",
					},
				},
				repository: &database.Repository{
					RepositoryType: types.ModelRepo,
					Path:           "abc/def",
				},
			},
			want: types.Repository{
				HTTPCloneURL: "https://opencsg.com/models/abc/def.git",
				SSHCloneURL:  "git@opencsg.com:models/abc/def.git",
			},
		},
		{
			name: "Test BuildCloneInfo when SSHDomain without ssh:// prefix",
			args: args{
				config: &config.Config{
					APIServer: struct {
						Port         int    `env:"STARHUB_SERVER_SERVER_PORT, default=8080"`
						PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN, default=http://localhost:8080"`
						SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN, default=git@localhost:2222"`
					}{
						Port:         8080,
						PublicDomain: "https://opencsg.com",
						SSHDomain:    "git@opencsg.com",
					},
				},
				repository: &database.Repository{
					RepositoryType: types.ModelRepo,
					Path:           "abc/def",
				},
			},
			want: types.Repository{
				HTTPCloneURL: "https://opencsg.com/models/abc/def.git",
				SSHCloneURL:  "git@opencsg.com:models/abc/def.git",
			},
		},
		{
			name: "Test BuildCloneInfo when SSHDomain without ssh:// prefix and port",
			args: args{
				config: &config.Config{
					APIServer: struct {
						Port         int    `env:"STARHUB_SERVER_SERVER_PORT, default=8080"`
						PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN, default=http://localhost:8080"`
						SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN, default=git@localhost:2222"`
					}{
						Port:         8080,
						PublicDomain: "https://opencsg.com",
						SSHDomain:    "ssh://git@opencsg.com:2222",
					},
				},
				repository: &database.Repository{
					RepositoryType: types.ModelRepo,
					Path:           "abc/def",
				},
			},
			want: types.Repository{
				HTTPCloneURL: "https://opencsg.com/models/abc/def.git",
				SSHCloneURL:  "ssh://git@opencsg.com:2222/models/abc/def.git",
			},
		},
		{
			name: "Test BuildCloneInfo when SSHDomain is IPv6 with ssh:// prefix and port",
			args: args{
				config: &config.Config{
					APIServer: struct {
						Port         int    `env:"STARHUB_SERVER_SERVER_PORT, default=8080"`
						PublicDomain string `env:"STARHUB_SERVER_PUBLIC_DOMAIN, default=http://localhost:8080"`
						SSHDomain    string `env:"STARHUB_SERVER_SSH_DOMAIN, default=git@localhost:2222"`
					}{
						Port:         8080,
						PublicDomain: "https://opencsg.com",
						SSHDomain:    "ssh://[2001:db8::8a2e:370:7334]:2222",
					},
				},
				repository: &database.Repository{
					RepositoryType: types.ModelRepo,
					Path:           "abc/def",
				},
			},
			want: types.Repository{
				HTTPCloneURL: "https://opencsg.com/models/abc/def.git",
				SSHCloneURL:  "ssh://[2001:db8::8a2e:370:7334]:2222/models/abc/def.git",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildCloneInfo(tt.args.config, tt.args.repository); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildCloneInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "Test IsValidName when name is valid", args: args{name: "abc"}, want: true},
		{name: "Test IsValidName when name is valid", args: args{name: "abc_def"}, want: true},
		{name: "Test IsValidName when name is valid", args: args{name: "abc-def"}, want: true},
		{name: "Test IsValidName when name is invalid", args: args{name: "abc/def"}, want: false},
		{name: "Test IsValidName when name is invalid", args: args{name: "abc def"}, want: false},
		{name: "Test IsValidName when name is invalid", args: args{name: "abc__def"}, want: false},
		{name: "Test IsValidName when name is invalid", args: args{name: "a"}, want: false},
		{name: "Test IsValidName when name is invalid", args: args{name: "abc..def"}, want: false},
		{name: "Test IsValidName when name is invalid", args: args{name: "--def"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := IsValidName(tt.args.name); got != tt.want {
				t.Errorf("IsValidName() = %v, want %v", got, tt.want)
			}
		})
	}
}
