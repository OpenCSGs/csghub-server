package gitaly

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

const (
	// MetaFileIdentifier is the string appearing at the first line of LFS pointer files.
	// https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
	MetaFileIdentifier = "version https://git-lfs.github.com/spec/v1"

	// MetaFileOidPrefix appears in LFS pointer files on a line before the sha256 hash.
	MetaFileOidPrefix = "oid sha256:"
)

var (
	// ErrMissingPrefix occurs if the content lacks the LFS prefix
	ErrMissingPrefix = errors.New("content lacks the LFS prefix")

	// ErrInvalidStructure occurs if the content has an invalid structure
	ErrInvalidStructure = errors.New("content has an invalid structure")

	// ErrInvalidOIDFormat occurs if the oid has an invalid format
	ErrInvalidOIDFormat = errors.New("OID has an invalid format")

	oidPattern = regexp.MustCompile(`^[a-f\d]{64}$`)
)

// ReadPointerFromBuffer will return a pointer if the provided byte slice is a pointer file or an error otherwise.
func ReadPointerFromBuffer(buf []byte) (types.Pointer, error) {
	var p types.Pointer

	headString := string(buf)
	if !strings.HasPrefix(headString, MetaFileIdentifier) {
		return p, ErrMissingPrefix
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return p, ErrInvalidStructure
	}

	oid := strings.TrimPrefix(splitLines[1], MetaFileOidPrefix)
	if len(oid) != 64 || !oidPattern.MatchString(oid) {
		return p, ErrInvalidOIDFormat
	}
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if err != nil {
		return p, err
	}

	p.Oid = oid
	p.Size = size

	return p, nil
}
