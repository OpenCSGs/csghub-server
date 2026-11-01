# ReBAC 使用方法

开始之前，先看自己属于哪种情况：

1. **需要修改或更新模型**：比如新增资源类型、增加关系，或者调整权限规则，需要新增模型版本、执行 migration，并在准备好后更新 Latest。具体看第 1～3 节和第 6 节。
2. **仅使用现有模型**：比如给用户授权、取消授权、检查权限，或者查询能访问的资源，直接初始化 ReBAC 并调用接口即可，**不需要修改模型、新增模型版本或更新 Latest**。具体看第 4～5 节。这里说的是数据库已经完成现有模型的初始化；首次部署仍需执行第 3 节的初始化 migration。

给用户新增或删除关系，是在使用模型，不是更新模型。只有现有模型表达不了需要的权限规则时，才需要修改模型。

## 1、怎么修改模型

模型文件放在 `builder/store/database/openfga`：

- `dsl_v1_0.go`：用户、组织、命名空间和仓库权限。
- `dsl_v1_1.go`：增加 `organization.member_from_child`，可以查询子组织成员。
- `dsl_v1_2.go`：增加知识库权限，目前默认用这个版本。

要改权限规则，就从当前版本复制一份新模型。比如从 `dsl_v1_2.go` 复制成 `dsl_v1_3.go`，把常量名改成 `AuthorizationModelVer1_3`，然后修改里面的 DSL。已经使用过的旧模型保留下来，不要直接覆盖。

模型 ID 在 [`common/types/rebac.go`](../../common/types/rebac.go) 中定义。新模型需要新增一个 ID，例如 `OpenFgaAuthorizationModelIDVer1_3`，值用 `ulid.Make().String()` 生成。等新模型的 migration 执行完，再将 `OpenFgaAuthorizationModelIDLatest` 指向它并重新部署服务。

这里的 1.0、1.1、1.2 是项目自己的模型版本。模型开头的 `schema 1.1` 是 OpenFGA 的语法版本，新增项目模型版本时不用跟着改。

如果新增了对象类型、关系或权限，记得一起更新 `builder/rebac` 下的 `object_type.go`、`permission.go` 和 `schema.go`，否则 ReBAC 会把新类型或关系当成不支持的参数。

## 2、模型怎么写

先看一个简化的仓库模型：

```text
model
  schema 1.1

type user

type repository
  relations
    define owner: [user]
    define writer: [user]
    define reader: [user]

    define can_write: owner or writer
    define can_read: can_write or reader
```

这段模型表示：

- `owner`、`writer`、`reader` 是可以写入的关系，`[user]` 表示关系的主体是用户。
- `can_write` 表示 owner 和 writer 都能写。
- `can_read` 表示能写的人以及 reader 都能读。

所以，给用户写入 `reader` 关系后，检查 `can_read` 就会得到允许。`can_read`、`can_write` 是算出来的权限，不要把它们当成关系写入。

项目的完整模型还包含组织继承。例如 `can_read from organization` 表示沿仓库的 `organization` 关系，检查关联组织的读取权限。修改时保留完整模型里的其他定义，上面的例子只是帮助理解写法。

