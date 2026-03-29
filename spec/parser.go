package spec

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
)

type ParsedSpec struct {
	Doc *openapi3.T
}

func Parse(data []byte) (*ParsedSpec, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}

	if err := doc.Validate(context.Background()); err != nil {
		return nil, err
	}

	return &ParsedSpec{Doc: doc}, nil
}
