# 开票模块接口文档（充值开票）

> 适用版本：`feat/recharge-invoice` 分支。开票模式从「按账单周期」改为「按充值订单」：
> 可开票金额 = 已支付、未关闭、未开过票的充值订单实付金额合计（不含代金券、活动赠送）。
> 接口路由不变，仅出入参有调整，见各节说明。

## 通用约定

| 项 | 说明 |
|---|---|
| Base Path | `/api/v1/accounting/invoice` |
| 鉴权 | `Authorization: Bearer <token>`（ApiKey），所有接口必需；本地用系统 ApiKey 调试时还需追加查询参数 `current_user=<username>`（或 `current_user_uuid=<uuid>`）标识当前登录用户，门户前端走登录态无需传 |
| `:uuid` | 用户或组织的 UUID（路径参数） |
| `:id` | 发票 ID，即数据库自增 id（路径参数） |
| 金额单位 | 元（`amount`、`invoice_amount` 等） |
| 时间格式 | RFC3339，如 `2026-09-01T08:30:00Z` |
| 月份格式 | `YYYY-MM` |
| 成功响应 | `{"msg": "OK", "data": ...}`，HTTP 200 |
| 参数错误 | `{"msg": "..."}`，HTTP 400 |
| 业务/系统错误 | `{"msg": "..."}`（或含 `code`/`context`），HTTP 500 |

curl 环境变量：

```bash
BASE=https://<portal-host>          # accounting 服务入口
TOKEN=<api-key>                     # 用户的 ApiKey
UUID=<用户或组织uuid>                # 目标开票主体
AUTH="Authorization: Bearer $TOKEN"
CT="Content-Type: application/json"
```

---

## 1. 开票仪表盘

`POST /api/v1/accounting/invoice/:uuid/dashboard`

开票管理页顶部的金额汇总。统计范围为请求的月份区间（闭区间，含首尾月）。

**请求体**（`start_month`、`end_month` 必填；`end_month` 不能晚于当前月，且不能早于 `start_month`）：

```json
{ "start_month": "2026-01", "end_month": "2026-09" }
```

**响应**：

```json
{
  "msg": "OK",
  "data": {
    "current_month_non_invoicable": 0,
    "invoiced_amount": 1200.0,
    "uninvoiced_amount": 355.0
  }
}
```

| 字段 | 说明 |
|---|---|
| `uninvoiced_amount` | 可开票金额：区间内已支付、未关闭、未开票的**充值订单**实付合计。不含代金券、活动赠送（两者不产生充值订单，天然被排除） |
| `invoiced_amount` | 区间内申请开票（`apply_time`）且非失败状态的发票金额合计 |
| `current_month_non_invoicable` | 兼容保留字段，恒为 `0`（充值成功当月即可开票） |

> 统计口径沿用历史账单周期的月度区间：`end_month` 当月的充值订单不计入 `uninvoiced_amount`/`invoiced_amount`（下月才会进入统计）。当月可开票金额以「可开票订单列表」为准。

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/$UUID/dashboard" \
  -H "$AUTH" -H "$CT" \
  -d '{"start_month":"2026-01","end_month":"2026-09"}'
```

---

## 2. 可开票订单列表

`POST /api/v1/accounting/invoice/:uuid/invoicable`

列出可申请开票的充值订单，前端渲染为勾选列表。

**请求体**：

| 字段 | 必填 | 说明 |
|---|---|---|
| `page` | 是 | ≥ 1 |
| `page_size` | 是 | 1–100 |
| `start_month` / `end_month` | 否 | 按 `time_succeeded` 过滤；**两者都传才生效**，校验规则同仪表盘 |

```json
{ "page": 1, "page_size": 10, "start_month": "2026-08", "end_month": "2026-09" }
```

**响应**：

```json
{
  "msg": "OK",
  "data": {
    "data": [
      {
        "order_no": "202608151234567890",
        "recharge_time": "2026-08-15T12:00:00Z",
        "amount": 100.0
      },
      {
        "order_no": "202609010830001234",
        "recharge_time": "2026-09-01T08:30:00Z",
        "amount": 255.0
      }
    ],
    "total": 2
  }
}
```

> 注意 `data.data` 是订单数组、`data.total` 是总数（外层 `data` 是统一响应信封）。

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/$UUID/invoicable" \
  -H "$AUTH" -H "$CT" \
  -d '{"page":1,"page_size":10}'
```

---

## 3. 申请开票

`POST /api/v1/accounting/invoice/:uuid/create`

对勾选的一批充值订单生成**一张**发票。开票金额由服务端按订单实付合计计算，请求中不传金额。

**请求体**：

| 字段 | 必填 | 说明 |
|---|---|---|
| `title_id` | 是 | 发票抬头 ID（来自抬头列表接口） |
| `recharge_order_nos` | 是 | 充值订单号数组，≥ 1 项；支持多个订单合并开一张发票 |

