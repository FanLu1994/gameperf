package model

import "time"

// Session 一次采集会话
type Session struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`       // 标签，如 "v1.0优化前"
	Process   string     `json:"process"`    // 目标进程名/PID
	PID       int32      `json:"pid"`        // 目标进程PID
	Platform  string     `json:"platform"`   // windows / linux / android
	Package   string     `json:"package"`    // Android 包名
	DeviceID  string     `json:"device_id"`  // Android 设备序列号
	Status    string     `json:"status"`     // running / stopped
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Interval  string     `json:"interval"`   // 采样间隔
	Tags      string     `json:"tags"`       // 逗号分隔标签
}

// Sample 一条采样数据 — 覆盖 CPU/GPU/内存/磁盘/网络/帧时间
type Sample struct {
	ID        int64   `json:"id"`
	SessionID string  `json:"session_id"`
	Timestamp int64   `json:"timestamp"`  // unix秒
	Elapsed   float64 `json:"elapsed"`    // 距采集开始的秒数

	// === CPU ===
	CPU            float64 `json:"cpu"`              // 进程CPU%
	CPUCores       string  `json:"cpu_cores"`        // JSON: 各核心使用率 [12.3, 45.6, ...]
	CPUFreq        float64 `json:"cpu_freq"`         // CPU频率MHz
	CPUTimeUser    float64 `json:"cpu_time_user"`    // 用户态时间(s)
	CPUTimeSystem  float64 `json:"cpu_time_system"`  // 内核态时间(s)
	CtxSwitches    int64   `json:"ctx_switches"`     // 上下文切换次数

	// === 内存 ===
	Memory        float64 `json:"memory"`         // RSS MB
	MemoryVMS     float64 `json:"memory_vms"`     // 虚拟内存 MB
	PageFaults    int64   `json:"page_faults"`    // 页错误次数
	HandleCount   int32   `json:"handle_count"`   // 句柄数(Windows)/FD数(Linux)
	GDIObjects    int32   `json:"gdi_objects"`    // GDI对象数(Windows)

	// === GPU ===
	GPUUtil       float64 `json:"gpu_util"`       // GPU使用率%
	GPUMem        float64 `json:"gpu_mem"`        // GPU显存已用MB
	GPUMemTotal   float64 `json:"gpu_mem_total"`  // GPU显存总量MB
	GPUTemp       float64 `json:"gpu_temp"`       // GPU温度℃
	GPUClock      float64 `json:"gpu_clock"`      // GPU核心频率MHz
	GPUMemClock   float64 `json:"gpu_mem_clock"`  // GPU显存频率MHz
	GPUPower      float64 `json:"gpu_power"`      // GPU功耗W
	GPUFanSpeed   float64 `json:"gpu_fan_speed"`  // GPU风扇转速%

	// === 磁盘 I/O ===
	DiskReadBytes  int64   `json:"disk_read_bytes"`   // 累计读字节
	DiskWriteBytes int64   `json:"disk_write_bytes"`  // 累计写字节
	DiskReadOps    int64   `json:"disk_read_ops"`     // 累计读次数
	DiskWriteOps   int64   `json:"disk_write_ops"`    // 累计写次数
	DiskReadBPS    float64 `json:"disk_read_bps"`     // 读速率 B/s
	DiskWriteBPS   float64 `json:"disk_write_bps"`    // 写速率 B/s

	// === 网络 I/O ===
	NetBytesSent   int64   `json:"net_bytes_sent"`    // 累计发送字节
	NetBytesRecv   int64   `json:"net_bytes_recv"`    // 累计接收字节
	NetConnCount   int32   `json:"net_conn_count"`    // TCP连接数
	NetSentBPS     float64 `json:"net_sent_bps"`      // 发送速率 B/s
	NetRecvBPS     float64 `json:"net_recv_bps"`      // 接收速率 B/s

	// === 帧时间 & FPS ===
	FPS           float64 `json:"fps"`              // 当前帧率
	FrameTime     float64 `json:"frame_time"`       // 帧时间ms
	FrameTimeMin  float64 `json:"frame_time_min"`   // 最近N帧最小帧时间
	FrameTimeMax  float64 `json:"frame_time_max"`   // 最近N帧最大帧时间
	FPS1Low       float64 `json:"fps_1_low"`        // 1% Low FPS
	FPS01Low      float64 `json:"fps_01_low"`       // 0.1% Low FPS
	JankCount     int32   `json:"jank_count"`       // 卡顿帧数
	StutterRate   float64 `json:"stutter_rate"`     // 卡顿率%

	// === 线程 ===
	Threads       int32   `json:"threads"`          // 线程数

	// === 电池 (Android) ===
	BatteryLevel  float64 `json:"battery_level"`   // 电量%
	BatteryTemp   float64 `json:"battery_temp"`    // 电池温度℃
	BatteryPower  float64 `json:"battery_power"`   // 功耗mW
	BatteryVoltage float64 `json:"battery_voltage"` // 电压mV
	BatteryCurrent float64 `json:"battery_current"` // 电流mA

	// === 温度 (Android) ===
	CPUTemp       float64 `json:"cpu_temp"`        // CPU温度℃
}

// SessionSummary 会话统计摘要
type SessionSummary struct {
	Session      Session  `json:"session"`
	Duration     float64  `json:"duration"`
	SampleCount  int      `json:"sample_count"`

	// CPU 统计
	AvgCPU       float64  `json:"avg_cpu"`
	MaxCPU       float64  `json:"max_cpu"`
	MinCPU       float64  `json:"min_cpu"`
	P95CPU       float64  `json:"p95_cpu"`
	P99CPU       float64  `json:"p99_cpu"`

	// 内存统计
	AvgMemory    float64  `json:"avg_memory"`
	MaxMemory    float64  `json:"max_memory"`
	MinMemory    float64  `json:"min_memory"`
	P95Memory    float64  `json:"p95_memory"`
	P99Memory    float64  `json:"p99_memory"`

	// GPU 统计
	AvgGPU       float64  `json:"avg_gpu"`
	MaxGPU       float64  `json:"max_gpu"`
	P95GPU       float64  `json:"p95_gpu"`
	MaxGPUTemp   float64  `json:"max_gpu_temp"`
	AvgGPUPower  float64  `json:"avg_gpu_power"`
	MaxGPUMem    float64  `json:"max_gpu_mem"`

	// FPS 统计
	AvgFPS       float64  `json:"avg_fps"`
	MinFPS       float64  `json:"min_fps"`
	MaxFPS       float64  `json:"max_fps"`
	P1FPS        float64  `json:"p1_fps"`
	P5FPS        float64  `json:"p5_fps"`
	P95FPS       float64  `json:"p95_fps"`
	FPSStability float64  `json:"fps_stability"`  // FPS稳定性%(1-std/avg)

	// 帧时间统计
	AvgFrameTime  float64 `json:"avg_frame_time"`
	P95FrameTime  float64 `json:"p95_frame_time"`
	P99FrameTime  float64 `json:"p99_frame_time"`
	MaxFrameTime  float64 `json:"max_frame_time"`

	// 卡顿统计
	TotalJankCount int32   `json:"total_jank_count"`
	AvgStutterRate float64 `json:"avg_stutter_rate"`

	// 磁盘 I/O 统计
	AvgDiskReadBPS  float64 `json:"avg_disk_read_bps"`
	MaxDiskReadBPS  float64 `json:"max_disk_read_bps"`
	AvgDiskWriteBPS float64 `json:"avg_disk_write_bps"`
	MaxDiskWriteBPS float64 `json:"max_disk_write_bps"`

	// 网络 I/O 统计
	AvgNetSentBPS  float64 `json:"avg_net_sent_bps"`
	MaxNetSentBPS  float64 `json:"max_net_sent_bps"`
	AvgNetRecvBPS  float64 `json:"avg_net_recv_bps"`
	MaxNetRecvBPS  float64 `json:"max_net_recv_bps"`

	// 句柄/线程
	MaxThreads     int32   `json:"max_threads"`
	MaxHandleCount int32   `json:"max_handle_count"`

	// 电池统计 (Android)
	MinBatteryLevel float64 `json:"min_battery_level"`
	MaxBatteryTemp  float64 `json:"max_battery_temp"`
	AvgBatteryPower float64 `json:"avg_battery_power"`

	// 温度统计 (Android)
	MaxCPUTemp      float64 `json:"max_cpu_temp"`
}

// FrameTimeAnalysis 帧时间分布分析
type FrameTimeAnalysis struct {
	SessionID    string         `json:"session_id"`
	FrameCount   int            `json:"frame_count"`
	Histogram    []Bucket       `json:"histogram"`    // 帧时间分布直方图
	JankFrames   []JankFrame    `json:"jank_frames"`  // 卡顿帧详情
	StutterSections []StutterSection `json:"stutter_sections"` // 卡顿段
}

// Bucket 直方图桶
type Bucket struct {
	RangeStart float64 `json:"range_start"` // ms
	RangeEnd   float64 `json:"range_end"`
	Count      int     `json:"count"`
}

// JankFrame 卡顿帧
type JankFrame struct {
	Elapsed     float64 `json:"elapsed"`
	FrameTime   float64 `json:"frame_time"`
	Severity    string  `json:"severity"`    // jank / big_jank
}

// StutterSection 连续卡顿段
type StutterSection struct {
	StartElapsed float64 `json:"start_elapsed"`
	EndElapsed   float64 `json:"end_elapsed"`
	Duration     float64 `json:"duration"`
	FrameCount   int     `json:"frame_count"`
}

// CompareResult 对比结果
type CompareResult struct {
	Summaries []SessionSummary `json:"summaries"`
}

// SystemInfo 系统信息（采集开始时记录一次）
type SystemInfo struct {
	SessionID     string `json:"session_id"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPUModel      string `json:"cpu_model"`
	CPUCores      int    `json:"cpu_cores"`
	TotalMemory   int64  `json:"total_memory"` // MB
	GPUName       string `json:"gpu_name"`
	GPUDriver     string `json:"gpu_driver"`
	GPUVRAM       int64  `json:"gpu_vram"`     // MB
	DeviceModel   string `json:"device_model"`   // Android 设备型号
	OSVersion      string `json:"os_version"`      // e.g. "14" (Android) or "17.2" (iOS)
	AndroidAPI    int    `json:"android_api"`     // e.g. 34
}
