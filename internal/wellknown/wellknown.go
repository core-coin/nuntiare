package wellknown

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/core-coin/nuntiare/internal/config"
	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

// TokensResponse represents the response from .well-known/tokens.json
type TokensResponse struct {
	Tokens     []string   `json:"tokens"`
	Pagination Pagination `json:"pagination"`
}

// Pagination represents the pagination info in the tokens response
type Pagination struct {
	Limit   int    `json:"limit"`
	HasNext bool   `json:"hasNext"`
	Cursor  string `json:"cursor"`
}

// TokenMetadata represents detailed information about a single token
type TokenMetadata struct {
	Blockchain string `json:"blockchain"`
	Network    string `json:"network"`
	Ticker     string `json:"ticker"`
	Name       string `json:"name"`
	Decimals   int    `json:"decimals"`
	Symbol     string `json:"symbol"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
}

// WellKnownService manages fetching and caching token information from well-known service
type WellKnownService struct {
	logger  *logger.Logger
	config  *config.Config
	baseURL string
	network string
	client  *http.Client

	// In-memory cache
	tokenCache []*models.Token
	cacheMutex sync.RWMutex

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWellKnownService creates a new WellKnownService instance
func NewWellKnownService(
	logger *logger.Logger,
	config *config.Config,
) *WellKnownService {
	ctx, cancel := context.WithCancel(context.Background())
	return &WellKnownService{
		logger:     logger,
		config:     config,
		baseURL:    config.WellKnownURL,
		network:    config.GetNetworkName(), // Derive from NETWORK_ID
		tokenCache: make([]*models.Token, 0),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// FetchAndUpdateTokens fetches all tokens from the well-known service and updates the in-memory cache
func (w *WellKnownService) FetchAndUpdateTokens() error {
	w.logger.Info("Fetching tokens from well-known service")

	// Fetch all token addresses with pagination
	tokenAddresses, err := w.fetchAllTokenAddresses()
	if err != nil {
		return fmt.Errorf("failed to fetch token addresses: %w", err)
	}

	w.logger.Info(fmt.Sprintf("Found %d tokens from well-known service", len(tokenAddresses)))

	// Fetch metadata for each token concurrently with worker pool
	const maxConcurrent = 20 // Limit concurrent requests
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var cacheMutex sync.Mutex
	newCache := make([]*models.Token, 0, len(tokenAddresses))

	for _, address := range tokenAddresses {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			metadata, err := w.fetchTokenMetadata(addr)
			if err != nil {
				w.logger.Error("Failed to fetch token metadata", "address", addr, "error", err)
				return
			}

			// Only process CBC20 and CBC721 tokens
			if metadata.Type != "CBC20" && metadata.Type != "CBC721" {
				w.logger.Debug("Skipping non-CBC20/CBC721 token", "address", addr, "type", metadata.Type)
				return
			}

			token := &models.Token{
				Address:   addr,
				Name:      metadata.Name,
				Symbol:    metadata.Symbol,
				Decimals:  metadata.Decimals,
				Type:      metadata.Type,
				Network:   metadata.Network,
				UpdatedAt: time.Now().Unix(),
			}

			cacheMutex.Lock()
			newCache = append(newCache, token)
			cacheMutex.Unlock()

			w.logger.Debug("Token cached", "address", addr, "symbol", metadata.Symbol, "type", metadata.Type)
		}(address)
	}

	wg.Wait() // Wait for all goroutines to complete

	// Update the cache atomically
	w.cacheMutex.Lock()
	w.tokenCache = newCache
	w.cacheMutex.Unlock()

	w.logger.Info(fmt.Sprintf("Successfully cached %d tokens in memory", len(newCache)))

	return nil
}

// fetchAllTokenAddresses fetches all token addresses using pagination
func (w *WellKnownService) fetchAllTokenAddresses() ([]string, error) {
	var allAddresses []string
	cursor := ""

	for {
		url := fmt.Sprintf("%s/.well-known/tokens/%s/tokens.json?limit=1000", w.baseURL, w.network)
		if cursor != "" {
			url = fmt.Sprintf("%s&cursor=%s", url, cursor)
		}

		resp, err := w.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tokens list: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		}

		var tokensResp TokensResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokensResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode tokens response: %w", err)
		}
		resp.Body.Close()

		allAddresses = append(allAddresses, tokensResp.Tokens...)

		if !tokensResp.Pagination.HasNext {
			break
		}

		cursor = tokensResp.Pagination.Cursor
	}

	return allAddresses, nil
}

// fetchTokenMetadata fetches detailed metadata for a specific token
func (w *WellKnownService) fetchTokenMetadata(address string) (*TokenMetadata, error) {
	url := fmt.Sprintf("%s/.well-known/tokens/%s/%s.json", w.baseURL, w.network, address)

	resp, err := w.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch token metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var metadata TokenMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode token metadata: %w", err)
	}

	return &metadata, nil
}

// GetAllTokens returns all cached tokens (thread-safe)
func (w *WellKnownService) GetAllTokens() []*models.Token {
	w.cacheMutex.RLock()
	defer w.cacheMutex.RUnlock()

	// Return a copy to prevent external modifications
	tokens := make([]*models.Token, len(w.tokenCache))
	copy(tokens, w.tokenCache)
	return tokens
}

// StartPeriodicUpdate starts a goroutine that updates tokens periodically
func (w *WellKnownService) StartPeriodicUpdate() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		// Initial fetch with retry logic
		backoff := 5 * time.Second
		maxBackoff := 5 * time.Minute

		for {
			if err := w.FetchAndUpdateTokens(); err != nil {
				w.logger.Error("Failed to fetch tokens on startup, retrying...", "error", err, "retry_in", backoff)

				// Wait with context cancellation support
				select {
				case <-time.After(backoff):
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				case <-w.ctx.Done():
					w.logger.Info("WellKnown service stopped during initial fetch")
					return
				}
			}
			w.logger.Info("Successfully loaded initial token list")
			break
		}

		// Update every hour
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.logger.Info("Starting periodic token update")
				if err := w.FetchAndUpdateTokens(); err != nil {
					w.logger.Error("Failed to fetch tokens during periodic update", "error", err)
				}
			case <-w.ctx.Done():
				w.logger.Info("WellKnown service periodic update stopped")
				return
			}
		}
	}()
}

// Stop gracefully stops the WellKnownService
func (w *WellKnownService) Stop() {
	w.logger.Info("Stopping WellKnown service")
	w.cancel()
	w.wg.Wait()
	w.logger.Info("WellKnown service stopped")
}
