// Package openapi embeds the generated OpenAPI 3 contract so hugi-api can
// serve it (GET /v1/hugimethara/openapi.json and .yaml) for the API reference.
package openapi

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"go.yaml.in/yaml/v3"
)

//go:embed openapi.yaml
var YAML []byte

var (
	jsonOnce sync.Once
	jsonDoc  []byte
	jsonErr  error
)

// JSON returns the contract as JSON, converted once from the embedded YAML.
func JSON() ([]byte, error) {
	jsonOnce.Do(func() {
		var doc any
		if err := yaml.Unmarshal(YAML, &doc); err != nil {
			jsonErr = fmt.Errorf("openapi: decode embedded YAML: %w", err)
			return
		}
		jsonDoc, jsonErr = json.Marshal(doc)
	})
	return jsonDoc, jsonErr
}
