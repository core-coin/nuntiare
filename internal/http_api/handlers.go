package http_api

import (
	"net/http"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/gin-gonic/gin"
)

// register is a handler for the /register endpoint.
func (s *HTTPServer) register(c *gin.Context) {
	originator := c.Query("originator")
	subscriber := c.Query("subscriber")
	destination := c.Query("destination")
	network := c.Query("network")
	telegram := c.Query("telegram")
	email := c.Query("email")

	notificationProvider := models.NotificationProvider{
		TelegramProvider: models.TelegramProvider{
			Username: telegram,
		},
		EmailProvider: models.EmailProvider{
			Email: email,
		},
		Address: destination,
	}
	err := s.nuntiare.RegisterNewWallet(&models.Wallet{
		Address:              destination,
		SubscriptionAddress:  subscriber,
		Originator:           originator,
		Whitelisted:          false,
		Network:              network,
		CreatedAt:            time.Now().Unix(),
		Paid:                 false,
		NotificationProvider: notificationProvider,
	})
	if err != nil {
		s.logger.Debug("failed to register wallet", "error", err)
		c.JSON(http.StatusInternalServerError, "Failed to register wallet")
	}
}

// isSubscribed is a handler for the /is_subscribed endpoint.
// It returns boolean indicating if the given address has subscription enabled.
func (s *HTTPServer) isSubscribed(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address is required"})
		return
	}
	wallet, err := s.nuntiare.GetWallet(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get wallet"})
		return
	}
	subscription, err := s.nuntiare.CheckWalletSubscription(wallet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get subscription"})
		return
	}
	c.JSON(http.StatusOK, subscription)
}
