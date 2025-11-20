package http_api

import (
	"net/http"
	"strings"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/validation"
	"github.com/gin-gonic/gin"
)

// RegisterRequest represents the JSON body for wallet registration
type RegisterRequest struct {
	Origin      string `json:"origin" binding:"required"`
	OriginID    string `json:"originid" binding:"required,min=34,max=34"` // Alphanumeric UUID, 34 chars
	Subscriber  string `json:"subscriber" binding:"required"`
	Destination string `json:"destination" binding:"required"`
	Network     string `json:"network" binding:"required,oneof=xcb xab"`
	OS          string `json:"os"`   // Operating system (ios, android, web, etc.)
	Lang        string `json:"lang"` // Language (en, es, fr, etc.)
	Telegram    string `json:"telegram"`
	Email       string `json:"email" binding:"omitempty,email"`
}

// RegisterResponse represents the success response for registration
type RegisterResponse struct {
	Success             bool   `json:"success"`
	Message             string `json:"message"`
	Address             string `json:"address"`
	SubscriptionAddress string `json:"subscription_address"`
}

// CancelRequest represents the JSON body for canceling notifications
type CancelRequest struct {
	Destination string `json:"destination" binding:"required"`
	OriginID    string `json:"origin_id" binding:"required"`
}

// SubscriptionResponse represents the subscription status with expiration
type SubscriptionResponse struct {
	Subscribed bool  `json:"subscribed"`
	ExpiresAt  int64 `json:"expires_at,omitempty"` // Unix timestamp, only if subscribed
	Active     bool  `json:"active"`                // Whether notifications are enabled
}

// register is a handler for the /register endpoint.
func (s *HTTPServer) register(c *gin.Context) {
	var req RegisterRequest

	// Parse and validate JSON request body
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Debug("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate address formats
	if err := validation.ValidateAddress(req.Subscriber); err != nil {
		s.logger.Debug("Invalid subscriber address", "error", err, "address", req.Subscriber)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid subscriber address: " + err.Error(),
		})
		return
	}

	if err := validation.ValidateAddress(req.Destination); err != nil {
		s.logger.Debug("Invalid destination address", "error", err, "address", req.Destination)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid destination address: " + err.Error(),
		})
		return
	}

	// Require at least one notification method
	if req.Telegram == "" && req.Email == "" {
		s.logger.Debug("No notification method provided", "destination", req.Destination)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "At least one notification method (telegram or email) is required",
		})
		return
	}

	existingWallet, err := s.nuntiare.GetWallet(req.Destination)
	if err == nil && existingWallet != nil {
		// Wallet exists - verify OriginID for authentication
		if existingWallet.OriginID != req.OriginID {
			s.logger.Warn("OriginID mismatch for wallet update", "destination", req.Destination)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid origin_id",
			})
			return
		}

		// Update notification providers and re-activate if cancelled
		s.logger.Info("Wallet already exists, updating notification providers and reactivating", "destination", req.Destination)

		err = s.nuntiare.UpdateNotificationProviderAndReactivate(req.Destination, req.Telegram, req.Email)
		if err != nil {
			s.logger.Error("Failed to update notification provider", "error", err, "destination", req.Destination)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update notification provider",
			})
			return
		}

		s.logger.Info("Notification providers updated and wallet reactivated", "destination", req.Destination)
		c.JSON(http.StatusOK, RegisterResponse{
			Success:             true,
			Message:             "Notification providers updated successfully",
			Address:             req.Destination,
			SubscriptionAddress: existingWallet.SubscriptionAddress,
		})
		return
	}


	// Create notification provider for new wallet
	notificationProvider := models.NotificationProvider{
		TelegramProvider: models.TelegramProvider{
			Username: req.Telegram,
		},
		EmailProvider: models.EmailProvider{
			Email: req.Email,
		},
		Address: req.Destination,
	}

	// Register new wallet
	err = s.nuntiare.RegisterNewWallet(&models.Wallet{
		Address:              req.Destination,
		SubscriptionAddress:  req.Subscriber,
		OriginID:             req.OriginID,
		Originator:           req.Origin,
		Whitelisted:          false,
		Network:              req.Network,
		OS:                   req.OS,
		Lang:                 req.Lang,
		CreatedAt:            time.Now().Unix(),
		Active:               true,
		Paid:                 false,
		NotificationProvider: notificationProvider,
	})

	if err != nil {
		s.logger.Error("Failed to register wallet", "error", err, "destination", req.Destination)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to register wallet",
		})
		return
	}

	// Success response
	s.logger.Info("Wallet registered successfully", "destination", req.Destination, "origin", req.Origin)
	c.JSON(http.StatusCreated, RegisterResponse{
		Success:             true,
		Message:             "Wallet registered successfully",
		Address:             req.Destination,
		SubscriptionAddress: req.Subscriber,
	})
}

// isSubscribed is a handler for the /is_subscribed endpoint.
// It returns boolean indicating if the given address has subscription enabled.
func (s *HTTPServer) isSubscribed(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address is required"})
		return
	}

	// Validate address format
	if err := validation.ValidateAddress(address); err != nil {
		s.logger.Debug("Invalid address", "error", err, "address", address)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address format: " + err.Error()})
		return
	}

	wallet, err := s.nuntiare.GetWallet(address)
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "record not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get wallet"})
		}
		return
	}

	// Wallet should never be nil here, but defensive check
	if wallet == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		return
	}

	subscribed, err := s.nuntiare.CheckWalletSubscription(wallet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get subscription"})
		return
	}

	response := SubscriptionResponse{
		Subscribed: subscribed,
		Active:     wallet.Active,
	}

	// Include expiration timestamp only if subscribed
	if subscribed {
		response.ExpiresAt = wallet.SubscriptionExpiresAt
	}

	c.JSON(http.StatusOK, response)
}

// handleTelegramWebhook processes incoming Telegram webhook updates
func (s *HTTPServer) handleTelegramWebhook(c *gin.Context) {
	var update interface{}

	if err := c.ShouldBindJSON(&update); err != nil {
		s.logger.Debug("Invalid webhook payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := s.nuntiare.ProcessTelegramWebhook(update); err != nil {
		s.logger.Error("Failed to process Telegram update", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// cancel is a handler for the /cancel endpoint.
// It deactivates notifications while keeping the subscription active.
func (s *HTTPServer) cancel(c *gin.Context) {
	var req CancelRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Debug("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate address format
	if err := validation.ValidateAddress(req.Destination); err != nil {
		s.logger.Debug("Invalid destination address", "error", err, "address", req.Destination)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid destination address: " + err.Error(),
		})
		return
	}

	// Get wallet
	wallet, err := s.nuntiare.GetWallet(req.Destination)
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Wallet not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get wallet",
			})
		}
		return
	}

	// Verify OriginID
	if wallet.OriginID != req.OriginID {
		s.logger.Warn("OriginID mismatch for wallet cancel", "destination", req.Destination)
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid origin_id",
		})
		return
	}

	// Cancel (deactivate) wallet
	err = s.nuntiare.CancelWallet(req.Destination)
	if err != nil {
		s.logger.Error("Failed to cancel wallet", "error", err, "destination", req.Destination)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to cancel notifications",
		})
		return
	}

	s.logger.Info("Wallet notifications cancelled", "destination", req.Destination)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notifications cancelled successfully. Subscription remains active.",
	})
}
