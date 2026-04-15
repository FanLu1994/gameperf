package db

import (
	"os"
	"testing"
	"time"

	"gameperf/internal/model"
)

func TestCreateAndGetSession(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_test.db")

	s := &model.Session{
		ID:        "test-123",
		Name:      "test-session",
		Platform:  "windows",
		PID:       1234,
		Status:    "running",
		StartTime: time.Now(),
		Interval:  "1s",
	}
	if err := d.CreateSession(s); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := d.GetSession("test-123")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.Name != "test-session" {
		t.Errorf("name = %q, want %q", got.Name, "test-session")
	}
	if got.Platform != "windows" {
		t.Errorf("platform = %q, want %q", got.Platform, "windows")
	}
}

func TestListSessions(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_list_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_list_test.db")

	for i := 0; i < 3; i++ {
		s := &model.Session{
			ID:        "list-" + string(rune('A'+i)),
			Name:      "session-" + string(rune('A'+i)),
			Platform:  "android",
			Package:   "com.test.app",
			Status:    "running",
			StartTime: time.Now(),
		}
		if err := d.CreateSession(s); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	sessions, err := d.ListSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("got %d sessions, want 3", len(sessions))
	}
}

func TestInsertAndGetSamples(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_samples_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_samples_test.db")

	s := &model.Session{ID: "samp-1", Name: "test", Platform: "windows", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		sample := &model.Sample{
			SessionID:    "samp-1",
			Timestamp:    now + int64(i),
			Elapsed:      float64(i),
			CPU:          50.0 + float64(i),
			Memory:       100.0 + float64(i)*10,
			GPUUtil:      30.0,
			FPS:          60.0,
			FrameTime:    16.67,
			DiskReadBPS:  1024 * 1024,
			DiskWriteBPS: 512 * 1024,
			NetSentBPS:   2048,
			NetRecvBPS:   8192,
			Threads:      42,
		}
		if err := d.InsertSample(sample); err != nil {
			t.Fatalf("insert sample %d: %v", i, err)
		}
	}

	samples, err := d.GetSamples("samp-1", 0, 0)
	if err != nil {
		t.Fatalf("get samples: %v", err)
	}
	if len(samples) != 10 {
		t.Errorf("got %d samples, want 10", len(samples))
	}
	if samples[0].CPU != 50.0 {
		t.Errorf("first sample CPU = %f, want 50.0", samples[0].CPU)
	}
}

