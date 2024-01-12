# Migration

We use `Golang`'s [bun](https://bun.uptrace.dev/guide/migrations.html) package to manage database migrations. Migration files can be either a `.go` file or a `.sql` file.

## Migration Naming

A migration file has two parts in its name: a timestamp and a migration name (indicating what the migration file does). For example, when creating a migration file to create a `users` table, you can use either `create_users.go` or `create_users.sql` as the file name. It is recommended to have a migration file focus on a single task for flexibility during both migration execution and rollback.

## Creating the Database

Before using Starhub Server locally, it is necessary to manually create the database and configure three environment variables in the system.

| Variable | Meaning | Default Value |
| --- | --- | --- |
| STARHUB_DATABASE_DRIVER | Database driver, e.g., pg | pg |
| STARHUB_DATABASE_DSN | Database connection DSN | postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable |
| STARHUB_DATABASE_TIMEZONE | Database timezone | Asia/Shanghai |

## Initializing Database Migrations

After creating the database, execute the command to manually initialize migrations.

```bash
# Compile the project
go build -o bin/starhub ./cmd/csghub-server

# Initialize migrations
./bin/starhub migration init
```

By specifying the migration initialization command, two tables (`bun_migrations` and `bun_migration_locks`) will be created in the database to manage migration versions.

## Executing Database Migrations

After initializing database migrations, it's necessary to execute them to initialize the database.

```bash
./bin/starhub migration migrate
```

## Creating Database Migrations

During the contribution process to Starhub Server, there might be a need to add fields or tables. In such cases, create database migration files to expand or modify the existing database. There are two commands for creating migration files.

```bash
# Create a .sql format migration file
./bin/starhub migration create_sql <migration_name>

# Create a .go format migration file
./bin/starhub migration create_go <migration_name>
```

For example, to create a migration file named `create_users` for creating a `users` table, you can execute `./bin/starhub migration create_go create_users`. This command will create a migration file in the `builder/store/migrations` directory with a format like `20240103065315_create_users.go`.

Then, you can add the following content to this file:

```golang
package migrations

import (
    "context"

    "github.com/uptrace/bun"
)

func init() {
    Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
        return createTables(ctx, db, User{})
    }, func(ctx context.Context, db *bun.DB) error {
        return dropTables(ctx, db, User{})
    })
}

type User struct {
    ID       int64  `bun:",pk,autoincrement" json:"id"`
    Username string `bun:",notnull" json:"username"`
    times
}
```

After that, execute `./bin/starhub migration migrate`, and the users table will be created with four columns: `id`, `username`, `created_at`, and `updated_at`.

At this point, you may need to add a unique index to the `username` field. You can create a `.sql` format migration file for this task. Use the command `./bin/starhub migration create_sql add_index_for_users` to create a migration file named `add_index_for_users`. This command will generate two files, one named `20240104063114_add_index_for_users.up.sql` and the other named `20240104063114_add_index_for_users.down.sql`. The difference lies in the presence of the `up` and `down` keywords before `.sql`. The file with `up` contains SQL statements executed during migration (`./bin/starhub migration migrate`), and the file with `down` contains statements executed during rollback (`./bin/starhub migration rollback`). Add the following content to `20240104063114_add_index_for_users.up.sql`:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
```

In `20240104063114_add_index_for_users.down.sql`, add the following content:

```sql
CREATE INDEX IF EXISTS idx_users_username;
```

This way, executing `./bin/starhub migration migrate` will create an index named `idx_users_username`, and executing `./bin/starhub migration rollback` will remove the index named `idx_users_username`.

*Note: If you need to write multiple SQL statements in a .sql file, use -bun:split to separate them. For example:*

```sql
SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_path ON repositories(path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_user_id ON repositories(user_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_git_path ON repositories(git_path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_repository_type ON repositories(repository_type);

```
