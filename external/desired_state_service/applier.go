package desired_state_service

import (
	"context"
	"fmt"
	"github.com/MeshManager/MeshManagerAgent/external/env_service"
	"io"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	docs := strings.Split(yamlContent, "---")
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

		if err := m.Apply(ctx, obj); err != nil {
			return fmt.Errorf("리소스 적용 실패: %v", err)
		}
	}
	return nil
}
