package http_api

import (
	"fmt"

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
	// port is the port on which the server will listen
	port int

	// nuntiare is the main application struct
	nuntiare models.NuntiareI
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(nuntiare models.NuntiareI, port int, logger *logger.Logger) models.APIServer {
	router := gin.Default()

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
	if err := s.router.Run(fmt.Sprintf("0.0.0.0:%v", s.port)); err != nil {
		s.logger.Fatal("Failed to start the HTTP server: ", err)
	}
}
