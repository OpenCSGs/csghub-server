package scheduler

import (
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestDeployRunner_getHttpCloneURLWithToken(t *testing.T) {
	type fields struct {
		repo            *RepoInfo
		task            *database.DeployTask
		ir              imagerunner.Runner
		store           database.DeployTaskStore
		tokenStore      database.AccessTokenStore
		urs             database.UserResourcesStore
		deployStartTime time.Time
		deployCfg       common.DeployConfig
	}
	type args struct {
		httpCloneUrl string
		username     string
		token        string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{name: "test with username and token", fields: fields{}, args: args{httpCloneUrl: "https://opencsg.com/abc/def.git", username: "username", token: "token"}, want: "https://username:token@opencsg.com/abc/def.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &DeployRunner{
				repo:            tt.fields.repo,
				task:            tt.fields.task,
				ir:              tt.fields.ir,
				store:           tt.fields.store,
				tokenStore:      tt.fields.tokenStore,
				urs:             tt.fields.urs,
				deployStartTime: tt.fields.deployStartTime,
				deployCfg:       tt.fields.deployCfg,
				logReporter:     mockReporter.NewMockLogCollector(t),
			}
			if got := tr.getHttpCloneURLWithToken(tt.args.httpCloneUrl, tt.args.username, tt.args.token); got != tt.want {
				t.Errorf("DeployRunner.getHttpCloneURLWithToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