具体语法可以看 OpenFGA 的[模型编写文档](https://openfga.dev/docs/configuration-language)和[建模入门](https://openfga.dev/docs/modeling/getting-started)。模型版本的处理方式见[不可变模型文档](https://openfga.dev/docs/getting-started/immutable-models)。

## 3、把模型写入数据库

模型文件写好后，还需要通过 migration 把它存进数据库。可以直接参考：

[`20260827160000_update_openfga_store_model.go`](../store/database/migrations/20260827160000_update_openfga_store_model.go)

先生成一份新的 migration（或者手动复制一份20260827160000_update_openfga_store_model.go）：

```bash
go run -tags saas cmd/csghub-server/main.go --config config.toml migration create_go update_openfga_model
```

参考文件里的核心代码如下。新增版本时，把两个 1.1 常量换成新版本对应的常量即可，import 也可以参考原文件：

```go
// init registers model version 1.1 and its rollback.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return insertAuthorizationModel(
			ctx,
			db,
			types.OpenFgaAuthorizationModelIDVer1_1,
			modelopenfga.AuthorizationModelVer1_1,
		)
	}, func(ctx context.Context, db *bun.DB) error {
		return deleteAuthorizationModel(ctx, db, types.OpenFgaAuthorizationModelIDVer1_1)
	})
}
```

`insertAuthorizationModel` 会把 DSL 转成 OpenFGA 的模型并写入数据库，`deleteAuthorizationModel` 用于回滚这一个模型。新版本要用新 ID，重复使用旧 ID 会报错。

第一次初始化数据库时，先执行 `init`，再执行 `migrate`；后续新增模型执行 `migrate` 即可。`config.toml` 换成自己的配置文件，需要配置好 PostgreSQL 和 Redis。

```bash
# Run once to create migration tracking tables.
go run -tags saas cmd/csghub-server/main.go --config config.toml migration init

# Apply pending migrations.
go run -tags saas cmd/csghub-server/main.go --config config.toml migration migrate
```

首次迁移会建 OpenFGA 的表、初始化 Store、写入模型并回填已有关系。Store 和模型 1.0 的初始化在 `20260813140000_init_openfga_store_model.go`，知识库模型和关系回填在 `20260831171517_add_knowledge_base_rebac_model.go`。

只更新模型不会自动给资源授权。如果新模型增加了关系，已有资源需要的关系也要在 migration 中回填。

## 4、初始化 ReBAC

服务已经初始化 PostgreSQL 数据库，并执行完 migration 后，直接通过 factory 创建：

```go
// Import "opencsg.com/csghub-server/builder/rebac/factory".
authorizer, err := factory.NewAuthorizer()
if err != nil {
	return err
}
```

这种方式会使用默认的授权模型，也就是 `OpenFgaAuthorizationModelIDLatest`， 创建后把 `authorizer` 传给需要检查权限的组件，业务代码使用 `rebac.Authorizer` 接口即可。默认 Provider 会复用，不需要每次请求都重新初始化，也不要在请求结束时关闭它。

如果需要固定使用某个模型，比如做 migration 回填，可以自己创建 Provider：

```go
// Import builder/rebac, builder/rebac/openfga, and common/types.
provider, err := openfga.NewCustomProvider(
	openfga.WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_2),
)
if err != nil {
	return err
}
defer provider.Close()

authorizer, err := rebac.NewAuthorizer(provider)
if err != nil {
	return err
}
// Use authorizer before this task returns and closes the provider.
```

这个例子适合在一个任务函数里使用，任务结束再关闭 Provider。默认读取应用数据库；如果已有独立的 pgx pool，再加 `openfga.WithPGXPool(pool)`。

## 5、怎么调用

下面用“给一个用户授予仓库读取权限”为例。代码片段放在返回 `error` 的业务函数中，`ctx` 用请求的 context，`authorizer` 用前面初始化的实例。

### identifiers.go：准备用户和资源

[`identifiers.go`](identifiers.go) 用来把业务里的 UUID、ID 转成 ReBAC 的引用。`Subject` 表示谁获得权限，`Object` 表示权限作用在哪个资源上。这些方法只创建引用，不会创建业务资源，也不会自动授权。

先准备用户和仓库引用，需导入 `opencsg.com/csghub-server/builder/rebac`：

```go
// Use the user's UUID and the repository's database ID.
user := rebac.UserSubject(userUUID)
repository := rebac.RepositoryObject(42)
```

用户用 UUID，仓库用数据库 ID，不要传用户名或仓库路径。

其他资源也用这个文件里的方法创建引用：

- `UserObject(userUUID)`：把用户账号作为要保护的资源，比如检查谁可以修改这个账号。给用户授权时用 `UserSubject`，保护用户账号时用 `UserObject`。
- `OrganizationObject(orgUUID)`：组织资源，参数是组织 UUID。
- `NamespaceObject(namespaceUUID)`：命名空间资源，参数是命名空间 UUID。
- `KnowledgeBaseObject(id)`：知识库资源，参数是数据库 ID。它和 `RepositoryObject(id)` 的 ID 都必须大于 0。
- `OrganizationMembers(orgUUID)`：组织成员集合，表示 `organization:<UUID>#member`。可以把它作为仓库 reader、writer 或 admin 的 Subject，给整个组织的成员授权。
- `OrganizationMembersFromChild(orgUUID)`：包含子组织成员的集合，表示 `organization:<UUID>#member_from_child`。能否写入某个资源的关系，要看模型是否允许这种主体；当前仓库角色允许的是 `organization#member`。
- `AnonymousSubject()`：匿名用户引用，表示 `user:anonymous`，用于检查未登录用户的公开读取权限。
- `PublicUserWildcard()`：所有用户的引用，表示 `user:*`。比如将它写入知识库的 `public` 关系，表示公开可读。

没有专门的构造方法时，也可以用下面几个通用方法：

- `NewSubject(objectType, id)`：创建指定类型的主体。比如组织作为关联关系的主体时，用 `NewSubject(ObjectTypeOrganization, orgUUID)`。
- `NewObject(objectType, id)`：创建指定类型的资源引用。
- `NewUserset(objectType, id, relation)`：创建 `type:id#relation` 这样的成员集合引用。
- `NewWildcardSubject(objectType)`：创建 `type:*` 这样的通配主体引用，`PublicUserWildcard()` 就是它的 user 类型版本。

文件里还有 `SystemObject()`，返回 `platform:platform`。当前默认模型和 schema 没有注册 platform，使用前需要先补上对应定义。

新增资源类型时，可以在这里补一个专门的构造方法，业务代码就不用每次手动拼类型和 ID；同时仍需在模型及 ReBAC schema 中注册这个类型。

### Write：给用户授权

```go
// Grant the user the repository's reader relation.
relationships := []rebac.Relationship{{
	Subject:  user,
	Relation: rebac.RelationReader,
	Object:   repository,
}}
if err := authorizer.Write(ctx, relationships); err != nil {
	return err
}
```

这个关系就是“用户是仓库 42 的 reader”。同样可以写 `RelationWriter` 或 `RelationOwner`。重复写入已有关系会被忽略；一次最多写 100 条，大批量数据自己分批，数组不能为空。

### Check：看看用户有没有权限

```go
// Check effective read permission on the repository.
request := rebac.CheckRequest{
	Subject:     user,
	Relation:    rebac.RepositoryCanRead,
	Object:      repository,
	Consistency: rebac.ConsistencyHigher,
}
decision, err := authorizer.Check(ctx, request)
if err != nil {
	return err
}
if !decision.Allowed {
	return rebac.ErrDenied
}
```

有权限看 `decision.Allowed`。没有权限时 `err` 仍然是 `nil`，不要只判断错误。这里检查 `RepositoryCanRead`，直接授权和组织继承的权限都会算进去。

如果只是要拦住没有权限的操作，可以直接用 `Authorize`：

```go
// Return ErrDenied when the permission check is denied.
if err := authorizer.Authorize(ctx, request); err != nil {
	return err
}
```

### Delete：取消授权

删除刚才写入的那组关系：

```go
// Remove the same reader relation written above.
if err := authorizer.Delete(ctx, relationships); err != nil {
	return err
}
```

删除时用户、关系和对象要与写入时一致。关系不存在会被忽略；批量删除也要分批。取消 reader 后，如果用户还有 owner、writer 或继承权限，仍然可能读到仓库，可以再用 `Check` 确认。

### ListObjects：列出用户能访问的资源

项目里的方法名是 `ListObjects`。比如查询用户能读取的仓库，额外导入 `fmt`：

```go
// List repositories the user can read.
result, err := authorizer.ListObjects(ctx, rebac.ListObjectsRequest{
	Subject:     user,
	Relation:    rebac.RepositoryCanRead,
	ObjectType:  rebac.ObjectTypeRepository,
	Consistency: rebac.ConsistencyHigher,
})
if err != nil {
	return err
}
for _, object := range result.Objects {
	fmt.Println(object.ID)
}
```

返回的 `result.Objects` 是对象引用，里面有 `Type` 和 `ID`。拿到 ID 后，再查业务数据库获取仓库详情。

默认最多返回 1000 个对象，可以在配置里调整：

```toml
[rebac]
openfga_list_object_max_result = 1000
```

修改配置后重启服务。这个接口没有业务分页，结果也受查询时限影响，不要把返回数量当成用户全部可访问资源的总数。

另外两个常用方法：

- `BatchCheck`：一次检查多个权限，默认最多 50 项，每项使用不同的 `CorrelationID`，结果从 `result.Results` 获取，逐项判断 `Err` 和 `Decision.Allowed`。
- `ListSubjects`：查询谁对某个对象有权限，传入 `Object`、`Relation` 和 `SubjectType`，结果从 `result.Subjects` 获取。

更多查询方式见 [OpenFGA 关系查询文档](https://openfga.dev/docs/interacting/relationship-queries)。

## 6、新增模型版本和 Latest

新增模型版本，是把一套新的权限规则存到数据库；`Latest` 是告诉业务代码默认使用哪一套规则。数据库里新增了模型，业务代码不会自动切过去。

项目中的完整名称是 `OpenFgaAuthorizationModelIDLatest`，定义在 `common/types/rebac.go`。现在它指向 1.2：

```go
// Use model version 1.2 for the default provider.
const OpenFgaAuthorizationModelIDLatest = OpenFgaAuthorizationModelIDVer1_2
```

比如要新增 1.3，可以这样做：

1. 复制 `dsl_v1_2.go` 为 `dsl_v1_3.go`，定义 `AuthorizationModelVer1_3`，在里面修改权限规则。
2. 在 `common/types/rebac.go` 新增 `OpenFgaAuthorizationModelIDVer1_3`，使用新生成的 ULID。
3. 新增 migration，把 `OpenFgaAuthorizationModelIDVer1_3` 和 `AuthorizationModelVer1_3` 一起传给 `insertAuthorizationModel`。执行 migration，写入新模型；需要的新关系也一起回填。新增类型或关系时，同步更新 ReBAC 的 schema。
4. 新模型和关系准备好后，把 `Latest` 改成下面这样，重新构建并部署服务：

```go
// Switch the default provider to model version 1.3 after its migration is applied.
const OpenFgaAuthorizationModelIDLatest = OpenFgaAuthorizationModelIDVer1_3
```

这里的 1.3 是新增版本的例子，需要先添加对应的 DSL 和 ID 常量。

**migration 里用固定版本，业务默认入口用 Latest。** 比如创建 1.3 的 migration，就明确写 `OpenFgaAuthorizationModelIDVer1_3`，不要写 `OpenFgaAuthorizationModelIDLatest`。否则以后 Latest 改成 1.4，重新执行这份 migration 时就可能把 1.3 的 DSL 配上 1.4 的 ID。

`factory.NewAuthorizer()` 创建的默认 Provider 会在初始化时记住 Latest 指向的 ID，后续 `Write`、`Delete`、`Check`、`ListObjects` 都使用这个模型。Latest 是代码里的常量，修改后需要重新构建、部署；已经运行的服务不会自动更新。

如果只想在某个任务中使用新模型，可以先通过 `WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_3)` 指定它，不必马上改 Latest。等确认新模型可用后，再切换业务的默认版本。切换前要确保对应 ID 已写入数据库，否则调用会报找不到模型。
