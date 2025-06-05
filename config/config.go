package config

import (
	"os"
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
}
