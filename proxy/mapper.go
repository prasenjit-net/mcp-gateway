package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func MapResponse(resp *http.Response, maxBytes int64) ([]MCPContent, error) {
	defer resp.Body.Close()

	lr := &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}

	truncated := int64(len(body)) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}

	ct := resp.Header.Get("Content-Type")
	mimeType := strings.Split(ct, ";")[0]
	mimeType = strings.TrimSpace(mimeType)

	switch {
	case mimeType == "application/json":
		var v interface{}
		text := string(body)
		if err := json.Unmarshal(body, &v); err == nil {
			if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
				text = string(pretty)
			}
		}
		if truncated {
			text += "\n[truncated]"
		}
		return []MCPContent{{Type: "text", Text: text}}, nil

	case strings.HasPrefix(mimeType, "text/"):
		text := string(body)
		if truncated {
			text += "\n[truncated]"
		}
		return []MCPContent{{Type: "text", Text: text}}, nil

	default:
		data := base64.StdEncoding.EncodeToString(body)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		result := MCPContent{
			Type:     "resource",
			MimeType: fmt.Sprintf("%s (status %d)", mimeType, resp.StatusCode),
			Data:     data,
		}
		if truncated {
			result.MimeType += " [truncated]"
		}
		return []MCPContent{result}, nil
	}
}
