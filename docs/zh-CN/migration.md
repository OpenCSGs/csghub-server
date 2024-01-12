# Migration

我们使用 `Golang` 的 [bun](https://bun.uptrace.dev/guide/migrations.html) 包来进行数据库迁移的管理。迁移文件可以是一个 `.go` 文件，也可以是一个 `.sql` 文件。

## Migration 名称

Migration 文件的名字有两个部分，时间戳和迁移名（也就是这个迁移文件做了什么），例如我们新建一个迁移文件来创建一个 users 表，可以使用 create_users.go 或 create_users.sql 这两种文件名。一个迁移文件尽量只做一件事情，这样会便于使用，在回滚和执行迁移的时候都可以做到更加灵活。

## 创建数据库

在本地使用 Starhub Server 之前，需要手动创建数据库并配置系统中的三个环境变量。

| 变量 | 含义 | 默认值 |
| --- | --- | --- |
| STARHUB_DATABASE_DRIVER | 数据库的 dirver，例如 pg | pg |
| STARHUB_DATABASE_DSN | 数据库的连接 DSN | postgresql://postgres:postgres@localhost:5432/starhub_server?sslmode=disable |
| STARHUB_DATABASE_TIMEZONE | 数据库的时区 | Asia/Shanghai |

## 初始化数据库迁移

创建数据库之后，需要执行命令手动初始化迁移。

```bash
# 编译项目
go build -o bin/starhub  ./cmd/csghub-server

# 初始化迁移
./bin/starhub migration init
```
指定初始化迁移命令后，数据库中会创建两个表用来管理数据库迁移的版本，这个两个表是 `bun_migrations` 和 `bun_migration_locks`。

## 执行数据库迁移

在初始化数据库迁移之后，我们需要执行数据库迁移来初始化数据库。

```bash
./bin/starhub migration migrate
```

## 创建数据库迁移

在对 Starhub Server 进行贡献的过程中，很有可能需要添加一些字段或者表，这个时候就需要创建数据库迁移文件来对现有的数据库进行扩充或者修改。创建数据库迁移的命令有两个。

```bash
# 创建 .sql 格式的迁移文件
./bin/starhub migration create_sql <迁移名称>

# 创建 .go 格式的迁移文件
./bin/starhub migration create_go <迁移名称>
```

例如我们需要创建一个名字为 create_users 的迁移文件来创建一个 users 表，我们可以执行 `./bin/starhub migration create_go create_users` 来创建一个迁移文件。这个命令会在 `builder/store/migrations` 目录中新建一个格式例如 `20240103065315_create_users.go` 的文件。

然后我们可以在这个文件中添加如下内容：

```go
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
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	Username    string `bun:",notnull" json:"username"`
	times
}
```

然后执行 `./bin/starhub migration migrate` ，就会创建一个 users 表。这个表会有四个列，分别是 `id` `username` `created_at` `updated_at` 。

这个时候我们可能需要对用户的 `username` 字段加上唯一索引，我们可以创建一个 `.sql` 格式的迁移文件来做这个事情。使用命令 `./bin/starhub migration create_sql add_index_for_users` 命令来创建一个名为 `add_index_for_users` 的迁移文件，这个命令会生成两个文件，一个名字格式为 `20240104063114_add_index_for_users.up.sql` ；另一个名字格式为 `20240104063114_add_index_for_users.down.sql` 。它们的不同在于 `.sql` 前的 `up` 和 `down`

，其中包含 `up` 关键字的代表是在执行数据库迁移时所执行的文件，也就是执行 `./bin/starhub migration migrate` 时执行的迁移文件；包含 `down` 关键字的则是在执行数据库迁移回滚的时候所执行的文件，也就是执行 `./bin/starhub migration rollback` 时执行的迁移文件。 我们在`20240104063114_add_index_for_users.up.sql`这个文件中添加如下内容：

```sql

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
```

在`20240104063114_add_index_for_users.down.sql` 中添加如下内容：

```sql
CREATE INDEX IF EXISTS idx_users_username;
```

这样在执行 `./bin/starhub migration migrate` 时会创建一个名为 `idx_users_username` 的索引，在执行 `./bin/starhub migration rollback` 时会将名为 `idx_users_username` 的索引删掉。

*注意：如果需要在 `.sql` 文件中书写多条 SQL 语句时，需要使用 `-bun:split` 将多条 SQL 语句隔开。例如：*

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