package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func SendMetric(data map[string]interface{}) error {
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(
		"https://your-api-endpoint.com/resources",
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
