package http_api

// routes sets up the routes for the HTTP server.
func (s *HTTPServer) routes() {
	s.router.GET("/api/v1/subscription", s.register)
	s.router.GET("/api/v1/is_subscribed", s.isSubscribed)
}