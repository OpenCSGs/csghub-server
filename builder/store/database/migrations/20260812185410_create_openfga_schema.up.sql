-- OpenFGA storage schema after migrations 001 through 006.
-- The schema is kept as one Bun migration so a fresh Starhub database receives
-- the same final OpenFGA layout without replaying OpenFGA's upstream history.

CREATE TABLE IF NOT EXISTS tuple (
    store TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    _user TEXT NOT NULL,
    user_type TEXT NOT NULL,
    ulid TEXT NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL,
    condition_name TEXT,
    condition_context BYTEA,
    PRIMARY KEY (store, object_type, object_id, relation, _user)
);

--bun:split

CREATE INDEX IF NOT EXISTS idx_tuple_partial_user
    ON tuple (store, object_type, object_id, relation, _user)
    WHERE user_type = 'user';

--bun:split

CREATE INDEX IF NOT EXISTS idx_tuple_partial_userset
    ON tuple (store, object_type, object_id, relation, _user)
    WHERE user_type = 'userset';

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_tuple_ulid ON tuple (ulid);

--bun:split

-- The collated object ID index is the final reverse-lookup index from
-- OpenFGA migration 006; it replaces idx_reverse_lookup_user.
CREATE INDEX IF NOT EXISTS idx_user_lookup ON tuple (
    store,
    _user,
    relation,
    object_type,
    object_id COLLATE "C"
);

--bun:split

CREATE TABLE IF NOT EXISTS authorization_model (
    store TEXT NOT NULL,
    authorization_model_id TEXT NOT NULL,
    type TEXT NOT NULL,
    type_definition BYTEA,
    schema_version TEXT NOT NULL DEFAULT '1.0',
    serialized_protobuf BYTEA,
    PRIMARY KEY (store, authorization_model_id, type)
);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_authorization_model_id
    ON authorization_model (authorization_model_id);

--bun:split

CREATE TABLE IF NOT EXISTS store (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

--bun:split

CREATE TABLE IF NOT EXISTS assertion (
    store TEXT NOT NULL,
    authorization_model_id TEXT NOT NULL,
    assertions BYTEA,
    PRIMARY KEY (store, authorization_model_id)
);

--bun:split

CREATE TABLE IF NOT EXISTS changelog (
    store TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    _user TEXT NOT NULL,
    operation INTEGER NOT NULL,
    ulid TEXT NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL,
    condition_name TEXT,
    condition_context BYTEA,
    PRIMARY KEY (store, ulid, object_type)
);
