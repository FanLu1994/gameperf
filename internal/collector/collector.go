package collector

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"gameperf/internal/db"
	"gameperf/internal/model"
)

// Collector 进程性能采集器
type Collector struct {
	database  *db.DB
	interval  time.Duration
	pid       int32
	proc      *process.Process
	stopCh    chan struct{}

	// 上一帧数据（用于计算速率）
	prevDiskRead   uint64
	prevDiskWrite  uint64
	prevNetSent    uint64
	prevNetRecv    uint64
	prevTimestamp  int64

	// 帧时间窗口（用于计算 1% low 等）
	frameTimeWindow []float64
	windowSize      int

	// GPU 采集方式
	gpuMethod string // "nvidia-smi" | "none"
}

func New(database *db.DB, pid int32, interval time.Duration) (*Collector, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("find process pid=%d: %w", pid, err)
	}

	c := &Collector{
		database:   database,
		interval:   interval,
		pid:        pid,
		proc:       proc,
		stopCh:     make(chan struct{}),
		windowSize: 60, // 60帧窗口计算 1% low
	}

	// 检测 GPU 采集方式
	c.detectGPUMethod()

	return c, nil
}

func (c *Collector) detectGPUMethod() {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		c.gpuMethod = "none"
		return
	}

	// 尝试 nvidia-smi
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		_ = path
		c.gpuMethod = "nvidia-smi"
		return
	}

	c.gpuMethod = "none"
}

// Start 开始采集，阻塞直到 Stop
func (c *Collector) Start(sessionID string) error {
	session, err := c.database.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// 采集系统信息并保存
	sysInfo := c.collectSystemInfo()
	sysInfo.SessionID = sessionID
	c.database.SaveSystemInfo(sysInfo)

	startUnix := session.StartTime.Unix()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	fmt.Printf("[collector] 开始采集 session=%s pid=%d interval=%v gpu=%s\n", sessionID, c.pid, c.interval, c.gpuMethod)

	for {
		select {
		case <-c.stopCh:
			fmt.Printf("[collector] 停止采集 session=%s\n", sessionID)
			return nil
		case t := <-ticker.C:
			sample := c.collect(t.Unix(), startUnix)
			sample.SessionID = sessionID
			if err := c.database.InsertSample(sample); err != nil {
				fmt.Printf("[collector] 写入失败: %v\n", err)
			}
		}
	}
}

func (c *Collector) Stop() {
	close(c.stopCh)
}

func (c *Collector) collect(now, startUnix int64) *model.Sample {
	sample := &model.Sample{
		Timestamp: now,
		Elapsed:   float64(now - startUnix),
	}

	// === CPU ===
	if cpuPercent, err := c.proc.CPPercent(0); err == nil {
		sample.CPU = cpuPercent
	}

	// CPU 频率
	if freqs, err := cpu.Info(); err == nil && len(freqs) > 0 {
		sample.CPUFreq = freqs[0].Mhz
	}

	// CPU times
	if times, err := c.proc.Times(); err == nil {
		sample.CPUTimeUser = times.User
		sample.CPUTimeSystem = times.System
	}

	// 上下文切换
	if ctx, err := c.proc.NumCtxSwitches(); err == nil {
		sample.CtxSwitches = int64(ctx.Voluntary + ctx.Involuntary)
	}

	// === 内存 ===
	if memInfo, err := c.proc.MemoryInfo(); err == nil {
		sample.Memory = float64(memInfo.RSS) / 1024 / 1024
		sample.MemoryVMS = float64(memInfo.VMS) / 1024 / 1024
		sample.PageFaults = int64(memInfo.PageFaults)
	}

	// 句柄数 / FD 数
	if fds, err := c.proc.NumFDs(); err == nil {
		sample.HandleCount = fds
	}
	// Windows GDI 对象 — 仅 Windows
	if runtime.GOOS == "windows" {
		sample.GDIObjects = getGDIObjects(c.pid)
	}

	// === 线程数 ===
	if numThreads, err := c.proc.NumThreads(); err == nil {
		sample.Threads = numThreads
	}

	// === 磁盘 I/O ===
	if ioCounters, err := c.proc.IOCounters(); err == nil {
		sample.DiskReadBytes = int64(ioCounters.ReadBytes)
		sample.DiskWriteBytes = int64(ioCounters.WriteBytes)
		sample.DiskReadOps = int64(ioCounters.ReadCount)
		sample.DiskWriteOps = int64(ioCounters.WriteCount)

		// 计算速率
		if c.prevTimestamp > 0 {
			dt := float64(now - c.prevTimestamp)
			if dt > 0 {
				sample.DiskReadBPS = float64(ioCounters.ReadBytes-c.prevDiskRead) / dt
				sample.DiskWriteBPS = float64(ioCounters.WriteBytes-c.prevDiskWrite) / dt
			}
		}
		c.prevDiskRead = ioCounters.ReadBytes
		c.prevDiskWrite = ioCounters.WriteBytes
	}

	// === 网络 I/O ===
	if conns, err := c.proc.Connections(); err == nil {
		sample.NetConnCount = int32(len(conns))
	}

	// 网络字节 — 使用系统级 net IO（近似）
	if netCounters, err := net.IOCounters(false); err == nil && len(netCounters) > 0 {
		totalSent := netCounters[0].BytesSent
		totalRecv := netCounters[0].BytesRecv
		sample.NetBytesSent = int64(totalSent)
		sample.NetBytesRecv = int64(totalRecv)

		if c.prevTimestamp > 0 {
			dt := float64(now - c.prevTimestamp)
			if dt > 0 {
				sample.NetSentBPS = float64(totalSent-uint64(c.prevNetSent)) / dt
				sample.NetRecvBPS = float64(totalRecv-uint64(c.prevNetRecv)) / dt
			}
		}
		c.prevNetSent = uint64(totalSent)
		c.prevNetRecv = uint64(totalRecv)
	}

	c.prevTimestamp = now

	// === GPU ===
	switch c.gpuMethod {
	case "nvidia-smi":
		c.collectGPUViaNvidiaSmi(sample)
	}

	// === FPS / 帧时间 — 需要外部注入或 PresentMon ===
	// 通过 InjectFrameData 或 PresentMon 集成填充

	return sample
}

