package http_api

// routes sets up the routes for the HTTP server.
func (s *HTTPServer) routes() {
	s.router.POST("/api/v1/subscription", s.register)
	s.router.GET("/api/v1/is_subscribed", s.isSubscribed)
	s.router.POST("/api/v1/cancel", s.cancel)
	s.router.POST("/api/v1/telegram/webhook", s.handleTelegramWebhook)
}