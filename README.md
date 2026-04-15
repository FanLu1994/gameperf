# GamePerf 🎮

游戏性能数据采集 + 可视化对比面板。支持 **Windows / Linux / Android** 三平台。

对标 PerfDog / MSI Afterburner，面向游戏 QA 工程师。

## 支持平台

| 平台 | 采集方式 | 指标覆盖 |
|------|---------|---------|
| 🖥️ Windows/Linux | gopsutil 进程级采集 + nvidia-smi GPU | CPU/内存/GPU/磁盘/网络/帧时间 |
| 📱 Android | ADB shell 远程采集（无需安装APK/无需root） | CPU/内存/GPU/FPS/磁盘/网络/电池/温度 |

## 采集指标

### 通用指标
- **CPU**: 使用率%、频率、用户态/内核态时间、上下文切换
- **内存**: RSS/VMS MB、页错误、句柄数
- **GPU**: 使用率%、显存MB、温度℃、频率MHz、功耗W、风扇% (NVIDIA/Adreno/Mali)
- **磁盘 I/O**: 读写字节、操作数、实时速率 B/s
- **网络 I/O**: 收发字节、连接数、实时速率 B/s
- **帧时间**: FPS、帧时间ms、1% Low FPS、Jank检测、卡顿率
- **统计**: Avg/Min/Max/P1/P5/P95/P99、FPS稳定性指数、帧时间分布直方图

### Android 专用
- **电池**: 电量%、温度℃、电压mV、电流mA、功耗mW
- **温度**: CPU温度、GPU温度、电池温度
- **FPS**: SurfaceFlinger 帧时间采集（支持所有游戏引擎）

## 快速开始

### 后端

```bash
go mod tidy
go build -o gameperf .
./gameperf --port 9090 --data ./data
```

### 前端

```bash
cd web && npm install && npm run dev
```

### 一键构建

```bash
chmod +x build.sh && ./build.sh
# 访问 http://localhost:9090
```

## Android 使用

1. PC 安装 ADB，USB 连接 Android 设备
2. 打开游戏
3. 在面板选择 📱 Android → 选择设备 → 选择包名 → 开始采集

**无需安装任何 APK，无需 root。**

### Android 采集原理

| 指标 | 采集方法 | 需要Root |
|------|---------|---------|
| CPU% | dumpsys cpuinfo | ❌ |
| 内存 PSS | dumpsys meminfo | ❌ |
| GPU 使用率 | Adreno: /sys/class/kgsl/kgsl-3d0/gpubusy | ❌ |
| GPU 温度 | /sys/class/thermal/thermal_zone* | ❌ |
| FPS/帧时间 | SurfaceFlinger --latency | ❌ |
| 磁盘 I/O | /proc/pid/io | ❌ |
| 网络 I/O | dumpsys netstats | ❌ |
| 电池 | /sys/class/power_supply/battery/* | ❌ |
| 温度 | /sys/class/thermal/thermal_zone* | ❌ |

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/sessions | 创建会话（platform: windows/android） |
| POST | /api/sessions/:id/start | 开始采集 |
| POST | /api/sessions/:id/stop | 停止采集 |
| GET | /api/sessions/:id/summary | 统计摘要 |
| GET | /api/sessions/:id/frame-analysis | 帧时间分布 |
| GET | /api/sessions/:id/system | 系统/设备信息 |
| GET | /api/compare?ids=a,b | 多会话对比 |
| GET | /api/android/devices | 列出ADB设备 |
| GET | /api/android/packages?device_id=x | 列出已安装包 |

### Windows 采集示例

```bash
curl -X POST http://localhost:9090/api/sessions \
  -d '{"name":"v1.0优化前","pid":12345,"platform":"windows"}'
curl -X POST http://localhost:9090/api/sessions/{id}/start \
  -d '{"pid":12345,"interval":"1s"}'
```

### Android 采集示例

```bash
curl -X POST http://localhost:9090/api/sessions \
  -d '{"name":"Android测试","platform":"android","package":"com.game.example","device_id":"adb123"}'
curl -X POST http://localhost:9090/api/sessions/{id}/start \
  -d '{"package":"com.game.example","device_id":"adb123","interval":"2s"}'
```

## 项目结构

```
gameperf/
├── main.go
├── internal/
│   ├── model/types.go              # 数据模型（50+ 字段）
│   ├── db/sqlite.go                # SQLite + 统计 + 帧时间分析
│   ├── collector/
│   │   ├── collector.go            # Windows/Linux 采集器
│   │   └── android.go             # Android ADB 采集器
│   └── api/server.go               # REST API
├── web/src/
│   ├── views/
│   │   ├── Dashboard.vue           # 实时监控（支持双平台切换）
│   │   ├── History.vue             # 历史记录 + 帧时间分析
│   │   └── Compare.vue             # 多维对比
│   └── api.js
├── go.mod
└── README.md
```

## GPU 支持

| 平台 | GPU | 采集方式 |
|------|-----|---------|
| Windows/Linux | NVIDIA | nvidia-smi |
| Android | Qualcomm Adreno | /sys/class/kgsl/ sysfs |
| Android | ARM Mali | /sys/class/misc/mali0/ sysfs |
| Android | 其他 | 温度/FPS 代理指标 |

## 使用场景

1. 🎮 PC/手游版本优化前后对比
2. 📈 长时间运行内存泄漏监控
3. 🔥 GPU 温度/功耗监控
4. 🎯 FPS 稳定性 + 卡顿分析
5. 📱 Android 电池功耗 + 降频检测
6. ⚖️ 多版本 / 多设备 A/B 对比
7. 📊 QA 自动化性能基线
