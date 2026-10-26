package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible chat completions endpoint.
type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

// NewClient creates a client for a server such as llama.cpp's llama-server, with
// baseURL ending in /v1.
func NewClient(baseURL, model, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: timeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest uses the sampling the model card recommends for the 1.8B and 7B
// models. top_k and repetition_penalty are extensions that llama.cpp, vLLM and
// SGLang accept.
type chatRequest struct {
	Model             string        `json:"model"`
	Messages          []chatMessage `json:"messages"`
	Temperature       float64       `json:"temperature"`
	TopP              float64       `json:"top_p"`
	TopK              int           `json:"top_k"`
	RepetitionPenalty float64       `json:"repetition_penalty"`
	MaxTokens         int           `json:"max_tokens"`
}

type chatResponse struct {
	// Model is the model that answered; llama.cpp names the one it loaded.
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Translate sends the prompt as the only message: the models have no system
// prompt. Placeholders are masked on the way and restored in the answer.
func (c *Client) Translate(ctx context.Context, req Request) (Answer, error) {
	masked, unmask := Mask(req.Text)
	req.Text = masked

	body, err := json.Marshal(chatRequest{
		Model:             c.model,
		Messages:          []chatMessage{{Role: "user", Content: Prompt(req)}},
		Temperature:       0.7,
		TopP:              0.6,
		TopK:              20,
		RepetitionPenalty: 1.05,
		MaxTokens:         1024,
	})
	if err != nil {
		return Answer{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Answer{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	res, err := c.http.Do(httpReq)
	if err != nil {
		return Answer{}, err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		detail, err := io.ReadAll(io.LimitReader(res.Body, 512))
		if err != nil {
			return Answer{}, fmt.Errorf("translation server answered %s", res.Status)
		}

		return Answer{}, fmt.Errorf("translation server answered %s: %s", res.Status, strings.TrimSpace(string(detail)))
	}

	var parsed chatResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return Answer{}, fmt.Errorf("decode translation answer: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return Answer{}, ErrEmptyAnswer
	}

	model := parsed.Model
	if model == "" {
		model = c.model
	}

	return Answer{Text: unmask(parsed.Choices[0].Message.Content), Model: model}, nil
}

// Models returns the models the server serves, as its answers name them, and the
// configured model, which a server such as vLLM names its answers after instead.
func (c *Client) Models(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	res, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("translation server answered %s", res.Status)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode model list: %w", err)
	}

	models := make([]string, 0, len(parsed.Data)+1)
	if c.model != "" {
		models = append(models, c.model)
	}

	for _, model := range parsed.Data {
		if model.ID != "" && model.ID != c.model {
			models = append(models, model.ID)
		}
	}

	return models, nil
}
