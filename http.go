package openai

// types and functions for HTTP requests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"os"
	"strings"
)

const (
	baseURL            = "https://api.openai.com/v1"
	defaultContentType = "application/json"

	kContentType        = "Content-Type"
	kContentDisposition = "Content-Disposition"
	kAuthorization      = "Authorization"
	kOrganization       = "OpenAI-Organization"
	kBeta               = "OpenAI-Beta"
)

var (
	StreamData = []byte("data: ")
	StreamDone = []byte("[DONE]")
)

// isSuccessStatus checks if HTTP status code indicates success
func isSuccessStatus(code int) bool {
	return code >= 200 && code < 300
}

// CommonResponse struct for responses with common properties
type CommonResponse struct {
	Object *string `json:"object,omitempty"`
	Error  *Error  `json:"error,omitempty"`
	Type   *string `json:"type,omitempty"`
}

// Error struct for response error property
type Error struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   any     `json:"param,omitempty"`
	Code    *string `json:"code,omitempty"`
}

// GeminiError struct for gemini response error
type GeminiError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Usage struct for reponses
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type callback func(response ChatCompletion, done bool, err error)

// err converts `Error` to `error`.
func (e *Error) err() error {
	es := map[string]any{
		"type":    e.Type,
		"message": e.Message,
	}
	if e.Code != nil {
		es["code"] = *e.Code
	}
	if e.Param != nil {
		es["param"] = e.Param
	}

	if bytes, err := json.Marshal(es); err == nil {
		return fmt.Errorf(string(bytes))
	} else {
		return fmt.Errorf(fmt.Sprintf("%+v", es))
	}
}

func streamWithCtx(ctx context.Context, res *http.Response, cb callback) {
	defer res.Body.Close()

	fn := ToolCall{Type: "function"}

	scanner := bufio.NewScanner(res.Body)
	toolIndex := 0
	toolCalls := []ToolCall{}
	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			cb(ChatCompletion{}, true, ctx.Err())
			return
		default:
		}

		var entry ChatCompletion
		b := scanner.Bytes()
		switch {
		case len(b) == 0:
			continue
		case bytes.HasPrefix(b, StreamData):
			if bytes.HasSuffix(b, StreamDone) {
				if len(entry.Choices) <= 0 {
					entry.Choices = []ChatCompletionChoice{
						{Message: ChatMessage{ToolCalls: []ToolCall{}}},
					}
				}

				cb(entry, true, nil)
				return
			}
			if err := json.Unmarshal(b[len(StreamData):], &entry); err != nil {
				cb(entry, true, err)
				return
			}
			if entry.Type != nil {
				entryType := *entry.Type
				if entryType == "ping" {
					continue
				}
			}

			// Safe access to entry.Choices and tool calls
			if len(entry.Choices) > 0 && len(entry.Choices[0].Delta.ToolCalls) > 0 {
				toolCall := entry.Choices[0].Delta.ToolCalls[0]
				// if there are multiple tools in the response, detect a change in index
				if toolCall.Index != nil && *toolCall.Index != toolIndex {
					toolCalls = append(toolCalls, fn)
					toolIndex++
					fn = ToolCall{Type: "function", Index: &toolIndex}
				}

				if toolCall.ID != "" {
					fn.ID = toolCall.ID
				}

				if toolCall.Function.Name != "" {
					fn.Function.Name = toolCall.Function.Name
				} else if toolCall.Function.Arguments != "" {
					fn.Function.Arguments = fn.Function.Arguments + toolCall.Function.Arguments
				}
			}
			// Safe access to finish reason
			if len(entry.Choices) > 0 && (entry.Choices[0].FinishReason == "tool_calls" ||
				(entry.Choices[0].FinishReason == "stop" && fn.ID != "")) {
				// append last function call
				toolCalls = append(toolCalls, fn)
				entry.Choices[0].Message.ToolCalls = toolCalls

				cb(entry, false, nil)
				cb(entry, true, nil)

				return
			}
			cb(entry, false, nil)
		}
	}
	// Check for scanner error
	if err := scanner.Err(); err != nil {
		cb(ChatCompletion{}, true, err)
	}
}

