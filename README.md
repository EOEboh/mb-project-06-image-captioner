# Project 06: Image Caption Generator

> **Bootcamp Day 15** · Tier 2: Integration · Est. build time: 60 min

Upload an image and get an AI-generated caption: a natural description, screen-reader-ready alt text, a social media caption, or a detailed breakdown, all powered by a local vision model via Ollama.

---

## What You Will Build

A single-page image captioner where:

- The user uploads a JPEG, PNG, or WEBP image (HTMX multipart form)
- Go reads the file, validates its actual content type, and base64-encodes the raw bytes
- The encoded image is attached to a chat message's `Images` field and sent to `llava:7b`, a multimodal model
- The user picks a caption mode: Describe, Alt Text, Social Caption, or Detailed
- The result fragment shows the uploaded image next to the generated caption, with a copy button

---

## Key Concepts Introduced

| Concept | What It Teaches |
|---|---|
| Multimodal `ai.Message` | Extending the shared `Message` struct with an `Images []string` field that text-only models simply ignore |
| Base64 image encoding | `base64.StdEncoding.EncodeToString()` turns raw image bytes into the string format Ollama's vision models expect |
| `http.DetectContentType` | Validating a file's real format by sniffing its bytes, not trusting the filename extension |
| Vision model prompting | Why short, literal, concrete instructions outperform long structured prompts for image tasks |
| Data URIs | Rendering the exact base64 string sent to the AI directly in an `<img src="data:...">` tag, with no second copy or temp file |
| Accessibility-aware output | Generating screen-reader-appropriate alt text as a distinct mode from a creative caption |

---

## Prerequisites

