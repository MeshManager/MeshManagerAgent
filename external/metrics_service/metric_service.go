package metrics_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/MeshManager/MeshManagerAgent/external/env_service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetricService struct {
	K8sClient client.Client
}

func New(client client.Client) *MetricService {
	return &MetricService{K8sClient: client}
}

func (s *MetricService) CollectAndSend(ctx context.Context) error {
	// 1. 네임스페이스 조회
	nsList, err := s.listNamespaces(ctx)
	if err != nil {
		return err
	}

	// 2. 모든 데이터 수집
	var allData []map[string]interface{}
	for _, ns := range nsList.Items {
		svcList, deployList, err := s.listResources(ctx, ns.Name)
		if err != nil {
			continue
		}

		payload := map[string]interface{}{
			"namespace":   ns.Name,
			"services":    ExtractServiceInfo(svcList),
			"deployments": ExtractDeploymentInfo(deployList),
		}
		allData = append(allData, payload)
	}

	var namespacesData interface{}
	if len(allData) == 0 {
		namespacesData = nil
	} else {
		namespacesData = allData
	}

	// 3. 해시 생성 로직 추가
	hash, err := GenerateHashFromNamespaces(namespacesData)
	if err != nil {
		return fmt.Errorf("해시 생성 실패: %v", err)
	}

	// 4. uuid 로딩
	uuid, err := env_service.GetAgentUuid()
	if err != nil {
		return fmt.Errorf("uuid 로딩 실패: %v", err)
	}

	// 5. 통합 데이터 전송
	return SendMetric(map[string]interface{}{
		"uuid":       uuid,
		"hash":       hash,
		"namespaces": namespacesData,
	})
}

// listNamespaces
func (s *MetricService) listNamespaces(ctx context.Context) (*corev1.NamespaceList, error) {
	list := &corev1.NamespaceList{}
	err := s.K8sClient.List(ctx, list, client.MatchingLabels{"istio-injection": "enabled"})
	return list, err
}

func (s *MetricService) listResources(ctx context.Context, namespace string) (*corev1.ServiceList, *appsv1.DeploymentList, error) {
	// 서비스 조회
	svcList := &corev1.ServiceList{}
	if err := s.K8sClient.List(ctx, svcList,
		client.InNamespace(namespace)); err != nil {
		return nil, nil, err
	}

	// 디플로이먼트 조회
	deployList := &appsv1.DeploymentList{}
	if err := s.K8sClient.List(ctx, deployList,
		client.InNamespace(namespace)); err != nil {
		return nil, nil, err
	}

	return svcList, deployList, nil
}

func SendMetric(data map[string]interface{}) error {

	agentUrl, err := env_service.GetAgentUrl()
	if err != nil {
		return fmt.Errorf("agentUrl 로딩 실패: %v", err)
	}
	fmt.Println(agentUrl)

	metricUrl, err := env_service.MakeAgentURL(env_service.checkAgentStatus)

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(
		agentUrl,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API 요청 실패: %s", resp.Status)
	}
	return nil
}

// InitConnectAgent to init connection to Backend
func InitConnectAgent() error {
	uuid, exists := os.LookupEnv("UUID")
	if !exists {
		return fmt.Errorf("UUID 환경변수가 필요합니다")
	}
	if uuid == "" {
		return fmt.Errorf("UUID 환경변수 값이 비어 있습니다")
	}

	agentName, exists := os.LookupEnv("AGENT_NAME")
	if !exists {
		return fmt.Errorf("AGENT_NAME 환경변수가 필요합니다")
	}
	if agentName == "" {
		return fmt.Errorf("AGENT_NAME 환경변수 값이 비어 있습니다")
	}

	agentUrl, exists := os.LookupEnv("AGENT_URL")
	if !exists {
		return fmt.Errorf("AGENT_URL 환경변수가 필요합니다")
	}
	if agentUrl == "" {
		return fmt.Errorf("AGENT_URL 환경변수 값이 비어 있습니다")
	}

	data := map[string]string{
		"name":      agentName,
		"clusterId": uuid,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON 마샬링 실패: %v", err)
	}

	resp, err := http.Post(
		agentUrl,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("POST 요청 실패: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API 요청 실패: %s", resp.Status)
	}

	return nil
}
