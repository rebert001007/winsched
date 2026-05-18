# WinSched

Windows 后台定时任务服务，开机自动运行，支持 cron 定时执行任意命令/脚本，提供 HTTP API 动态管理任务。

## 快速开始

```powershell
# 构建
go build -ldflags="-s -w" -trimpath -o winsched.exe .

# 安装为 Windows 服务（需管理员权限）
.\winsched.exe install

# 启动
sc start winsched

# 前台调试运行
.\winsched.exe run .\config.yaml
```

## 配置文件

服务默认读取 `C:\ProgramData\winsched\config.yaml`。

```yaml
logging:
  file: "C:\\ProgramData\\winsched\\service.log"
  level: "info"              # debug | info | warn | error

api:
  port: 15732                # HTTP API 监听端口
  enabled: true

proxy:
  host: "127.0.0.1"          # 代理地址
  port: 10808                # 代理端口

tasks:
  - name: "daily-report"     # 任务名（唯一）
    description: "日报生成"
    cron: "0 9 * * *"        # 每天 9:00
    command: "powershell.exe"
    args:
      - "-File"
      - "C:\\scripts\\report.ps1"
    timeout: "5m"            # 超时时间
    enabled: true            # 是否启用
    use_proxy: false         # 是否在执行前验证代理可用
```

## Cron 表达式

支持标准 5 字段格式：`分 时 日 月 星期`

| 示例 | 说明 |
|------|------|
| `0 9 * * *` | 每天 9:00 |
| `*/30 * * * *` | 每 30 分钟 |
| `0 0 * * 0` | 每周日 0:00 |
| `@every 1h30m` | 每 1 小时 30 分钟 |
| `@daily` | 每天 0:00 |

## HTTP API

所有接口监听 `127.0.0.1`，仅本机可访问。

### 健康检查
```
GET /api/health
→ {"ok":true,"data":{"status":"ok"}}
```

### 列出任务
```
GET /api/tasks
→ {"ok":true,"data":[{...task list...}]}
```

### 添加任务
```
POST /api/tasks
Content-Type: application/json

{
  "name": "my-task",
  "description": "任务描述",
  "cron": "@every 10m",
  "command": "cmd.exe",
  "args": ["/c", "echo hello"],
  "timeout": "30s",
  "enabled": true,
  "use_proxy": false
}
→ {"ok":true,"data":{...created task...}}
```

### 修改任务
```
PUT /api/tasks/{name}
Content-Type: application/json

{"cron": "0 */2 * * *", "enabled": false}
→ {"ok":true,"data":{"updated":"my-task"}}
```

### 删除任务
```
DELETE /api/tasks/{name}
→ {"ok":true,"data":{"removed":"my-task"}}
```

### 执行记录
```
GET /api/executions?n=20
→ {"ok":true,"data":[{...execution records...}]}

GET /api/tasks/{name}/executions?n=10
→ {"ok":true,"data":[{...task execution history...}]}
```

## 集成方式

本服务通过 HTTP API 供任意语言调用（不限于 Go）。Claude Code 可直接通过 `/winsched` skill 管理任务。

**任意语言调用示例**（等价于 curl）：

```powershell
# PowerShell - 添加任务
Invoke-RestMethod -Uri http://127.0.0.1:15732/api/tasks -Method Post -Body (@{name="job";cron="@every 10m";command="cmd.exe";args=@("/c","echo hi");timeout="30s";enabled=$true} | ConvertTo-Json) -ContentType "application/json"

# Python - 查看任务
import requests
r = requests.get("http://127.0.0.1:15732/api/tasks")
print(r.json())

# 任意语言只需发送 HTTP JSON 请求到上述端点即可
```

## Proxy 机制

当任务设置 `use_proxy: true` 时，每次执行前会先 TCP 连接检测代理是否可达。如果不可达，每 2 秒重试一次，最长等待 30 秒。超时则跳过本次执行并记录错误日志。

## 命令行

```
winsched.exe install           # 注册 Windows 服务（开机自启）
winsched.exe uninstall         # 移除服务
winsched.exe run [config.yaml] # 前台运行（调试用）
```

## 日志

- 文件日志：`C:\ProgramData\winsched\service.log`（可配置）
- Windows Event Log：来源名称 `winsched`
- 前台调试模式额外输出到控制台

### 执行记录

```
GET /api/executions?n=50              → 最近 N 条执行记录（默认 20）
GET /api/tasks/{name}/executions?n=10  → 某任务的执行历史
```

每条记录包含：`task_name`、`start_time`、`end_time`、`status`（running/success/failed/timeout）、`error`、`output`。
## 部署

1. 编译 `go build -ldflags="-s -w" -trimpath -o winsched.exe .`
2. 创建目录 `C:\ProgramData\winsched\`
3. 复制 `winsched.exe` 和 `config.yaml` 到该目录
4. 管理员运行 `winsched.exe install`
5. `sc start winsched` 启动
