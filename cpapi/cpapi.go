package cpapi

import (
	"bytes"
	"cpfeedman/config"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CpApi enables API calls to Check Point Security Management API

type CpApi struct {
	CheckPointServer      string // CHECKPOINT_SERVER - e.g. 192.168.100.100 or mtest1-6eno66u6.maas.checkpoint.com
	CheckPointCloudMgmtId string // CHECKPOINT_CLOUD_MGMT_ID - relevant for Smart=1 Cloud
	CheckPointApiKey      string // CHECKPOINT_API_KEY - API key for the Check Point management server

	Url string // URL for the Check Point API, constructed from CheckPointServer and CheckPointCloudMgmtId

	httpClient *http.Client // HTTP client for making API requests

	CheckPointSid          string    // SID for the Check Point session, used for authentication
	CheckPointSidExpiresAt time.Time // Timestamp when the SID expires, used for session management
}

func (cpApi *CpApi) LoadFromConfig(cfg *config.Config) {
	cpApi.CheckPointServer = cfg.CheckPointServer
	cpApi.CheckPointCloudMgmtId = cfg.CheckPointCloudMgmtId
	cpApi.CheckPointApiKey = cfg.CheckPointApiKey

	if cpApi.CheckPointCloudMgmtId != "" {
		cpApi.Url = "https://" + cpApi.CheckPointServer + "/" + cpApi.CheckPointCloudMgmtId + "/web_api/"
	} else {
		cpApi.Url = "https://" + cpApi.CheckPointServer + "/web_api/"
	}
}

func NewCpApiFromConfig(cfg *config.Config) *CpApi {
	cpApi := &CpApi{}
	cpApi.LoadFromConfig(cfg)

	// Create an HTTP client with a custom transport that skips TLS verification
	// TODO: make this configurable
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cpApi.httpClient = &http.Client{Transport: tr}

	return cpApi
}

func (cpApi *CpApi) ApiCallWithLogin(cmd string, payload *map[string]interface{}, headers *map[string]string) (string, error) {

	if cpApi.CheckPointSid == "" || time.Now().After(cpApi.CheckPointSidExpiresAt) {
		fmt.Println("SID is empty or expired, logging in...")
		_, err := cpApi.Login()
		if err != nil {
			return "", fmt.Errorf("failed to login to Check Point API: %w", err)
		}
	}
	return cpApi.ApiCall(cmd, payload, headers)
}

func (cpApi *CpApi) ApiCall(cmd string, payload *map[string]interface{}, headers *map[string]string) (string, error) {

	url := cpApi.Url + cmd

	if payload == nil {
		payload = &map[string]interface{}{}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// fmt.Println("Payload for API call:", string(payloadBytes))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")

	if cpApi.CheckPointSid != "" {
		req.Header.Add("X-chkp-sid", cpApi.CheckPointSid)
		fmt.Println("Using SID:", cpApi.CheckPointSid)
	}

	if headers != nil {
		for key, value := range *headers {
			req.Header.Add(key, value)
		}
	}

	resp, err := cpApi.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Response body:", bodyStr)
		return "", fmt.Errorf("CP API call failed with status: %s", resp.Status)
	}

	return bodyStr, nil
}

type LoginResponse struct {
	UID            string `json:"uid"`
	Sid            string `json:"sid"`
	URL            string `json:"url"`
	SessionTimeout int    `json:"session-timeout"`
	LastLoginWasAt struct {
		Posix   int64  `json:"posix"`
		Iso8601 string `json:"iso-8601"`
	} `json:"last-login-was-at"`
	APIServerVersion string `json:"api-server-version"`
	UserName         string `json:"user-name"`
	UserUID          string `json:"user-uid"`
}

func (cpApi *CpApi) Login() (*LoginResponse, error) {
	payload := map[string]interface{}{
		"api-key":         cpApi.CheckPointApiKey,
		"session-name":    "cpfeedman-session",
		"session-timeout": 60 * 60, // 1 hour
	}
	resp, err := cpApi.ApiCall("login", &payload, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to login to Check Point API: %w", err)
	}

	var loginResp LoginResponse
	err = json.Unmarshal([]byte(resp), &loginResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal login response: %w", err)
	}

	if loginResp.Sid != "" {
		cpApi.CheckPointSid = loginResp.Sid
		// add expiration time based on session timeout and current time, decrease by 5 minutes to allow for session expiration
		cpApi.CheckPointSidExpiresAt = time.Now().Add(time.Duration(loginResp.SessionTimeout-5*60) * time.Second)
		fmt.Println("Login successful, SID:", cpApi.CheckPointSid)
		fmt.Println("Now:", time.Now())
		fmt.Println("SID expires at:", cpApi.CheckPointSidExpiresAt)
	}

	return &loginResp, nil

}

func (cpApi *CpApi) Logout() (string, error) {

	resp, err := cpApi.ApiCall("logout", nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to logout to Check Point API: %w", err)
	}

	cpApi.CheckPointSid = ""
	// Reset the expiration time for the SID
	cpApi.CheckPointSidExpiresAt = time.Time{}

	return resp, nil
}

func (cpApi *CpApi) ShowHosts() (string, error) {
	payload := map[string]interface{}{
		"limit": 500, // Adjust limit as needed
	}
	resp, err := cpApi.ApiCallWithLogin("show-hosts", &payload, nil)
	if err != nil {
		return "", fmt.Errorf("failed to show hosts: %w", err)
	}

	return resp, nil
}

type ShowGatewaysResponse struct {
	Objects []struct {
		UID    string `json:"uid"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Domain struct {
			UID        string `json:"uid"`
			Name       string `json:"name"`
			DomainType string `json:"domain-type"`
		} `json:"domain"`
		Icon  string `json:"icon"`
		Color string `json:"color"`
	} `json:"objects"`
	From  int `json:"from"`
	To    int `json:"to"`
	Total int `json:"total"`
}

func (cpApi *CpApi) GatewayNames() ([]string, error) {
	payload := map[string]interface{}{
		"limit":         500,        // Adjust limit as needed
		"details-level": "standard", // Use "full" for more details
	}
	resp, err := cpApi.ApiCallWithLogin("show-simple-gateways", &payload, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to show gateways: %w", err)
	}

	var gatewaysResp ShowGatewaysResponse
	err = json.Unmarshal([]byte(resp), &gatewaysResp)
	if err != nil {
		return []string{}, fmt.Errorf("failed to unmarshal gateways response: %w", err)
	}

	gatewayNames := make([]string, 0, len(gatewaysResp.Objects))
	for _, gw := range gatewaysResp.Objects {
		gatewayNames = append(gatewayNames, gw.Name)
	}

	return gatewayNames, nil
}

type ShowNetworkFeedsResponse struct {
	Objects []struct {
		UID    string `json:"uid"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Domain struct {
			UID        string `json:"uid"`
			Name       string `json:"name"`
			DomainType string `json:"domain-type"`
		} `json:"domain"`
		Icon  string `json:"icon"`
		Color string `json:"color"`
	} `json:"objects"`
	From  int `json:"from"`
	To    int `json:"to"`
	Total int `json:"total"`
}

func (cpApi *CpApi) FeedNames() ([]string, error) {
	payload := map[string]interface{}{
		"limit":         500,        // Adjust limit as needed
		"details-level": "standard", // Use "full" for more details
	}
	resp, err := cpApi.ApiCallWithLogin("show-network-feeds", &payload, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to show feeds: %w", err)
	}

	var feedsResp ShowNetworkFeedsResponse
	err = json.Unmarshal([]byte(resp), &feedsResp)
	if err != nil {
		return []string{}, fmt.Errorf("failed to unmarshal feeds response: %w", err)
	}

	feedNames := make([]string, 0, len(feedsResp.Objects))
	for _, feed := range feedsResp.Objects {
		feedNames = append(feedNames, feed.Name)
	}

	return feedNames, nil
}

type RunScriptResponse struct {
	Tasks []struct {
		Target string `json:"target"`
		TaskID string `json:"task-id"`
	} `json:"tasks"`
}

func (resp *RunScriptResponse) GetTaskIds() []string {
	if resp == nil || len(resp.Tasks) == 0 {
		return []string{}
	}

	taskIds := make([]string, len(resp.Tasks))
	for i, task := range resp.Tasks {
		taskIds[i] = task.TaskID
	}

	return taskIds
}

func (cpApi *CpApi) RunScript(script string, scriptName string, targets []string) (*RunScriptResponse, error) {
	resp, err := cpApi.ApiCallWithLogin("run-script", &map[string]interface{}{
		"script":      script,
		"targets":     targets,
		"script-name": scriptName,
		"script-type": "one time",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to run script: %w", err)
	}

	var runScriptResp RunScriptResponse
	err = json.Unmarshal([]byte(resp), &runScriptResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal run script response: %w", err)
	}

	return &runScriptResp, nil
}

type TaskDetail struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Domain struct {
		UID        string `json:"uid"`
		Name       string `json:"name"`
		DomainType string `json:"domain-type"`
	} `json:"domain"`
	TaskID             string `json:"task-id"`
	TaskName           string `json:"task-name"`
	Status             string `json:"status"`
	ProgressPercentage int    `json:"progress-percentage"`
	StartTime          struct {
		Posix   int64  `json:"posix"`
		Iso8601 string `json:"iso-8601"`
	} `json:"start-time"`
	LastUpdateTime struct {
		Posix   int64  `json:"posix"`
		Iso8601 string `json:"iso-8601"`
	} `json:"last-update-time"`
	Suppressed  bool `json:"suppressed"`
	TaskDetails []struct {
		UID    string `json:"uid"`
		Domain struct {
			UID        string `json:"uid"`
			Name       string `json:"name"`
			DomainType string `json:"domain-type"`
		} `json:"domain"`
		Color             string `json:"color"`
		StatusCode        string `json:"statusCode"`
		StatusDescription string `json:"statusDescription"`
		TaskNotification  string `json:"taskNotification"`
		GatewayID         string `json:"gatewayId"`
		GatewayName       string `json:"gatewayName"`
		TransactionID     int    `json:"transactionId"`
		ResponseMessage   string `json:"responseMessage"`
		ResponseError     string `json:"responseError"`
		MetaInfo          struct {
			ValidationState string `json:"validation-state"`
			LastModifyTime  struct {
				Posix   int64  `json:"posix"`
				Iso8601 string `json:"iso-8601"`
			} `json:"last-modify-time"`
			LastModifier string `json:"last-modifier"`
			CreationTime struct {
				Posix   int64  `json:"posix"`
				Iso8601 string `json:"iso-8601"`
			} `json:"creation-time"`
			Creator string `json:"creator"`
		} `json:"meta-info"`
		Tags        []any  `json:"tags"`
		Icon        string `json:"icon"`
		Comments    string `json:"comments"`
		DisplayName string `json:"display-name"`
	} `json:"task-details"`
	Comments string `json:"comments"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
	Tags     []any  `json:"tags"`
	MetaInfo struct {
		Lock            string `json:"lock"`
		ValidationState string `json:"validation-state"`
		LastModifyTime  struct {
			Posix   int64  `json:"posix"`
			Iso8601 string `json:"iso-8601"`
		} `json:"last-modify-time"`
		LastModifier string `json:"last-modifier"`
		CreationTime struct {
			Posix   int64  `json:"posix"`
			Iso8601 string `json:"iso-8601"`
		} `json:"creation-time"`
		Creator string `json:"creator"`
	} `json:"meta-info"`
	ReadOnly         bool `json:"read-only"`
	AvailableActions struct {
		Edit   string `json:"edit"`
		Delete string `json:"delete"`
		Clone  string `json:"clone"`
	} `json:"available-actions"`
}

func (td *TaskDetail) GetTaskResponseMessage() string {
	if td == nil || len(td.TaskDetails) == 0 {
		return ""
	}

	// decode from base64 if needed
	encodedMessage := td.TaskDetails[0].ResponseMessage
	// fmt.Println("Encoded response message:", encodedMessage)
	decodedMessage, err := base64.StdEncoding.DecodeString(encodedMessage)
	if err == nil {
		return string(decodedMessage)
	}

	return ""
}

type ShowTasksResponse struct {
	Tasks []TaskDetail `json:"tasks"`
}

// tasks by status
func (resp *ShowTasksResponse) GetTasksByStatus() map[string]int {
	if resp == nil || len(resp.Tasks) == 0 {
		return map[string]int{}
	}

	tasksByStatus := make(map[string]int)
	for _, task := range resp.Tasks {
		tasksByStatus[task.Status]++
	}

	return tasksByStatus
}

// unfinished taskIds
func (resp *ShowTasksResponse) GetUnfinishedTaskIds() []string {
	if resp == nil || len(resp.Tasks) == 0 {
		return []string{}
	}

	unfinishedTaskIds := make([]string, 0)
	for _, task := range resp.Tasks {
		if task.Status == "in progress" {
			unfinishedTaskIds = append(unfinishedTaskIds, task.TaskID)
		}
	}

	return unfinishedTaskIds
}

// finished tasks
func (resp *ShowTasksResponse) GetFinishedTasksDetail() []TaskDetail {
	if resp == nil || len(resp.Tasks) == 0 {
		return []TaskDetail{}
	}

	finishedTaskDetails := make([]TaskDetail, 0)
	for _, task := range resp.Tasks {
		if task.Status == "succeeded" {
			finishedTaskDetails = append(finishedTaskDetails, task)
		}
	}

	return finishedTaskDetails
}

func (cpApi *CpApi) ShowTasks(taskIds []string) (*ShowTasksResponse, error) {
	payload := map[string]interface{}{
		"task-id":       taskIds,
		"details-level": "full", // Use "standard" for less details
	}
	resp, err := cpApi.ApiCallWithLogin("show-task", &payload, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to show tasks: %w", err)
	}

	var showTasksResp ShowTasksResponse
	err = json.Unmarshal([]byte(resp), &showTasksResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal show tasks response: %w", err)
	}

	return &showTasksResp, nil
}

func (cpApi *CpApi) KickFeed(feed string, targets []string) (*RunScriptResponse, error) {
	script := fmt.Sprintf("(echo '---'; date; echo \"%s\" ; dynamic_objects -efo_update \"%s\" ) | tee -a /var/log/kicked.log", feed, feed)
	resp, err := cpApi.RunScript(script, "kick feed "+feed, targets)
	if err != nil {
		return nil, fmt.Errorf("failed to kick feed %s: %w", feed, err)
	}
	return resp, nil
}
