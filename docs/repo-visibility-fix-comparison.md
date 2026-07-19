# 仓库列表可见性修复方案对比：path 前缀匹配 vs namespace_id 列

## 背景

`PublicToUser`（各类仓库列表接口）与 `BatchGetRepoExtra` 原来按 `repository.user_id` 过滤私有仓库可见性：

```sql
repository.private = false OR repository.user_id IN (自己的用户ID, 各组织创建者的用户ID)
```

由于 `repository.user_id` 是**仓库创建者的个人用户 ID**、`org.UserID` 是**组织创建者的个人用户 ID**，二者都不表达"命名空间归属"，导致：

1. **越权泄露**：组织任意成员可看到组织创建者名下所有私有仓库（含其个人命名空间及其他无关组织下的）
2. **漏显示**：组织内其他成员创建的组织私有仓库，当前用户反而看不到

正确语义应与单仓库鉴权的权威实现（`GetUserRepoPermission` / `CheckCurrentUserPermission`）一致：**私有仓库可见 ⇔ 当前用户是仓库所在命名空间的属主，或该组织命名空间的成员**。

围绕"如何在 SQL 中表达命名空间归属"，有两种方案。

## 方案一：path 前缀匹配（已采用）

### 做法

利用 `repository.path` 的既有约定（`<namespace>/<name>`），过滤条件改为：

```sql
repository.private = false
OR repository.path LIKE '<username>/%' ESCAPE '\'
OR repository.path LIKE '<org1>/%'     ESCAPE '\'
OR ...
```

component 层收集 `user.Username` + 各 `org.Name`（组织路径）传给 store 层；`escapeLikePattern` 转义命名空间中的 `_` / `%`。

### 改动范围

- `component/repo.go`：`PublicToUser`、`BatchGetRepoExtra`
- `builder/store/database/repository.go`：`PublicToUser`、`publicToUserTrending`（接口签名 `[]int64` → `[]string`）
- mock 重新生成 + 对应单测

**纯读路径改动，无 schema 变更、无数据回填、无写入路径改动。**

### 优点

- **单一真相来源**：整个代码库中命名空间的权威编码就是 path 前缀（`NamespaceAndName()`、`namespaceStore.FindByPath()`、`filter.Owner` 过滤、git_path 派生均如此），本方案与之完全一致，不引入需要同步维护的冗余数据
- **改动小、风险低**：不涉及迁移、回填、写入路径，安全修复可快速合入
- **数据库兼容**：`LIKE 'prefix/%'` 在 PG 与 sqlite（测试路径）下行为一致；同一查询中 `filter.Owner` 已长期使用相同模式
- 前缀 LIKE 是可索引化（sargable）谓词，后续可加 `text_pattern_ops` 索引优化

### 缺点

- 归属语义耦合于"path 第一段"这一字符串约定；命名空间改名依赖所有 repo path 被正确重写
- 逐行字符串前缀比较，理论开销高于整型等值比较（见下文性能分析）
- 命名空间数量多时（用户加入大量组织）OR 条件变长

### 性能分析

影响可忽略：

- 原条件 `private = false OR user_id IN (...)` 因 OR 上低选择性的 `private = false`，本来就是**逐行 filter 而非索引驱动**（`repositories` 表虽有 `user_id` 索引，但从未被此查询使用）；新条件执行计划形状相同，不存在"索引退化为全表扫描"
- OR 短路求值：绝大多数行（公开仓库）在 `private = false` 即返回，LIKE 仅对私有行求值；前缀不匹配时只比较头几个字符
- 每行最多 N 次前缀比较（N = 1 + 所属组织数，通常个位数），相对查询中的 join、排序、count 完全不在一个量级

## 方案二：repositories 表增加 namespace_id 列

### 做法

`repositories` 增加 `namespace_id` 外键指向 `namespaces` 表（GitLab `projects.namespace_id` 同款设计），过滤条件变为：

