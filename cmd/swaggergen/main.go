package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

func main() {
	input := flag.String("input", "", "Swagger 2.0 JSON input")
	output := flag.String("output", "", "OpenAPI 3 YAML output")
	flag.Parse()
	if *input == "" || *output == "" {
		fatalf("both --input and --output are required")
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fatalf("read Swagger document: %v", err)
	}
	data = bytes.ReplaceAll(data, []byte("#/definitions/dto."), []byte("#/components/schemas/"))
	data = bytes.ReplaceAll(data, []byte("#/definitions/"), []byte("#/components/schemas/"))
	data = bytes.ReplaceAll(data, []byte(`"dto.`), []byte(`"`))
	var swagger openapi2.T
	if err := json.Unmarshal(data, &swagger); err != nil {
		fatalf("parse Swagger document: %v", err)
	}
	spec, err := openapi2conv.ToV3(&swagger)
	if err != nil {
		fatalf("convert Swagger document to OpenAPI: %v", err)
	}
	spec.OpenAPI = "3.0.3"
	if len(spec.Servers) == 0 {
		spec.Servers = openapi3.Servers{{URL: "/"}}
	}
	if err := spec.Validate(context.Background()); err != nil {
		fatalf("validate OpenAPI document: %v", err)
	}
	encoded, err := yaml.Marshal(spec)
	if err != nil {
		fatalf("encode OpenAPI document: %v", err)
	}
	if err := os.WriteFile(*output, encoded, 0o644); err != nil {
		fatalf("write OpenAPI document: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