- Go 1.22+ (`go version`)
- Ollama installed and running (`ollama --version`)
- `llava:7b` pulled (this project's setup step; ~4.5 GB download)

```bash
# Verify Ollama is running
curl http://localhost:11434/api/tags
```

---

## Setup

```bash
# 1. Clone the project
git clone https://github.com/EOEboh/mb-project-06-image-captioner
cd mb-project-06-image-captioner

# 2. Pull the vision model
make setup

# 3. Run the server
make run
# → http://localhost:8080
```

`make setup` pulls `llava:7b`. This is a noticeably larger download than the text models used in Projects 01-05 (around 4.5 GB vs under 2 GB), so allow extra time on the first run.

This project has zero external Go dependencies. Everything needed (`encoding/base64`, `net/http`, `mime/multipart`) is in the standard library.

---

## Project Structure

```
mb-project-06-image-captioner/
├── main.go                  HTTP server, ServeMux, routes
├── go.mod                   No external dependencies
├── Makefile                 make run, make setup, make pull-vision-model
├── .env.example             Environment variable reference
├── ai/
│   └── ollama.go            Chat() and ChatStream() + new Images field on Message
├── handlers/
│   └── caption.go           Index and Caption handlers, encoding, validation, prompts
├── templates/
│   └── index.html           "index" (upload page) + "result" (HTMX fragment)
└── static/
    └── style.css             Shared design tokens
```

---

## Architecture

```
Browser                         Go Server                          Ollama
  |                                 |                                  |
  | HTMX POST /caption              |                                  |
  | multipart/form-data             |                                  |
  | (image.jpg, mode=describe)      |                                  |
  |--------------------------------> |                                  |
  |                                 | r.FormFile("image")              |
  |                                 | io.ReadAll(file)                 |
  |                                 | http.DetectContentType(bytes)    |
  |                                 | base64.StdEncoding.EncodeTo...   |
  |                                 |                                  |
  |                                 | ai.Chat(VisionModel, []Message{  |
  |                                 |   {Role:"user",                  |
  |                                 |    Content: prompt,              |
  |                                 |    Images: []string{encoded}}    |
  |                                 | })------------------------------> |
  |                                 |                                  |
  |                                 |                       caption    |
  |                                 | <--------------------------------|
  |                                 |                                  |
  |                                 | ExecuteTemplate("result", data)  |
  |  200 OK                         |                                  |
  |  Content-Type: text/html        |                                  |
  |  <img src="data:image/jpeg;     |                                  |
  |    base64,...">                 |                                  |
  |  <p>caption text</p>            |                                  |
  | <-------------------------------- |                                  |
  |                                 |                                  |
  | HTMX swaps into #result        |                                  |
```

---

## Key Files Explained

### `ai/ollama.go`

**The `Images` field is the entire multimodal mechanism:**

```go
type Message struct {
    Role    string   `json:"role"`
    Content string   `json:"content"`
    Images  []string `json:"images,omitempty"` // new in Project 06
}
```

`omitempty` means this field is invisible in the JSON body for every prior project's text-only messages. Nothing about `Chat()` or `ChatStream()` changes. A vision model reads `Images`; a text model ignores it. This is why the same `ai.Chat()` function used since Project 02 works here unmodified, just called with `ai.VisionModel` instead of `ai.DefaultModel`.

### `handlers/caption.go`

**Reading and validating the upload:**

```go
file, header, err := r.FormFile("image")
defer file.Close()

imageBytes, err := io.ReadAll(file)          // full bytes in memory
mimeType := http.DetectContentType(imageBytes) // sniff, don't trust the extension
```

Unlike Project 04's PDF reader (which needed `io.ReaderAt` for random access), base64 encoding needs the complete byte slice regardless, so a simple `io.ReadAll` is the correct and simplest choice here.

**The encode-and-attach step:**

```go
encoded := base64.StdEncoding.EncodeToString(imageBytes)

caption, err := ai.Chat(ai.VisionModel, []ai.Message{
    {Role: "user", Content: prompt, Images: []string{encoded}},
})
```

One base64 string, one field, one `ai.Chat()` call. No separate vision API, no different transport.

**Reusing the encoded image for the preview:**

```html
<img src="data:{{.ImageMIME}};base64,{{.ImageData}}" alt="{{.AltText}}">
```

The exact base64 string sent to Ollama is rendered directly in the browser via a data URI. This is a deliberate choice: the user sees precisely what the model received, with zero extra encoding, storage, or a second file read.

---

## Why Short Prompts for Vision Models

Every text project in this bootcamp used long, structured system prompts with explicit headings (Projects 02, 03, 04). This project's prompts are short and direct:

```go
"Describe what is happening in this image in one to two clear, natural sentences. " +
"No preamble such as 'This image shows'. Begin directly with the description."
```

Vision-language models are tuned on instruction-following datasets that favour concrete, literal requests over long structured outlines. A heavily structured prompt (multiple headings, numbered rules, conditional fallback instructions) tends to confuse smaller vision models or cause them to ignore the image and respond generically. Keep vision prompts short, specific about output length, and explicit about banning preamble.

---

## Makefile Commands

```bash
make run               # start the server on :8080
make build              # compile to bin/app
make setup              # pull llava:7b
make pull-vision-model  # pull llava:7b directly
make help               # list all commands
```

---

## Off-Day Extensions

| # | Extension | What It Builds Toward |
|---|---|---|
| E1 | Add a batch mode: upload up to 5 images, caption all in one request | Multiple `Images` entries, concurrent AI calls |
| E2 | Let the user ask a follow-up question about the image ("What color is the car?") | Multi-turn multimodal conversation |
| E3 | Add an "Extract Text" mode using the image for OCR-style transcription | Prompt specialisation within the same flow |
| E4 | Resize large images server-side before encoding (using `image/draw`) to reduce token cost | Go's standard image processing packages |
| E5 | Cache captions by image hash (SHA-256) to avoid re-captioning the same upload | Content-addressed caching pattern |

---

## What Is Next

**Project 07: API Documentation Generator** — returns to text-only AI but introduces multi-pass prompting over source code: one pass identifies HTTP handlers, a second pass generates OpenAPI-style documentation for each one. First project to process Go source code as AI input.