func TestGetSummary(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_summary_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_summary_test.db")

	s := &model.Session{ID: "sum-1", Name: "summary-test", Platform: "android", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	now := time.Now().Unix()
	cpuids := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for i, cpu := range cpuids {
		sample := &model.Sample{
			SessionID: "sum-1",
			Timestamp: now + int64(i),
			Elapsed:   float64(i),
			CPU:       cpu,
			Memory:    200.0,
			FPS:       60.0,
		}
		d.InsertSample(sample)
	}

	summary, err := d.GetSummary("sum-1")
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if summary.SampleCount != 10 {
		t.Errorf("sample_count = %d, want 10", summary.SampleCount)
	}
	if summary.AvgCPU != 55.0 {
		t.Errorf("avg_cpu = %f, want 55.0", summary.AvgCPU)
	}
	if summary.MaxCPU != 100.0 {
		t.Errorf("max_cpu = %f, want 100.0", summary.MaxCPU)
	}
	if summary.MinCPU != 10.0 {
		t.Errorf("min_cpu = %f, want 10.0", summary.MinCPU)
	}
	if summary.P95CPU < 90.0 {
		t.Errorf("p95_cpu = %f, want >= 90.0", summary.P95CPU)
	}
	if summary.AvgFPS != 60.0 {
		t.Errorf("avg_fps = %f, want 60.0", summary.AvgFPS)
	}
}

func TestDeleteSessionTransaction(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_delete_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_delete_test.db")

	s := &model.Session{ID: "del-1", Name: "delete-test", Platform: "windows", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	sample := &model.Sample{SessionID: "del-1", Timestamp: time.Now().Unix(), Elapsed: 1.0, CPU: 50.0}
	d.InsertSample(sample)

	if err := d.DeleteSession("del-1"); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	_, err = d.GetSession("del-1")
	if err == nil {
		t.Error("session should be deleted")
	}

	samples, _ := d.GetSamples("del-1", 0, 0)
	if len(samples) != 0 {
		t.Errorf("samples should be deleted, got %d", len(samples))
	}
}

func TestSetSessionStatus(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_status_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_status_test.db")

	s := &model.Session{ID: "sts-1", Name: "status-test", Platform: "windows", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	if err := d.SetSessionStatus("sts-1", "error"); err != nil {
		t.Fatalf("set status: %v", err)
	}

	got, _ := d.GetSession("sts-1")
	if got.Status != "error" {
		t.Errorf("status = %q, want %q", got.Status, "error")
	}
}

func TestFrameTimeAnalysis(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_frame_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_frame_test.db")

	s := &model.Session{ID: "frame-1", Name: "frame-test", Platform: "windows", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	now := time.Now().Unix()
	// 正常帧 16ms，中间插一个 jank (50ms)
	for i := 0; i < 20; i++ {
		ft := 16.67
		if i == 10 { ft = 50.0 }
		d.InsertSample(&model.Sample{
			SessionID: "frame-1", Timestamp: now + int64(i), Elapsed: float64(i),
			FrameTime: ft, FPS: 1000.0 / ft,
		})
	}

	analysis, err := d.GetFrameTimeAnalysis("frame-1")
	if err != nil {
		t.Fatalf("frame analysis: %v", err)
	}
	if analysis.FrameCount != 20 {
		t.Errorf("frame_count = %d, want 20", analysis.FrameCount)
	}
	if len(analysis.Histogram) == 0 {
		t.Error("histogram should not be empty")
	}
}

func TestSystemInfo(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_sysinfo_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_sysinfo_test.db")

	info := &model.SystemInfo{
		SessionID:  "sys-1",
		OS:         "android",
		Arch:       "arm64",
		OSVersion:  "14",
		AndroidAPI: 34,
		DeviceModel: "Pixel 8",
	}
	if err := d.SaveSystemInfo(info); err != nil {
		t.Fatalf("save system info: %v", err)
	}

	got, err := d.GetSystemInfo("sys-1")
	if err != nil {
		t.Fatalf("get system info: %v", err)
	}
	if got.OSVersion != "14" {
		t.Errorf("os_version = %q, want %q", got.OSVersion, "14")
	}
	if got.DeviceModel != "Pixel 8" {
		t.Errorf("device_model = %q, want %q", got.DeviceModel, "Pixel 8")
	}
}

func TestPagination(t *testing.T) {
	d, err := New(os.TempDir() + "/gameperf_page_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	defer os.Remove(os.TempDir() + "/gameperf_page_test.db")

	s := &model.Session{ID: "page-1", Name: "page-test", Platform: "windows", Status: "running", StartTime: time.Now()}
	d.CreateSession(s)

	now := time.Now().Unix()
	for i := 0; i < 100; i++ {
		d.InsertSample(&model.Sample{
			SessionID: "page-1", Timestamp: now + int64(i), Elapsed: float64(i), CPU: float64(i),
		})
	}

	// Get first 10
	page1, err := d.GetSamples("page-1", 10, 0)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1) != 10 {
		t.Errorf("page1 got %d, want 10", len(page1))
	}

	// Get second 10
	page2, err := d.GetSamples("page-1", 10, 10)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 10 {
		t.Errorf("page2 got %d, want 10", len(page2))
	}

	// Ensure pages are different
	if page1[0].CPU == page2[0].CPU {
		t.Error("pages should have different data")
	}
}