```sql
repository.private = false OR repository.namespace_id IN (?, ?, ...)
```

### 改动范围（远大于方案一）

1. **Schema 迁移 + 数据回填**：加可空列 → `UPDATE repositories r SET namespace_id = n.id FROM namespaces n WHERE split_part(r.path,'/',1) = n.path` → 校验 NULL（重点排查 mirror / multisync 来源仓库）→ 加 NOT NULL + FK
2. **所有写入路径补齐**（每漏一处就是一个权限 bug）：
   - `repoStore.CreateRepo` 直接调用点 2 处（`component/repo.go:277`、`:544`）
   - `UpdateOrCreateRepo` 在 `component/multi_sync.go` 中 6 处
   - 8 个业务 store 的 `CreateAndUpdateRepoPath`（skill/code/model/mcp_server/dataset/space/prompt 等）
   - 仓库路径变更/转移流程需同步更新 namespace_id
3. **读路径多一跳**：用户服务 `rpc.User.Orgs` 只有组织 path 没有 namespace id，需额外查询 `namespaces` 表或扩展 user 服务返回结构
4. 一致性保障：需要对账机制（或触发器）防止 path 前缀与 namespace_id 漂移

### 优点

- **建模正确**：归属关系显式化为外键，不依赖字符串约定
- 整型等值匹配，可索引，无 LIKE 转义问题
- 命名空间改名/仓库转移时归属判断更健壮

### 缺点

- **引入第二真相来源**：path 前缀与 namespace_id 必须永远同步，任何写入路径遗漏都会产生新的权限 bug——而本次要修的恰恰是权限 bug
- 改动面大：迁移 + 回填 + 十余处写入路径 + 读路径扩展 + 全链路测试
- 对本查询的性能收益有限：瓶颈在 `private = false OR ...` 的低选择性 OR 结构，换成整型 IN 依然是逐行 filter，除非配合 partial index / UNION 重写
- 回填期间的历史脏数据风险（path 与 namespaces 表不一致的存量数据）

## 对比总结

| 维度 | 方案一：path LIKE 前缀 | 方案二：namespace_id 列 |
|---|---|---|
| 修复正确性 | ✅ 与权威权限模型一致 | ✅ 与权威权限模型一致 |
| 改动范围 | 2 个文件的读路径 + 测试 | 迁移/回填 + 10+ 写入路径 + 读路径 |
| 真相来源 | 单一（path） | 双份（path + namespace_id），需持续保证同步 |
| 新增权限 bug 风险 | 低 | 中（任一写入路径遗漏即越权/漏显示） |
| 查询性能 | 与原实现相当（逐行 filter，短路求值） | 略优，但计划形状不变，收益有限 |
| 数据库兼容性 | PG / sqlite 通用 | 迁移为 PG 编写；测试路径需适配 |
| 命名空间改名健壮性 | 依赖 path 重写 | 更健壮 |
| 上线周期 | 立即 | 需分阶段（回填→双写→对账→切读→加约束） |

## 结论与建议

- **本次安全修复采用方案一**：与现有权限模型和代码约定完全一致，改动小、可快速合入堵住越权漏洞
- **方案二作为独立重构项另行评估**：若未来确有命名空间改名健壮性或查询性能诉求，再按"加列→回填→双写→对账→切读"分阶段实施
- 折中优化（如后续列表查询确有性能压力）：PG 生成列 `namespace text GENERATED ALWAYS AS (split_part(path,'/',1)) STORED` + 索引——从 path 自动派生、不可能不同步，可获得方案二的大部分查询收益而无双写风险（注意 sqlite 测试路径需兜底）

## 附：本次修复验证结果

- `go build ./...`（含 `-tags ee` / `-tags saas`）通过
- `go test ./component/ ./builder/store/database/` 全部通过（含真实测试库上的私有可见性、trending、search、owner filter 用例）
