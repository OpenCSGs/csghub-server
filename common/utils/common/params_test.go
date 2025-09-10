package common

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoTypeFromString(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    types.RepositoryType
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				path: "models",
			},
			want:    types.ModelRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "datasets",
			},
			want:    types.DatasetRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "codes",
			},
			want:    types.CodeRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "spaces",
			},
			want:    types.SpaceRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "prompts",
			},
			want:    types.PromptRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "mcpservers",
			},
			want:    types.MCPServerRepo,
			wantErr: false,
		},
		{
			name: "valid",
			args: args{
				path: "templates",
			},
			want:    types.TemplateRepo,
			wantErr: false,
		},
		{
			name: "invalid",
			args: args{
				path: "model",
			},
			want:    types.UnknownRepo,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RepoTypeFromString(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RepoTypeFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RepoTypeFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPerAndPageFromContext(t *testing.T) {
	type args struct {
		ctx *gin.Context
	}
	tests := []struct {
		name        string
		args        args
		wantPerInt  int
		wantPageInt int
		wantErr     bool
	}{

		{
			name: "valid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=10&page=1",
						},
					},
				},
			},
			wantPerInt:  10,
			wantPageInt: 1,
			wantErr:     false,
		},
		{
			name: "valid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=10&page=2",
						},
					},
				},
			},
			wantPerInt:  10,
			wantPageInt: 2,
			wantErr:     false,
		},
		{
			name: "invalid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=0&page=2",
						},
					},
				},
			},
			wantPerInt:  0,
			wantPageInt: 0,
			wantErr:     true,
		},
		{
			name: "invalid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=-100&page=2",
						},
					},
				},
			},
			wantPerInt:  -100,
			wantPageInt: 0,
			wantErr:     true,
		},
		{
			name: "invalid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=110&page=2",
						},
					},
				},
			},
			wantPerInt:  110,
			wantPageInt: 0,
			wantErr:     true,
		},
		{
			name: "invalid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=10&page=0",
						},
					},
				},
			},
			wantPerInt:  10,
			wantPageInt: 0,
			wantErr:     true,
		},
		{
			name: "invalid",
			args: args{
				ctx: &gin.Context{
					Request: &http.Request{
						URL: &url.URL{
							RawQuery: "per=10&page=-1",
						},
					},
				},
			},
			wantPerInt:  10,
			wantPageInt: -1,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPerInt, gotPageInt, err := GetPerAndPageFromContext(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPerAndPageFromContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPerInt != tt.wantPerInt {
				t.Errorf("GetPerAndPageFromContext() gotPerInt = %v, want %v", gotPerInt, tt.wantPerInt)
			}
			if gotPageInt != tt.wantPageInt {
				t.Errorf("GetPerAndPageFromContext() gotPageInt = %v, want %v", gotPageInt, tt.wantPageInt)
			}
		})
	}
}
