package metrics_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"net/http"
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

	// 2. 각 네임스페이스 처리
	for _, ns := range nsList.Items {
		svcList, deployList, err := s.listResources(ctx, ns.Name)
		if err != nil {
			continue
		}

		// 3. 데이터 전송
		if err := s.sendData(ns.Name, svcList, deployList); err != nil {
			return err
		}
	}
	return nil
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

func (s *MetricService) sendData(namespace string, svcList *corev1.ServiceList, deployList *appsv1.DeploymentList) error {
	payload := map[string]interface{}{
		"namespace":   namespace,
		"services":    ExtractServiceInfo(svcList),
		"deployments": ExtractDeploymentInfo(deployList),
	}
	return SendMetric(payload)
}

func SendMetric(data map[string]interface{}) error {
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(
		"https://192.168.0.137:8080/resources",
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
