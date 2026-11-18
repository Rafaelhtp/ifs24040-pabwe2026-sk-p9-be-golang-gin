package docs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/swaggo/swag"
)

func TestGetSwaggerJSON_DynamicHostAndScheme(t *testing.T) {
	var spec map[string]any
	if err := json.Unmarshal(GetSwaggerJSON("example.com:8081", "https"), &spec); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if spec["host"] != "example.com:8081" {
		t.Fatalf("unexpected host: %v", spec["host"])
	}
	schemes, _ := spec["schemes"].([]any)
	if len(schemes) != 1 || schemes[0] != "https" {
		t.Fatalf("unexpected schemes: %v", spec["schemes"])
	}

	var bare map[string]any
	if err := json.Unmarshal(GetSwaggerJSON("", ""), &bare); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := bare["host"]; ok {
		t.Fatal("host should be omitted")
	}
	if _, ok := bare["schemes"]; ok {
		t.Fatal("schemes should be omitted")
	}
}

func TestRender_InvalidJSON(t *testing.T) {
	if _, err := render([]byte("{bad"), "h", "http"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSpecFilesAndRegistration(t *testing.T) {
	if len(APIJSON()) == 0 {
		t.Fatal("api.json empty")
	}
	doc, err := swag.ReadDoc()
	if err != nil || doc == "" {
		t.Fatalf("swag doc not registered: %v", err)
	}
	for _, f := range []string{"swagger.json", "swagger.yaml", "api.json"} {
		b, err := os.ReadFile(f)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s missing: %v", f, err)
		}
	}
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	_ = json.Unmarshal(APIJSON(), &spec)
	count := 0
	for _, methods := range spec.Paths {
		count += len(methods)
	}
	if count != 19 {
		t.Fatalf("expected 19 operations, got %d", count)
	}
}
