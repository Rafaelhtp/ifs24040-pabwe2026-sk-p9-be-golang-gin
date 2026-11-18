// Package docs menyediakan spesifikasi OpenAPI/Swagger Delcom Blog.
package docs

import (
	_ "embed"
	"encoding/json"

	"github.com/swaggo/swag"
)

//go:embed api.json
var apiJSON []byte

// APIJSON mengembalikan spesifikasi mentah tanpa host/scheme.
func APIJSON() []byte { return apiJSON }

func render(raw []byte, host, scheme string) ([]byte, error) {
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	if host != "" {
		spec["host"] = host
	}
	if scheme != "" {
		spec["schemes"] = []string{scheme}
	}
	return json.MarshalIndent(spec, "", "  ")
}

// GetSwaggerJSON mengembalikan spesifikasi dengan host dan scheme dinamis.
// Host/scheme kosong berarti dihilangkan sehingga Swagger UI memakai origin halaman.
func GetSwaggerJSON(host, scheme string) []byte {
	out, _ := render(apiJSON, host, scheme)
	return out
}

type swaggerSpec struct{}

func (swaggerSpec) ReadDoc() string { return string(GetSwaggerJSON("", "")) }

func init() {
	swag.Register(swag.Name, &swaggerSpec{})
}
