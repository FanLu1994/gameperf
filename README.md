# GamePerf 🎮

游戏性能数据采集 + 可视化对比面板。对标 PerfDog / MSI Afterburner / PresentMon，面向游戏 QA 工程师。

## 采集指标

### CPU
- 进程 CPU 使用率 %
- CPU 频率 MHz
- 用户态 / 内核态时间
- 上下文切换次数

### 内存
- RSS 物理内存 MB
- VMS 虚拟内存 MB
- 页错误次数
- 句柄数 / FD 数（Windows GDI 对象数）

### GPU（NVIDIA）
- GPU 使用率 %
- 显存已用 / 总量 MB
- GPU 温度 ℃
- GPU 核心频率 / 显存频率 MHz
- GPU 功耗 W
- 风扇转速 %

### 磁盘 I/O
- 累计读写字节
- 读写操作次数
- 实时读写速率 B/s

### 网络 I/O
- 累计发送 / 接收字节
- TCP 连接数
- 实时发送 / 接收速率 B/s

### 帧时间 & FPS
- 当前 FPS
- 帧时间 ms
- 1% Low FPS / 0.1% Low FPS
- 卡顿帧（Jank）检测
- 卡顿率 %
- PresentMon 集成支持

### 统计摘要
- 所有指标的 Avg / Max / Min / P1 / P5 / P95 / P99
- FPS 稳定性指数
- 帧时间分布直方图
- 卡顿段自动识别

## 技术栈

- **后端**: Go + Gin + SQLite + gopsutil + nvidia-smi
- **前端**: Vue 3 + ECharts + Element Plus（暗色主题）

## 快速开始

### 后端

```bash
go mod tidy
go build -o gameperf .
./gameperf --port 9090 --data ./data
```

### 前端

```bash
cd web
npm install
npm run dev      # 开发模式
npm run build    # 构建到 web/dist/
```

### 一键构建

```bash
chmod +x build.sh && ./build.sh
# 访问 http://localhost:9090
```

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/sessions | 创建会话 |
| GET | /api/sessions | 列出所有会话 |
| POST | /api/sessions/:id/start | 开始采集（pid + interval + enable_presentmon） |
| POST | /api/sessions/:id/stop | 停止采集 |
| DELETE | /api/sessions/:id | 删除会话 |
| GET | /api/sessions/:id/samples | 获取采样数据 |
| GET | /api/sessions/:id/summary | 统计摘要 |
| GET | /api/sessions/:id/frame-analysis | 帧时间分布分析 |
| GET | /api/sessions/:id/system | 系统硬件信息 |
| POST | /api/sessions/:id/samples | 手动注入数据 |
| GET | /api/compare?ids=a,b | 多会话对比 |
| GET | /api/info | 服务器运行时信息 |

### 使用示例

```bash
# 创建并开始采集（带 GPU + PresentMon）
curl -X POST http://localhost:9090/api/sessions \
  -d '{"name":"v1.0优化前","pid":12345}'

curl -X POST http://localhost:9090/api/sessions/{id}/start \
  -d '{"pid":12345,"interval":"1s","enable_presentmon":true}'

# 手动注入 FPS 数据
curl -X POST http://localhost:9090/api/sessions/{id}/samples \
  -d '{"fps":60,"frame_time":16.67,"jank_count":0}'

# 获取帧时间分布分析
curl http://localhost:9090/api/sessions/{id}/frame-analysis

# 对比两个会话
curl "http://localhost:9090/api/compare?ids=xxx,yyy"
```

## 项目结构

```
gameperf/
├── main.go
├── internal/
│   ├── model/types.go          # 数据模型（40+ 字段）
│   ├── db/sqlite.go            # SQLite + 统计 + 帧时间分析
│   ├── collector/collector.go  # CPU/内存/GPU/磁盘/网络/帧时间采集
│   └── api/server.go           # REST API
├── web/src/
│   ├── App.vue                 # 暗色 UI
│   ├── api.js
│   ├── router.js
│   └── views/
│       ├── Dashboard.vue       # 实时监控（10+ 图表）
│       ├── History.vue         # 历史记录 + 帧时间直方图
│       └── Compare.vue         # 多维叠加对比
├── go.mod
└── README.md
```

## GPU 支持

目前支持 NVIDIA GPU（通过 nvidia-smi），自动检测：

- 有 `nvidia-smi` 命令 → 自动采集 GPU 使用率、显存、温度、频率、功耗、风扇
- 无 GPU 或非 NVIDIA → GPU 相关字段为 0，不影响其他采集

## PresentMon FPS 采集

Windows 下可集成 PresentMon 获取精确帧时间：

```bash
# 设置 PresentMon 路径
set PRESENTMON_PATH=C:\Tools\PresentMon64-1.10.0.exe

# 或启动时指定
curl -X POST .../start -d '{"enable_presentmon":true,"presentmon_path":"C:\\Tools\\PresentMon.exe"}'
```

## 使用场景

1. 🎮 游戏版本优化前后对比
2. 📈 长时间运行内存泄漏监控
3. 🔥 GPU 温度/功耗监控
4. 🎯 FPS 稳定性 + 卡顿分析
5. 💾 磁盘/网络瓶颈定位
6. ⚖️ 多版本 / 多配置 A/B 对比
7. 📊 QA 自动化性能基线
