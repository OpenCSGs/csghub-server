package database

type TagStore struct {
	db *DB
}

func NewTagStore(db *DB) *TagStore {
	return &TagStore{
		db: db,
	}
}

type Tag struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	ParentID int64  `bun:",pk" json:"parent_id"`
	Name     string `bun:",notnull" json:"name"`
	TagType  string `bun:",notnull" json:"tag_type"`
	Group    string `bun:",notnull" json:"group"`
	times
}
