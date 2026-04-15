package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"gameperf/internal/collector"
	"gameperf/internal/db"
	"gameperf/internal/model"
)

type Server struct {
	mu             sync.RWMutex
	db             *db.DB
	collectors     map[string]*collector.Collector
	androidCols    map[string]*collector.AndroidCollector
	iosCols        map[string]*collector.IOSCollector
	presentMonCols map[string]*collector.PresentMonCollector
	engine         *gin.Engine
}

func NewServer(database *db.DB) *Server {
	s := &Server{
		db:             database,
		collectors:     make(map[string]*collector.Collector),
		androidCols:    make(map[string]*collector.AndroidCollector),
		iosCols:        make(map[string]*collector.IOSCollector),
		presentMonCols: make(map[string]*collector.PresentMonCollector),
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"*"},
		MaxAge:       12 * time.Hour,
	}))

	api := r.Group("/api")
	{
		api.POST("/sessions", s.createSession)
		api.GET("/sessions", s.listSessions)
		api.GET("/sessions/:id", s.getSession)
		api.POST("/sessions/:id/start", s.startCollect)
		api.POST("/sessions/:id/stop", s.stopCollect)
		api.DELETE("/sessions/:id", s.deleteSession)
		api.GET("/sessions/:id/samples", s.getSamples)
		api.GET("/sessions/:id/summary", s.getSummary)
		api.POST("/sessions/:id/samples", s.injectSample)
		api.GET("/sessions/:id/frame-analysis", s.getFrameAnalysis)
		api.GET("/sessions/:id/system", s.getSystemInfo)
		api.GET("/compare", s.compare)
		api.GET("/info", s.getServerInfo)

		// Android 专用
		api.GET("/android/devices", s.listAndroidDevices)
		api.GET("/android/packages", s.listAndroidPackages)
		// iOS 专用
		api.GET("/ios/devices", s.listIOSDevices)
		api.GET("/ios/apps", s.listIOSApps)
		api.GET("/ios/check", s.checkIOSPrereqs)
	}

	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) { c.File("./web/dist/index.html") })

	s.engine = r
	return s
}

func (s *Server) Run(addr string) error {
	fmt.Printf("[server] 启动 http://%s  OS=%s/%s\n", addr, runtime.GOOS, runtime.GOARCH)
	return s.engine.Run(addr)
}

// --- Session CRUD ---

