package db

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	_ "github.com/mattn/go-sqlite3"

	"gameperf/internal/model"
)

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			process TEXT NOT NULL DEFAULT '',
			pid INTEGER NOT NULL DEFAULT 0,
			platform TEXT NOT NULL DEFAULT 'windows',
			package_name TEXT NOT NULL DEFAULT '',
			device_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'running',
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			interval TEXT NOT NULL DEFAULT '1s',
			tags TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			elapsed REAL NOT NULL,
			cpu REAL NOT NULL DEFAULT 0,
			cpu_cores TEXT NOT NULL DEFAULT '',
			cpu_freq REAL NOT NULL DEFAULT 0,
			cpu_time_user REAL NOT NULL DEFAULT 0,
			cpu_time_system REAL NOT NULL DEFAULT 0,
			ctx_switches INTEGER NOT NULL DEFAULT 0,
			memory REAL NOT NULL DEFAULT 0,
			memory_vms REAL NOT NULL DEFAULT 0,
			page_faults INTEGER NOT NULL DEFAULT 0,
			handle_count INTEGER NOT NULL DEFAULT 0,
			gdi_objects INTEGER NOT NULL DEFAULT 0,
			gpu_util REAL NOT NULL DEFAULT 0,
			gpu_mem REAL NOT NULL DEFAULT 0,
			gpu_mem_total REAL NOT NULL DEFAULT 0,
			gpu_temp REAL NOT NULL DEFAULT 0,
			gpu_clock REAL NOT NULL DEFAULT 0,
			gpu_mem_clock REAL NOT NULL DEFAULT 0,
			gpu_power REAL NOT NULL DEFAULT 0,
			gpu_fan_speed REAL NOT NULL DEFAULT 0,
			disk_read_bytes INTEGER NOT NULL DEFAULT 0,
			disk_write_bytes INTEGER NOT NULL DEFAULT 0,
			disk_read_ops INTEGER NOT NULL DEFAULT 0,
			disk_write_ops INTEGER NOT NULL DEFAULT 0,
			disk_read_bps REAL NOT NULL DEFAULT 0,
			disk_write_bps REAL NOT NULL DEFAULT 0,
			net_bytes_sent INTEGER NOT NULL DEFAULT 0,
			net_bytes_recv INTEGER NOT NULL DEFAULT 0,
			net_conn_count INTEGER NOT NULL DEFAULT 0,
			net_sent_bps REAL NOT NULL DEFAULT 0,
			net_recv_bps REAL NOT NULL DEFAULT 0,
			fps REAL NOT NULL DEFAULT 0,
			frame_time REAL NOT NULL DEFAULT 0,
			frame_time_min REAL NOT NULL DEFAULT 0,
			frame_time_max REAL NOT NULL DEFAULT 0,
			fps_1_low REAL NOT NULL DEFAULT 0,
			fps_01_low REAL NOT NULL DEFAULT 0,
			jank_count INTEGER NOT NULL DEFAULT 0,
			stutter_rate REAL NOT NULL DEFAULT 0,
			threads INTEGER NOT NULL DEFAULT 0,
			battery_level REAL NOT NULL DEFAULT 0,
			battery_temp REAL NOT NULL DEFAULT 0,
			battery_power REAL NOT NULL DEFAULT 0,
			battery_voltage REAL NOT NULL DEFAULT 0,
			battery_current REAL NOT NULL DEFAULT 0,
			cpu_temp REAL NOT NULL DEFAULT 0,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);
		CREATE INDEX IF NOT EXISTS idx_samples_session ON samples(session_id);
		CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(session_id, timestamp);
		CREATE TABLE IF NOT EXISTS system_info (
			session_id TEXT PRIMARY KEY,
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			cpu_model TEXT NOT NULL DEFAULT '',
			cpu_cores INTEGER NOT NULL DEFAULT 0,
			total_memory INTEGER NOT NULL DEFAULT 0,
			gpu_name TEXT NOT NULL DEFAULT '',
			gpu_driver TEXT NOT NULL DEFAULT '',
			gpu_vram INTEGER NOT NULL DEFAULT 0,
			device_model TEXT NOT NULL DEFAULT '',
			android_version TEXT NOT NULL DEFAULT '',
			android_api INTEGER NOT NULL DEFAULT 0
		);
	`)
	return err
}

// --- Session CRUD ---

func (d *DB) CreateSession(s *model.Session) error {
	_, err := d.conn.Exec(
		`INSERT INTO sessions (id, name, process, pid, platform, package_name, device_id, status, start_time, interval, tags)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Process, s.PID, s.Platform, s.Package, s.DeviceID, s.Status, s.StartTime, s.Interval, s.Tags,
	)
	return err
}

