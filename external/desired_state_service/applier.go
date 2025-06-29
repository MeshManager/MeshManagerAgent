package desired_state_service

import (
	"context"
	"fmt"
	"github.com/MeshManager/MeshManagerAgent/external/env_service"
	"github.com/MeshManager/MeshManagerAgent/external/slack_metric_exporter"
	"io"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

type MetricServiceDynamic struct {
	dynamicClient dynamic.Interface
	restMapper    *restmapper.DeferredDiscoveryRESTMapper
}

func NewDynamicService(config *rest.Config) (*MetricServiceDynamic, error) {
	// 1. 캐시 설정
	discoveryCacheDir := "/tmp/k8s-discovery-cache"
	httpCacheDir := ""
	ttl := 10 * time.Minute

	// 2. Cached Discovery Client 생성
	cachedClient, err := disk.NewCachedDiscoveryClientForConfig(
		config,
		discoveryCacheDir,
		httpCacheDir,
		ttl,
	)
	if err != nil {
		return nil, fmt.Errorf("캐시된 discovery client 생성 실패: %v", err)
	}

	// 3. RESTMapper 생성 (CachedDiscoveryInterface 사용)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)

	// 4. Dynamic Client 생성
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("동적 클라이언트 생성 실패: %v", err)
	}

	return &MetricServiceDynamic{
		dynamicClient: dynamicClient,
		restMapper:    mapper,
	}, nil
}

func (m *MetricServiceDynamic) Apply(ctx context.Context, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	mapping, err := m.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("리소스 매핑 실패: %v", err)
	}

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = m.dynamicClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		dr = m.dynamicClient.Resource(mapping.Resource)
	}

	_, err = dr.Apply(
		ctx,
		obj.GetName(),
		obj,
		metav1.ApplyOptions{FieldManager: "metric-service"},
	)
	return err
}

func (m *MetricServiceDynamic) ApplyYAMLFromURL(ctx context.Context) error {

	logger := log.FromContext(ctx)

	// 1. 환경변수 조회 (외부 스코프 변수 선언)
	url, _ := env_service.MakeAgentURL(env_service.YAML)

	// 2. 환경변수 없을 경우 기본값 설정
	if url == "" {
		logger.Error(
			fmt.Errorf("DESIRED_STATE_URL 환경변수 누락"),
			"필수 값이 없어 기본 URL 사용",
		)
		url = "http://192.168.0.137:8080/yaml"
		logger.Info("기본 URL 설정 완료", "url", url)
	}

	// 3. YAML 다운로드
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("YAML 다운로드 실패[URL: %s]: %v", url, err)
	}
	defer resp.Body.Close()

	// 4. 데이터 읽기
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("YAML 데이터 읽기 실패: %v", err)
	}

	// logger.Info(string(data))

	// 5. 적용
	return m.ApplyYAML(ctx, string(data))
}

