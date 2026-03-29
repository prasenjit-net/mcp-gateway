package spec

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"mcp-gateway/store"
)

type ToolDefinition struct {
	Name               string
	Description        string
	InputSchema        map[string]interface{}
	OperationID        string
	SpecID             string
	Method             string
	PathTemplate       string
	Upstream           string
	PassthroughAuth    bool
	PassthroughCookies bool
	PassthroughHeaders []string
}

func ExtractTools(specID, specName, upstream string, parsed *ParsedSpec, passthroughAuth bool, passthroughCookies bool, passthroughHeaders []string) ([]*ToolDefinition, []*store.OperationRecord, error) {
	var tools []*ToolDefinition
	var ops []*store.OperationRecord

	doc := parsed.Doc
	if doc.Paths == nil {
		return tools, ops, nil
	}

	type entry struct {
		method string
		op     *openapi3.Operation
		path   string
	}

	for pathStr, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		var entries []entry
		if pathItem.Get != nil {
			entries = append(entries, entry{"GET", pathItem.Get, pathStr})
		}
		if pathItem.Post != nil {
			entries = append(entries, entry{"POST", pathItem.Post, pathStr})
		}
		if pathItem.Put != nil {
			entries = append(entries, entry{"PUT", pathItem.Put, pathStr})
		}
		if pathItem.Patch != nil {
			entries = append(entries, entry{"PATCH", pathItem.Patch, pathStr})
		}
		if pathItem.Delete != nil {
			entries = append(entries, entry{"DELETE", pathItem.Delete, pathStr})
		}
		if pathItem.Head != nil {
			entries = append(entries, entry{"HEAD", pathItem.Head, pathStr})
		}

		for _, e := range entries {
			op := e.op
			toolName := op.OperationID
			if toolName == "" {
				toolName = sanitizeName(e.method + "_" + e.path)
			}

			description := op.Summary
			if description == "" {
				description = op.Description
			}

			allParams := make(openapi3.Parameters, 0)
			allParams = append(allParams, pathItem.Parameters...)
			allParams = append(allParams, op.Parameters...)

			var bodySchema map[string]interface{}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				if mt, ok := op.RequestBody.Value.Content["application/json"]; ok && mt.Schema != nil && mt.Schema.Value != nil {
					bodySchema = schemaToMap(mt.Schema.Value)
				}
			}

			inputSchema := buildInputSchema(allParams, bodySchema)

			var tags []string
			if op.Tags != nil {
				tags = op.Tags
			}

			tool := &ToolDefinition{
				Name:               toolName,
				Description:        description,
				InputSchema:        inputSchema,
				OperationID:        toolName,
				SpecID:             specID,
				Method:             e.method,
				PathTemplate:       e.path,
				Upstream:           upstream,
				PassthroughAuth:    passthroughAuth,
				PassthroughCookies: passthroughCookies,
				PassthroughHeaders: passthroughHeaders,
			}
			tools = append(tools, tool)

			opID := fmt.Sprintf("%s-%s-%s", specID, strings.ToLower(e.method), sanitizeName(e.path))
			opRec := &store.OperationRecord{
				ID:          opID,
				SpecID:      specID,
				OperationID: toolName,
				Method:      e.method,
				Path:        e.path,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        tags,
				Enabled:     true,
			}
			ops = append(ops, opRec)
		}
	}

	return tools, ops, nil
}

func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.Trim(s, "_")
	return s
}

func buildInputSchema(params openapi3.Parameters, bodySchema map[string]interface{}) map[string]interface{} {
	properties := map[string]interface{}{}
	required := []string{}

	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		p := paramRef.Value
		var propSchema map[string]interface{}
		if p.Schema != nil && p.Schema.Value != nil {
			propSchema = schemaToMap(p.Schema.Value)
		} else {
			propSchema = map[string]interface{}{"type": "string"}
		}
		properties[p.Name] = propSchema
		if p.Required || p.In == "path" {
			required = append(required, p.Name)
		}
	}

	if bodySchema != nil {
		if bodyProps, ok := bodySchema["properties"].(map[string]interface{}); ok {
			for k, v := range bodyProps {
				properties[k] = v
			}
			if bodyRequired, ok := bodySchema["required"].([]interface{}); ok {
				for _, r := range bodyRequired {
					if rs, ok := r.(string); ok {
						required = append(required, rs)
					}
				}
			}
			if bodyRequired, ok := bodySchema["required"].([]string); ok {
				required = append(required, bodyRequired...)
			}
		} else {
			properties["body"] = bodySchema
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func schemaToMap(s *openapi3.Schema) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{"type": "string"}
	}
	m := map[string]interface{}{}
	if s.Type != nil {
		types := *s.Type
		if len(types) == 1 {
			m["type"] = types[0]
		} else if len(types) > 1 {
			m["type"] = []string(types)
		}
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Properties != nil {
		props := map[string]interface{}{}
		for k, v := range s.Properties {
			if v != nil && v.Value != nil {
				props[k] = schemaToMap(v.Value)
			}
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	return m
}
