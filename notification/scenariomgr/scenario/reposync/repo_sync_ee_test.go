//go:build ee || saas

package reposync

import (
	"testing"
)

func TestGetTaskURL(t *testing.T) {
	tests := []struct {
		name     string
		localURL string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid dataset URL",
			localURL: "https://opencsg-stg.com/datasets/AIWizards/gitea-go-sdk",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/datasets/AIWizards/gitea-go-sdk",
			wantErr:  false,
		},
		{
			name:     "valid model URL",
			localURL: "https://opencsg-stg.com/datasets/AIWizards/documentation-images",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/datasets/AIWizards/documentation-images",
			wantErr:  false,
		},
		{
			name:     "valid space URL",
			localURL: "https://opencsg-stg.com/spaces/demo/chatbot",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/spaces/demo/chatbot",
			wantErr:  false,
		},
		{
			name:     "valid model URL with Chroma",
			localURL: "https://opencsg-stg.com/models/AIWizards/Chroma",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/models/AIWizards/Chroma",
			wantErr:  false,
		},
		{
			name:     "URL with fragment",
			localURL: "https://opencsg-stg.com/models/test#section1",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/models/test",
			wantErr:  false,
		},
		{
			name:     "URL with port",
			localURL: "https://opencsg-stg.com:8080/datasets/test",
			want:     "https://opencsg-stg.com:8080/admin_panel/mirrors/datasets/test",
			wantErr:  false,
		},
		{
			name:     "URL with trailing slash",
			localURL: "https://opencsg-stg.com/datasets/test/",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/datasets/test/",
			wantErr:  false,
		},
		{
			name:     "root path URL",
			localURL: "https://opencsg-stg.com/",
			want:     "https://opencsg-stg.com/admin_panel/mirrors/",
			wantErr:  false,
		},
		{
			name:     "empty path URL",
			localURL: "https://opencsg-stg.com",
			want:     "https://opencsg-stg.com/admin_panel/mirrors",
			wantErr:  false,
		},
		{
			name:     "HTTP URL",
			localURL: "http://localhost:3000/datasets/test",
			want:     "http://localhost:3000/admin_panel/mirrors/datasets/test",
			wantErr:  false,
		},
		{
			name:     "invalid URL - missing scheme",
			localURL: "opencsg-stg.com/datasets/test",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "invalid URL - malformed",
			localURL: "://invalid-url",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "empty URL",
			localURL: "",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "URL without scheme",
			localURL: "opencsg-stg.com/datasets/test",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "URL without host",
			localURL: "https:///datasets/test",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTaskURL(tt.localURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("getTaskURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getTaskURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
