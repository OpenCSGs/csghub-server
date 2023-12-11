package database

type TagStore struct {
	db *DB
}

func NewTagStore(db *DB) *TagStore {
	return &TagStore{
		db: db,
	}
}

type TagScope string

const (
	ModelTagScope    TagScope = "model"
	DatabaseTagScope TagScope = "database"
)

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name"`
	Category string   `bun:",notnull" json:"category"`
	Group    string   `bun:",notnull" json:"group"`
	Scope    TagScope `bun:",notnull" json:"scope"`
	times
}
