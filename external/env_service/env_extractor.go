package env_service

import (
	"fmt"
	"os"
	"strings"
)

func GetAgentUrl() (string, error) {
	agentUrl := os.Getenv("AGENT_URL")
	if agentUrl == "" {
		return "", fmt.Errorf("AGENT_URL 환경변수가 설정되지 않았거나 비어 있습니다")
	}

	if !strings.HasPrefix(agentUrl, "http") {
		return "", fmt.Errorf("agentUrl이 http 또는 https가 아닙니다")
	}

	return agentUrl, nil
}

func GetClusterManagementUrl() (string, error) {
	agentUrl := os.Getenv("CLUSTER_MANAGEMENT_URL")
	if agentUrl == "" {
		return "", fmt.Errorf("CLUSTER_MANAGEMENT_URL 환경변수가 설정되지 않았거나 비어 있습니다")
	}

	if !strings.HasPrefix(agentUrl, "http") {
		return "", fmt.Errorf("CLUSTER_MANAGEMENT_URL이 http 또는 https가 아닙니다")
	}

	return agentUrl, nil
}

func GetDesiredStateUrl() (string, error) {
	desiredStateUrl := os.Getenv("DESIRED_STATE_URL")
	if desiredStateUrl == "" {
		return "", fmt.Errorf("DESIRED_STATE_URL 환경변수가 설정되지 않았거나 비어 있습니다")
	}

	if !strings.HasPrefix(desiredStateUrl, "http") {
		return "", fmt.Errorf("desiredStateUrl이 http 또는 https가 아닙니다")
	}

	return desiredStateUrl, nil
}

func GetAgentUuid() (string, error) {
	agentUuid := os.Getenv("UUID")
	if agentUuid == "" {
		return "", fmt.Errorf("UUID 환경변수가 설정되지 않았거나 비어 있습니다")
	}
	return agentUuid, nil
}

func GetAgentName() (string, error) {
	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		return "", fmt.Errorf("AGENT_NAME 환경변수가 설정되지 않았거나 비어 있습니다")
	}
	return agentName, nil
}

func GetSlackWebHookUrl() (string, string, error) {
	slackWebHookUrl := os.Getenv("SLACK_WEB_HOOK_URL")
	if slackWebHookUrl == "" {
		return "", "", fmt.Errorf("SLACK_WEB_HOOK_URL 환경변수가 설정되지 않았거나 비어 있습니다")
	}

	parts := strings.Split(slackWebHookUrl, ":") // 예: "apikey:channelid"
	if len(parts) != 2 {
		return "", "", fmt.Errorf("SLACK_WEB_HOOK_URL 형식이 잘못되었습니다 (예: apikey,channelid)")
	}

	apiKey := strings.TrimSpace(parts[0])
	channelId := strings.TrimSpace(parts[1])
	return channelId, apiKey, nil
}
