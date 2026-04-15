package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gameperf/internal/db"
	"gameperf/internal/model"
)

// IOSCollector 通过 pymobiledevice3/tidevice 采集 iOS 设备性能数据
// 要求：macOS 宿主机 + pymobiledevice3 已安装 (pip install pymobiledevice3)
type IOSCollector struct {
	database  *db.DB
	interval  time.Duration
	bundleID  string
	deviceUDID string
	appName   string
	pid       int
	stopCh    chan struct{}

	// 上一帧数据
	prevDiskRead  int64
	prevDiskWrite int64
	prevTimestamp int64
}

func NewIOSCollector(database *db.DB, bundleID, deviceUDID string, interval time.Duration) *IOSCollector {
	return &IOSCollector{
		database:   database,
		interval:   interval,
		bundleID:   bundleID,
		deviceUDID: deviceUDID,
		stopCh:     make(chan struct{}),
	}
}

// run 执行 pymobiledevice3 命令
func (ic *IOSCollector) run(args ...string) (string, error) {
	fullArgs := args
	if ic.deviceUDID != "" {
		// 在 usbmux 相关命令中加 --udid
		hasUDID := false
		for _, a := range args {
			if a == "--udid" {
				hasUDID = true
				break
			}
		}
		if !hasUDID {
			// 对 DVT 命令，用 --tunnel 或 --rsd；简单起见先试直接跑
		}
	}
	cmd := exec.Command("pymobiledevice3", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("pymobiledevice3 %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// findAppName 从 bundle ID 推断 app name（取最后一个 component）
func (ic *IOSCollector) findAppName() string {
	parts := strings.Split(ic.bundleID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ic.bundleID
}

// getProcessList 获取进程列表
func (ic *IOSCollector) getProcessList() ([]map[string]interface{}, error) {
	out, err := ic.run("developer", "dvt", "proclist")
	if err != nil {
		return nil, err
	}
	var processes []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &processes); err != nil {
		return nil, fmt.Errorf("parse proclist: %w", err)
	}
	return processes, nil
}

// findPID 查找目标 App 的 PID
func (ic *IOSCollector) findPID() (int, error) {
	processes, err := ic.getProcessList()
	if err != nil {
		return 0, err
	}
	for _, p := range processes {
		if bundleID, ok := p["bundleID"].(string); ok && bundleID == ic.bundleID {
			if pid, ok := p["pid"].(float64); ok {
				return int(pid), nil
			}
		}
		// 也可以匹配 realAppName
		if realAppName, ok := p["realAppName"].(string); ok && realAppName == ic.bundleID {
			if pid, ok := p["pid"].(float64); ok {
				return int(pid), nil
			}
		}
	}
	return 0, fmt.Errorf("process %s not found (app may not be running)", ic.bundleID)
}

// collectSystemInfo 采集 iOS 设备信息
func (ic *IOSCollector) collectSystemInfo() *model.SystemInfo {
	info := &model.SystemInfo{OS: "ios", Arch: "arm64"}

	// 设备信息通过 usbmux list 获取
	out, err := ic.run("usbmux", "list")
	if err == nil {
		var devices []map[string]interface{}
		if json.Unmarshal([]byte(out), &devices) == nil {
			for _, d := range devices {
				if udid, ok := d["Identifier"].(string); ok && udid == ic.deviceUDID {
					if name, ok := d["DeviceName"].(string); ok {
						info.DeviceModel = name
					}
				if ver, ok := d["ProductVersion"].(string); ok {
					info.OSVersion = ver
				}
					if product, ok := d["ProductType"].(string); ok {
						if info.DeviceModel == "" {
							info.DeviceModel = product
						}
					}
				}
			}
		}
	}

	// CPU 核数从 sysmon system 获取
	if out, err := ic.run("developer", "dvt", "sysmon", "system", "single"); err == nil {
		parseSysmonSystem(out, info)
	}

	return info
}

// Start 启动采集
func (ic *IOSCollector) Start(sessionID string) error {
	session, err := ic.database.GetSession(sessionID)
	if err != nil {
		return err
	}

	ic.appName = ic.findAppName()

	// 查找 PID
	pid, err := ic.findPID()
	if err != nil {
		return fmt.Errorf("iOS: %w", err)
	}
	ic.pid = pid
	fmt.Printf("[ios] 找到进程 PID=%d bundle=%s\n", pid, ic.bundleID)

	// 保存系统信息
	sysInfo := ic.collectSystemInfo()
	sysInfo.SessionID = sessionID
	if err := ic.database.SaveSystemInfo(sysInfo); err != nil {
		fmt.Printf("[ios] 保存系统信息失败: %v\n", err)
	}

	startUnix := session.StartTime.Unix()
	ticker := time.NewTicker(ic.interval)
	defer ticker.Stop()

	fmt.Printf("[ios] 开始采集 session=%s bundle=%s pid=%d interval=%v\n", sessionID, ic.bundleID, pid, ic.interval)

	for {
		select {
		case <-ic.stopCh:
			fmt.Printf("[ios] 停止采集 session=%s\n", sessionID)
			return nil
		case t := <-ticker.C:
			sample := ic.collect(t.Unix(), startUnix)
			sample.SessionID = sessionID
			if err := ic.database.InsertSample(sample); err != nil {
				fmt.Printf("[ios] 写入失败: %v\n", err)
			}
			// 刷新 PID
			if newPID, err := ic.findPID(); err == nil && newPID != ic.pid {
				fmt.Printf("[ios] PID 变化: %d -> %d\n", ic.pid, newPID)
				ic.pid = newPID
			}
		}
	}
}

func (ic *IOSCollector) Stop() {
	close(ic.stopCh)
}

func (ic *IOSCollector) collect(now, startUnix int64) *model.Sample {
	sample := &model.Sample{
		Timestamp: now,
		Elapsed:   float64(now - startUnix),
	}

	// === CPU / 内存 / 磁盘 / 线程 — sysmon process single ===
	ic.collectProcessSnapshot(sample, now)

	// === 电池 ===
	ic.collectBattery(sample)

	// === FPS — 尝试 tidevice perf 或 sysmon graphics ===
	ic.collectFPS(sample)

	// === 网络 — sysmon system ===
	ic.collectNetwork(sample, now)

	// === 温度 ===
	ic.collectThermal(sample)

	return sample
}

// sysmonProcessData pymobiledevice3 sysmon process single 输出结构
type sysmonProcessData struct {
	Pid               int     `json:"pid"`
	Name              string  `json:"name"`
	CpuUsage          float64 `json:"cpuUsage"`
	PhysFootprint     int64   `json:"physFootprint"`
	MemResidentSize   int64   `json:"memResidentSize"`
	VirtualSize       int64   `json:"virtualSize"`
	ThreadCount       int     `json:"threadCount"`
	DiskBytesRead     int64   `json:"diskBytesRead"`
	DiskBytesWritten  int64   `json:"diskBytesWritten"`
	DiskIOReadOps     int64   `json:"diskIOReadOps"`
	DiskIOWriteOps    int64   `json:"diskIOWriteOps"`
	NetBytesIn        int64   `json:"netBytesIn"`
	NetBytesOut       int64   `json:"netBytesOut"`
	CtxSwitches       int64   `json:"ctxSwitches"`
	PageFaults        int64   `json:"pageFaults"`
}

// collectProcessSnapshot 通过 sysmon process single 采集进程级数据
func (ic *IOSCollector) collectProcessSnapshot(sample *model.Sample, now int64) {
	out, err := ic.run("developer", "dvt", "sysmon", "process", "single", "-f", fmt.Sprintf("name=%s", ic.appName))
	if err != nil {
		// fallback: 按 PID 过滤
		out, err = ic.run("developer", "dvt", "sysmon", "process", "single", "-f", fmt.Sprintf("pid=%d", ic.pid))
		if err != nil {
			return
		}
	}

	// 输出可能是 JSON 数组
	var processData []sysmonProcessData
	if err := json.Unmarshal([]byte(out), &processData); err != nil {
		// 尝试单个对象
		var single sysmonProcessData
		if err2 := json.Unmarshal([]byte(out), &single); err2 != nil {
			return
		}
		processData = []sysmonProcessData{single}
	}

	// 找到目标进程
	var target *sysmonProcessData
	for i := range processData {
		if processData[i].Pid == ic.pid {
			target = &processData[i]
			break
		}
	}
	if target == nil && len(processData) > 0 {
		target = &processData[0]
	}
	if target == nil {
		return
	}

	// CPU
	sample.CPU = target.CpuUsage

	// 内存
	sample.Memory = float64(target.PhysFootprint) / 1024 / 1024 // bytes -> MB
	sample.MemoryVMS = float64(target.VirtualSize) / 1024 / 1024
	sample.PageFaults = target.PageFaults

	// 线程
	sample.Threads = int32(target.ThreadCount)

	// 磁盘 I/O
	sample.DiskReadBytes = target.DiskBytesRead
	sample.DiskWriteBytes = target.DiskBytesWritten
	sample.DiskReadOps = target.DiskIOReadOps
	sample.DiskWriteOps = target.DiskIOWriteOps

	if ic.prevTimestamp > 0 {
		dt := float64(now - ic.prevTimestamp)
		if dt > 0 {
			sample.DiskReadBPS = float64(target.DiskBytesRead-ic.prevDiskRead) / dt
			sample.DiskWriteBPS = float64(target.DiskBytesWritten-ic.prevDiskWrite) / dt
		}
	}
	ic.prevDiskRead = target.DiskBytesRead
	ic.prevDiskWrite = target.DiskBytesWritten

	// 网络（进程级）
	sample.NetBytesRecv = target.NetBytesIn
	sample.NetBytesSent = target.NetBytesOut
}

// collectBattery 采集电池
func (ic *IOSCollector) collectBattery(sample *model.Sample) {
	// pymobiledevice3 diagnostics battery monitor — 输出1行后退出
	out, err := ic.run("diagnostics", "battery", "monitor")
	if err != nil {
		return
	}

	// 输出格式: Python dict (类似 JSON 但可能有不标准格式)
	// {"CurrentCapacity": 85, "Voltage": 4256, "Temperature": 320, ...}
	// 尝试解析
	data := parsePyDict(out)

	if v, ok := data["CurrentCapacity"]; ok {
		if f, err := parseFloat(v); err == nil {
			sample.BatteryLevel = f
		}
	}
	if v, ok := data["Voltage"]; ok {
		if f, err := parseFloat(v); err == nil {
			sample.BatteryVoltage = f // mV
		}
	}
	if v, ok := data["Temperature"]; ok {
		if f, err := parseFloat(v); err == nil {
			sample.BatteryTemp = f / 100 // centidegrees -> ℃
		}
	}
	// 电流 — 估算功率
	if sample.BatteryVoltage > 0 {
		// iOS 不直接暴露电流，用温度和电量变化间接估算
		// 或者从 IOReg 读取 AppleSMC 下的 Amperage（需要额外命令）
	}
}

// collectFPS 采集 FPS — 通过 tidevice perf (如果可用)
func (ic *IOSCollector) collectFPS(sample *model.Sample) {
	// TODO: iOS FPS collection via tidevice perf or pymobiledevice3 DVT instruments
	// is not yet implemented. The previous code had dead/orphaned logic that never
	// actually captured FPS data. To implement this properly:
	//
	// 1. Launch `tidevice -u <udid> perf -B <bundleID> --json` as a persistent subprocess
	// 2. Parse its JSON output line-by-line for the "FPS" field
	// 3. Or use pymobiledevice3 developer dvt sysmon with a graphics template
	//
	// For now, FPS must be injected manually via API or the InjectFrameData helper.
}

// collectNetwork 系统级网络统计
func (ic *IOSCollector) collectNetwork(sample *model.Sample, now int64) {
	// sysmon system single 获取系统级网络
	out, err := ic.run("developer", "dvt", "sysmon", "system", "single")
	if err != nil {
		return
	}
	data := parseSysmonSystemKVs(out)

	if v, ok := data["netBytesIn"]; ok {
		if f, err := parseFloat(v); err == nil {
			// 系统级，仅作参考
			_ = f
		}
	}
}

// collectThermal 采集温度
func (ic *IOSCollector) collectThermal(sample *model.Sample) {
	// pymobiledevice3 没有直接温度命令
	// 可以通过 diagnostics 读取
	out, err := ic.run("diagnostics", "diagnostics", "query", "BatteryCurrentTemperature")
	if err == nil {
		_ = out
	}
}

// parsePyDict 解析 Python dict 格式字符串
func parsePyDict(s string) map[string]string {
	result := make(map[string]string)
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	// 按 , 分割（不处理嵌套）
	re := regexp.MustCompile(`'([^']+)'\s*:\s*'?([^',}]+)'?`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			result[m[1]] = strings.TrimSpace(m[2])
		}
	}
	// 也尝试 JSON 格式
	var jsonMap map[string]interface{}
	if json.Unmarshal([]byte(s), &jsonMap) == nil {
		for k, v := range jsonMap {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

// parseSysmonSystem 解析 sysmon system 输出
func parseSysmonSystem(out string, info *model.SystemInfo) {
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "physMemSize:") {
			val := strings.TrimPrefix(line, "physMemSize:")
			if v, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
				info.TotalMemory = v / 1024 / 1024 // bytes -> MB
			}
		}
	}
}

// parseSysmonSystemKVs 解析 key=value 格式
func parseSysmonSystemKVs(out string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			result[key] = val
		}
	}
	return result
}

