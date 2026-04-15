# GamePerf 🎮

游戏性能数据采集 + 可视化对比面板。支持 **Windows / Linux / Android / iOS** 四平台。

对标 PerfDog / MSI Afterburner，面向游戏 QA 工程师。

## 支持平台

| 平台 | 采集方式 | 宿主机 | 指标覆盖 |
|------|---------|--------|---------|
| 🖥️ Windows | gopsutil + nvidia-smi | 任意 OS | CPU/内存/GPU/磁盘/网络/帧时间 |
| 🐧 Linux | gopsutil + nvidia-smi | 任意 OS | CPU/内存/GPU/磁盘/网络 |
| 📱 Android | ADB shell 远程采集 | 任意 OS | CPU/内存/GPU/FPS/磁盘/网络/电池/温度 |
| 🍎 iOS | pymobiledevice3 instruments | **仅 macOS** | CPU/内存/FPS/磁盘/网络/电池/温度 |

## 采集指标

### 通用指标
- **CPU**: 使用率%、用户态/内核态、上下文切换
- **内存**: RSS/VMS MB、页错误
- **磁盘 I/O**: 读写字节、实时速率 B/s
- **网络 I/O**: 收发字节、实时速率 B/s
- **统计**: Avg/Min/Max/P1/P5/P95/P99、FPS稳定性指数

### GPU
| 平台 | GPU 厂商 | 采集方式 |
|------|---------|---------|
| Windows/Linux | NVIDIA | nvidia-smi |
| Android | Qualcomm Adreno | kgsl sysfs |
| Android | ARM Mali | mali0 sysfs |

### 帧时间 & FPS
- FPS、帧时间ms、1% Low / 0.1% Low FPS
- Jank 检测、卡顿率、帧时间分布直方图
- Android: SurfaceFlinger --latency
- Windows: PresentMon 集成
- iOS: CoreAnimation FPS (via instruments)

### 移动端专用 (Android + iOS)
- **电池**: 电量%、温度℃、电压mV、电流mA、功耗mW
- **温度**: CPU/GPU/电池温度、降频检测

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

## 平台使用

### Windows/Linux
输入目标进程 PID → 开始采集

### Android
1. PC 安装 ADB，USB 连接设备
2. 面板选 📱 Android → 选设备 → 选包名 → 开始
3. **无需安装 APK、无需 root**

### iOS
1. **macOS 宿主机** + `pip3 install pymobiledevice3`
2. USB 连接 iPhone/iPad，信任设备
3. 面板选 🍎 iOS → 选设备 → 输入 Bundle ID → 开始
4. **无需越狱、无需改 App、无需企业证书**

## 采集原理

### Android

| 指标 | 采集方法 | Root |
|------|---------|------|
| CPU% | dumpsys cpuinfo | ❌ |
| 内存 PSS | dumpsys meminfo | ❌ |
| GPU 使用率 | Adreno/Mali sysfs | ❌ |
| FPS | SurfaceFlinger --latency | ❌ |
| 磁盘 I/O | /proc/pid/io | ❌ |
| 网络 | dumpsys netstats | ❌ |
| 电池 | /sys/class/power_supply/ | ❌ |
| 温度 | /sys/class/thermal/ | ❌ |

### iOS

| 指标 | 采集方法 | 越狱 |
|------|---------|------|
| CPU% | DVT instruments sysmon | ❌ |
| 内存 physFootprint | DVT instruments sysmon | ❌ |
| FPS | CoreAnimation instruments | ❌ |
| 磁盘 I/O | sysmon process | ❌ |
| 网络 | sysmon process | ❌ |
| 电池 | diagnostics battery | ❌ |
| 温度 | diagnostics thermal | ❌ |
| GPU 使用率 | ❌ 需要 SDK 集成 | — |

## API

### 通用

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/sessions | 创建会话 (platform: windows/android/ios) |
| POST | /api/sessions/:id/start | 开始采集 |
| POST | /api/sessions/:id/stop | 停止采集 |
| GET | /api/sessions/:id/summary | 统计摘要 |
| GET | /api/sessions/:id/frame-analysis | 帧时间分布 |
| GET | /api/sessions/:id/system | 系统/设备信息 |
| GET | /api/compare?ids=a,b | 多会话对比 |

### Android

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/android/devices | 列出 ADB 设备 |
| GET | /api/android/packages | 列出已安装包 |

### iOS

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/ios/check | 检查依赖是否就绪 |
| GET | /api/ios/devices | 列出 iOS 设备 |
| GET | /api/ios/apps | 列出已安装 App |

## 项目结构

```
gameperf/
├── main.go
├── internal/
│   ├── model/types.go              # 数据模型（50+ 字段）
│   ├── db/sqlite.go                # SQLite + 统计 + 帧时间分析
│   ├── collector/
│   │   ├── collector.go            # Windows/Linux 采集器
│   │   ├── android.go              # Android ADB 采集器
│   │   └── ios.go                  # iOS instruments 采集器
│   └── api/server.go               # REST API
├── web/src/
│   ├── views/
│   │   ├── Dashboard.vue           # 实时监控（四平台切换）
│   │   ├── History.vue             # 历史记录 + 帧时间分析
│   │   └── Compare.vue             # 多维对比
│   └── api.js
├── go.mod
└── README.md
```

## 使用场景

1. 🎮 PC/手游版本优化前后对比
2. 📈 长时间运行内存泄漏监控
3. 🔥 GPU 温度/功耗监控
4. 🎯 FPS 稳定性 + 卡顿分析
5. 📱 Android 电池功耗 + 降频检测
6. 🍎 iOS 内存/CPU/电池监控
7. ⚖️ 跨平台对比（Android vs iOS 同游戏）
8. 📊 QA 自动化性能基线