```json
{
  "title_id": 1,
  "recharge_order_nos": ["202608151234567890", "202609010830001234"]
}
```

**响应**（创建成功，`data` 为空）：

```json
{ "msg": "OK" }
```

**错误**（HTTP 400，`msg` 内容）：

| 场景 | msg |
|---|---|
| 参数缺失 / 格式错误 | 绑定器错误信息 |
| 操作他人账户 | `permission denied`（HTTP 500） |
| 抬头不存在 | `invoice title not found`（HTTP 500） |
| 订单不存在 / 未支付成功 / 已开过票 / 不是该用户的订单 / 重复或并发提交 | `some recharge orders are not invoicable` |

> 服务端在创建事务内对订单加锁复核可开票状态：已被其它未失败发票占用的订单会导致整批创建失败并返回 400，不会出现一单多票。

**失败重试**：发票被管理员置为 `failed`（或删除）后，其包含的订单自动回到可开票列表，用户可重新勾选申请。

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/$UUID/create" \
  -H "$AUTH" -H "$CT" \
  -d '{"title_id":1,"recharge_order_nos":["202608151234567890","202609010830001234"]}'
```

---

## 4. 开票记录列表

`POST /api/v1/accounting/invoice/:uuid/list`

**请求体**：

| 字段 | 必填 | 说明 |
|---|---|---|
| `page` / `page_size` | 是 | 分页，`page_size` 1–100 |
| `search` | 否 | 按用户名等模糊搜索 |
| `status` | 否 | `processing` / `issued` / `failed` |

```json
{ "page": 1, "page_size": 10, "status": "processing" }
```

**响应**：

```json
{
  "msg": "OK",
  "data": {
    "data": [
      {
        "id": 12,
        "user_uuid": "d3b07384-d9a0-4b6f-9a1e-2c5f6a7b8c9d",
        "user_name": "zhangsan",
        "title_type": "enterprise_ordinary",
        "invoice_type": "vat",
        "bill_cycle": "",
        "invoice_title": "杭州开源科技有限公司",
        "apply_time": "2026-09-11T10:30:00+08:00",
        "invoice_amount": 355.0,
        "status": "processing",
        "reason": "",
        "invoice_date": "0001-01-01T00:00:00Z",
        "invoice_url": "",
        "taxpayer_id": "91330100MA27XXXXXX",
        "bank_name": "招商银行杭州分行",
        "bank_account": "5719000000000001",
        "registered_addr": "杭州市西湖区文一西路 96 号",
        "contact_phone": "13800000000",
        "email": "zhangsan@example.com",
        "created_at": "2026-09-11T10:30:00+08:00",
        "updated_at": "2026-09-11T10:30:00+08:00",
        "recharge_orders": [
          {
            "order_no": "202608151234567890",
            "recharge_time": "2026-08-15T12:00:00Z",
            "payment_type": "alipay",
            "amount": 100.0
          },
          {
            "order_no": "202609010830001234",
            "recharge_time": "2026-09-01T08:30:00Z",
            "payment_type": "wx_pub_qr",
            "amount": 255.0
          }
        ]
      },
      {
        "id": 7,
        "user_name": "zhangsan",
        "bill_cycle": "2024-01",
        "status": "issued",
        "invoice_amount": 1200.0,
        "recharge_orders": []
      }
    ],
    "total": 2
  }
}
```

> 上例第二条为历史账单周期发票的省略展示，其余字段结构相同。

**新旧发票区分**：

- 新的充值型发票：`bill_cycle` 为空字符串 `""`，`recharge_orders` 为其包含的订单明细
- 历史账单周期发票：`bill_cycle` 非空（如 `"2024-01"`），`recharge_orders` 为空数组

列表按发票创建时间倒序（id 倒序），最新申请的发票排在最前。

**`recharge_orders` 元素字段**：`order_no` 订单号、`recharge_time` 充值成功时间、`payment_type` 支付渠道（`alipay`、`wx_pub_qr` 等原始值）、`amount` 实付金额（元）。`invoice_amount` 等于各订单 `amount` 之和。

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/$UUID/list" \
  -H "$AUTH" -H "$CT" \
  -d '{"page":1,"page_size":10,"status":"processing"}'
```

---

## 5. 开票详情

`GET /api/v1/accounting/invoice/:id`

响应 `data` 为单条 `AccInvoiceResp`（结构同开票记录列表的元素）。仅发票所有者或有权限的管理员可查看，否则返回 500 `forbidden`。

```bash
curl -s "$BASE/api/v1/accounting/invoice/12" -H "$AUTH"
```

---

## 6. 管理员接口

前缀同上，需要管理员权限的 ApiKey。

### 6.1 记录列表

`POST /api/v1/accounting/invoice/admin/list` — 请求/响应同「开票记录列表」，返回全部用户的发票。

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/admin/list" \
  -H "$AUTH" -H "$CT" \
  -d '{"page":1,"page_size":10}'
