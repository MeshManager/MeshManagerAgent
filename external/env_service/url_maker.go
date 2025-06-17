package env_service

import (
	"fmt"
	"log"
)

type URL string

const (
	RegisterAgent    URL = "RegisterAgent"    //고객 k8s에 설치된 agent 정보 등록 API
	SaveClusterState URL = "SaveClusterState" //고객 k8s의 클러스터 리소스 상태 확인 API
	CheckAgentStatus URL = "CheckAgentStatus" //고객 k8s에 설치된 agent와의 연결 확인 API
	YAML             URL = "getyaml"          //TODO backend 조정 필요
)

func MakeAgentURL(urlType URL) (string, error) {
	agentName, err := GetAgentName()
	if err != nil {
		return "", err
	}

	var baseUrl string
	switch urlType {
	case YAML:
		baseUrl, err = GetDesiredStateUrl()
	case SaveClusterState:
		baseUrl, err = GetClusterManagementUrl()
	default:
		baseUrl, err = GetAgentUrl()
	}
	if err != nil {
		return "", err
	}

	var fullURL string
	switch urlType {
	case RegisterAgent:
		fullURL = fmt.Sprintf("%s/register", baseUrl)
	case SaveClusterState:
		fullURL = fmt.Sprintf("%s/", baseUrl)
	case CheckAgentStatus:
		fullURL = fmt.Sprintf("%s/%s/status", baseUrl, agentName)
	case YAML:
		fullURL = fmt.Sprintf("%s/%s", baseUrl, agentName)
	default:
		return "", fmt.Errorf("지원하지 않는 URL 타입: %s", urlType)
	}

	log.Printf("생성된 fullURL: %s", fullURL)

	return fullURL, nil
}
