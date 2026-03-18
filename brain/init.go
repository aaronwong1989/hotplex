package brain

import (
	"context"
	"log/slog"
	"time"

	"github.com/hrygo/hotplex/brain/llm"
)

// Init initializes the global Brain from environmental variables.
// It detects the provider and sets the Global Brain instance.
//
// IMPORTANT: This function MUST be called before using any Brain-dependent features:
//   - GlobalIntentRouter() requires Global() to be non-nil
//   - GlobalCompressor() requires Global() to be non-nil
//   - GlobalGuard() requires Global() to be non-nil
//
// If HOTPLEX_BRAIN_API_KEY is not set, Brain is disabled and features gracefully degrade.
func Init(logger *slog.Logger) error {
	config := LoadConfigFromEnv()

	if !config.Enabled {
		logger.Debug("Native Brain is disabled or missing configuration. Skipping.")
		return nil
	}

	var baseClient llm.LLMClient

	// 1. Initialize base client based on Protocol
	switch config.Model.Protocol {
	case "anthropic":
		baseClient = llm.NewAnthropicClient("", config.Model.Endpoint, config.Model.Model, logger)
		logger.Info("Anthropic brain client initialized", "model", config.Model.Model)
	case "openai":
		fallthrough
	default:
		// Default to OpenAI compatible client
		baseClient = llm.NewOpenAIClient("", config.Model.Endpoint, config.Model.Model, logger)
		logger.Debug("OpenAI brain client initialized", "model", config.Model.Model)
	}

	// 2. Initialize orchestration & observability components (shared)
	var metricsCollector *llm.MetricsCollector
	if config.Metrics.Enabled {
		metricsCollector = llm.NewMetricsCollector(llm.MetricsConfig{
			Enabled:           true,
			ServiceName:       config.Metrics.ServiceName,
			MaxLatencySamples: 1000,
		})
	}

	var costCalculator *llm.CostCalculator
	if config.Cost.Enabled {
		costCalculator = llm.NewCostCalculator()
	}

	var rateLimiter *llm.RateLimiter
	if config.RateLimit.Enabled {
		rateLimiter = llm.NewRateLimiter(llm.RateLimitConfig{
			RequestsPerSecond: config.RateLimit.RPS,
			BurstSize:         config.RateLimit.Burst,
			MaxQueueSize:      config.RateLimit.QueueSize,
			QueueTimeout:      config.RateLimit.QueueTimeout,
			PerModel:          config.RateLimit.PerModel,
		})
	}

	var router *llm.Router
	if config.Router.Enabled {
		modelConfigs := config.Router.Models
		if len(modelConfigs) == 0 {
			pricing := llm.DefaultModelPricing()
			for _, p := range pricing {
				modelConfigs = append(modelConfigs, llm.ModelConfig{
					Name:            p.ModelName,
					Provider:        p.Provider,
					CostPer1KInput:  p.CostPer1KInput,
					CostPer1KOutput: p.CostPer1KOutput,
					Enabled:         true,
				})
			}
		}

		router = llm.NewRouter(llm.RouterConfig{
			DefaultStrategy:  llm.RouteStrategy(config.Router.DefaultStage),
			Models:           modelConfigs,
			ScenarioModelMap: make(map[llm.Scenario]string),
			FallbackModel:    config.Model.Model,
			Logger:           logger,
		}, metricsCollector)
	}

	// 3. Apply shared middleware wrapping
	client := baseClient

	// Retry
	client = llm.NewRetryClient(client, config.Retry.MaxAttempts, config.Retry.MinWaitMs, config.Retry.MaxWaitMs)

	// Cache
	if config.Cache.Enabled && config.Cache.Size > 0 {
		client = llm.NewCachedClient(client, config.Cache.Size)
	}

	// Rate limiting
	if rateLimiter != nil {
		client = llm.NewRateLimitedClient(client, rateLimiter)
	}

	// 4. Register global brain instance
	SetGlobal(&enhancedBrainWrapper{
		client:         client,
		config:         config,
		metrics:        metricsCollector,
		costCalculator: costCalculator,
		router:         router,
		rateLimiter:    rateLimiter,
		logger:         logger,
	})

	// 5. Initialize specialized brain components
	if config.IntentRouter.Enabled {
		InitIntentRouter(IntentRouterConfig{
			Enabled:             config.IntentRouter.Enabled,
			ConfidenceThreshold: config.IntentRouter.ConfidenceThreshold,
			CacheSize:           config.IntentRouter.CacheSize,
		}, logger)
	}

	if config.Memory.Enabled {
		sessionTTL, _ := time.ParseDuration(config.Memory.SessionTTL)
		if sessionTTL == 0 {
			sessionTTL = 24 * time.Hour
		}
		InitMemory(CompressionConfig{
			Enabled:          config.Memory.Enabled,
			TokenThreshold:   config.Memory.TokenThreshold,
			TargetTokenCount: config.Memory.TargetTokenCount,
			PreserveTurns:    config.Memory.PreserveTurns,
			MaxSummaryTokens: config.Memory.MaxSummaryTokens,
			CompressionRatio: config.Memory.CompressionRatio,
			SessionTTL:       sessionTTL,
		}, logger)
	}

	if config.Guard.Enabled {
		if err := InitGuard(GuardConfig{
			Enabled:            config.Guard.Enabled,
			InputGuardEnabled:  config.Guard.InputGuardEnabled,
			OutputGuardEnabled: config.Guard.OutputGuardEnabled,
			Chat2ConfigEnabled: config.Guard.Chat2ConfigEnabled,
			MaxInputLength:     config.Guard.MaxInputLength,
			ScanDepth:          config.Guard.ScanDepth,
			Sensitivity:        config.Guard.Sensitivity,
			AdminUsers:         config.Guard.AdminUsers,
			AdminChannels:      config.Guard.AdminChannels,
			ResponseTimeout:    config.Guard.ResponseTimeout,
			RateLimitRPS:       config.Guard.RateLimitRPS,
			RateLimitBurst:     config.Guard.RateLimitBurst,
		}, logger); err != nil {
			logger.Warn("Failed to initialize SafetyGuard", "error", err)
		}
	}

	logger.Info("Native Brain initialized",
		"provider", config.Model.Provider,
		"protocol", config.Model.Protocol,
		"model", config.Model.Model,
		"cache", config.Cache.Enabled,
		"metrics", config.Metrics.Enabled,
		"intent_router", config.IntentRouter.Enabled)

	return nil
}

