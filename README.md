# GamePerf 🎮

游戏性能数据采集 + 可视化对比面板。

采集目标进程的 CPU、内存、线程数等指标，支持多次运行叠加对比分析。

## 功能

- 📊 **实时采集** — 指定进程 PID，定时采集 CPU/内存/线程数
- 📈 **可视化面板** — ECharts 实时曲线图，暗色主题
- 📋 **历史记录** — 所有采集会话管理，统计摘要（Avg/Max/P95）
- ⚖️ **对比分析** — 多次运行数据叠加对比，一眼看出优化效果
- 💉 **手动注入** — FPS 等指标可通过 API 手动注入

## 技术栈

- **后端**: Go + Gin + SQLite + gopsutil
- **前端**: Vue 3 + ECharts + Element Plus

## 快速开始

### 1. 后端

```bash
cd gameperf
go mod tidy
go build -o gameperf .
./gameperf --port 9090 --data ./data
```

### 2. 前端

```bash
cd web
npm install
npm run dev    # 开发模式，自动代理到后端
npm run build  # 构建到 web/dist/
```

### 3. 一键启动（build 后）

```bash
cd gameperf
./gameperf
# 访问 http://localhost:9090
```

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/sessions | 创建采集会话 |
| GET | /api/sessions | 列出所有会话 |
| GET | /api/sessions/:id | 获取会话详情 |
| POST | /api/sessions/:id/start | 开始采集（传 pid + interval） |
| POST | /api/sessions/:id/stop | 停止采集 |
| DELETE | /api/sessions/:id | 删除会话 |
| GET | /api/sessions/:id/samples | 获取采样数据 |
| GET | /api/sessions/:id/summary | 统计摘要 |
| POST | /api/sessions/:id/samples | 手动注入采样数据 |
| GET | /api/compare?ids=a,b | 对比多个会话 |

### 创建并开始采集示例

```bash
# 1. 创建会话
curl -X POST http://localhost:9090/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"name":"v1.0优化前","process":"game.exe"}'

# 2. 开始采集
curl -X POST http://localhost:9090/api/sessions/{id}/start \
  -H "Content-Type: application/json" \
  -d '{"pid":12345,"interval":"1s"}'

# 3. 注入 FPS 数据（可选）
curl -X POST http://localhost:9090/api/sessions/{id}/samples \
  -H "Content-Type: application/json" \
  -d '{"fps":60,"timestamp":1710000000}'

# 4. 停止采集
curl -X POST http://localhost:9090/api/sessions/{id}/stop
```

## 使用场景

1. 游戏版本优化前后对比
2. 长时间运行内存泄漏监控
3. 压测场景性能基线记录
4. 多版本/多配置 A/B 对比

## 项目结构

```
gameperf/
├── main.go                    # 入口
├── internal/
│   ├── api/server.go          # HTTP API
│   ├── collector/collector.go # 性能采集器
│   ├── db/sqlite.go           # SQLite 存储
│   └── model/types.go         # 数据模型
├── web/                       # Vue 3 前端
│   ├── src/
│   │   ├── App.vue
│   │   ├── api.js
│   │   ├── router.js
│   │   └── views/
│   │       ├── Dashboard.vue  # 实时监控
│   │       ├── History.vue    # 历史记录
│   │       └── Compare.vue    # 对比分析
│   └── package.json
├── go.mod
└── README.md
```

## TODO

- [ ] GPU 显存采集（NVIDIA SMI）
- [ ] Windows PresentMon FPS 采集
- [ ] 报告导出（PDF/HTML）
- [ ] WebSocket 实时推送
- [ ] Docker 部署支持
