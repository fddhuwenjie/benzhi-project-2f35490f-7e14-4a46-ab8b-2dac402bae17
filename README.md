# shelter-drill-gate

`shelter-drill-gate` 是面向社区应急管理人员的避难场所启用演练核验系统。系统围绕单次演练保存可修订草稿、布设预览与冻结基线、现场检查、规则偏差、版本化整改证据、定向复验、结构化独立复核和不可变启用决定书，并通过连续 SHA-256 审计摘要提供可追溯性。

浏览器工作台由 Go 服务直接提供，不需要 Node.js。业务数据保存在 SQLite；每个写请求使用 `request_id` 保证重试幂等，并使用 `expected_version` 阻止陈旧页面覆盖新数据。

## 构建

```bash
go build ./cmd/sheltergate
```

## 运行

默认监听高位回环地址 `127.0.0.1:19081`，数据库文件为当前目录的 `sheltergate.db`：

```bash
go run ./cmd/sheltergate
```

可以显式指定监听地址和数据库路径：

```bash
go run ./cmd/sheltergate -addr=127.0.0.1:19123 -db=./data/sheltergate.db
```

未传入 `-addr` 时，也可通过 `PORT` 指定端口。服务只会绑定到 `127.0.0.1:<PORT>`。浏览器打开对应地址后即可完成建档到启用决定的完整流程。

## 测试

运行全部单元测试和集成测试：

```bash
go test ./...
```

运行有界自检。该命令会创建临时数据库、启动真实 HTTP 服务，完成草稿修订、基线预览与冻结、合格演练和独立审批，校验审计链与决定书摘要及重复导出稳定性，然后关闭监听并自行退出：

```bash
go run ./cmd/sheltergate -selfcheck -addr=127.0.0.1:19081
```

## 主要接口

工作台入口为 `GET /`。同源 `/api/drills` 接口提供演练建档和列表读取，`PATCH /api/drills/{id}` 修订草稿资料。演练详情下提供基线预览与冻结、检查记录、偏差整改版本、定向复验、复核事项回应、再次送审、时间线读取，以及决定书在线核验和稳定导出。详情读模型同时返回实时检查进度、规则统计和完整整改/复核历史。

所有状态变更请求必须使用 `Content-Type: application/json`，携带唯一的 `request_id` 和当前 `expected_version`。基线预览是只读请求，冻结时可携带预览返回的 `preview_digest` 防止确认过期内容。决定书核验与导出同样只读，不增加版本或审计事件。
