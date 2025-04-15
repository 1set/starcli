package config

import (
	"os"

	"github.com/spf13/viper"
)

var (
	emptyStr string
)

// SetDefaults sets the default values in Viper for the configuration.
func SetDefaults() {
	host, _ := os.Hostname()
	viper.SetDefault("host_name", host)
	viper.SetDefault("resend_api_key", emptyStr)
	viper.SetDefault("sender_domain", emptyStr)
}

// GetHostname returns the host name from the configuration.
func GetHostname() string {
	return viper.GetString("host_name")
}

// GetResendAPIKey returns the resend API key from the configuration.
func GetResendAPIKey() string {
	return viper.GetString("resend_api_key")
}

// GetSenderDomain returns the email sender domain from the configuration.
func GetSenderDomain() string {
	return viper.GetString("sender_domain")
}

// GetOpenAIProvider returns the OpenAI services provider from the configuration.
func GetOpenAIProvider() string {
	return viper.GetString("openai_provider")
}

// GetOpenAIEndpoint returns the OpenAI services endpoint URL from the configuration.
func GetOpenAIEndpoint() string {
	return viper.GetString("openai_endpoint_url")
}

// GetOpenAIKey returns the OpenAI services API key from the configuration.
func GetOpenAIKey() string {
	return viper.GetString("openai_api_key")
}

// GetOpenAIGPTModel returns the OpenAI services GPT model name from the configuration.
func GetOpenAIGPTModel() string {
	return viper.GetString("openai_gpt_model")
}

// GetOpenAIDallEModel returns the OpenAI services DALL-E model name from the configuration.
func GetOpenAIDallEModel() string {
	return viper.GetString("openai_dalle_model")
}
