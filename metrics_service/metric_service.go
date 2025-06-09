package metrics_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	// 3. 통합 데이터 전송
	return SendMetric(map[string]interface{}{"namespaces": namespacesData})
}

// Helper functions
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

	url, exists := os.LookupEnv("AGENTURL")
	if !exists {
		_ = fmt.Errorf("AGENTURL 환경변수가 필요합니다")
	}
	if url == "" {
		_ = fmt.Errorf("AGENTURL 환경변수 값이 비어 있습니다")
	}
	fmt.Println(url)

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(
		url,
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
