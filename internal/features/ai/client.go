package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrEmptyCompletion dikembalikan bila OpenAI tidak memberi pilihan jawaban.
var ErrEmptyCompletion = errors.New("respons OpenAI kosong")

// doJSON menjalankan request, memastikan status 2xx lalu men-decode body JSON ke out.
func doJSON(hc *http.Client, req *http.Request, out any) error {
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ChatClient adalah klien HTTP OpenAI Chat Completions.
type ChatClient struct {
	endpoint string
	key      string
	model    string
	http     *http.Client
}

// NewChatClient membuat ChatClient. endpoint contoh: https://delcom.org/api/v1
func NewChatClient(endpoint, key, model string, hc *http.Client) *ChatClient {
	return &ChatClient{endpoint: endpoint, key: key, model: model, http: hc}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete mengirim prompt sistem + pengguna dan mengembalikan teks jawaban pertama.
func (c *ChatClient) Complete(ctx context.Context, system, user string) (string, error) {
	payload, _ := json.Marshal(chatRequest{
		Model:       c.model,
		Temperature: 0.7,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	url := strings.TrimRight(c.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)

	var out chatResponse
	if err := doJSON(c.http, req, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", ErrEmptyCompletion
	}
	return out.Choices[0].Message.Content, nil
}