// postCBResponses sends HTTP POST request with streaming callback for responses API
func (c *Client) postCBResponses(endpoint string, params map[string]any, cb responseCallback) (response []byte, err error) {
	return c.postCBResponsesWithContext(context.Background(), endpoint, params, cb)
}

// postCBResponsesWithContext sends HTTP POST request with streaming callback and context for responses API
func (c *Client) postCBResponsesWithContext(ctx context.Context, endpoint string, params map[string]any, cb responseCallback) (response []byte, err error) {
	if params == nil {
		params = map[string]any{}
	}
	url := baseURL
	if c.baseURL != nil {
		url = *c.baseURL
	}
	apiURL := fmt.Sprintf("%s/%s", url, endpoint)

	var req *http.Request
	// application/json
	var serialized []byte
	if serialized, err = json.Marshal(params); err == nil {
		if req, err = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(serialized)); err != nil {
			return nil, fmt.Errorf("failed to create application/json request: %s", err)
		}

		// set content-type header
		req.Header.Set(kContentType, defaultContentType)
	}

	// set authentication headers
	req.Header.Set(kAuthorization, fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set(kOrganization, c.OrganizationID)

	if c.Verbose {
		if dumped, err := httputil.DumpRequest(req, true); err == nil {
			log.Printf("dump request:\n\n%s", string(dumped))
		}
	}

	// send request and return response bytes
	var resp *http.Response
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		defer resp.Body.Close()
		errbody := struct {
			Error Error `json:"error"`
		}{}
		if err := json.NewDecoder(resp.Body).Decode(&errbody); err != nil {
			return nil, fmt.Errorf("failed to decode error body: %v", err)
		}
		return nil, errbody.Error.err()
	}

	go streamResponsesWithCtx(ctx, resp, cb)

	return nil, nil
}

// streamResponsesWithCtx handles streaming responses for the responses API
func streamResponsesWithCtx(ctx context.Context, res *http.Response, cb responseCallback) {
	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			cb(ResponseStreamEvent{}, true, ctx.Err())
			return
		default:
		}

		b := scanner.Bytes()
		if len(b) == 0 {
			continue
		}

		// Skip event: lines
		if bytes.HasPrefix(b, []byte("event:")) {
			continue
		}

		// Process data: lines
		if bytes.HasPrefix(b, []byte("data: ")) {
			dataBytes := bytes.TrimPrefix(b, []byte("data: "))

			// Check for [DONE] marker
			if bytes.Equal(dataBytes, []byte("[DONE]")) {
				cb(ResponseStreamEvent{}, true, nil)
				return
			}

			// Parse JSON event
			var event ResponseStreamEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				cb(ResponseStreamEvent{}, true, err)
				return
			}

			// Check if this is a completion event
			done := event.Type == "response.completed" || event.Type == "response.failed" || event.Type == "response.cancelled"
			cb(event, done, nil)

			if done {
				return
			}
		}
	}

	// Check for scanner error
	if err := scanner.Err(); err != nil {
		cb(ResponseStreamEvent{}, true, err)
	}
}

// FileParam struct for multipart requests
type FileParam struct {
	bs []byte
}

// NewFileParamFromBytes returns a new FileParam with given bytes
func NewFileParamFromBytes(bs []byte) FileParam {
	return FileParam{
		bs: bs,
	}
}

// NewFileParamFromFilepath returns a new FileParam with bytes read from given filepath
func NewFileParamFromFilepath(path string) (f FileParam, err error) {
	var bs []byte
	if bs, err = os.ReadFile(path); err == nil {
		return FileParam{
			bs: bs,
		}, nil
	}
	return FileParam{}, err
}