func (d *DB) ListSessions() ([]model.Session, error) {
	rows, err := d.conn.Query(`SELECT id, name, process, pid, platform, package_name, device_id, status, start_time, end_time, interval, tags FROM sessions ORDER BY start_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []model.Session
	for rows.Next() {
		var s model.Session
		var endTime sql.NullTime
		if err := rows.Scan(&s.ID, &s.Name, &s.Process, &s.PID, &s.Platform, &s.Package, &s.DeviceID, &s.Status, &s.StartTime, &endTime, &s.Interval, &s.Tags); err != nil {
			return nil, err
		}
		if endTime.Valid {
			s.EndTime = &endTime.Time
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (d *DB) GetSession(id string) (*model.Session, error) {
	var s model.Session
	var endTime sql.NullTime
	err := d.conn.QueryRow(
		`SELECT id, name, process, pid, platform, package_name, device_id, status, start_time, end_time, interval, tags FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.Process, &s.PID, &s.Platform, &s.Package, &s.DeviceID, &s.Status, &s.StartTime, &endTime, &s.Interval, &s.Tags)
	if err != nil {
		return nil, err
	}
	if endTime.Valid {
		s.EndTime = &endTime.Time
	}
	return &s, nil
}

func (d *DB) StopSession(id string) error {
	_, err := d.conn.Exec(`UPDATE sessions SET status = 'stopped', end_time = datetime('now') WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteSession(id string) error {
	d.conn.Exec(`DELETE FROM samples WHERE session_id = ?`, id)
	d.conn.Exec(`DELETE FROM system_info WHERE session_id = ?`, id)
	_, err := d.conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// --- SystemInfo ---

func (d *DB) SaveSystemInfo(info *model.SystemInfo) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO system_info (session_id, os, arch, cpu_model, cpu_cores, total_memory, gpu_name, gpu_driver, gpu_vram, device_model, android_version, android_api)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		info.SessionID, info.OS, info.Arch, info.CPUModel, info.CPUCores, info.TotalMemory, info.GPUName, info.GPUDriver, info.GPUVRAM,
		info.DeviceModel, info.AndroidVersion, info.AndroidAPI,
	)
	return err
}

func (d *DB) GetSystemInfo(sessionID string) (*model.SystemInfo, error) {
	var info model.SystemInfo
	err := d.conn.QueryRow(
		`SELECT session_id, os, arch, cpu_model, cpu_cores, total_memory, gpu_name, gpu_driver, gpu_vram, device_model, android_version, android_api FROM system_info WHERE session_id = ?`, sessionID,
	).Scan(&info.SessionID, &info.OS, &info.Arch, &info.CPUModel, &info.CPUCores, &info.TotalMemory, &info.GPUName, &info.GPUDriver, &info.GPUVRAM,
		&info.DeviceModel, &info.AndroidVersion, &info.AndroidAPI)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// --- Samples ---

func (d *DB) InsertSample(s *model.Sample) error {
	_, err := d.conn.Exec(`
		INSERT INTO samples (
			session_id, timestamp, elapsed,
			cpu, cpu_cores, cpu_freq, cpu_time_user, cpu_time_system, ctx_switches,
			memory, memory_vms, page_faults, handle_count, gdi_objects,
			gpu_util, gpu_mem, gpu_mem_total, gpu_temp, gpu_clock, gpu_mem_clock, gpu_power, gpu_fan_speed,
			disk_read_bytes, disk_write_bytes, disk_read_ops, disk_write_ops, disk_read_bps, disk_write_bps,
			net_bytes_sent, net_bytes_recv, net_conn_count, net_sent_bps, net_recv_bps,
			fps, frame_time, frame_time_min, frame_time_max, fps_1_low, fps_01_low, jank_count, stutter_rate,
			threads,
			battery_level, battery_temp, battery_power, battery_voltage, battery_current,
			cpu_temp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.Timestamp, s.Elapsed,
		s.CPU, s.CPUCores, s.CPUFreq, s.CPUTimeUser, s.CPUTimeSystem, s.CtxSwitches,
		s.Memory, s.MemoryVMS, s.PageFaults, s.HandleCount, s.GDIObjects,
		s.GPUUtil, s.GPUMem, s.GPUMemTotal, s.GPUTemp, s.GPUClock, s.GPUMemClock, s.GPUPower, s.GPUFanSpeed,
		s.DiskReadBytes, s.DiskWriteBytes, s.DiskReadOps, s.DiskWriteOps, s.DiskReadBPS, s.DiskWriteBPS,
		s.NetBytesSent, s.NetBytesRecv, s.NetConnCount, s.NetSentBPS, s.NetRecvBPS,
		s.FPS, s.FrameTime, s.FrameTimeMin, s.FrameTimeMax, s.FPS1Low, s.FPS01Low, s.JankCount, s.StutterRate,
		s.Threads,
		s.BatteryLevel, s.BatteryTemp, s.BatteryPower, s.BatteryVoltage, s.BatteryCurrent,
		s.CPUTemp,
	)
	return err
}

const sampleSelect = `
SELECT id, session_id, timestamp, elapsed,
	cpu, cpu_cores, cpu_freq, cpu_time_user, cpu_time_system, ctx_switches,
	memory, memory_vms, page_faults, handle_count, gdi_objects,
	gpu_util, gpu_mem, gpu_mem_total, gpu_temp, gpu_clock, gpu_mem_clock, gpu_power, gpu_fan_speed,
	disk_read_bytes, disk_write_bytes, disk_read_ops, disk_write_ops, disk_read_bps, disk_write_bps,
	net_bytes_sent, net_bytes_recv, net_conn_count, net_sent_bps, net_recv_bps,
	fps, frame_time, frame_time_min, frame_time_max, fps_1_low, fps_01_low, jank_count, stutter_rate,
	threads,
	battery_level, battery_temp, battery_power, battery_voltage, battery_current,
	cpu_temp
FROM samples`

func scanSample(rows *sql.Rows) (*model.Sample, error) {
	var s model.Sample
	err := rows.Scan(
		&s.ID, &s.SessionID, &s.Timestamp, &s.Elapsed,
		&s.CPU, &s.CPUCores, &s.CPUFreq, &s.CPUTimeUser, &s.CPUTimeSystem, &s.CtxSwitches,
		&s.Memory, &s.MemoryVMS, &s.PageFaults, &s.HandleCount, &s.GDIObjects,
		&s.GPUUtil, &s.GPUMem, &s.GPUMemTotal, &s.GPUTemp, &s.GPUClock, &s.GPUMemClock, &s.GPUPower, &s.GPUFanSpeed,
		&s.DiskReadBytes, &s.DiskWriteBytes, &s.DiskReadOps, &s.DiskWriteOps, &s.DiskReadBPS, &s.DiskWriteBPS,
		&s.NetBytesSent, &s.NetBytesRecv, &s.NetConnCount, &s.NetSentBPS, &s.NetRecvBPS,
		&s.FPS, &s.FrameTime, &s.FrameTimeMin, &s.FrameTimeMax, &s.FPS1Low, &s.FPS01Low, &s.JankCount, &s.StutterRate,
		&s.Threads,
		&s.BatteryLevel, &s.BatteryTemp, &s.BatteryPower, &s.BatteryVoltage, &s.BatteryCurrent,
		&s.CPUTemp,
	)
	return &s, err
}

func (d *DB) GetSamples(sessionID string) ([]model.Sample, error) {
	rows, err := d.conn.Query(sampleSelect+` WHERE session_id = ? ORDER BY timestamp`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var samples []model.Sample
	for rows.Next() {
		s, err := scanSample(rows)
		if err != nil {
			return nil, err
		}
		samples = append(samples, *s)
	}
	return samples, nil
}

// --- Summary ---

func (d *DB) GetSummary(sessionID string) (*model.SessionSummary, error) {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	samples, err := d.GetSamples(sessionID)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return &model.SessionSummary{Session: *session}, nil
	}
	summary := &model.SessionSummary{Session: *session, SampleCount: len(samples)}

	cpus := make([]float64, len(samples))
	mems := make([]float64, len(samples))
	var gpus, gpuTemps, gpuMems, gpuPowers, fpss, frameTimes []float64
	var diskReadBPS, diskWriteBPS, netSentBPS, netRecvBPS []float64
	var batteryPowers, batteryTemps, cpuTemps []float64
	minBattery := 100.0

	for i, s := range samples {
		cpus[i] = s.CPU
		mems[i] = s.Memory
		if s.GPUUtil > 0 { gpus = append(gpus, s.GPUUtil) }
		if s.GPUTemp > 0 { gpuTemps = append(gpuTemps, s.GPUTemp) }
		if s.GPUMem > 0 { gpuMems = append(gpuMems, s.GPUMem) }
		if s.GPUPower > 0 { gpuPowers = append(gpuPowers, s.GPUPower) }
		if s.FPS > 0 { fpss = append(fpss, s.FPS) }
		if s.FrameTime > 0 { frameTimes = append(frameTimes, s.FrameTime) }
		if s.DiskReadBPS > 0 { diskReadBPS = append(diskReadBPS, s.DiskReadBPS) }
		if s.DiskWriteBPS > 0 { diskWriteBPS = append(diskWriteBPS, s.DiskWriteBPS) }
		if s.NetSentBPS > 0 { netSentBPS = append(netSentBPS, s.NetSentBPS) }
		if s.NetRecvBPS > 0 { netRecvBPS = append(netRecvBPS, s.NetRecvBPS) }
		if s.BatteryPower > 0 { batteryPowers = append(batteryPowers, s.BatteryPower) }
		if s.BatteryTemp > 0 { batteryTemps = append(batteryTemps, s.BatteryTemp) }
		if s.CPUTemp > 0 { cpuTemps = append(cpuTemps, s.CPUTemp) }
		if s.BatteryLevel > 0 && s.BatteryLevel < minBattery { minBattery = s.BatteryLevel }
		if s.Threads > summary.MaxThreads { summary.MaxThreads = s.Threads }
		if s.HandleCount > summary.MaxHandleCount { summary.MaxHandleCount = s.HandleCount }
	}

	sort.Float64s(cpus)
	summary.AvgCPU = avg(cpus); summary.MinCPU = cpus[0]; summary.MaxCPU = cpus[len(cpus)-1]
	summary.P95CPU = percentile(cpus, 95); summary.P99CPU = percentile(cpus, 99)

	sort.Float64s(mems)
	summary.AvgMemory = avg(mems); summary.MinMemory = mems[0]; summary.MaxMemory = mems[len(mems)-1]
	summary.P95Memory = percentile(mems, 95); summary.P99Memory = percentile(mems, 99)

	if len(gpus) > 0 { sort.Float64s(gpus); summary.AvgGPU = avg(gpus); summary.MaxGPU = gpus[len(gpus)-1]; summary.P95GPU = percentile(gpus, 95) }
	if len(gpuTemps) > 0 { sort.Float64s(gpuTemps); summary.MaxGPUTemp = gpuTemps[len(gpuTemps)-1] }
	if len(gpuPowers) > 0 { summary.AvgGPUPower = avg(gpuPowers) }
	if len(gpuMems) > 0 { sort.Float64s(gpuMems); summary.MaxGPUMem = gpuMems[len(gpuMems)-1] }

	if len(fpss) > 0 {
		sort.Float64s(fpss)
		summary.AvgFPS = avg(fpss); summary.MinFPS = fpss[0]; summary.MaxFPS = fpss[len(fpss)-1]
		summary.P1FPS = percentile(fpss, 1); summary.P5FPS = percentile(fpss, 5); summary.P95FPS = percentile(fpss, 95)
		summary.FPSStability = 1.0 - (stddev(fpss) / avg(fpss))
		if summary.FPSStability < 0 { summary.FPSStability = 0 }
	}
	if len(frameTimes) > 0 {
		sort.Float64s(frameTimes)
		summary.AvgFrameTime = avg(frameTimes); summary.P95FrameTime = percentile(frameTimes, 95)
		summary.P99FrameTime = percentile(frameTimes, 99); summary.MaxFrameTime = frameTimes[len(frameTimes)-1]
	}
	summary.TotalJankCount = int32(sumField(samples, func(s model.Sample) float64 { return float64(s.JankCount) }))

	if len(diskReadBPS) > 0 { summary.AvgDiskReadBPS = avg(diskReadBPS); sort.Float64s(diskReadBPS); summary.MaxDiskReadBPS = diskReadBPS[len(diskReadBPS)-1] }
	if len(diskWriteBPS) > 0 { summary.AvgDiskWriteBPS = avg(diskWriteBPS); sort.Float64s(diskWriteBPS); summary.MaxDiskWriteBPS = diskWriteBPS[len(diskWriteBPS)-1] }
	if len(netSentBPS) > 0 { summary.AvgNetSentBPS = avg(netSentBPS); sort.Float64s(netSentBPS); summary.MaxNetSentBPS = netSentBPS[len(netSentBPS)-1] }
	if len(netRecvBPS) > 0 { summary.AvgNetRecvBPS = avg(netRecvBPS); sort.Float64s(netRecvBPS); summary.MaxNetRecvBPS = netRecvBPS[len(netRecvBPS)-1] }

	// 电池 & 温度
	if minBattery < 100 { summary.MinBatteryLevel = minBattery }
	if len(batteryTemps) > 0 { sort.Float64s(batteryTemps); summary.MaxBatteryTemp = batteryTemps[len(batteryTemps)-1] }
	if len(batteryPowers) > 0 { summary.AvgBatteryPower = avg(batteryPowers) }
	if len(cpuTemps) > 0 { sort.Float64s(cpuTemps); summary.MaxCPUTemp = cpuTemps[len(cpuTemps)-1] }

	if session.EndTime != nil {
		summary.Duration = session.EndTime.Sub(session.StartTime).Seconds()
	} else {
		summary.Duration = float64(samples[len(samples)-1].Elapsed)
	}
	return summary, nil
}

// GetFrameTimeAnalysis 帧时间分布分析（复用不变）
func (d *DB) GetFrameTimeAnalysis(sessionID string) (*model.FrameTimeAnalysis, error) {
	samples, err := d.GetSamples(sessionID)
	if err != nil {
		return nil, err
	}
	analysis := &model.FrameTimeAnalysis{SessionID: sessionID}
	var frameTimes []float64
	for _, s := range samples {
		if s.FrameTime > 0 { frameTimes = append(frameTimes, s.FrameTime) }
	}
	analysis.FrameCount = len(frameTimes)
	if len(frameTimes) == 0 { return analysis, nil }

	minFT, maxFT := frameTimes[0], frameTimes[0]
	for _, ft := range frameTimes {
		if ft < minFT { minFT = ft }
		if ft > maxFT { maxFT = ft }
	}
	bucketCount := 30
	bucketSize := (maxFT - minFT) / float64(bucketCount)
	if bucketSize == 0 { bucketSize = 1 }
	buckets := make([]model.Bucket, bucketCount)
	for i := 0; i < bucketCount; i++ {
		buckets[i] = model.Bucket{RangeStart: minFT + float64(i)*bucketSize, RangeEnd: minFT + float64(i+1)*bucketSize}
	}
	for _, ft := range frameTimes {
		idx := int((ft - minFT) / bucketSize)
		if idx >= bucketCount { idx = bucketCount - 1 }
		buckets[idx].Count++
	}
	analysis.Histogram = buckets

	var jankFrames []model.JankFrame
	var stutterSections []model.StutterSection
	inStutter := false
	stutterStart, stutterCount := 0.0, 0
	for i := 1; i < len(frameTimes); i++ {
		prev, curr := frameTimes[i-1], frameTimes[i]
		if prev > 0 && curr > prev*2 {
			severity := "jank"
			if curr > prev*3 { severity = "big_jank" }
			elapsed := 0.0
			if i < len(samples) { elapsed = samples[i].Elapsed }
			jankFrames = append(jankFrames, model.JankFrame{Elapsed: elapsed, FrameTime: curr, Severity: severity})
			if !inStutter { inStutter = true; stutterStart = elapsed; stutterCount = 1 } else { stutterCount++ }
		} else if inStutter {
			elapsed := 0.0
			if i < len(samples) { elapsed = samples[i].Elapsed }
			stutterSections = append(stutterSections, model.StutterSection{StartElapsed: stutterStart, EndElapsed: elapsed, Duration: elapsed - stutterStart, FrameCount: stutterCount})
			inStutter = false
		}
	}
	analysis.JankFrames = jankFrames
	analysis.StutterSections = stutterSections
	return analysis, nil
}

// --- Helpers ---

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 { return 0 }
	return sorted[int(float64(len(sorted)-1)*p/100)]
}
func avg(vals []float64) float64 {
	if len(vals) == 0 { return 0 }
	var sum float64
	for _, v := range vals { sum += v }
	return sum / float64(len(vals))
}
func stddev(vals []float64) float64 {
	if len(vals) < 2 { return 0 }
	mean := avg(vals)
	var sum float64
	for _, v := range vals { d := v - mean; sum += d*d }
	return math.Sqrt(sum / float64(len(vals)))
}
func sumField(samples []model.Sample, fn func(model.Sample) float64) float64 {
	var sum float64
	for _, s := range samples { sum += fn(s) }
	return sum
}
