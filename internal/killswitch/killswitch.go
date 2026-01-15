package killswitch

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	parameterName = "/colettedn/kill-switch"
	cacheTTL      = 30 * time.Second
)

type KillSwitch struct {
	mu        sync.RWMutex
	disabled  bool
	lastCheck time.Time
	ssmClient *ssm.Client
}

func New() *KillSwitch {
	ks := &KillSwitch{}

	// Initialize AWS SSM client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("[KILLSWITCH] Failed to load AWS config: %v (kill switch disabled)", err)
		return ks
	}
	ks.ssmClient = ssm.NewFromConfig(cfg)

	// Initial check
	ks.refresh()

	// Background refresh
	go ks.backgroundRefresh()

	return ks
}

// IsDisabled returns true if generation should be halted
func (ks *KillSwitch) IsDisabled() bool {
	if ks.ssmClient == nil {
		return false // Fail open if SSM not configured
	}

	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.disabled
}

// ForceRefresh triggers an immediate check (call this on rate limit violations)
func (ks *KillSwitch) ForceRefresh() {
	if ks.ssmClient == nil {
		return
	}
	ks.refresh()
}

func (ks *KillSwitch) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := ks.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: strPtr(parameterName),
	})

	ks.mu.Lock()
	defer ks.mu.Unlock()

	ks.lastCheck = time.Now()

	if err != nil {
		log.Printf("[KILLSWITCH] Failed to get parameter: %v", err)
		return
	}

	if output.Parameter != nil && output.Parameter.Value != nil {
		wasDisabled := ks.disabled
		ks.disabled = *output.Parameter.Value == "disabled"

		// Log state changes
		if ks.disabled && !wasDisabled {
			log.Printf("[KILLSWITCH] Generation DISABLED via kill switch")
		} else if !ks.disabled && wasDisabled {
			log.Printf("[KILLSWITCH] Generation ENABLED via kill switch")
		}
	}
}

func (ks *KillSwitch) backgroundRefresh() {
	ticker := time.NewTicker(cacheTTL)
	for range ticker.C {
		ks.refresh()
	}
}

func strPtr(s string) *string {
	return &s
}
