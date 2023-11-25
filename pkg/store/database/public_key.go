package database

type PublicKey struct {
	ID     int    `bun:",pk,autoincrement" json:"id"`
	UserID string `bun:",notnull" json:"user_id"`
	Value  string `bun:",notnull" json:"value"`
	User   User   `bun:"rel:belongs-to,join:user_id=id"`
	times
}
