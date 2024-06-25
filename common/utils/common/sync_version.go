package common

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

func AddPrefixBySourceID(sourceID int64, originString string) string {
	return fmt.Sprintf("%s%s", getprefixBySourceID(sourceID), originString)
}

func TrimPrefixCloneURLBySourceID(url, repoType, namespace, name string, sourceID int64) string {
	namespace, _ = strings.CutPrefix(namespace, getprefixBySourceID(sourceID))
	return fmt.Sprintf("%s/%ss/%s/%s.git", url, repoType, namespace, name)
}

func getprefixBySourceID(sourceID int64) string {
	var prefix string
	if sourceID == int64(types.SyncVersionSourceOpenCSG) {
		prefix = types.OpenCSGPrefix
	} else if sourceID == int64(types.SyncVersionSourceHF) {
		prefix = types.HuggingfacePrefix
	}
	return prefix
}
