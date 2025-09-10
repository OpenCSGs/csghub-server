package validator

import (
	"errors"
	"strings"

	"opencsg.com/csghub-server/common/utils/common"
)

func ValidateRepoPath(path string) error {
	if path == "" {
		return errors.New("repo path is required")
	}
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		return errors.New("repo path must be in the format of <owner>/<repo>")
	}

	if parts[0] == "" || parts[1] == "" {
		return errors.New("repo path must be in the format of <owner>/<repo>")
	}

	_, err := common.IsValidName(parts[0])
	if err != nil {
		return err
	}

	_, err = common.IsValidName(parts[1])
	if err != nil {
		return err
	}

	return nil
}
