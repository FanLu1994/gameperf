package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"gameperf/internal/collector"
	"gameperf/internal/db"
	"gameperf/internal/model"
)

type Server struct {
	db         *db.DB
	collectors map[string]*collector.Collector
	engine     *gin.Engine
}

func NewServer(database *db.DB) *Server {
	s := &Server{
		db:         database,
		collectors: make(map[string]*collector.Collector),
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
		api.POST("/sessions", s.createSession)
		api.GET("/sessions", s.listSessions)
		api.GET("/sessions/:id", s.getSession)
		api.POST("/sessions/:id/start", s.startCollect)
		api.POST("/sessions/:id/stop", s.stopCollect)
		api.DELETE("/sessions/:id", s.deleteSession)
		api.GET("/sessions/:id/samples", s.getSamples)
		api.GET("/sessions/:id/summary", s.getSummary)
		api.POST("/sessions/:id/samples", s.injectSample)
		api.GET("/compare", s.compare)
	}

	// 静态文件 — 前端 build 后嵌入
	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	s.engine = r
	return s
}

func (s *Server) Run(addr string) error {
	fmt.Printf("[server] 启动 http://%s\n", addr)
	return s.engine.Run(addr)
}

// --- handlers ---

func (s *Server) createSession(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Process string `json:"process"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := &model.Session{
		ID:        generateID(),
		Name:      req.Name,
		Process:   req.Process,
		Status:    "created",
		StartTime: time.Now(),
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
		PID      int    `json:"pid"`
		Interval string `json:"interval"` // e.g. "1s", "500ms"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	interval := time.Second
	if req.Interval != "" {
		if d, err := time.ParseDuration(req.Interval); err == nil {
			interval = d
		}
	}

	coll, err := collector.New(s.db, int32(req.PID), interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.collectors[id] = coll
	session.Status = "running"
	session.StartTime = time.Now()
	s.db.CreateSession(session)

	go coll.Start(id)

	c.JSON(http.StatusOK, gin.H{"status": "running", "session": session})
}

func (s *Server) stopCollect(c *gin.Context) {
	id := c.Param("id")

	if coll, ok := s.collectors[id]; ok {
		coll.Stop()
		delete(s.collectors, id)
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

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
