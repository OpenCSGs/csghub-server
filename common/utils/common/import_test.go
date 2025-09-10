package common

import (
	"testing"
)

func TestConvertURLWithAuth(t *testing.T) {
	type args struct {
		baseURL  string
		username string
		password string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "should return error when baseURL is empty",
			args: args{
				baseURL:  "",
				username: "username",
				password: "password",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "should return right url with http schema",
			args: args{
				baseURL:  "http://example.com",
				username: "username",
				password: "password",
			},
			want:    "http://username:password@example.com",
			wantErr: false,
		},
		{
			name: "should return right url with https schema",
			args: args{
				baseURL:  "https://example.com",
				username: "username",
				password: "password",
			},
			want:    "https://username:password@example.com",
			wantErr: false,
		},
		{
			name: "should return error with unknown schema",
			args: args{
				baseURL:  "httpa://example.com",
				username: "username",
				password: "password",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertURLWithAuth(tt.args.baseURL, tt.args.username, tt.args.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertURLWithAuth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertURLWithAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}
