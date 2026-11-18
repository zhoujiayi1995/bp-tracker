# 家庭血压记录（bp-tracker）

主要在手机浏览器上使用的家庭血压记录工具：记录自己和家人的高压、低压、脉搏，填写病史和常用药，并查看趋势折线图。界面按年长者的使用习惯设计：大字号、大按钮，录入流程尽量简单。

## 功能概述

面向手机浏览器的家庭血压本。四个页签：记录、趋势、家人、我的。界面用大字号和大按钮，文案按年长者阅读习惯写。

- **记录血压**：每次可连测 1–3 次，服务端取平均后保存，原始测量值一并保留。脉搏和备注选填。保存前按中国高血压分级给出一句话提示（偏低、正常、稍高、偏高；高压 ≥180 或低压 ≥110 时提示尽快就医）。偏离常见范围的数值会先弹窗确认。记录页同时显示今天有没有量过、测量要点，以及最近记录；有权限的人可以修改或删除。
- **趋势**：近 7、30、90 天。近 7 天默认「按时段」，更长范围默认「每天平均」。三种显示方式：
  - 「按时段」：一天分成上午（12:00 前）、下午（12:00–18:00）、晚上（18:00 起），各段取平均；
  - 「每天平均」：一天一个点；
  - 「每次测量」：每条记录一个点。

  图上标出高压 140、低压 90 的参考线，可选择是否显示脉搏。页顶给出这段时间的平均血压，以及和前一段相比是升高、降低还是持平。病史和常用药在同一页展示，方便就诊时出示。
- **家人与权限**：
  - 本人的档案可读可写；
  - 用 6 位邀请码加入家庭后，成员之间可以互相查看，但只读；邀请码可重置，旧码立即失效；
  - 「代管家人」：例如子女为不使用手机的父母建档并代为记录。创建者可读写，其他家庭成员只读。删除代管档案会级联删除其全部记录。
- **账号**：注册、登录、修改自己的称呼和病史用药、修改密码（其他设备上的会话会失效）、退出登录。
- **PWA**：可在手机上「添加到主屏幕」，像 App 一样打开。

## 技术栈与结构

- 后端：Go（标准库 `net/http`）和 SQLite（`modernc.org/sqlite`，纯 Go，无需 CGO）。
- 前端：Vue 3、Vant 4、ECharts、Pinia、Vite。
- 构建产物是**单个二进制文件**，前端通过 `go:embed` 打包在里面，部署时只需这个文件和一个数据库文件。

```text
cmd/server          entrypoint
internal/config     flags / env
internal/db         SQLite open + embedded migrations
internal/auth       users, sessions, login throttling
internal/authz      single source of truth for read/write permissions
internal/family     family groups and invite codes
internal/profile    self / managed profiles
internal/reading    readings, averaging, am/pm trend buckets
internal/httpapi    HTTP handlers, middleware, SPA serving
web/                Vue 3 + Vant frontend (built into web/dist and embedded)
```

## 环境要求

- Go **1.26 及以上**（`go.mod` 的要求）。如果系统自带的 Go 版本较旧，可以单独安装一个新版本，用 `GO=` 指定给 make：

  ```bash
  go install golang.org/dl/go1.27.4@latest && go1.27.4 download
  make build GO=go1.27.4
  ```

- Node.js 20 及以上，仅构建前端时需要。

## 本地开发

开两个终端：

```bash
# terminal 1: API server on :8080 (database at ./data/bp.db, created automatically)
make dev-server

# terminal 2: Vite dev server with /api proxied to :8080
make dev-web
```

手机调试：让手机和电脑连接同一局域网，然后在手机上访问 Vite 输出的局域网地址。

## 构建与运行

```bash
make build                 # builds the frontend, then bin/bp-tracker
./bin/bp-tracker -db /var/lib/bp-tracker/bp.db -addr :8080
```

### 配置

