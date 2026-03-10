package diag

import (
	"regexp"
	"strings"
)

// RedactionLevel controls how aggressive the redaction is.
type RedactionLevel int

const (
	// RedactStandard redacts common sensitive patterns.
	RedactStandard RedactionLevel = iota
	// RedactAggressive redacts more patterns including potential internal IPs.
	RedactAggressive
)

// Pre-compiled regex patterns at package level for efficiency.
var (
	// Standard patterns
	reAPIKey          = regexp.MustCompile(`(?i)(api[_-]?key|apikey)[\s:=]+["']?[\w-]{20,}["']?`)
	reBearerToken     = regexp.MustCompile(`(?i)bearer\s+[\w-\.]+`)
	reSlackToken      = regexp.MustCompile(`xox[baprs]-[\w-]+`)
	reGenericToken    = regexp.MustCompile(`(?i)(token|access_token|auth_token)[\s:=]+["']?[\w-]{20,}["']?`)
	reSecret          = regexp.MustCompile(`(?i)(secret|password|passwd|pwd)[\s:=]+["']?[^\s"']{8,}["']?`)
	rePrivateKey      = regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----`)
	reAWSKeyID        = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	reAWSSecretKey    = regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key[\s:=]+["']?[\w/+=]{40}["']?`)
	reGitHubToken     = regexp.MustCompile(`gh[pou]_[\w]{36}`) // Consolidated: ghp, gho, ghu
	reAnthropicKey   = regexp.MustCompile(`sk-ant-api[\w-]+`)
	reOpenAIKey       = regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`)
	reConnectionString = regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:[^@]+@`)
	reJWT             = regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`)
	reEmail           = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	reCreditCard      = regexp.MustCompile(`\b[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}\b`)

	// Aggressive patterns
	rePrivateIP10    = regexp.MustCompile(`\b10\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	rePrivateIP172   = regexp.MustCompile(`\b172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}\b`)
	rePrivateIP192   = regexp.MustCompile(`\b192\.168\.\d{1,3}\.\d{1,3}\b`)
	reLocalhost      = regexp.MustCompile(`\blocalhost\b`)
)

// redactionPattern defines a pattern to redact.
type redactionPattern struct {
	pattern     *regexp.Regexp
	replacement string
}

// standardPatterns returns the standard redaction patterns.
func standardPatterns() []redactionPattern {
	return []redactionPattern{
		{pattern: reAPIKey, replacement: "[REDACTED_API_KEY]"},
		{pattern: reBearerToken, replacement: "bearer [REDACTED_TOKEN]"},
		{pattern: reSlackToken, replacement: "[REDACTED_SLACK_TOKEN]"},
		{pattern: reGenericToken, replacement: "[REDACTED_TOKEN]"},
		{pattern: reSecret, replacement: "[REDACTED_SECRET]"},
		{pattern: rePrivateKey, replacement: "[REDACTED_PRIVATE_KEY]"},
		{pattern: reAWSKeyID, replacement: "[REDACTED_AWS_KEY]"},
		{pattern: reAWSSecretKey, replacement: "[REDACTED_AWS_SECRET]"},
		{pattern: reGitHubToken, replacement: "[REDACTED_GITHUB_TOKEN]"},
		{pattern: reAnthropicKey, replacement: "[REDACTED_ANTHROPIC_KEY]"},
		{pattern: reOpenAIKey, replacement: "[REDACTED_OPENAI_KEY]"},
		{pattern: reConnectionString, replacement: "$1://[REDACTED_USER]:[REDACTED_PASS]@"},
		{pattern: reJWT, replacement: "[REDACTED_JWT]"},
		{pattern: reEmail, replacement: "[REDACTED_EMAIL]"},
		{pattern: reCreditCard, replacement: "[REDACTED_CC]"},
	}
}

// aggressivePatterns returns additional patterns for aggressive redaction.
func aggressivePatterns() []redactionPattern {
	return []redactionPattern{
		{pattern: rePrivateIP10, replacement: "[REDACTED_IP]"},
		{pattern: rePrivateIP172, replacement: "[REDACTED_IP]"},
		{pattern: rePrivateIP192, replacement: "[REDACTED_IP]"},
		{pattern: reLocalhost, replacement: "[REDACTED_HOST]"},
	}
}

// Redactor handles sensitive information redaction.
type Redactor struct {
	patterns []redactionPattern
	level    RedactionLevel
}

// NewRedactor creates a new Redactor with the specified level.
func NewRedactor(level RedactionLevel) *Redactor {
	patterns := standardPatterns()
	if level == RedactAggressive {
		patterns = append(patterns, aggressivePatterns()...)
	}
	return &Redactor{
		patterns: patterns,
		level:    level,
	}
}

// Redact applies all redaction patterns to the input string.
func (r *Redactor) Redact(input string) string {
	result := input
	for _, rp := range r.patterns {
		result = rp.pattern.ReplaceAllString(result, rp.replacement)
	}
	return result
}

// RedactBytes applies redaction to a byte slice.
func (r *Redactor) RedactBytes(input []byte) []byte {
	return []byte(r.Redact(string(input)))
}

// RedactMapValues redacts values in a map that match sensitive keys.
func (r *Redactor) RedactMapValues(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		lowerKey := strings.ToLower(k)
		if isSensitiveKey(lowerKey) {
			result[k] = "[REDACTED]"
		} else {
			switch val := v.(type) {
			case string:
				result[k] = r.Redact(val)
			case map[string]any:
				result[k] = r.RedactMapValues(val)
			default:
				result[k] = v
			}
		}
	}
	return result
}

// isSensitiveKey checks if a key name suggests sensitive data.
func isSensitiveKey(key string) bool {
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"secret", "token", "api_key", "apikey",
		"private_key", "privatekey",
		"access_key", "accesskey",
		"auth", "credential", "cred",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(key, pattern) {
			return true
		}
	}
	return false
}

// Global default redactor for convenience
var defaultRedactor = NewRedactor(RedactStandard)

// Redact is a convenience function using the default redactor.
func Redact(input string) string {
	return defaultRedactor.Redact(input)
}

// RedactBytes is a convenience function using the default redactor.
func RedactBytes(input []byte) []byte {
	return defaultRedactor.RedactBytes(input)
}
