# EE 组织架构导入

`trigger import-organization-units` 仅存在于 EE 构建中。运行前需要完成数据库迁移，并设置 `STARHUB_SERVER_ORGANIZATION_ENABLE_UNIT=true`。

```bash
./csghub-server trigger import-organization-units \
  --config local.toml \
  --input cmd/csghub-server/cmd/trigger/importorganization/example.yaml \
  --access_token "$ADMIN_ACCESS_TOKEN" \
  --check
```

`--check` 校验 YAML、CSGHub Access Token、平台管理员身份及数据库状态，不创建数据、不初始化 SSO/OpenFGA。删除 `--check` 后执行导入。使用系统现有的 CSGHub Access Token（数据库 app 为 git）；网关 API key、Mirror Token 和登录 JWT 不适用。

输入格式参考 [示例 YAML](example.yaml)：

- `organization` 定义唯一根组织；`units` 是其直属子组织列表，可为空数组或省略。
- 节点必填 `name`（展示名）和 `path`（namespace 标识），可选 `description`、`children`；根节点的子组织必须写在 `units` 中。
- `path` 使用现有 namespace 命名校验：2–64 个字符、字母开头、字母或数字结尾，仅支持字母、数字、`-_.`，且不允许连续重复特殊字符。
- name/path 在整棵树内分别唯一，大小写不同也视为冲突；同时检查现存组织和用户 namespace。name 不允许空值、首尾空白或控制字符。
- 不支持 `members`、未知字段、重复 YAML key、别名、多文档或隐式数字/布尔/空值。树最多 32 层（包含根），文件最多 4 MiB。

根组织处理：

| 数据库状态 | 行为 |
| --- | --- |
| 无有效多级根组织 | 创建根组织和子树；令牌用户成为根组织 admin |
| 根组织的 name 或 path 与 YAML 不同 | 报错 |
| 根组织 name/path 都相同，且没有子组织 | 复用根组织，创建子树；保留现有成员关系 |
| 根组织匹配但已有子组织 | 报错，不合并、不覆盖、不删除 |

匹配 name/path 使用精确值；同名不同 path 也不会创建第二个根组织。复用根组织时保留原描述；旧根组织若缺少结构节点，会在创建事务内补齐根 unit 及其自身闭包关系。已软删除的组织不作为有效根或子组织，但外部 SSO 的残留标识仍可能导致导入失败。

所有组织、namespace、unit、闭包关系及新根 admin 成员都在同一 SQL 事务中写入，任一步失败即回滚。执行时使用与现有根创建相同的 PostgreSQL 锁，并锁定已有根，防止并发重复初始化。

实际导入沿用现有组织创建行为，同步组织 SSO 身份和 OpenFGA 的 namespace、父子关系及根 admin 权限。这不是成员导入或普通用户注册。SSO/OpenFGA 无法加入 SQL 事务：同步失败或数据库提交失败时，会尝试撤销本次新建的外部身份和权限，清理失败会输出对应 UUID。进程被强制终止或外部服务不可用时，外部补偿不能保证完成。`--check` 不验证这些外部写入服务的健康状态。

命令不会记录 Access Token，但 `--access_token` 参数对能够查看进程命令行的本机用户可见。示例通过已有环境变量展开令牌，避免将令牌字面量写进命令历史。
