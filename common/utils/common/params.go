package common

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func GetNamespaceAndNameFromContext(ctx *gin.Context) (namespace string, name string, err error) {
	namespace = ctx.Param("namespace")
	name = ctx.Param("name")
	namespace_mapped := ctx.GetString("namespace_mapped")
	if namespace_mapped != "" {
		namespace = namespace_mapped
	}
	name_mapped := ctx.GetString("name_mapped")
	if name_mapped != "" {
		name = name_mapped
	}
	if namespace == "" || name == "" {
		err = errors.New("invalid namespace or name")
		ext := errorx.Ctx().Set("param", "namespace or name")
		err = errorx.ReqParamInvalid(err, ext)
		return
	}
	return
}

func GetPerAndPageFromContext(ctx *gin.Context) (perInt int, pageInt int, err error) {
	per := ctx.Query("per")
	if per == "" {
		per = "50"
	}
	perInt, err = strconv.Atoi(per)
	if err != nil {
		ext := errorx.Ctx().Set("query", "per")
		err = errorx.ReqParamInvalid(err, ext)
		return
	}
	page := ctx.Query("page")
	if page == "" {
		page = "1"
	}
	pageInt, err = strconv.Atoi(page)
	if err != nil {
		return
	}
	return
}

func RepoTypeFromContext(ctx *gin.Context) types.RepositoryType {
	rawRp, exist := ctx.Get("repo_type")
	slog.Debug("get repo type from context", "repo_type", rawRp, "exists", exist)
	if !exist {
		return types.UnknownRepo
	}
	return rawRp.(types.RepositoryType)
}

func SetRepoTypeContext(ctx *gin.Context, t types.RepositoryType) {
	ctx.Set("repo_type", t)
}

func RepoTypeFromParam(ctx *gin.Context) types.RepositoryType {
	rawRp := ctx.Param("repo_type")
	slog.Debug("get repo type from parameters", "repo_type", rawRp)
	return types.RepositoryType(rawRp)
}

func CalculateSSHKeyFingerprint(key string) (string, error) {
	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return "", err
	}
	fingerPrint := ssh.FingerprintSHA256(parsedKey)
	fingerPrint = strings.Split(fingerPrint, ":")[1]
	return fingerPrint, nil
}

func CalculateAuthorizedSSHKeyFingerprint(key string) (string, error) {
	decodedKey, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	hash := sha256.Sum256(decodedKey)
	base64Hash := base64.RawStdEncoding.EncodeToString(hash[:])
	return base64Hash, nil
}

func CalculateSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return hex.EncodeToString(bs)
}

func GetValidTimeDuration(ctx *gin.Context) (string, string) {
	defaultDuration := "30m"
	defaultRange := "1m"
	rawDuration := ctx.Query("last_duration")
	if len(rawDuration) < 1 {
		return defaultDuration, defaultRange
	}

	timeRange, ok := types.MonitorValidDurations[rawDuration]
	if ok {
		return rawDuration, timeRange
	}
	return defaultDuration, defaultRange
}

func GetNamespaceAndNameFromPath(path string) (namespace string, name string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		err = errors.New("invalid namespace or name")
		return
	}
	namespace = parts[0]
	name = parts[1]
	return
}