// sends HTTP request with context
func (c *Client) doWithContext(ctx context.Context, method, endpoint string, params map[string]any) (response []byte, err error) {
	if params == nil {
		params = map[string]any{}
	}
	url := baseURL
	if c.baseURL != nil {
		url = *c.baseURL
	}
	apiURL := fmt.Sprintf("%s/%s", url, endpoint)

	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, method, apiURL, nil); err == nil {
		// parameters
		queries := req.URL.Query()
		for k, v := range params {
			queries.Add(k, fmt.Sprintf("%+v", v))
		}
		req.URL.RawQuery = queries.Encode()

		// headers
		req.Header.Set(kAuthorization, fmt.Sprintf("Bearer %s", c.APIKey))
		req.Header.Set(kOrganization, c.OrganizationID)
		if c.beta != nil {
			req.Header.Set(kBeta, *c.beta)
		}

		if c.Verbose {
			if dumped, err := httputil.DumpRequest(req, true); err == nil {
				log.Printf("dump request:\n\n%s", string(dumped))
			}
		}

		req.Close = true

		// send request and return response bytes
		var resp *http.Response
		resp, err = c.httpClient.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			if response, err = io.ReadAll(resp.Body); err == nil {
				if c.Verbose {
					log.Printf("API response for %s: '%s'", endpoint, string(response))
				}

				if !isSuccessStatus(resp.StatusCode) {
					err = fmt.Errorf("http status %d", resp.StatusCode)
				}

				return response, err
			}
		}
	}

	return nil, err
}

// sends HTTP GET request with context
func (c *Client) getWithContext(ctx context.Context, endpoint string, params map[string]any) (response []byte, err error) {
	return c.doWithContext(ctx, http.MethodGet, endpoint, params)
}

// sends HTTP GET request
func (c *Client) get(endpoint string, params map[string]any) (response []byte, err error) {
	return c.getWithContext(context.Background(), endpoint, params)
}

// sends HTTP DELETE request with context
func (c *Client) deleteWithContext(ctx context.Context, endpoint string, params map[string]any) (response []byte, err error) {
	return c.doWithContext(ctx, http.MethodDelete, endpoint, params)
}

// sends HTTP DELETE request
func (c *Client) delete(endpoint string, params map[string]any) (response []byte, err error) {
	return c.deleteWithContext(context.Background(), endpoint, params)
}

// sends HTTP POST request with context
func (c *Client) postWithContext(ctx context.Context, endpoint string, params map[string]any) (response []byte, err error) {
	if params == nil {
		params = map[string]any{}
	}

	url := baseURL
	if c.baseURL != nil {
		url = *c.baseURL
	}
	apiURL := fmt.Sprintf("%s/%s", url, endpoint)

	var req *http.Request

	if hasFileInParams(params) {
		// multipart/form-data
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for k, v := range params {
			switch val := v.(type) {
			case FileParam:
				bs := val.bs
				filename := fmt.Sprintf("%s.%s", k, getExtension(bs))

				var part io.Writer
				if part, err = writer.CreatePart(mimeHeaderForBytes(bs, k, filename)); err == nil {
					if _, err = io.Copy(part, bytes.NewReader(bs)); err != nil {
						return nil, fmt.Errorf("could not write bytes to multipart for param '%s': %s", k, err)
					}
				} else {
					return nil, fmt.Errorf("could not create part for param '%s': %s", k, err)
				}
			default:
				if err := writer.WriteField(k, fmt.Sprintf("%v", v)); err != nil {
					return nil, fmt.Errorf("could not write field with key: %s, value: %v", k, v)
				}
			}
		}

		if err = writer.Close(); err != nil {
			return nil, fmt.Errorf("error while closing multipart form data writer: %s", err)
		}

		if req, err = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body); err != nil {
			return nil, fmt.Errorf("failed to create multipart request: %s", err)
		}

		// set content-type header
		req.Header.Set("Content-Type", writer.FormDataContentType())
	} else {
		// application/json
		var serialized []byte
		if serialized, err = json.Marshal(params); err == nil {
			if req, err = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(serialized)); err != nil {
				return nil, fmt.Errorf("failed to create application/json request: %s", err)
			}

			// set content-type header
			req.Header.Set(kContentType, defaultContentType)
		}
	}

	// set authentication headers
	req.Header.Set(kAuthorization, fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set(kOrganization, c.OrganizationID)
	if c.beta != nil {
		req.Header.Set(kBeta, *c.beta)
	}

	if c.Verbose {
		if dumped, err := httputil.DumpRequest(req, true); err == nil {
			log.Printf("dump request:\n\n%s", string(dumped))
		}
	}
	req.Close = true

	// send request and return response bytes
	var resp *http.Response
	resp, err = c.httpClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		if response, err = io.ReadAll(resp.Body); err == nil {
			if c.Verbose {
				log.Printf("API response for %s: '%s'", endpoint, string(response))
			}

			if !isSuccessStatus(resp.StatusCode) {
				err = fmt.Errorf("http status %d", resp.StatusCode)
			}

			return response, err
		}
	}

	return nil, err
}

