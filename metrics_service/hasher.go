package metrics_service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func generateHashFromNamespaces(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("JSON 마샬링 실패: %v", err)
	}

	hash := sha256.New()
	hash.Write(jsonData)
	return hex.EncodeToString(hash.Sum(nil)), nil
}
