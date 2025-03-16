package http_api

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/gin-gonic/gin"
)

// register is a handler for the /register endpoint.
func (s *HTTPServer) register(c *gin.Context) {
	// id -> serialNumber: `${origin}-${subscriptionAddress}-${props.destination}-${new Date(Date.now()).toISOString().replace(/[-T:]/g, '').slice(0, 12)}`
	id := c.PostForm("id")
	url, err := url.Parse(c.PostForm("url"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
	}
	origin := strings.Split(id, "-")[0]
	subscriptionAddress := strings.Split(id, "-")[1]
	monitorAddress := strings.Split(id, "-")[2]
	walletType := strings.Split(id, "-")[3]
	network := strings.Split(id, "-")[4]
	dateStr := strings.Split(id, "-")[5] // ISO date reformatted and timezone - example: 2503071030-0500

	// Parse the date string
	date, err := time.Parse("0601021504-0700", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
		return
	}

	err = s.nuntiare.RegisterNewWallet(&models.Wallet{
		Address:             monitorAddress,
		SubscriptionAddress: subscriptionAddress,
		HookURL:             url.String(),
		Origin:              origin,
		Whitelisted:         false,
		WalletType:          walletType,
		Network:             network,
		CreatedAt:           date.Unix(),
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
	subscription, err := s.nuntiare.CheckWalletSubscription(&models.Wallet{Address: address})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get subscription"})
		return
	}
	c.JSON(http.StatusOK, subscription)
}
