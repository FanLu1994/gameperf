package api

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"gameperf/internal/collector"
	"gameperf/internal/db"
	"gameperf/internal/model"
)

type Server struct {
	db             *db.DB
	collectors     map[string]*collector.Collector
	presentMonCols map[string]*collector.PresentMonCollector
	engine         *gin.Engine
}

func NewServer(database *db.DB) *Server {
	s := &Server{
		db:             database,
		collectors:     make(map[string]*collector.Collector),
		presentMonCols: make(map[string]*collector.PresentMonCollector),
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	{
		// Session 管理
		api.POST("/sessions", s.createSession)
		api.GET("/sessions", s.listSessions)
		api.GET("/sessions/:id", s.getSession)
		api.POST("/sessions/:id/start", s.startCollect)
		api.POST("/sessions/:id/stop", s.stopCollect)
		api.DELETE("/sessions/:id", s.deleteSession)

		// 数据查询
		api.GET("/sessions/:id/samples", s.getSamples)
		api.GET("/sessions/:id/summary", s.getSummary)
		api.POST("/sessions/:id/samples", s.injectSample)

		// 帧时间分析
		api.GET("/sessions/:id/frame-analysis", s.getFrameAnalysis)

		// 系统信息
		api.GET("/sessions/:id/system", s.getSystemInfo)

		// 对比
		api.GET("/compare", s.compare)

		// 运行时信息
		api.GET("/info", s.getServerInfo)
	}

	// 静态文件
	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	s.engine = r
	return s
}

func (s *Server) Run(addr string) error {
	fmt.Printf("[server] 启动 http://%s\n", addr)
	fmt.Printf("[server] OS=%s Arch=%s\n", runtime.GOOS, runtime.GOARCH)
	return s.engine.Run(addr)
}

// --- handlers ---

func (s *Server) createSession(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Process  string `json:"process"`
		PID      int    `json:"pid"`
		Interval string `json:"interval"`
		Tags     string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := &model.Session{
		ID:        generateID(),
		Name:      req.Name,
		Process:   req.Process,
		PID:       int32(req.PID),
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
	if sessions == nil {
		sessions = []model.Session{}
	}
	c.JSON(http.StatusOK, sessions)
}

func (s *Server) getSession(c *gin.Context) {
	session, err := s.db.GetSession(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (s *Server) startCollect(c *gin.Context) {
	id := c.Param("id")
	session, err := s.db.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	var req struct {
		PID             int    `json:"pid"`
		Interval        string `json:"interval"`
		PresentMonPath  string `json:"presentmon_path"`
		EnablePresentMon bool  `json:"enable_presentmon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pid := int32(req.PID)
	if pid == 0 {
		pid = session.PID
	}

	interval := time.Second
	if req.Interval != "" {
		if d, err := time.ParseDuration(req.Interval); err == nil {
			interval = d
		}
	}

	coll, err := collector.New(s.db, pid, interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.collectors[id] = coll
	session.Status = "running"
	session.PID = pid
	session.Interval = req.Interval

	// 启动性能采集
	go coll.Start(id)

	// 启动 PresentMon（可选）
	if req.EnablePresentMon {
		pmPath := req.PresentMonPath
		if pmPath == "" {
			pmPath, _ = collector.CheckPresentMon()
		}
		if pmPath != "" {
			pmColl, err := collector.NewPresentMonCollector(s.db, id, pid, pmPath)
			if err == nil {
				s.presentMonCols[id] = pmColl
				go pmColl.StartPresentMon(pmPath)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "running", "session": session})
}

func (s *Server) stopCollect(c *gin.Context) {
	id := c.Param("id")

	if coll, ok := s.collectors[id]; ok {
		coll.Stop()
		delete(s.collectors, id)
	}
	if pm, ok := s.presentMonCols[id]; ok {
		pm.Stop()
		delete(s.presentMonCols, id)
	}

	if err := s.db.StopSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (s *Server) deleteSession(c *gin.Context) {
	id := c.Param("id")
	if coll, ok := s.collectors[id]; ok {
		coll.Stop()
		delete(s.collectors, id)
	}
	if pm, ok := s.presentMonCols[id]; ok {
		pm.Stop()
		delete(s.presentMonCols, id)
	}
	if err := s.db.DeleteSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) getSamples(c *gin.Context) {
	samples, err := s.db.GetSamples(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if samples == nil {
		samples = []model.Sample{}
	}
	c.JSON(http.StatusOK, samples)
}

func (s *Server) getSummary(c *gin.Context) {
	summary, err := s.db.GetSummary(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// injectSample 手动注入采样数据（用于 FPS 等无法自动采集的指标）
func (s *Server) injectSample(c *gin.Context) {
	id := c.Param("id")
	var sample model.Sample
	if err := c.ShouldBindJSON(&sample); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sample.SessionID = id
	if sample.Timestamp == 0 {
		sample.Timestamp = time.Now().Unix()
	}
	if err := s.db.InsertSample(&sample); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sample)
}

func (s *Server) getFrameAnalysis(c *gin.Context) {
	analysis, err := s.db.GetFrameTimeAnalysis(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analysis)
}

func (s *Server) getSystemInfo(c *gin.Context) {
	info, err := s.db.GetSystemInfo(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system info not found"})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (s *Server) compare(c *gin.Context) {
	idsStr := c.Query("ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids parameter required"})
		return
	}

	ids := strings.Split(idsStr, ",")
	var result model.CompareResult

	for _, id := range ids {
		summary, err := s.db.GetSummary(strings.TrimSpace(id))
		if err != nil {
			continue
		}
		result.Summaries = append(result.Summaries, *summary)
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) getServerInfo(c *gin.Context) {
	gpuStatus := "none"
	pmStatus := "not available"

	// 检查 nvidia-smi
	if path, err := collector.New(s.db, 0, time.Second).CheckPresentMon(); err == nil {
		_ = path
	}
	_ = gpuStatus
	_ = pmStatus

	c.JSON(http.StatusOK, gin.H{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"version":  "1.1.0",
		"features": []string{"cpu", "memory", "threads", "disk_io", "network_io", "gpu_nvidia", "frametime", "jank_detection", "presentmon"},
	})
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
