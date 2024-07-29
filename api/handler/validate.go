package handler

import (
	"github.com/go-playground/validator/v10"
	"opencsg.com/csghub-server/common/types"
)

var (
	Validate *validator.Validate = validator.New()
)

func init() {
	Validate.RegisterValidation("validateMinMaxReplica", validateMinMaxReplicaOfDeploy)
}

func validateMinMaxReplicaOfDeploy(fl validator.FieldLevel) bool {
	req := fl.Top().Interface().(*types.DeployUpdateReq)
	if *req.MinReplica < 0 || *req.MaxReplica < 0 || *req.MinReplica > *req.MaxReplica {
		return false
	}
	return true
}