func (s *Server) createSession(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Process  string `json:"process"`
		PID      int    `json:"pid"`
		Platform string `json:"platform"`   // "windows" / "android"
		Package  string `json:"package"`    // Android 包名
		DeviceID string `json:"device_id"`  // Android 设备ID
		Interval string `json:"interval"`
		Tags     string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	platform := req.Platform
	if platform == "" {
		platform = "windows"
	}

	session := &model.Session{
		ID:        generateID(),
		Name:      req.Name,
		Process:   req.Process,
		PID:       int32(req.PID),
		Platform:  platform,
		Package:   req.Package,
		DeviceID:  req.DeviceID,
		Status:    "created",
		StartTime: time.Now(),
		Interval:  req.Interval,
		Tags:      req.Tags,
	}

	if err := s.db.CreateSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (s *Server) listSessions(c *gin.Context) {
	sessions, err := s.db.ListSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sessions == nil { sessions = []model.Session{} }
	c.JSON(http.StatusOK, sessions)
}

func (s *Server) getSession(c *gin.Context) {
	session, err := s.db.GetSession(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

// onCollectorError is a callback invoked when a collector goroutine fails.
// It updates the session status to "error" in the database.
func (s *Server) onCollectorError(sessionID string, err error) {
	fmt.Printf("[collector] session %s failed: %v\n", sessionID, err)
	s.db.SetSessionStatus(sessionID, "error")
}

func (s *Server) startCollect(c *gin.Context) {
	id := c.Param("id")
	session, err := s.db.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var req struct {
		PID              int    `json:"pid"`
		Interval         string `json:"interval"`
		PresentMonPath   string `json:"presentmon_path"`
		EnablePresentMon bool   `json:"enable_presentmon"`
		// Android 专用
		Package  string `json:"package"`
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	interval := time.Second
	if req.Interval != "" {
		if d, err := time.ParseDuration(req.Interval); err == nil { interval = d }
	}

	platform := session.Platform
	if platform == "" {
		// 兼容旧 session
		if session.Package != "" || req.Package != "" {
			platform = "android"
		} else {
			platform = "windows"
		}
	}

	switch platform {
	case "android":
		pkg := session.Package
		if pkg == "" { pkg = req.Package }
		deviceID := session.DeviceID
		if deviceID == "" { deviceID = req.DeviceID }

		if pkg == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "package name required for android"})
			return
		}

		ac := collector.NewAndroidCollector(s.db, pkg, deviceID, interval)
		s.mu.Lock()
		s.androidCols[id] = ac
		s.mu.Unlock()
		session.Status = "running"
		session.Package = pkg
		session.DeviceID = deviceID

		go func() {
			if err := ac.Start(id); err != nil {
				s.onCollectorError(id, err)
			}
		}()

	case "ios":
		bundle := session.Package
		if bundle == "" { bundle = req.Package }
		udid := session.DeviceID
		if udid == "" { udid = req.DeviceID }

		if bundle == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bundle ID required for ios"})
			return
		}

		ic := collector.NewIOSCollector(s.db, bundle, udid, interval)
		s.mu.Lock()
		s.iosCols[id] = ic
		s.mu.Unlock()
		session.Status = "running"
		session.Package = bundle
		session.DeviceID = udid

		go func() {
			if err := ic.Start(id); err != nil {
				s.onCollectorError(id, err)
			}
		}()

	default: // windows / linux
		pid := int32(req.PID)
		if pid == 0 { pid = session.PID }

		coll, err := collector.New(s.db, pid, interval)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s.mu.Lock()
		s.collectors[id] = coll
		s.mu.Unlock()
		session.Status = "running"
		session.PID = pid
		session.Interval = req.Interval
		go func() {
			coll.Start(id)
		}()

		if req.EnablePresentMon {
			pmPath := req.PresentMonPath
			if pmPath == "" { pmPath, _ = collector.CheckPresentMon() }
			if pmPath != "" {
				if pmColl, err := collector.NewPresentMonCollector(s.db, id, pid, pmPath); err == nil {
					s.mu.Lock()
					s.presentMonCols[id] = pmColl
					s.mu.Unlock()
					go func() {
						if err := pmColl.StartPresentMon(pmPath); err != nil {
							s.onCollectorError(id, err)
						}
					}()
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "running", "session": session})
}

func (s *Server) stopCollect(c *gin.Context) {
	id := c.Param("id")
	s.mu.Lock()
	if coll, ok := s.collectors[id]; ok { coll.Stop(); delete(s.collectors, id) }
	if ac, ok := s.androidCols[id]; ok { ac.Stop(); delete(s.androidCols, id) }
	if ic, ok := s.iosCols[id]; ok { ic.Stop(); delete(s.iosCols, id) }
	if pm, ok := s.presentMonCols[id]; ok { pm.Stop(); delete(s.presentMonCols, id) }
	s.mu.Unlock()
	s.db.StopSession(id)
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (s *Server) deleteSession(c *gin.Context) {
	id := c.Param("id")
	s.mu.Lock()
	if coll, ok := s.collectors[id]; ok { coll.Stop(); delete(s.collectors, id) }
	if ac, ok := s.androidCols[id]; ok { ac.Stop(); delete(s.androidCols, id) }
	if ic, ok := s.iosCols[id]; ok { ic.Stop(); delete(s.iosCols, id) }
	if pm, ok := s.presentMonCols[id]; ok { pm.Stop(); delete(s.presentMonCols, id) }
	s.mu.Unlock()
	s.db.DeleteSession(id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) getSamples(c *gin.Context) {
	limit := 5000
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}
	samples, err := s.db.GetSamples(c.Param("id"), limit, offset)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	if samples == nil { samples = []model.Sample{} }
	c.JSON(http.StatusOK, samples)
}

func (s *Server) getSummary(c *gin.Context) {
	summary, err := s.db.GetSummary(c.Param("id"))
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, summary)
}

func (s *Server) injectSample(c *gin.Context) {
	id := c.Param("id")
	var sample model.Sample
	if err := c.ShouldBindJSON(&sample); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	sample.SessionID = id
	if sample.Timestamp == 0 { sample.Timestamp = time.Now().Unix() }
	if err := s.db.InsertSample(&sample); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusCreated, sample)
}

func (s *Server) getFrameAnalysis(c *gin.Context) {
	analysis, err := s.db.GetFrameTimeAnalysis(c.Param("id"))
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, analysis)
}

func (s *Server) getSystemInfo(c *gin.Context) {
	info, err := s.db.GetSystemInfo(c.Param("id"))
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}); return }
	c.JSON(http.StatusOK, info)
}

func (s *Server) compare(c *gin.Context) {
	ids := strings.Split(c.Query("ids"), ",")
	if len(ids) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 10 IDs allowed for comparison"})
		return
	}
	var result model.CompareResult
	for _, id := range ids {
		if summary, err := s.db.GetSummary(strings.TrimSpace(id)); err == nil {
			result.Summaries = append(result.Summaries, *summary)
		}
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) getServerInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"version":   "2.0.0",
		"platforms": []string{"windows", "linux", "android", "ios"},
		"features":  []string{"cpu", "memory", "gpu_adreno", "gpu_mali", "gpu_nvidia", "fps", "frametime", "jank", "disk_io", "network_io", "battery", "thermal", "presentmon", "ios_instruments"},
	})
}

// --- Android 专用 API ---

func (s *Server) listAndroidDevices(c *gin.Context) {
	devices, err := collector.ListAdbDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "adb not available: " + err.Error()})
		return
	}
	if devices == nil { devices = []string{} }
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

func (s *Server) listAndroidPackages(c *gin.Context) {
	deviceID := c.Query("device_id")
	packages, err := collector.ListAndroidPackages(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if packages == nil { packages = []string{} }
	c.JSON(http.StatusOK, gin.H{"packages": packages})
}

// --- iOS 专用 API ---

func (s *Server) listIOSDevices(c *gin.Context) {
	devices, err := collector.ListIOSDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pymobiledevice3 not available: " + err.Error()})
		return
	}
	if devices == nil { devices = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

func (s *Server) listIOSApps(c *gin.Context) {
	udid := c.Query("device_id")
	apps, err := collector.ListIOSApps(udid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if apps == nil { apps = []string{} }
	c.JSON(http.StatusOK, gin.H{"apps": apps})
}

func (s *Server) checkIOSPrereqs(c *gin.Context) {
	path, err := collector.CheckIOSPrerequisites()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ready": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ready": true, "path": path})
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback should never happen with crypto/rand
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
