// Package ai provides a thin, reusable client for communicating with Ollama.
//
// Project 06 extends the scaffold contract with multimodal support.
// Every prior project's Message only ever carried text. Vision models like
// LLaVA accept an additional Images field: a slice of base64-encoded image
// strings attached to a single message. The rest of the contract (Chat,
// ChatStream, the chatRequest/streamChunk shapes) is unchanged from P01-P05.
package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	// DefaultModel is the general-purpose text model used in Projects 01-05.
	DefaultModel = "llama3.2:3b"

	// VisionModel is the multimodal model used in this project.
	// LLaVA ("Large Language and Vision Assistant") accepts both text and
	// images in a single message and is the standard local vision model
	// for Ollama as of 2026.
	//
	// Pull it with: ollama pull llava:7b
	VisionModel = "llava:7b"

	ollamaBaseURL = "http://localhost:11434"
	chatEndpoint  = ollamaBaseURL + "/api/chat"
)

// Message is a single turn in a conversation.
//
// Images is new in Project 06. It holds base64-encoded image data (no
// "data:image/..." prefix, just the raw base64 string). Ollama's /api/chat
// endpoint accepts this field on any message; text-only models ignore it,
// vision models read it alongside Content.
//
// Leaving Images nil for text-only messages (as every prior project's
// Message implicitly does) keeps this struct fully backward compatible:
// omitempty means the field never appears in the JSON body unless set.
type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type streamChunk struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// Chat sends messages to Ollama and returns the complete response as a string.
// Unchanged from prior projects. Used here because a caption should arrive
// complete, not as a word-by-word stream — the UI shows a single result card.
func Chat(model string, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	resp, err := http.Post(chatEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: ollama unreachable — is `ollama serve` running? %w", err)
	}
	defer resp.Body.Close()

	var result streamChunk
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ai: decode response: %w", err)
	}
	return result.Message.Content, nil
}

// ChatStream calls onChunk for every token as it arrives. Unchanged from
// prior projects. Not used in this project's caption flow, but kept so the
// ai/ package remains a complete, swappable unit across all 10 projects.
func ChatStream(model string, messages []Message, onChunk func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("ai: marshal request: %w", err)
	}

	resp, err := http.Post(chatEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ai: ollama unreachable — is `ollama serve` running? %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk streamChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			if err := onChunk(chunk.Message.Content); err != nil {
				return nil
			}
		}
		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}
