package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OpenAIClient struct {
	endpoint string
	key      string
	model    string
	http     *http.Client
}

func NewOpenAIClient(endpoint, key, model string) *OpenAIClient {
	return &OpenAIClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		key:      key,
		model:    model,
		http:     &http.Client{Timeout: 120 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func buildPrompt(topic, category string, sources []SearchSource) string {
	var b strings.Builder
	b.WriteString("Tulis artikel blog berbahasa Indonesia tentang topik: " + topic + ".\n")
	b.WriteString("Kategori: " + category + ".\n")
	if len(sources) > 0 {
		b.WriteString("Gunakan konteks hasil pencarian web berikut sebagai referensi:\n")
		for i, s := range sources {
			b.WriteString(fmt.Sprintf("%d. %s (%s): %s\n", i+1, s.Title, s.URL, s.Snippet))
		}
	}
	b.WriteString("Balas HANYA dengan JSON valid berformat {\"title\":\"\",\"category\":\"\",\"excerpt\":\"\",\"content\":\"\"}, ")
	b.WriteString("dengan content berupa Markdown.")
	return b.String()
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func (c *OpenAIClient) Generate(ctx context.Context, topic, category string, sources []SearchSource) (*Article, error) {
	body, _ := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: "Kamu adalah penulis blog profesional."},
			{Role: "user", Content: buildPrompt(topic, category, sources)},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai mengembalikan status %d", resp.StatusCode)
	}

	var chat chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chat); err != nil {
		return nil, err
	}
	if len(chat.Choices) == 0 {
		return nil, errors.New("respons openai tidak memiliki choices")
	}

	var article Article
	if err := json.Unmarshal([]byte(extractJSON(chat.Choices[0].Message.Content)), &article); err != nil {
		return nil, fmt.Errorf("format artikel dari AI tidak valid: %w", err)
	}
	if article.Title == "" || article.Content == "" {
		return nil, errors.New("artikel dari AI tidak lengkap")
	}
	return &article, nil
}