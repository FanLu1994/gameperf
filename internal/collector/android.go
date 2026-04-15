package collector

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gameperf/internal/db"
	"gameperf/internal/model"
)

// AndroidCollector 通过 ADB 采集 Android 设备性能数据
type AndroidCollector struct {
	database    *db.DB
	interval    time.Duration
	packageName string
	deviceID    string
	pid         int32
	stopCh      chan struct{}

	// GPU 厂商检测
	gpuVendor string // "adreno" / "mali" / "powervr" / "unknown"

	// SurfaceFlinger surface 名
	surfaceName string

	// 上一帧数据（计算速率用）
	prevDiskRead  int64
	prevDiskWrite int64
	prevNetSent   int64
	prevNetRecv   int64
	prevTimestamp int64

	// 上一帧 FPS 数据
	prevFrameReady int64 // SurfaceFlinger last frame timestamp (ns)
	frameCount     int
	fpsWindowStart int64
}

func NewAndroidCollector(database *db.DB, packageName, deviceID string, interval time.Duration) *AndroidCollector {
	return &AndroidCollector{
		database:    database,
		interval:    interval,
		packageName: packageName,
		deviceID:    deviceID,
		stopCh:      make(chan struct{}),
	}
}

// adb 执行 ADB 命令并返回输出
func (ac *AndroidCollector) adb(args ...string) (string, error) {
	fullArgs := args
	if ac.deviceID != "" {
		fullArgs = append([]string{"-s", ac.deviceID}, args...)
	}
	cmd := exec.Command("adb", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("adb %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// detectGPUVendor 检测 GPU 厂商
func (ac *AndroidCollector) detectGPUVendor() string {
	// 检查 Adreno
	if out, err := ac.adb("shell", "ls", "/sys/class/kgsl/kgsl-3d0/gpubusy"); err == nil && out != "" {
		return "adreno"
	}
	// 检查 Mali
	if out, err := ac.adb("shell", "ls", "/sys/class/misc/mali0/device/utilization"); err == nil && out != "" {
		return "mali"
	}
	// 通过 ro.hardware 推断
	hw, _ := ac.adb("shell", "getprop", "ro.hardware")
	hw = strings.ToLower(hw)
	if strings.Contains(hw, "qcom") || strings.Contains(hw, "sdm") || strings.Contains(hw, "sm") {
		return "adreno"
	}
	if strings.Contains(hw, "exynos") || strings.Contains(hw, "mt") {
		return "mali"
	}
	return "unknown"
}

// findSurfaceName 查找游戏的 SurfaceFlinger surface 名
func (ac *AndroidCollector) findSurfaceName() string {
	out, err := ac.adb("shell", "dumpsys", "SurfaceFlinger", "--list")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ac.packageName) {
			return line
		}
	}
	return ""
}

// getPID 获取进程 PID
func (ac *AndroidCollector) getPID() (int32, error) {
	out, err := ac.adb("shell", "pidof", ac.packageName)
	if err != nil || out == "" {
		return 0, fmt.Errorf("process %s not found", ac.packageName)
	}
	// pidof 可能返回多个 PID（空格分隔），取第一个
	pidStr := strings.Fields(out)[0]
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return int32(pid), nil
}

// collectSystemInfo 采集 Android 设备信息
func (ac *AndroidCollector) collectSystemInfo() *model.SystemInfo {
	info := &model.SystemInfo{OS: "android", Arch: "arm64"}

	// 设备型号
	if model, _ := ac.adb("shell", "getprop", "ro.product.model"); model != "" {
		info.DeviceModel = model
	}
	// Android 版本
	if ver, _ := ac.adb("shell", "getprop", "ro.build.version.release"); ver != "" {
		info.AndroidVersion = ver
	}
	// API level
	if api, _ := ac.adb("shell", "getprop", "ro.build.version.sdk"); api != "" {
		if v, err := strconv.Atoi(api); err == nil {
			info.AndroidAPI = v
		}
	}
	// CPU 核数
	if cores, _ := ac.adb("shell", "nproc"); cores != "" {
		if v, err := strconv.Atoi(cores); err == nil {
			info.CPUCores = v
		}
	}
	// CPU 型号
	if cpuinfo, _ := ac.adb("shell", "grep", "-m1", "Hardware", "/proc/cpuinfo"); cpuinfo != "" {
		info.CPUModel = strings.TrimPrefix(cpuinfo, "Hardware\t: ")
	}
	// 总内存
	if mem, _ := ac.adb("shell", "grep", "MemTotal", "/proc/meminfo"); mem != "" {
		// MemTotal:       16384 kB
		fields := strings.Fields(mem)
		if len(fields) >= 2 {
			if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				info.TotalMemory = v / 1024 // kB -> MB
			}
		}
	}
	// GPU 型号
	switch ac.gpuVendor {
	case "adreno":
		if name, _ := ac.adb("shell", "cat", "/sys/class/kgsl/kgsl-3d0/gpu_model"); name != "" {
			info.GPUName = name
		} else {
			info.GPUName = "Qualcomm Adreno"
		}
	case "mali":
		info.GPUName = "ARM Mali"
	}

	return info
}