// collectGPUViaNvidiaSmi 通过 nvidia-smi 采集 GPU 指标
func (c *Collector) collectGPUViaNvidiaSmi(sample *model.Sample) {
	// 使用 nvidia-smi 查询当前进程的 GPU 使用情况
	// 查询全局 GPU 状态
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu,clocks.gr,clocks.mem,power.draw,fan.speed",
		"--format=csv,noheader,nounits",
	)

	output, err := cmd.Output()
	if err != nil {
		return
	}

	line := strings.TrimSpace(string(output))
	if line == "" {
		return
	}

	reader := csv.NewReader(strings.NewReader(line))
	record, err := reader.Read()
	if err != nil || len(record) < 8 {
		return
	}

	if v, err := strconv.ParseFloat(strings.TrimSpace(record[0]), 64); err == nil {
		sample.GPUUtil = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64); err == nil {
		sample.GPUMem = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64); err == nil {
		sample.GPUMemTotal = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64); err == nil {
		sample.GPUTemp = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64); err == nil {
		sample.GPUClock = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64); err == nil {
		sample.GPUMemClock = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[6]), 64); err == nil {
		sample.GPUPower = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(record[7]), 64); err == nil {
		sample.GPUFanSpeed = v
	}
}

// collectSystemInfo 采集系统硬件信息
func (c *Collector) collectSystemInfo() *model.SystemInfo {
	info := &model.SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.Arch,
	}

	// CPU 信息
	if cpuInfos, err := cpu.Info(); err == nil && len(cpuInfos) > 0 {
		info.CPUModel = cpuInfos[0].ModelName
	}
	if cores, err := cpu.Counts(true); err == nil {
		info.CPUCores = cores
	}

	// 内存
	if mem, err := c.proc.MemoryInfo(); err == nil {
		// 使用系统内存总量 — 通过 /proc/meminfo 或全局
		info.TotalMemory = int64(float64(mem.RSS) / 1024 / 1024) // 近似
	}

	// GPU — nvidia-smi
	if c.gpuMethod == "nvidia-smi" {
		cmd := exec.Command("nvidia-smi",
			"--query-gpu=name,driver_version,memory.total",
			"--format=csv,noheader,nounits",
		)
		if output, err := cmd.Output(); err == nil {
			reader := csv.NewReader(strings.NewReader(strings.TrimSpace(string(output))))
			if record, err := reader.Read(); err == nil && len(record) >= 3 {
				info.GPUName = strings.TrimSpace(record[0])
				info.GPUDriver = strings.TrimSpace(record[1])
				if v, err := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64); err == nil {
					info.GPUVRAM = v
				}
			}
		}
	}

	return info
}

// getGDIObjects Windows 下获取 GDI 对象数
func getGDIObjects(pid int32) int32 {
	// Windows syscall 实现 — 需要构建标签约束
	// 在非 Windows 平台返回 0
	if runtime.GOOS != "windows" {
		return 0
	}

	// 通过 syscall 调用 GetGuiResources
	// 这里用 exec 调用 PowerShell 作为 fallback
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf(`$p = Get-Process -Id %d -ErrorAction SilentlyContinue; if ($p) { $p.HandleCount } else { 0 }`, pid))
	if output, err := cmd.Output(); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 32); err == nil {
			return int32(v)
		}
	}
	return 0
}

