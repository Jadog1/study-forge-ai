package config

// ModelPrice holds per-million-token pricing for an AI model in USD.
type ModelPrice struct {
	InputPerMillion  float64 `yaml:"input_per_million" json:"input_per_million"`
	OutputPerMillion float64 `yaml:"output_per_million" json:"output_per_million"`
}

// builtInPrices is a table of approximate well-known model prices in USD per
// million tokens. User-configured prices in Config.ModelPrices take precedence.
var builtInPrices = map[string]ModelPrice{
	// OpenAI chat models
	"gpt-4o":            {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4o-mini":       {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gpt-4o-2024-11-20": {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4-turbo":       {InputPerMillion: 10.00, OutputPerMillion: 30.00},
	"gpt-4":             {InputPerMillion: 30.00, OutputPerMillion: 60.00},
	"gpt-3.5-turbo":     {InputPerMillion: 0.50, OutputPerMillion: 1.50},
	"gpt-5-mini":        {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"o1":                {InputPerMillion: 15.00, OutputPerMillion: 60.00},
	"o1-mini":           {InputPerMillion: 3.00, OutputPerMillion: 12.00},
	"o3-mini":           {InputPerMillion: 1.10, OutputPerMillion: 4.40},
	// OpenAI embedding models
	"text-embedding-3-small": {InputPerMillion: 0.02},
	"text-embedding-3-large": {InputPerMillion: 0.13},
	"text-embedding-ada-002": {InputPerMillion: 0.10},
	// Anthropic Claude
	"claude-3-5-sonnet-20241022": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-5-haiku-20241022":  {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"claude-3-opus-20240229":     {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"claude-3-sonnet-20240229":   {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-haiku-20240307":    {InputPerMillion: 0.25, OutputPerMillion: 1.25},
	"claude-4-5-haiku":           {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"claude-4-5-sonnet":          {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	// Voyage AI embedding models
	"voyage-3-large":   {InputPerMillion: 0.18},
	"voyage-3":         {InputPerMillion: 0.06},
	"voyage-3-lite":    {InputPerMillion: 0.02},
	"voyage-finance-2": {InputPerMillion: 0.12},
	"voyage-code-3":    {InputPerMillion: 0.18},
}

// LookupModelPrice returns pricing for a model. Config-level overrides take
// precedence over built-in prices. Returns (0, 0, false) if no price is found.
func LookupModelPrice(model string, cfg *Config) (inputPPM, outputPPM float64, found bool) {
	if cfg != nil && cfg.ModelPrices != nil {
		if p, ok := cfg.ModelPrices[model]; ok {
			return p.InputPerMillion, p.OutputPerMillion, true
		}
	}
	if p, ok := builtInPrices[model]; ok {
		return p.InputPerMillion, p.OutputPerMillion, true
	}
	return 0, 0, false
}

// ComputeCost returns the USD cost given token counts and per-million prices.
func ComputeCost(inputTokens, outputTokens int, inputPPM, outputPPM float64) float64 {
	return float64(inputTokens)/1_000_000*inputPPM + float64(outputTokens)/1_000_000*outputPPM
}

// CostForTokens looks up pricing for model (with cfg overrides) and returns
// the computed cost in USD. Returns 0 if no pricing information is found.
func CostForTokens(model string, inputTokens, outputTokens int, cfg *Config) float64 {
	iPPM, oPPM, found := LookupModelPrice(model, cfg)
	if !found {
		return 0
	}
	return ComputeCost(inputTokens, outputTokens, iPPM, oPPM)
}

// SetModelPrice saves a custom price for model into cfg.ModelPrices and
// persists the updated config to disk.
func SetModelPrice(model string, inputPPM, outputPPM float64, cfg *Config) error {
	if cfg.ModelPrices == nil {
		cfg.ModelPrices = make(map[string]ModelPrice)
	}
	cfg.ModelPrices[model] = ModelPrice{InputPerMillion: inputPPM, OutputPerMillion: outputPPM}
	return Save(cfg)
}

// BuiltInModelPrices returns a copy of the built-in pricing table.
func BuiltInModelPrices() map[string]ModelPrice {
	out := make(map[string]ModelPrice, len(builtInPrices))
	for k, v := range builtInPrices {
		out[k] = v
	}
	return out
}