// Start 启动采集循环
func (ac *AndroidCollector) Start(sessionID string) error {
	// 检测 GPU 厂商
	ac.gpuVendor = ac.detectGPUVendor()
	fmt.Printf("[android] GPU vendor: %s\n", ac.gpuVendor)

	// 获取 PID
	pid, err := ac.getPID()
	if err != nil {
		return err
	}
	ac.pid = pid

	// 查找 Surface 名
	ac.surfaceName = ac.findSurfaceName()
	fmt.Printf("[android] surface: %s\n", ac.surfaceName)

	// 保存系统信息
	session, err := ac.database.GetSession(sessionID)
	if err != nil {
		return err
	}
	sysInfo := ac.collectSystemInfo()
	sysInfo.SessionID = sessionID
	ac.database.SaveSystemInfo(sysInfo)

	startUnix := session.StartTime.Unix()
	ticker := time.NewTicker(ac.interval)
	defer ticker.Stop()

	fmt.Printf("[android] 开始采集 session=%s pkg=%s pid=%d interval=%v\n", sessionID, ac.packageName, pid, ac.interval)

	// 重置 gfxinfo
	ac.adb("shell", "dumpsys", "gfxinfo", ac.packageName, "reset")

	for {
		select {
		case <-ac.stopCh:
			fmt.Printf("[android] 停止采集 session=%s\n", sessionID)
			return nil
		case t := <-ticker.T:
			sample := ac.collect(t.Unix(), startUnix)
			sample.SessionID = sessionID
			if err := ac.database.InsertSample(sample); err != nil {
				fmt.Printf("[android] 写入失败: %v\n", err)
			}
			// 刷新 PID（进程可能重启）
			if newPID, err := ac.getPID(); err == nil && newPID != ac.pid {
				fmt.Printf("[android] PID 变化: %d -> %d\n", ac.pid, newPID)
				ac.pid = newPID
			}
		}
	}
}

func (ac *AndroidCollector) Stop() {
	close(ac.stopCh)
}

func (ac *AndroidCollector) collect(now, startUnix int64) *model.Sample {
	sample := &model.Sample{
		Timestamp: now,
		Elapsed:   float64(now - startUnix),
	}

	// === CPU === (dumpsys cpuinfo)
	ac.collectCPU(sample)

	// === 内存 === (dumpsys meminfo)
	ac.collectMemory(sample)

	// === GPU ===
	switch ac.gpuVendor {
	case "adreno":
		ac.collectGPUAdreno(sample)
	case "mali":
		ac.collectGPUMali(sample)
	}

	// === FPS / 帧时间 === (SurfaceFlinger)
	ac.collectFPS(sample)

	// === 磁盘 I/O === (/proc/pid/io)
	ac.collectDiskIO(sample, now)

	// === 网络 === (dumpsys netstats)
	ac.collectNetwork(sample, now)

	// === 电池 ===
	ac.collectBattery(sample)

	// === 温度 ===
	ac.collectThermal(sample)

	// === 线程数 ===
	if out, err := ac.adb("shell", "ls", fmt.Sprintf("/proc/%d/task", ac.pid)); err == nil {
		sample.Threads = int32(len(strings.Fields(out)))
	}

	return sample
}