```

### 6.2 详情

`GET /api/v1/accounting/invoice/admin/:id` — 响应同「开票详情」。

```bash
curl -s "$BASE/api/v1/accounting/invoice/admin/12" -H "$AUTH"
```

### 6.3 更新开票状态

`PUT /api/v1/accounting/invoice/admin/:id`

| 字段 | 必填 | 说明 |
|---|---|---|
| `status` | 是 | 仅允许 `issued`（已开票）或 `failed`（开票失败） |
| `invoice_url` | `status=issued` 时必填 | 发票文件链接 |
| `reason` | `status=failed` 时必填 | 失败原因 |

> `failed` 是终态：已失败的发票不可再改回 `issued` 或其他状态（返回 400），如需开票请重新申请（其订单此时已在可开票列表中）。这保证同一订单始终只有一张有效发票。

**将发票标记为已开票：**

```bash
curl -s -X PUT "$BASE/api/v1/accounting/invoice/admin/12" \
  -H "$AUTH" -H "$CT" \
  -d '{"status":"issued","invoice_url":"https://invoice.example.com/xxx.pdf"}'
```

**将发票标记为失败**（其包含的充值订单会自动回到可开票列表）：

```bash
curl -s -X PUT "$BASE/api/v1/accounting/invoice/admin/12" \
  -H "$AUTH" -H "$CT" \
  -d '{"status":"failed","reason":"抬头税号错误"}'
```

**响应**：`data` 为更新后的发票对象（`AccInvoiceResp`）。

### 6.4 删除发票

`DELETE /api/v1/accounting/invoice/admin/:id` — 删除后其包含的充值订单重新可开票。

```bash
curl -s -X DELETE "$BASE/api/v1/accounting/invoice/admin/12" -H "$AUTH"
```

---

## 7. 发票抬头（本次未改动，供联调参考）

| 接口 | 方法与路径 |
|---|---|
| 抬头列表 | `POST /api/v1/accounting/invoice/:uuid/title/list` |
| 创建抬头 | `POST /api/v1/accounting/invoice/:uuid/title` |
| 更新抬头 | `PUT /api/v1/accounting/invoice/title/:id` |
| 删除抬头 | `DELETE /api/v1/accounting/invoice/title/:id` |
| 抬头类型枚举 | `GET /api/v1/accounting/invoice/title/types` → `["enterprise_ordinary"]` |
| 发票类型枚举 | `GET /api/v1/accounting/invoice/types` → `["ordinary", "vat"]` |

创建抬头示例（`title_id` 由此获得）：

```bash
curl -s -X POST "$BASE/api/v1/accounting/invoice/$UUID/title" \
  -H "$AUTH" -H "$CT" \
  -d '{
    "title": "杭州开源科技有限公司",
    "title_type": "enterprise_ordinary",
    "invoice_type": "vat",
    "tax_id": "91330100MA27XXXXXX",
    "address": "杭州市西湖区文一西路 96 号",
    "bank_name": "招商银行杭州分行",
    "bank_account": "5719000000000001",
    "contact_phone": "13800000000",
    "email": "zhangsan@example.com",
    "is_default": true
  }'
```

---

## 附录 A：手测造数

跳过真实支付，直接插入一条已支付充值订单即可进入可开票列表：

```sql
-- closed / succeeded 列可为 NULL（历史迁移未加约束），必须显式给 false，
-- 否则该订单会因 closed = false 过滤条件被排除在可开票列表之外
INSERT INTO account_recharge (recharge_uuid, order_no, user_uuid, from_user_uuid,
  amount, currency, channel, payment_uuid, succeeded, closed, time_succeeded)
VALUES ('r-test-1', 'ord-test-1', '<目标用户uuid>', '<同上>', 10000,
  'CNY', 'alipay', 'pay-test-1', true, false, now());
-- amount 单位为分，10000 = 100 元
```

验证闭环：可开票列表出现该订单 → 申请开票 → 订单从可开票列表消失 → 记录列表/详情的 `recharge_orders` 含该订单 → 重复提交 create 报错 → 管理员置为 `failed` 后订单回到可开票列表。

## 附录 B：本次改动一览

| 接口 | 变化 |
|---|---|
| 仪表盘 | 响应结构不变；`current_month_non_invoicable` 恒为 0，`uninvoiced_amount` 改为可开票充值订单合计 |
| 可开票列表 | 行结构改为 `{order_no, recharge_time, amount}` |
| 申请开票 | 请求改为 `{title_id, recharge_order_nos[]}`，金额服务端计算；订单不可开票（含重复/并发申请）返回 400；事务内加锁防一单多票 |
| 记录列表 / 详情 / 管理员详情与改状态 | 响应新增 `recharge_orders` |
| `failed` / 删除后的订单 | 自动回到可开票列表，可重新申请开一张新发票 |
| 抬头相关、types、admin/list、admin DELETE | 无变化 |
