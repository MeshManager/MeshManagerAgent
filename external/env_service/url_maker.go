package env_service

import (
	"fmt"
)

type URL string

const (
	RegisterAgent    URL = "RegisterAgent"    //고객 k8s에 설치된 agent 정보 등록 API
	SaveClusterState URL = "SaveClusterState" //고객 k8s의 클러스터 리소스 상태 확인 API
	CheckAgentStatus URL = "CheckAgentStatus" //고객 k8s에 설치된 agent와의 연결 확인 API
)

func MakeAgentURL(urlType URL) (string, error) {
	agentUrl, err := GetAgentUrl()
	if err != nil {
		return "", err
	}

	agentName, err := GetAgentName()
	if err != nil {
		return "", err
	}

	var path string
	switch urlType {
	case RegisterAgent:
	case SaveClusterState:
	case CheckAgentStatus:
	default:
		return "", fmt.Errorf("지원하지 않는 URL 타입: %s", urlType)
	}

	fullURL := fmt.Sprintf("%s/%s%s", agentUrl, agentName, path)
	return fullURL, nil
}