// --- 设备/应用列表 API ---

// ListIOSDevices 列出通过 USB 连接的 iOS 设备
func ListIOSDevices() ([]map[string]interface{}, error) {
	cmd := exec.Command("pymobiledevice3", "usbmux", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pymobiledevice3 usbmux list: %w", err)
	}
	var devices []map[string]interface{}
	if err := json.Unmarshal(out, &devices); err != nil {
		return nil, fmt.Errorf("parse device list: %w", err)
	}
	return devices, nil
}

// ListIOSApps 列出设备上已安装的用户 App
func ListIOSApps(deviceUDID string) ([]string, error) {
	args := []string{"apps", "list", "--type", "User"}
	if deviceUDID != "" {
		args = append([]string{"--udid", deviceUDID}, args...)
	}
	cmd := exec.Command("pymobiledevice3", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}

	// 输出是 JSON dict: {"com.example.app": {"CFBundleIdentifier": ..., ...}}
	var appsMap map[string]map[string]interface{}
	if err := json.Unmarshal(out, &appsMap); err != nil {
		return nil, fmt.Errorf("parse apps: %w", err)
	}

	var bundleIDs []string
	for id, info := range appsMap {
		name := id
		if displayName, ok := info["CFBundleDisplayName"].(string); ok && displayName != "" {
			name = fmt.Sprintf("%s (%s)", displayName, id)
		}
		bundleIDs = append(bundleIDs, name)
	}
	return bundleIDs, nil
}

// CheckIOSPrerequisites 检查 iOS 采集依赖是否就绪
func CheckIOSPrerequisites() (string, error) {
	// 检查 pymobiledevice3
	path, err := exec.LookPath("pymobiledevice3")
	if err != nil {
		return "", fmt.Errorf("pymobiledevice3 not found. Install: pip3 install pymobiledevice3")
	}

	// 检查 macOS
	if _, err := exec.LookPath("system_profiler"); err != nil {
		return path, fmt.Errorf("iOS collection requires macOS")
	}

	return path, nil
}