// collectCPU 通过 dumpsys cpuinfo 采集 CPU%
func (ac *AndroidCollector) collectCPU(sample *model.Sample) {
	out, err := ac.adb("shell", "dumpsys", "cpuinfo")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, fmt.Sprintf("%d/%s", ac.pid, ac.packageName)) {
			// "50% 3895/com.game.example: 41% user + 9% kernel"
			re := regexp.MustCompile(`(\d+(?:\.\d+)?)%\s+\d+/`)
			if re.MatchString(line) {
				val := re.FindStringSubmatch(line)[1]
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					sample.CPU = v
				}
			}
			// 提取 user/system
			if parts := strings.Split(line, ": "); len(parts) >= 2 {
				detail := parts[1]
				re2 := regexp.MustCompile(`(\d+(?:\.\d+)?)%\s+user`)
				if m := re2.FindStringSubmatch(detail); len(m) > 1 {
					if v, err := strconv.ParseFloat(m[1], 64); err == nil {
						sample.CPUTimeUser = v
					}
				}
				re3 := regexp.MustCompile(`(\d+(?:\.\d+)?)%\s+kernel`)
				if m := re3.FindStringSubmatch(detail); len(m) > 1 {
					if v, err := strconv.ParseFloat(m[1], 64); err == nil {
						sample.CPUTimeSystem = v
					}
				}
			}
			break
		}
	}
}

// collectMemory 通过 dumpsys meminfo 采集内存
func (ac *AndroidCollector) collectMemory(sample *model.Sample) {
	out, err := ac.adb("shell", "dumpsys", "meminfo", ac.packageName)
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TOTAL") || strings.HasPrefix(line, "TOTAL PSS:") {
			// "TOTAL   70035    68500    ..." or "TOTAL PSS: 70035 kB"
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "TOTAL" || f == "TOTAL" && i+1 < len(fields) {
					// 跳过 "TOTAL" 本身，下一个是 PSS Total
				}
			}
			// 简单方式：取 TOTAL 开头行的第2个字段（kB）
			if len(fields) >= 2 {
				// 找到纯数字字段
				for _, f := range fields[1:] {
					if v, err := strconv.ParseInt(f, 10, 64); err == nil {
						sample.Memory = float64(v) / 1024 // kB -> MB
						break
					}
				}
			}
		}
		if strings.HasPrefix(line, "TOTAL SWAP") && len(strings.Fields(line)) >= 3 {
			fields := strings.Fields(line)
			if v, err := strconv.ParseInt(fields[len(fields)-1], 10, 64); err == nil {
				_ = v // swap 数据暂存
			}
		}
	}
}

// collectGPUAdreno Adreno GPU 采集
func (ac *AndroidCollector) collectGPUAdreno(sample *model.Sample) {
	// GPU busy %
	if out, err := ac.adb("shell", "cat", "/sys/class/kgsl/kgsl-3d0/gpubusy"); err == nil {
		// "active_ticks total_ticks"
		fields := strings.Fields(out)
		if len(fields) >= 2 {
			busy, _ := strconv.ParseFloat(fields[0], 64)
			total, _ := strconv.ParseFloat(fields[1], 64)
			if total > 0 {
				sample.GPUUtil = busy / total * 100
			}
		}
	}
	// GPU clock
	if out, err := ac.adb("shell", "cat", "/sys/class/kgsl/kgsl-3d0/gpuclk"); err == nil {
		if v, err := strconv.ParseFloat(out, 64); err == nil {
			sample.GPUClock = v / 1e6 // Hz -> MHz
		}
	}
	// GPU 温度 — 从 thermal zone 找
	ac.collectGPUTempFromThermal(sample)
}

// collectGPUMali Mali GPU 采集
func (ac *AndroidCollector) collectGPUMali(sample *model.Sample) {
	if out, err := ac.adb("shell", "cat", "/sys/class/misc/mali0/device/utilization"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.GPUUtil = v
		}
	}
	if out, err := ac.adb("shell", "cat", "/sys/kernel/gpu/gpu_clock"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.GPUClock = v
		}
	}
	ac.collectGPUTempFromThermal(sample)
}

