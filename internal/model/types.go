package model

import "time"

// Session 一次采集会话
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`       // 标签，如 "v1.0优化前"
	Process   string    `json:"process"`    // 目标进程名
	Status    string    `json:"status"`     // running / stopped
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// Sample 一条采样数据
type Sample struct {
	ID        int64   `json:"id"`
	SessionID string `json:"session_id"`
	Timestamp int64   `json:"timestamp"`  // unix秒
	Elapsed   float64 `json:"elapsed"`    // 距采集开始的秒数
	CPU       float64 `json:"cpu"`        // CPU%
	Memory    float64 `json:"memory"`     // MB
	FPS       float64 `json:"fps"`        // 帧率(如有)
	GPUMem    float64 `json:"gpu_mem"`    // GPU显存MB(如有)
	Threads   int32   `json:"threads"`    // 线程数
}

// SessionSummary 会话统计摘要
type SessionSummary struct {
	Session      Session  `json:"session"`
	Duration     float64  `json:"duration"`      // 秒
	SampleCount  int      `json:"sample_count"`
	AvgCPU       float64  `json:"avg_cpu"`
	MaxCPU       float64  `json:"max_cpu"`
	MinCPU       float64  `json:"min_cpu"`
	P95CPU       float64  `json:"p95_cpu"`
	AvgMemory    float64  `json:"avg_memory"`
	MaxMemory    float64  `json:"max_memory"`
	MinMemory    float64  `json:"min_memory"`
	P95Memory    float64  `json:"p95_memory"`
	AvgFPS       float64  `json:"avg_fps"`
	MinFPS       float64  `json:"min_fps"`
	P5FPS        float64  `json:"p5_fps"`
}

// CompareResult 对比结果
type CompareResult struct {
	Summaries []SessionSummary `json:"summaries"`
}
