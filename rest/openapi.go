package rest

import (
	"fmt"
	"os"
	"path"

	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
	"gopkg.in/yaml.v3"
)

type OpenAPIInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type OpenAPIServer struct {
	URL string `yaml:"url"`
}

type OpenAPIParameter struct {
	Name        string         `yaml:"name"`
	In          string         `yaml:"in"`
	Required    bool           `yaml:"required"`
	Description string         `yaml:"description,omitempty"`
	Schema      map[string]any `yaml:"schema"`
}

type OpenAPIResponse struct {
	Description string         `yaml:"description"`
	Content     map[string]any `yaml:"content,omitempty"`
}

type OpenAPIOperation struct {
	Tags        []string                    `yaml:"tags"`
	Summary     string                      `yaml:"summary,omitempty"`
	OperationID string                      `yaml:"operationId"`
	Parameters  []any                       `yaml:"parameters,omitempty"`
	RequestBody map[string]any              `yaml:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `yaml:"responses"`
	Security    []map[string][]string       `yaml:"security,omitempty"`
}

type OpenAPIPathItem struct {
	Get    *OpenAPIOperation `yaml:"get,omitempty"`
	Post   *OpenAPIOperation `yaml:"post,omitempty"`
	Patch  *OpenAPIOperation `yaml:"patch,omitempty"`
	Delete *OpenAPIOperation `yaml:"delete,omitempty"`
}

type OpenAPIComponents struct {
	Schemas         map[string]any `yaml:"schemas"`
	SecuritySchemes map[string]any `yaml:"securitySchemes,omitempty"`
}

type OpenAPIDoc struct {
	OpenAPI    string                     `yaml:"openapi"`
	Info       OpenAPIInfo                `yaml:"info"`
	Servers    []OpenAPIServer            `yaml:"servers"`
	Paths      map[string]OpenAPIPathItem `yaml:"paths"`
	Components OpenAPIComponents          `yaml:"components"`
}

func GenerateOpenAPI(restDir string, proj *project.Project, entityTemplates []*RESTEntityTemplate) error {
	doc := OpenAPIDoc{
		OpenAPI: "3.1.0",
		Info: OpenAPIInfo{
			Title:   gcgstrings.ToCamelCase(proj.Identifier),
			Version: "1.0.0",
		},
		Servers: []OpenAPIServer{
			{URL: fmt.Sprintf("http://localhost:%s%s", proj.APIConfig.HTTPPort, proj.RESTConfig.BasePath)},
		},
		Paths: make(map[string]OpenAPIPathItem),
		Components: OpenAPIComponents{
			Schemas: make(map[string]any),
		},
	}

	// Add Security Scheme if auth enabled
	if proj.AuthConfig.Enabled {
		doc.Components.SecuritySchemes = map[string]any{
			"bearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		}
	}

	// Add generic Problem response schema (RFC 7807)
	doc.Components.Schemas["Problem"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":     map[string]any{"type": "string"},
			"title":    map[string]any{"type": "string"},
			"status":   map[string]any{"type": "integer"},
			"detail":   map[string]any{"type": "string"},
			"instance": map[string]any{"type": "string"},
		},
	}

	// Add Enums to components
	for _, enum := range proj.ProjectVersion.Enums {
		enumSchema := map[string]any{
			"type": "string",
			"enum": enumValuesList(enum),
		}
		doc.Components.Schemas[gcgstrings.ToCamelCase(enum.Identifier)] = enumSchema
	}

	// Add Entities and paths
	for _, et := range entityTemplates {
		entityName := et.Name

		// Construct entity schema properties
		properties := make(map[string]any)
		var required []string

		for _, field := range et.Fields {
			properties[field.Identifier()] = fieldToOpenAPISchema(field)
			if field.IsRequired() {
				required = append(required, field.Identifier())
			}
		}

		entitySchema := map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			entitySchema["required"] = required
		}
		doc.Components.Schemas[entityName] = entitySchema

		// Add List response envelope schema
		doc.Components.Schemas[entityName+"List"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"$ref": fmt.Sprintf("#/components/schemas/%s", entityName),
					},
				},
				"next_page_token": map[string]any{
					"type": "string",
				},
			},
		}

		// Setup route paths
		basePath := "/" + et.PluralPath

		// Helper maps for security
		var security []map[string][]string
		if proj.AuthConfig.Enabled {
			security = []map[string][]string{
				{"bearerAuth": []string{}},
			}
		}

		// Map single/composite keys for routes
		idParamInPath := "/{id}"
		idParameters := []any{
			OpenAPIParameter{
				Name:     "id",
				In:       "path",
				Required: true,
				Schema:   map[string]any{"type": "string"},
			},
		}
		if len(et.PrimaryKeys) > 1 {
			idParamInPath = ""
			idParameters = []any{}
			for _, pk := range et.PrimaryKeys {
				idParamInPath += fmt.Sprintf("/{%s}", pk.Identifier())
				idParameters = append(idParameters, OpenAPIParameter{
					Name:     pk.Identifier(),
					In:       "path",
					Required: true,
					Schema:   map[string]any{"type": "string"},
				})
			}
		}

		// 1. GET /plural
		listParams := []any{
			OpenAPIParameter{Name: "filter", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
			OpenAPIParameter{Name: "order_by", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
			OpenAPIParameter{Name: "page_size", In: "query", Required: false, Schema: map[string]any{"type": "integer"}},
			OpenAPIParameter{Name: "page_token", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
			OpenAPIParameter{Name: "include_fields", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
			OpenAPIParameter{Name: "exclude_fields", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
			OpenAPIParameter{Name: "skip_cache", In: "query", Required: false, Schema: map[string]any{"type": "boolean"}},
		}

		doc.Paths[basePath] = OpenAPIPathItem{
			Get: &OpenAPIOperation{
				Tags:        []string{entityName},
				Summary:     "List " + et.PluralPath,
				OperationID: "list" + entityName,
				Parameters:  listParams,
				Responses: map[string]OpenAPIResponse{
					"200": {
						Description: "List of " + et.PluralPath,
						Content: map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%sList", entityName)},
							},
						},
					},
					"400": {Description: "Bad Request", Content: problemResponseContent()},
					"500": {Description: "Internal Server Error", Content: problemResponseContent()},
				},
				Security: security,
			},
			Post: &OpenAPIOperation{
				Tags:        []string{entityName},
				Summary:     "Create " + entityName,
				OperationID: "create" + entityName,
				RequestBody: map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%s", entityName)},
						},
					},
				},
				Responses: map[string]OpenAPIResponse{
					"201": {
						Description: "Created " + entityName,
						Content: map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%s", entityName)},
							},
						},
					},
					"400": {Description: "Bad Request", Content: problemResponseContent()},
					"409": {Description: "Conflict", Content: problemResponseContent()},
					"500": {Description: "Internal Server Error", Content: problemResponseContent()},
				},
				Security: security,
			},
		}

		// 2. Route item: /plural/{id}
		doc.Paths[basePath+idParamInPath] = OpenAPIPathItem{
			Get: &OpenAPIOperation{
				Tags:        []string{entityName},
				Summary:     "Get " + entityName + " by ID",
				OperationID: "get" + entityName,
				Parameters:  idParameters,
				Responses: map[string]OpenAPIResponse{
					"200": {
						Description: "Successful operation",
						Content: map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%s", entityName)},
							},
						},
					},
					"404": {Description: "Not Found", Content: problemResponseContent()},
					"500": {Description: "Internal Server Error", Content: problemResponseContent()},
				},
				Security: security,
			},
			Patch: &OpenAPIOperation{
				Tags:        []string{entityName},
				Summary:     "Partially update " + entityName,
				OperationID: "patch" + entityName,
				Parameters:  idParameters,
				RequestBody: map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%s", entityName)},
						},
					},
				},
				Responses: map[string]OpenAPIResponse{
					"200": {
						Description: "Successful update",
						Content: map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": fmt.Sprintf("#/components/schemas/%s", entityName)},
							},
						},
					},
					"400": {Description: "Bad Request", Content: problemResponseContent()},
					"404": {Description: "Not Found", Content: problemResponseContent()},
					"409": {Description: "Version conflict", Content: problemResponseContent()},
					"500": {Description: "Internal Server Error", Content: problemResponseContent()},
				},
				Security: security,
			},
			Delete: &OpenAPIOperation{
				Tags:        []string{entityName},
				Summary:     "Delete " + entityName + " by ID",
				OperationID: "delete" + entityName,
				Parameters:  idParameters,
				Responses: map[string]OpenAPIResponse{
					"204": {Description: "No Content"},
					"404": {Description: "Not Found", Content: problemResponseContent()},
					"409": {Description: "Conflict", Content: problemResponseContent()},
					"500": {Description: "Internal Server Error", Content: problemResponseContent()},
				},
				Security: security,
			},
		}
	}

	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	// Mark as generated (this file is written directly, not via files.GenerateFile).
	yamlBytes = append([]byte("# "+files.GeneratedMarker+"\n"), yamlBytes...)

	// Canonical spec at the rest root for external consumers.
	if err = os.WriteFile(path.Join(restDir, "openapi.yaml"), yamlBytes, 0644); err != nil {
		return err
	}

	// Embedded copy next to the router so //go:embed openapi.yaml resolves
	// (go:embed cannot reference parent directories).
	return os.WriteFile(path.Join(restDir, "server", "openapi.yaml"), yamlBytes, 0644)
}

func enumValuesList(enum *nemgen.Enum) []string {
	var list []string
	for _, val := range enum.StaticValues {
		list = append(list, val.Identifier)
	}
	return list
}

func problemResponseContent() map[string]any {
	return map[string]any{
		"application/problem+json": map[string]any{
			"schema": map[string]any{
				"$ref": "#/components/schemas/Problem",
			},
		},
	}
}