// enhancedBrainWrapper satisfies Brain, StreamingBrain, RoutableBrain, and ObservableBrain interfaces.
type enhancedBrainWrapper struct {
	client interface {
		Chat(ctx context.Context, prompt string) (string, error)
		Analyze(ctx context.Context, prompt string, target any) error
		ChatStream(ctx context.Context, prompt string) (<-chan string, error)
		HealthCheck(ctx context.Context) HealthStatus
	}
	config         Config
	metrics        *llm.MetricsCollector
	costCalculator *llm.CostCalculator
	router         *llm.Router
	rateLimiter    *llm.RateLimiter
	logger         *slog.Logger
}

func (w *enhancedBrainWrapper) Chat(ctx context.Context, prompt string) (string, error) {
	return w.ChatWithModel(ctx, "", prompt)
}

func (w *enhancedBrainWrapper) Analyze(ctx context.Context, prompt string, target any) error {
	return w.AnalyzeWithModel(ctx, "", prompt, target)
}

func (w *enhancedBrainWrapper) ChatWithModel(ctx context.Context, model string, prompt string) (string, error) {
	// Apply timeout from config
	if w.config.Model.TimeoutS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(w.config.Model.TimeoutS)*time.Second)
		defer cancel()
	}

	// Select model via router if not specified
	if model == "" && w.router != nil {
		scenario := w.router.DetectScenario(prompt)
		strategy := llm.StrategyCostPriority // Default strategy
		if w.router.GetDefaultStrategy() != "" {
			strategy = w.router.GetDefaultStrategy()
		}
		selectedModel, err := w.router.SelectModel(ctx, scenario, strategy)
		if err == nil {
			model = selectedModel.Name
		} else if w.logger != nil {
			w.logger.Warn("Model selection failed, using default", "error", err)
		}
	}

	// Use default model if still empty
	if model == "" {
		model = w.config.Model.Model
	}

	// Apply rate limiting
	if w.rateLimiter != nil {
		if err := w.rateLimiter.WaitModel(ctx, model); err != nil {
			return "", err
		}
	}

	// Start metrics timer
	var timer *llm.RequestTimer
	if w.metrics != nil {
		timer = llm.NewRequestTimer(w.metrics, model, "chat")
	}

	// Execute request
	result, err := w.client.Chat(ctx, prompt)

	// Record metrics
	if timer != nil {
		inputTokens := w.costCalculator.CountTokens(prompt)
		outputTokens := w.costCalculator.CountTokens(result)
		cost := 0.0
		if w.costCalculator != nil {
			cost, _ = w.costCalculator.CalculateCost(model, inputTokens, outputTokens)
			_, _, _ = w.costCalculator.TrackRequest("default", model, inputTokens, outputTokens)
		}
		timer.Record(int64(inputTokens), int64(outputTokens), cost, err)
	}

	return result, err
}