命令行参数优先；未指定参数时读取对应的环境变量。

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-addr` | `BP_ADDR` | `:8080` | 监听地址 |
| `-db` | `BP_DB` | `./data/bp.db` | SQLite 文件路径，所在目录不存在时自动创建 |
| `-tz` | `BP_TZ` | `Asia/Shanghai` | 划分上午/下午/晚上、按天统计时使用的时区 |
| `-cookie-secure` | `BP_COOKIE_SECURE` | `false` | 在 HTTPS 后面部署时**必须开启** |

### Docker

镜像使用多阶段构建，最终基于 distroless，以非 root 用户（uid 65532）运行，大小约 15MB。

```bash
docker build -t bp-tracker .
docker run -d --name bp-tracker --restart unless-stopped \
  -p 8080:8080 \
  -v bp-data:/data \
  -e BP_COOKIE_SECURE=true \
  bp-tracker
```

如果要把数据挂载到宿主机目录（便于在宿主机上备份），需要先把目录的属主改为容器内的用户：

```bash
sudo mkdir -p /srv/bp-tracker && sudo chown 65532:65532 /srv/bp-tracker
docker run -d --name bp-tracker -p 8080:8080 -v /srv/bp-tracker:/data -e BP_COOKIE_SECURE=true bp-tracker
```

### HTTPS

账号密码和会话 Cookie 不能经明文 HTTP 在公网上传输，请在前面加一层反向代理来提供 HTTPS，并开启 `-cookie-secure`。以 Caddy 为例，它会自动申请证书：

```text
bp.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

说明：登录限流按客户端 IP 计算，且有意不信任 `X-Forwarded-For`，以免被伪造的请求头绕过。因此在反向代理后面，所有请求看到的都是代理的 IP；限流的 key 同时包含用户名，所以这只会让同一用户名的失败次数在所有来源之间合并计算。

## 备份与恢复

所有数据都在一个 SQLite 文件里（运行时旁边还有 `-wal` 和 `-shm` 文件）。**不要在服务运行时直接复制 `.db` 文件**，否则可能得到不一致的副本。应使用 `VACUUM INTO`，它可以在线生成一致的快照：

```bash
sqlite3 /srv/bp-tracker/bp.db "VACUUM INTO '/backup/bp-$(date +%Y%m%d).db'"
```

每天凌晨备份一次，并保留 30 天（crontab 示例；注意 crontab 里的 `%` 需要转义）：

```text
30 3 * * * sqlite3 /srv/bp-tracker/bp.db "VACUUM INTO '/backup/bp-$(date +\%Y\%m\%d).db'" && find /backup -name 'bp-*.db' -mtime +30 -delete
```

恢复步骤：停止服务，用备份文件替换 `bp.db`，删除旧的 `bp.db-wal` 和 `bp.db-shm`，然后启动服务。

## 测试

```bash
make test        # = go test ./... && (cd web && npm run test)
```

- Go 单元测试覆盖：权限矩阵、平均值计算与输入校验、上午/下午分桶、日期范围、会话续期、登录限流、家庭的加入与退出、档案的级联删除。
- `internal/httpapi/api_test.go` 是基于 `httptest` 的端到端测试，覆盖：注册、建立和加入家庭、代管成员、录入记录、各种越权写入返回 403、退出家庭后失去访问权限、CSRF 防护、登录限流、改密码后其他设备下线。
- 前端用 Vitest 测试血压分级、平均值、异常值判断和自动跳格逻辑。

## 安全设计要点

- 密码用 bcrypt 加密存储；登录时用户名不存在也会做一次等价的哈希比较，防止通过响应时间探测用户名是否存在。
- 会话保存在服务端，数据库里只存 token 的 SHA-256。Cookie 设置为 `HttpOnly` 和 `SameSite=Lax`，有效期 90 天，使用过程中自动续期。
- 所有非 GET 的 `/api` 请求都必须带 `X-Requested-With` 头，作为 CSRF 防护。
- 读写权限统一由 `internal/authz` 判断，handler 中不散落权限逻辑。
