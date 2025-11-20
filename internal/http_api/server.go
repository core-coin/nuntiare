package http_api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout = 10 * time.Second
)

// HTTPServer is the HTTP server struct that will serve the API
type HTTPServer struct {
	// logger is the logger instance
	logger *logger.Logger

	// router is the HTTP router
	router *gin.Engine
	// port is the port on which the server will listen
	port int

	// server is the underlying HTTP server
	server *http.Server

	// nuntiare is the main application struct
	nuntiare models.NuntiareI
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(nuntiare models.NuntiareI, port int, logger *logger.Logger) models.APIServer {
	router := gin.Default()

	// Add CORS middleware
	router.Use(corsMiddleware())

	server := &HTTPServer{
		router:   router,
		port:     port,
		nuntiare: nuntiare,
		logger:   logger,
	}

	// Define routes
	server.routes()

	return server
}

// Start starts the HTTP server
func (s *HTTPServer) Start() {
	addr := fmt.Sprintf("0.0.0.0:%v", s.port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	s.logger.Info("Starting HTTP server", "address", addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Fatal("Failed to start the HTTP server: ", err)
	}
}

// Shutdown gracefully shuts down the HTTP server
func (s *HTTPServer) Shutdown() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	s.logger.Info("Shutting down HTTP server...")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server shut down successfully")
	return nil
}
