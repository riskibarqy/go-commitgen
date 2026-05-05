package llm

// Request captures model input independent of any specific provider protocol.
type Request struct {
	Model       string
	Prompt      string
	Temperature float64
	TopP        float64
	MaxTokens   int
}
