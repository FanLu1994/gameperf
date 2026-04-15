package collector

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"gameperf/internal/db"
	"gameperf/internal/model"
)

// Collector 进程性能采集器
type Collector struct {
	db       *db.DB
	interval time.Duration
	pid      int32
	proc     *process.Process
	stopCh   chan struct{}
}

func New(database *db.DB, pid int32, interval time.Duration) (*Collector, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("find process pid=%d: %w", pid, err)
	}
	return &Collector{
		db:       database,
		interval: interval,
		pid:      pid,
		proc:     proc,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start 开始采集，阻塞直到 Stop
func (c *Collector) Start(sessionID string) error {
	session, err := c.db.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	startUnix := session.StartTime.Unix()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	fmt.Printf("[collector] 开始采集 session=%s pid=%d interval=%v\n", sessionID, c.pid, c.interval)

	for {
		select {
		case <-c.stopCh:
			fmt.Printf("[collector] 停止采集 session=%s\n", sessionID)
			return nil
		case t := <-ticker.C:
			sample := c.collect(t.Unix(), startUnix)
			sample.SessionID = sessionID
			if err := c.db.InsertSample(sample); err != nil {
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

	// CPU%
	if cpuPercent, err := c.proc.CPPercent(0); err == nil {
		sample.CPU = cpuPercent
	}

	// 内存 MB
	if memInfo, err := c.proc.MemoryInfo(); err == nil {
		sample.Memory = float64(memInfo.RSS) / 1024 / 1024
	}

	// 线程数
	if numThreads, err := c.proc.NumThreads(); err == nil {
		sample.Threads = numThreads
	}

	// FPS 和 GPU 显存 — 需要外部数据源或扩展
	// 默认 0，可通过 API 手动注入

	return sample
}
