package http_api

// routes sets up the routes for the HTTP server.
func (s *HTTPServer) routes() {
	s.router.POST("/subscription", s.register)
	// s.router.GET("/v1/passes/:passTypeIdentifier/:serialNumber", s.getPass)
	s.router.GET("/is_subscribed", s.isSubscribed)
}