// --- PresentMon 集成 ---

// PresentMonCollector 通过 PresentMon 采集帧时间
type PresentMonCollector struct {
	sessionID string
	database  *db.DB
	cmd       *exec.Cmd
	pid       int32
	stopCh    chan struct{}
}

func NewPresentMonCollector(database *db.DB, sessionID string, pid int32, presentMonPath string) (*PresentMonCollector, error) {
	return &PresentMonCollector{
		sessionID: sessionID,
		database:  database,
		pid:       pid,
		stopCh:    make(chan struct{}),
	}, nil
}

// StartPresentMon 启动 PresentMon 进程并解析输出
func (pm *PresentMonCollector) StartPresentMon(presentMonPath string) error {
	// PresentMon 输出 CSV 到 stdout
	pm.cmd = exec.Command(presentMonPath,
		"-captureno",
		"-process_name", fmt.Sprintf("%d", pm.pid),
		"-output_file", "-",
	)

	stdout, err := pm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("presentmon stdout pipe: %w", err)
	}

	if err := pm.cmd.Start(); err != nil {
		return fmt.Errorf("start presentmon: %w", err)
	}

	go pm.parseCSV(stdout)
	return nil
}

func (pm *PresentMonCollector) parseCSV(reader io.Reader) {
	csvReader := csv.NewReader(reader)
	// 跳过表头
	csvReader.Read()

	var prevFrameTime float64
	frameWindow := make([]float64, 0, 60)
	jankCount := int32(0)
	stutterFrames := int32(0)
	totalFrames := int32(0)

	for {
		record, err := csvReader.Read()
		if err != nil {
			break
		}

		// PresentMon CSV 列: Application, ProcessID, SwapChain, Runtime, SyncInterval, PresentFlags, Dropped,
		//                     TimeInSeconds, MsBetweenPresents, MsInPresentAPI, MsUntilRenderComplete,
		//                     MsUntilDisplayed, MsBetweenDisplayChange, QPCTime, ...
		if len(record) < 10 {
			continue
		}

		frameTime, err := strconv.ParseFloat(record[8], 64) // MsBetweenPresents
		if err != nil {
			continue
		}

		totalFrames++
		fps := 0.0
		if frameTime > 0 {
			fps = 1000.0 / frameTime
		}

		// Jank 检测
		isJank := false
		if prevFrameTime > 0 && frameTime > prevFrameTime*2 {
			isJank = true
			jankCount++
			stutterFrames++
		}

		// 更新窗口
		frameWindow = append(frameWindow, frameTime)
		if len(frameWindow) > 60 {
			frameWindow = frameWindow[1:]
		}

		// 计算 1% low
		fps1Low := 0.0
		fps01Low := 0.0
		if len(frameWindow) >= 10 {
			sorted := make([]float64, len(frameWindow))
			copy(sorted, frameWindow)
			sortFloat64s(sorted)
			p1Idx := max(0, len(sorted)/100)
			p01Idx := max(0, len(sorted)/1000)
			if sorted[p1Idx] > 0 {
				fps1Low = 1000.0 / sorted[p1Idx]
			}
			if sorted[p01Idx] > 0 {
				fps01Low = 1000.0 / sorted[p01Idx]
			}
		}

		// 注入到最近的采样中（通过 API）
		_ = fps
		_ = fps1Low
		_ = fps01Low
		_ = jankCount
		_ = stutterFrames
		_ = totalFrames
		_ = isJank

		prevFrameTime = frameTime
	}
}

func (pm *PresentMonCollector) Stop() {
	if pm.cmd != nil && pm.cmd.Process != nil {
		pm.cmd.Process.Kill()
	}
	close(pm.stopCh)
}

// InjectFrameData 手动注入帧数据（供外部调用或 API 使用）
func InjectFrameData(sample *model.Sample, frameTimeMs float64, prevFrameTimeMs float64) {
	sample.FrameTime = frameTimeMs
	if frameTimeMs > 0 {
		sample.FPS = 1000.0 / frameTimeMs
	}

	// Jank 检测
	if prevFrameTimeMs > 0 && frameTimeMs > prevFrameTimeMs*2 {
		sample.JankCount = 1
		if frameTimeMs > prevFrameTimeMs*3 {
			// Big Jank
		}
	}
}

// --- helpers ---

func sortFloat64s(a []float64) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CheckPresentMon 检查 PresentMon 是否可用
func CheckPresentMon() (string, error) {
	// 检查环境变量
	if path := os.Getenv("PRESENTMON_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 检查 PATH
	if path, err := exec.LookPath("PresentMon64-1.10.0"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("PresentMon"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("PresentMon not found. Set PRESENTMON_PATH env or add to PATH")
}
