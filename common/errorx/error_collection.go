package errorx

const errCollectionPrefix = "COLL-ERR"

const (
	repoAlreadyInCollection = iota
)

var (
	// the repository is already in the collection
	//
	// Description: The repository you are trying to add is already present in this collection.
	//
	// Description_ZH: 您尝试添加的仓库已经在此集合中。
	//
	// en-US: The repo was already in this collection
	//
	// zh-CN: 该仓库已经在此集合中
	//
	// zh-HK: 該倉庫已經在此集合中
	ErrRepoAlreadyInCollection error = CustomError{prefix: errCollectionPrefix, code: repoAlreadyInCollection}
)

func RepoAlreadyInCollection(err error, ctx context) error {
	if err == nil {
		return nil
	}
	return CustomError{
		prefix:  errCollectionPrefix,
		context: ctx,
		err:     err,
		code:    int(repoAlreadyInCollection),
	}
}



