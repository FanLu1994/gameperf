package db

import (
	"database/sql"
	"fmt"
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
			status TEXT NOT NULL DEFAULT 'running',
			start_time DATETIME NOT NULL,
			end_time DATETIME
		);
		CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			elapsed REAL NOT NULL,
			cpu REAL NOT NULL DEFAULT 0,
			memory REAL NOT NULL DEFAULT 0,
			fps REAL NOT NULL DEFAULT 0,
			gpu_mem REAL NOT NULL DEFAULT 0,
			threads INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);
		CREATE INDEX IF NOT EXISTS idx_samples_session ON samples(session_id);
		CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(session_id, timestamp);
	`)
	return err
}

func (d *DB) CreateSession(s *model.Session) error {
	_, err := d.conn.Exec(
		`INSERT INTO sessions (id, name, process, status, start_time) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Process, s.Status, s.StartTime,
	)
	return err
}

func (d *DB) ListSessions() ([]model.Session, error) {
	rows, err := d.conn.Query(`SELECT id, name, process, status, start_time, end_time FROM sessions ORDER BY start_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []model.Session
	for rows.Next() {
		var s model.Session
		var endTime sql.NullTime
		if err := rows.Scan(&s.ID, &s.Name, &s.Process, &s.Status, &s.StartTime, &endTime); err != nil {
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
		`SELECT id, name, process, status, start_time, end_time FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.Process, &s.Status, &s.StartTime, &endTime)
	if err != nil {
		return nil, err
	}
	if endTime.Valid {
		s.EndTime = &endTime.Time
	}
	return &s, nil
}

func (d *DB) StopSession(id string) error {
	_, err := d.conn.Exec(
		`UPDATE sessions SET status = 'stopped', end_time = datetime('now') WHERE id = ?`, id,
	)
	return err
}

func (d *DB) DeleteSession(id string) error {
	d.conn.Exec(`DELETE FROM samples WHERE session_id = ?`, id)
	_, err := d.conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (d *DB) InsertSample(s *model.Sample) error {
	_, err := d.conn.Exec(
		`INSERT INTO samples (session_id, timestamp, elapsed, cpu, memory, fps, gpu_mem, threads) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.Timestamp, s.Elapsed, s.CPU, s.Memory, s.FPS, s.GPUMem, s.Threads,
	)
	return err
}

func (d *DB) GetSamples(sessionID string) ([]model.Sample, error) {
	rows, err := d.conn.Query(
		`SELECT id, session_id, timestamp, elapsed, cpu, memory, fps, gpu_mem, threads FROM samples WHERE session_id = ? ORDER BY timestamp`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []model.Sample
	for rows.Next() {
		var s model.Sample
		if err := rows.Scan(&s.ID, &s.SessionID, &s.Timestamp, &s.Elapsed, &s.CPU, &s.Memory, &s.FPS, &s.GPUMem, &s.Threads); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	return samples, nil
}

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

	summary := &model.SessionSummary{
		Session:     *session,
		SampleCount: len(samples),
	}

	// 计算统计数据
	cpus := make([]float64, len(samples))
	mems := make([]float64, len(samples))
	fpss := make([]float64, 0)

	for i, s := range samples {
		cpus[i] = s.CPU
		mems[i] = s.Memory
		if s.FPS > 0 {
			fpss = append(fpss, s.FPS)
		}
		summary.AvgCPU += s.CPU
		summary.AvgMemory += s.Memory
	}

	n := float64(len(samples))
	summary.AvgCPU /= n
	summary.AvgMemory /= n

	sort.Float64s(cpus)
	sort.Float64s(mems)

	summary.MinCPU = cpus[0]
	summary.MaxCPU = cpus[len(cpus)-1]
	summary.P95CPU = percentile(cpus, 95)

	summary.MinMemory = mems[0]
	summary.MaxMemory = mems[len(mems)-1]
	summary.P95Memory = percentile(mems, 95)

	if len(fpss) > 0 {
		summary.AvgFPS = avg(fpss)
		sort.Float64s(fpss)
		summary.MinFPS = fpss[0]
		summary.P5FPS = percentile(fpss, 5)
	}

	if session.EndTime != nil {
		summary.Duration = session.EndTime.Sub(session.StartTime).Seconds()
	} else {
		summary.Duration = float64(samples[len(samples)-1].Elapsed)
	}

	return summary, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := float64(len(sorted)-1) * p / 100
	return sorted[int(idx)]
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