// collectGPUTempFromThermal 从 thermal zone 读 GPU 温度
func (ac *AndroidCollector) collectGPUTempFromThermal(sample *model.Sample) {
	// 遍历 thermal zones 找 gpu 相关的
	for i := 0; i < 30; i++ {
		zoneType, _ := ac.adb("shell", "cat", fmt.Sprintf("/sys/class/thermal/thermal_zone%d/type", i))
		if strings.Contains(strings.ToLower(zoneType), "gpu") {
			if temp, err := ac.adb("shell", "cat", fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp", i)); err == nil {
				if v, err := strconv.ParseFloat(strings.TrimSpace(temp), 64); err == nil {
					sample.GPUTemp = v / 1000 // millidegrees -> ℃
				}
			}
			return
		}
	}
}

// collectFPS 通过 SurfaceFlinger 采集 FPS
func (ac *AndroidCollector) collectFPS(sample *model.Sample) {
	if ac.surfaceName == "" {
		return
	}
	out, err := ac.adb("shell", "dumpsys", "SurfaceFlinger", "--latency", ac.surfaceName)
	if err != nil {
		// surface 名可能变了，重新查找
		ac.surfaceName = ac.findSurfaceName()
		return
	}

	lines := strings.Split(out, "\n")
	// 第1行是 refresh period (ns)
	// 后面是 128 行 frame data: desired_present  vsync  frame_ready
	var frameTimestamps []int64
	for _, line := range lines[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 {
			// 取第3列 frame_ready，非0表示有效帧
			if ready, err := strconv.ParseInt(fields[2], 10, 64); err == nil && ready > 0 {
				frameTimestamps = append(frameTimestamps, ready)
			}
		}
	}

	if len(frameTimestamps) < 2 {
		return
	}

	// 计算最近1秒内的帧数
	latest := frameTimestamps[len(frameTimestamps)-1]
	oneSecAgo := latest - 1_000_000_000 // 1秒 in ns
	count := 0
	for i := len(frameTimestamps) - 1; i >= 0; i-- {
		if frameTimestamps[i] >= oneSecAgo {
			count++
		} else {
			break
		}
	}

	if count > 0 {
		sample.FPS = float64(count)
	}

	// 计算最近两帧的帧时间
	if len(frameTimestamps) >= 2 {
		delta := frameTimestamps[len(frameTimestamps)-1] - frameTimestamps[len(frameTimestamps)-2]
		if delta > 0 {
			sample.FrameTime = float64(delta) / 1_000_000 // ns -> ms
		}
	}

	// Jank 检测: 帧时间 > 前一帧 * 2
	for i := 2; i < len(frameTimestamps) && i < len(frameTimestamps); i++ {
		prev := frameTimestamps[i-1] - frameTimestamps[i-2]
		curr := frameTimestamps[i] - frameTimestamps[i-1]
		if prev > 0 && curr > prev*2 {
			sample.JankCount++
		}
	}
}

// collectDiskIO 通过 /proc/pid/io 采集磁盘 I/O
func (ac *AndroidCollector) collectDiskIO(sample *model.Sample, now int64) {
	out, err := ac.adb("shell", "cat", fmt.Sprintf("/proc/%d/io", ac.pid))
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ": ", 2)
		if len(parts) < 2 {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		switch parts[0] {
		case "read_bytes":
			sample.DiskReadBytes = val
		case "write_bytes":
			sample.DiskWriteBytes = val
		case "rchar":
			// 逻辑读（含cache）
		case "wchar":
			// 逻辑写
		}
	}

	// 计算速率
	if ac.prevTimestamp > 0 {
		dt := float64(now - ac.prevTimestamp)
		if dt > 0 {
			sample.DiskReadBPS = float64(sample.DiskReadBytes-ac.prevDiskRead) / dt
			sample.DiskWriteBPS = float64(sample.DiskWriteBytes-ac.prevDiskWrite) / dt
		}
	}
	ac.prevDiskRead = sample.DiskReadBytes
	ac.prevDiskWrite = sample.DiskWriteBytes
}

