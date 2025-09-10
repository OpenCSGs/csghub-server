package types

import (
	"time"

	"opencsg.com/csghub-server/api/httpbase"
)

var (
	LicenseStatusActive   string = "active"
	LicenseStatusInactive string = "inactive"
	LicenseStatusExpired  string = "expired"

	PublicKeyType  string = "PUBLIC KEY"
	PrivateKeyType string = "RSA PRIVATE KEY"

	PEMHeader string = "-----BEGIN LICENSE KEY-----"
	PEMFooter string = "-----END LICENSE KEY-----"
)

type RSAInfo struct {
	Payload   []byte
	Signature []byte
}

type RSAPayload struct {
	Key string `json:"key"`
	DataBody
}

type CreateLicenseReq struct {
	Key string `json:"-"`
	DataBody
	Remark      string `json:"remark"`
	CurrentUser string `json:"current_user"`
}

type DataBody struct {
	Company    string    `json:"company" binding:"required"`
	Email      string    `json:"email" binding:"email"`
	Product    string    `json:"product" binding:"required"`
	Edition    string    `json:"edition" binding:"required"`
	MaxUser    int       `json:"max_user" binding:"required,min=1"`
	StartTime  time.Time `json:"start_time" binding:"required"`
	ExpireTime time.Time `json:"expire_time" binding:"required"`
	Extra      string    `json:"extra"`
	Version    string    `json:"version"`
}

type QueryLicenseReq struct {
	Product     string `json:"product"`
	Edition     string `json:"edition"`
	Search      string `json:"search"`
	CurrentUser string `json:"current_user"`
	Page        int    `json:"page"`
	Per         int    `json:"per"`
}

type ImportLicenseReq struct {
	Data        string `json:"data" binding:"required"`
	CurrentUser string `json:"-"`
}

type GetLicenseReq struct {
	ID          int64  `json:"id"`
	CreateFile  bool   `json:"create_file"`
	CurrentUser string `json:"current_user"`
}

type UpdateLicenseReq struct {
	Company     *string    `json:"company"`
	Email       *string    `json:"email"`
	Product     *string    `json:"product"`
	Edition     *string    `json:"edition"`
	MaxUser     *int       `json:"max_user"`
	StartTime   *time.Time `json:"start_time"`
	ExpireTime  *time.Time `json:"expire_time"`
	Extra       *string    `json:"extra"`
	Version     *string    `json:"version"`
	Remark      *string    `json:"remark"`
	CurrentUser string     `json:"-"`
}

type LicenseStatusResp struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
	DataBody
	Users int `json:"users"`
}

type LicenseStatusReq struct {
	CurrentUser string            `json:"product"`
	AuthType    httpbase.AuthType `json:"auth_type"`
}
