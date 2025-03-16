package http_api

import (
	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
	"github.com/gin-gonic/gin"
)

// HTTPServer is the HTTP server struct that will serve the API
type HTTPServer struct {
	// logger is the logger instance
	logger *logger.Logger

	// router is the HTTP router
	router *gin.Engine
	// addr is the address the server will listen on
	addr   string

	// nuntiare is the main application struct
	nuntiare models.NuntiareI
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(nuntiare models.NuntiareI, addr string) models.APIServer {
	router := gin.Default()

	server := &HTTPServer{
		router:   router,
		addr:     addr,
		nuntiare: nuntiare,
	}

	// Define routes
	server.routes()

	return server
}

// Start starts the HTTP server
func (s *HTTPServer) Start() {
	if err := s.router.Run(s.addr); err != nil {
		s.logger.Fatal("Failed to start the HTTP server: ", err)
	}
}