// collectNetwork 通过 dumpsys netstats 采集网络
func (ac *AndroidCollector) collectNetwork(sample *model.Sample, now int64) {
	// 获取 UID
	uidOut, err := ac.adb("shell", "dumpsys", "package", ac.packageName)
	if err != nil {
		return
	}
	var uid string
	for _, line := range strings.Split(uidOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "userId=") {
			uid = strings.TrimPrefix(line, "userId=")
			break
		}
	}
	if uid == "" {
		return
	}

	// 从 netstats 获取
	out, err := ac.adb("shell", "dumpsys", "netstats", "detail")
	if err != nil {
		return
	}

	var totalRx, totalTx int64
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "uid="+uid) {
			// 找后面的 rxBytes / txBytes
			if rxMatch := regexp.MustCompile(`rxBytes=(\d+)`).FindStringSubmatch(line); len(rxMatch) > 1 {
				if v, err := strconv.ParseInt(rxMatch[1], 10, 64); err == nil {
					totalRx += v
				}
			}
			if txMatch := regexp.MustCompile(`txBytes=(\d+)`).FindStringSubmatch(line); len(txMatch) > 1 {
				if v, err := strconv.ParseInt(txMatch[1], 10, 64); err == nil {
					totalTx += v
				}
			}
		}
	}

	sample.NetBytesRecv = totalRx
	sample.NetBytesSent = totalTx

	if ac.prevTimestamp > 0 {
		dt := float64(now - ac.prevTimestamp)
		if dt > 0 {
			sample.NetRecvBPS = float64(totalRx-ac.prevNetRecv) / dt
			sample.NetSentBPS = float64(totalTx-ac.prevNetSent) / dt
		}
	}
	ac.prevNetRecv = totalRx
	ac.prevNetSent = totalTx
	ac.prevTimestamp = now
}

// collectBattery 采集电池信息
func (ac *AndroidCollector) collectBattery(sample *model.Sample) {
	// 电量
	if out, err := ac.adb("shell", "cat", "/sys/class/power_supply/battery/capacity"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.BatteryLevel = v
		}
	}
	// 电池温度
	if out, err := ac.adb("shell", "cat", "/sys/class/power_supply/battery/temp"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.BatteryTemp = v / 10 // decidegrees -> ℃
		}
	}
	// 电压
	if out, err := ac.adb("shell", "cat", "/sys/class/power_supply/battery/voltage_now"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.BatteryVoltage = v / 1000 // microvolts -> mV
		}
	}
	// 电流
	if out, err := ac.adb("shell", "cat", "/sys/class/power_supply/battery/current_now"); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			sample.BatteryCurrent = v / 1000 // microamps -> mA
			// 功率 mW = |电压mV| * |电流mA| / 1000
			sample.BatteryPower = abs64(sample.BatteryVoltage) * abs64(sample.BatteryCurrent) / 1000
		}
	}
}

// collectThermal 采集温度
func (ac *AndroidCollector) collectThermal(sample *model.Sample) {
	// CPU 温度
	for i := 0; i < 30; i++ {
		zoneType, _ := ac.adb("shell", "cat", fmt.Sprintf("/sys/class/thermal/thermal_zone%d/type", i))
		zt := strings.ToLower(zoneType)
		if strings.Contains(zt, "cpu") && !strings.Contains(zt, "gpu") {
			if temp, err := ac.adb("shell", "cat", fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp", i)); err == nil {
				if v, err := strconv.ParseFloat(strings.TrimSpace(temp), 64); err == nil {
					sample.CPUTemp = v / 1000
					return
				}
			}
		}
	}
}

// ListAdbDevices 列出所有连接的 ADB 设备
func ListAdbDevices() ([]string, error) {
	cmd := exec.Command("adb", "devices")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}

	var devices []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			devices = append(devices, fields[0])
		}
	}
	return devices, nil
}

// ListAndroidPackages 列出设备上已安装的包（第三方）
func ListAndroidPackages(deviceID string) ([]string, error) {
	args := []string{"shell", "pm", "list", "packages", "-3"}
	if deviceID != "" {
		args = append([]string{"-s", deviceID}, args...)
	}
	cmd := exec.Command("adb", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			packages = append(packages, strings.TrimPrefix(line, "package:"))
		}
	}
	return packages, nil
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
