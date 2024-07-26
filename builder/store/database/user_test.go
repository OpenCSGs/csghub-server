package database

import (
	"reflect"
	"testing"
)

func TestUser_Roles(t *testing.T) {
	type fields struct {
		RoleMask string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		// TODO: Add test cases.
		{
			name: "test no role",
			fields: fields{
				RoleMask: "",
			},
			want: []string{},
		},
		{
			name: "test one role",
			fields: fields{
				RoleMask: "admin",
			},
			want: []string{"admin"},
		},
		{
			name: "test two roles",
			fields: fields{
				RoleMask: "admin,super_user",
			},
			want: []string{"admin", "super_user"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				RoleMask: tt.fields.RoleMask,
			}
			if got := u.Roles(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("User.Roles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_SetRoles(t *testing.T) {
	type fields struct {
		RoleMask string
	}
	type args struct {
		roles []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
		{
			name: "test no role",
			fields: fields{
				RoleMask: "",
			},
			args: args{
				roles: []string{""},
			},
		},
		{
			name: "test one role",
			fields: fields{
				RoleMask: "admin",
			},
			args: args{
				roles: []string{"admin"},
			},
		},
		{
			name: "test two roles",
			fields: fields{
				RoleMask: "admin,super_user",
			},
			args: args{
				roles: []string{"admin", "super_user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{}
			u.SetRoles(tt.args.roles)
			if u.RoleMask != tt.fields.RoleMask {
				t.Errorf("User.SetRoles() = %v, want %v", u.RoleMask, tt.fields.RoleMask)
			}
		})
	}
}