func (w *enhancedBrainWrapper) AnalyzeWithModel(ctx context.Context, model string, prompt string, target any) error {
	// Apply timeout from config
	if w.config.Model.TimeoutS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(w.config.Model.TimeoutS)*time.Second)
		defer cancel()
	}

	// Select model via router if not specified
	if model == "" && w.router != nil {
		scenario := llm.ScenarioAnalyze
		strategy := llm.StrategyCostPriority // Default strategy
		if w.router.GetDefaultStrategy() != "" {
			strategy = w.router.GetDefaultStrategy()
		}
		selectedModel, err := w.router.SelectModel(ctx, scenario, strategy)
		if err == nil {
			model = selectedModel.Name
		} else if w.logger != nil {
			w.logger.Warn("Model selection failed, using default", "error", err)
		}
	}

	// Use default model if still empty
	if model == "" {
		model = w.config.Model.Model
	}

	// Apply rate limiting
	if w.rateLimiter != nil {
		if err := w.rateLimiter.WaitModel(ctx, model); err != nil {
			return err
		}
	}

	// Start metrics timer
	var timer *llm.RequestTimer
	if w.metrics != nil {
		timer = llm.NewRequestTimer(w.metrics, model, "analyze")
	}

	// Execute request
	err := w.client.Analyze(ctx, prompt, target)

	// Record metrics
	if timer != nil {
		inputTokens := w.costCalculator.CountTokens(prompt)
		outputTokens := 100 // Estimate for structured output
		cost := 0.0
		if w.costCalculator != nil {
			cost, _ = w.costCalculator.CalculateCost(model, inputTokens, outputTokens)
			_, _, _ = w.costCalculator.TrackRequest("default", model, inputTokens, outputTokens)
		}
		timer.Record(int64(inputTokens), int64(outputTokens), cost, err)
	}

	return err
}

func (w *enhancedBrainWrapper) ChatStream(ctx context.Context, prompt string) (<-chan string, error) {
	// Apply timeout from config
	if w.config.Model.TimeoutS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(w.config.Model.TimeoutS)*time.Second)
		defer cancel()
	}

	// Apply rate limiting
	if w.rateLimiter != nil {
		if err := w.rateLimiter.WaitModel(ctx, w.config.Model.Model); err != nil {
			return nil, err
		}
	}

	return w.client.ChatStream(ctx, prompt)
}

func (w *enhancedBrainWrapper) HealthCheck(ctx context.Context) HealthStatus {
	return w.client.HealthCheck(ctx)
}

func (w *enhancedBrainWrapper) GetMetrics() llm.MetricsStats {
	if w.metrics == nil {
		return llm.MetricsStats{}
	}
	return w.metrics.GetStats()
}

func (w *enhancedBrainWrapper) GetCostCalculator() *llm.CostCalculator {
	return w.costCalculator
}

func (w *enhancedBrainWrapper) GetRouter() *llm.Router {
	return w.router
}

func (w *enhancedBrainWrapper) GetRateLimiter() *llm.RateLimiter {
	return w.rateLimiter
}
