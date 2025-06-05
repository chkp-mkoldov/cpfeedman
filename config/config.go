package config

import (
	"os"
	"strings"
)

// Config holds the configuration for the Check Point Feed Manager
// today it is populated from environment variables

type Config struct {
	// Check Point Security Management API
	CheckPointServer      string // CHECKPOINT_SERVER - e.g. 192.168.100.100 or mtest1-6eno66u6.maas.checkpoint.com
	CheckPointCloudMgmtId string // CHECKPOINT_CLOUD_MGMT_ID - relevant for Smart=1 Cloud
	CheckPointApiKey      string // CHECKPOINT_API_KEY - API key for the Check Point management server

	// AWS SQS Endpoint for CP Feed Manager
	CpFeedManSqsEndpoint string // CP_FEEDMAN_SQS_ENDPOINT - e.g. https://sqs.us-east-1.amazonaws.com/123456789012/cpfeedman

	CpFeedManNotifiedGateways []string // CP_FEEDMAN_NOTIFIED_GATEWAYS - comma-separated list of gateways to notify, e.g. gw10,gw20
}

// Load config from env variables
func (c *Config) LoadFromEnv() {
	if checkPointServer := os.Getenv("CHECKPOINT_SERVER"); checkPointServer != "" {
		c.CheckPointServer = checkPointServer
	}
	if checkPointCloudMgmtId := os.Getenv("CHECKPOINT_CLOUD_MGMT_ID"); checkPointCloudMgmtId != "" {
		c.CheckPointCloudMgmtId = checkPointCloudMgmtId
	}
	if checkPointApiKey := os.Getenv("CHECKPOINT_API_KEY"); checkPointApiKey != "" {
		c.CheckPointApiKey = checkPointApiKey
	}
	if cpFeedManSqsEndpoint := os.Getenv("CPFEEDMAN_SQS_ENDPOINT"); cpFeedManSqsEndpoint != "" {
		c.CpFeedManSqsEndpoint = cpFeedManSqsEndpoint
	}
	if cpFeedManNotifiedGateways := os.Getenv("CPFEEDMAN_NOTIFIED_GATEWAYS"); cpFeedManNotifiedGateways != "" {
		c.CpFeedManNotifiedGateways = splitCommaSeparated(cpFeedManNotifiedGateways)
	}
}

// splitCommaSeparated splits a comma-separated string into a slice of strings, trimming spaces.
func splitCommaSeparated(s string) []string {
	var result []string
	for _, part := range splitAndTrim(s, ",") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// splitAndTrim splits a string by a separator and trims spaces from each part.
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, p := range Split(s, sep) {
		trimmed := TrimSpace(p)
		parts = append(parts, trimmed)
	}
	return parts
}

func Split(s, sep string) []string {
	return strings.Split(s, sep)
}

func TrimSpace(s string) string {
	return strings.TrimSpace(s)
}
