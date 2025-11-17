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
	Subscriber  string `json:"subscriber" binding:"required"`
	Destination string `json:"destination" binding:"required"`
	Network     string `json:"network" binding:"required,oneof=xcb xab"`
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

	// Check if wallet already exists
	existingWallet, err := s.nuntiare.GetWallet(req.Destination)
	if err == nil && existingWallet != nil {
		s.logger.Debug("Wallet already registered", "destination", req.Destination)
		c.JSON(http.StatusConflict, gin.H{
			"success":              false,
			"error":                "Wallet already registered",
			"address":              req.Destination,
			"subscription_address": existingWallet.SubscriptionAddress,
		})
		return
	}

	// Create notification provider
	notificationProvider := models.NotificationProvider{
		TelegramProvider: models.TelegramProvider{
			Username: req.Telegram,
		},
		EmailProvider: models.EmailProvider{
			Email: req.Email,
		},
		Address: req.Destination,
	}

	// Register wallet
	err = s.nuntiare.RegisterNewWallet(&models.Wallet{
		Address:              req.Destination,
		SubscriptionAddress:  req.Subscriber,
		Originator:           req.Origin,
		Whitelisted:          false,
		Network:              req.Network,
		CreatedAt:            time.Now().Unix(),
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

	subscription, err := s.nuntiare.CheckWalletSubscription(wallet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get subscription"})
		return
	}
	c.JSON(http.StatusOK, subscription)
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