// ApplyYAML 추가: YAML 문자열 파싱 및 리소스 적용
func (m *MetricServiceDynamic) ApplyYAML(ctx context.Context, yamlContent string) error {
	// 1. 받은 istioroute 리소스 이름/네임스페이스 수집
	istioRoutes := make(map[string]map[string]struct{}) // ns -> name set

	docs := strings.Split(yamlContent, "---")
	var objs []*unstructured.Unstructured

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		obj := &unstructured.Unstructured{}
		decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := decoder.Decode([]byte(doc), nil, obj)
		if err != nil {
			return fmt.Errorf("YAML 파싱 실패: %v", err)
		}
		objs = append(objs, obj)

		if strings.ToLower(obj.GetKind()) == "istioroute" {
			ns := obj.GetNamespace()
			name := obj.GetName()
			if istioRoutes[ns] == nil {
				istioRoutes[ns] = make(map[string]struct{})
			}
			istioRoutes[ns][name] = struct{}{}
		}
	}

	slackChannel, slackAPIKEY, err := env_service.GetSlackWebHookUrl()
	logger := log.FromContext(ctx)
	if err != nil {
		logger.Info("slack 설정 안됨", err)
		slackChannel = "nil"
		slackAPIKEY = "nil"
	}

	if len(objs) == 0 {
		// 모든 IstioRoute 삭제
		if err := m.deleteAllIstioRoutes(ctx); err != nil {
			return fmt.Errorf("모든 IstioRoute 삭제 실패: %v", err)
		}
		return nil
	}

	// 2. 클러스터에서 현재 istioroute 목록 조회 및 삭제 대상 선별
	// GVR 얻기
	gvk := schema.GroupVersionKind{
		Group:   "mesh-manager.meshmanager.com", // 실제 Group/Version/Kind 확인 필요
		Version: "v1",
		Kind:    "IstioRoute",
	}
	mapping, err := m.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("istioroute 매핑 실패: %v", err)
	}

	// 네임스페이스별로
	for ns, names := range istioRoutes {
		dr := m.dynamicClient.Resource(mapping.Resource).Namespace(ns)
		list, err := dr.List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("istioroute 목록 조회 실패: %v", err)
		}
		for _, item := range list.Items {
			name := item.GetName()
			if _, found := names[name]; !found {

				// Slack에 삭제할 리소스 전송
				if slackChannel != "nil" && slackAPIKEY != "nil" {
					msg := fmt.Sprintf(":wastebasket: IstioRoute 삭제\n> *Namespace*: `%s`\n> *Name*: `%s`", ns, name)
					if slackErr := slack_metric_exporter.SendSlackMessage(slackAPIKEY, slackChannel, msg); slackErr != nil {
						logger.Info("Slack 알림 전송 실패: %v", slackErr)
					}
				}

				// 받은 YAML에 없는 istioroute는 삭제
				err := dr.Delete(ctx, name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("istioroute 삭제 실패: %v", err)
				}
			}
		}
	}

	// 3. 나머지 리소스 Apply
	for _, obj := range objs {
		if err := m.Apply(ctx, obj); err != nil {
			// Send Slack notification on apply failure
			msg := fmt.Sprintf(":exclamation: 리소스 적용 실패\n> *Type*: `%s`\n> *Namespace*: `%s`\n> *Name*: `%s`\n> *Error*: `%v`",
				obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
			if slackChannel != "nil" && slackAPIKEY != "nil" {
				if slackErr := slack_metric_exporter.SendSlackMessage(slackAPIKEY, slackChannel, msg); slackErr != nil {
					return fmt.Errorf("slack 알림 전송 실패: %v", slackErr)
				}
			}
			return fmt.Errorf("리소스 적용 실패: %v", err)
		} else {
			// Success case - send success notification
			if slackChannel != "nil" && slackAPIKEY != "nil" {
				logger.Info("Slack 정보 출력", "slackChannel", slackChannel, "slackAPIKEY", slackAPIKEY)

				msg := fmt.Sprintf(":white_check_mark: 리소스 적용 성공\n> *Type*: `%s`\n> *Namespace*: `%s`\n> *Name*: `%s`",
					obj.GetKind(), obj.GetNamespace(), obj.GetName())
				if slackErr := slack_metric_exporter.SendSlackMessage(slackAPIKEY, slackChannel, msg); slackErr != nil {
					logger.Info("Slack 알림 전송 실패", "error", slackErr)
					return nil
				}
			}
		}
	}
	return nil
}

func (m *MetricServiceDynamic) deleteAllIstioRoutes(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// 1. IstioRoute GVR(GroupVersionResource) 조회
	gvk := schema.GroupVersionKind{
		Group:   "mesh-manager.meshmanager.com",
		Version: "v1",
		Kind:    "IstioRoute",
	}
	mapping, err := m.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("istioroute 매핑 실패: %v", err)
	}

	// 2. Slack 설정 확인
	slackChannel, slackAPIKEY, slackErr := env_service.GetSlackWebHookUrl()
	if slackErr != nil {
		logger.Info("slack 설정 안됨", slackErr)
		slackChannel = "nil"
		slackAPIKEY = "nil"
	}

	// 3. 모든 네임스페이스에서 IstioRoute 리소스 조회
	dr := m.dynamicClient.Resource(mapping.Resource).Namespace(metav1.NamespaceAll)
	list, err := dr.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("istioroute 목록 조회 실패: %v", err)
	}

	// 4. 모든 IstioRoute 삭제
	for _, item := range list.Items {
		name := item.GetName()
		namespace := item.GetNamespace()

		// Slack 알림 전송
		if slackChannel != "nil" && slackAPIKEY != "nil" {
			msg := fmt.Sprintf(":wastebasket: IstioRoute 삭제\n> *Namespace*: `%s`\n> *Name*: `%s`",
				namespace, name)
			if err := slack_metric_exporter.SendSlackMessage(slackAPIKEY, slackChannel, msg); err != nil {
				logger.Info("Slack 알림 전송 실패", "error", err)
			}
		}

		// 리소스 삭제
		drNamespace := m.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
		if err := drNamespace.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("istioroute %s/%s 삭제 실패: %v", namespace, name, err)
		}
	}
	return nil
}