// sends HTTP POST request
func (c *Client) post(endpoint string, params map[string]any) (response []byte, err error) {
	return c.postWithContext(context.Background(), endpoint, params)
}

// sends HTTP POST request with streaming callback
func (c *Client) postCB(endpoint string, params map[string]any, cb callback) (response []byte, err error) {
	return c.postCBWithContext(context.Background(), endpoint, params, cb)
}

// sends HTTP POST request with streaming callback and context
func (c *Client) postCBWithContext(ctx context.Context, endpoint string, params map[string]any, cb callback) (response []byte, err error) {
	if params == nil {
		params = map[string]any{}
	}
	url := baseURL
	if c.baseURL != nil {
		url = *c.baseURL
	}
	apiURL := fmt.Sprintf("%s/%s", url, endpoint)

	var req *http.Request
	// application/json
	var serialized []byte
	if serialized, err = json.Marshal(params); err == nil {
		if req, err = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(serialized)); err != nil {
			return nil, fmt.Errorf("failed to create application/json request: %s", err)
		}

		// set content-type header
		req.Header.Set(kContentType, defaultContentType)
	}

	// set authentication headers
	req.Header.Set(kAuthorization, fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set(kOrganization, c.OrganizationID)

	if c.Verbose {
		if dumped, err := httputil.DumpRequest(req, true); err == nil {
			log.Printf("dump request:\n\n%s", string(dumped))
		}
	}

	// send request and return response bytes
	var resp *http.Response
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		defer resp.Body.Close()
		errbody := struct {
			Error Error `json:"error"`
		}{}
		if response, err = io.ReadAll(resp.Body); err == nil {
			if c.Verbose {
				log.Printf("API response for %s: '%s'", endpoint, string(response))
			}
			if err := json.Unmarshal(response, &errbody); err != nil {
				geminiErr := []struct {
					Error GeminiError `json:"error"`
				}{}
				if err := json.Unmarshal(response, &geminiErr); err != nil {
					return nil, fmt.Errorf("failed to decode error body: %v", err)
				}

				return nil, fmt.Errorf(geminiErr[0].Error.Message)
			}
		} else {
			return nil, err
		}

		return nil, errbody.Error.err()
	}

	go streamWithCtx(ctx, resp, cb)

	return nil, nil
}

// checks if given params include any file param
func hasFileInParams(params map[string]any) bool {
	for _, v := range params {
		if _, ok := v.(FileParam); ok {
			return true
		}
	}
	return false
}

// get file extension from given bytes array
//
// https://www.w3.org/Protocols/rfc1341/4_Content-Type.html
func getExtension(bytes []byte) string {
	types := strings.Split(http.DetectContentType(bytes), "/") // ex: "image/jpeg"
	if len(types) >= 2 {
		splitted := strings.Split(types[1], ";") // for removing subtype parameter
		if len(splitted) >= 1 {
			if splitted[0] == "wave" {
				return "wav"
			}
			if splitted[0] == "octet-stream" {
				return "mp3"
			}

			return splitted[0] // return subtype only
		}
	}
	return ""
}

// generates mime header
func mimeHeaderForBytes(bs []byte, key, filename string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set(kContentDisposition, fmt.Sprintf(`form-data; name="%s"; filename="%s"`, key, filename))
	h.Set(kContentType, http.DetectContentType(bs))
	return h
}
