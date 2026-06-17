package handlers

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/EOEboh/mb-project-06-image-captioner/ai"
)

// tmpl holds both named templates from index.html.
// "index"  — full HTML page, served on GET /
// "result" — HTML fragment, returned to HTMX on POST /caption
var tmpl = template.Must(template.ParseFiles("templates/index.html"))

// Limits and supported formats.
const (
	maxFileSize = 5 << 20 // 5 MB — vision models are sensitive to image size and
	// token cost grows with resolution, so we cap uploads early.
)

// allowedTypes maps the sniffed MIME type to a human-readable label.
// Detection uses http.DetectContentType on the actual file bytes, not the
// filename extension — the same "do not trust client-supplied metadata"
// principle applied to the .pdf check in Project 04.
var allowedTypes = map[string]string{
	"image/jpeg": "JPEG",
	"image/png":  "PNG",
	"image/webp": "WEBP",
}

// ── Data types ────────────────────────────────────────────────────────────────

// captionResult is the data passed to the "result" template.
// Covers success and error states in a single struct, same pattern as
// every prior project's fragment data type.
type captionResult struct {
	Error     string
	ImageData string // base64 string for inline <img> preview — reuses the same encoding sent to Ollama
	ImageMIME string
	FileName  string
	Mode      string        // which caption mode was requested
	ModeLabel string        // human-readable label for the mode
	Caption   template.HTML // the AI's response, lightly formatted
	AltText   string        // plain-text version of Caption, safe for the alt="" attribute
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Index serves the upload page.
func Index(w http.ResponseWriter, r *http.Request) {
	if err := tmpl.ExecuteTemplate(w, "index", nil); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// Caption handles POST /caption.
//
// ── What is new in Project 06 ─────────────────────────────────────────────────
//
// Every prior project sent text to Ollama. This project sends an image.
// The mechanism: read the uploaded file into memory, base64-encode the raw
// bytes, and attach that string to a Message's Images field. The model
// (LLaVA) receives both the encoded image and a text instruction in the
// same request and returns a text response describing what it sees.
//
// This is NOT a file upload to a server filesystem, and it is NOT a separate
// "vision API" — it is the same ai.Chat() function from every other project,
// called with a different model name and a Message that happens to carry
// an Images field. The unified Message contract (see ai/ollama.go) is what
// makes this a small addition rather than a parallel code path.
// ─────────────────────────────────────────────────────────────────────────────
func Caption(w http.ResponseWriter, r *http.Request) {
	// ── 1. Parse multipart form ───────────────────────────────────────────
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		renderFragment(w, captionResult{Error: "Could not parse upload: " + err.Error()})
		return
	}

	mode := r.FormValue("mode")
	if mode == "" {
		mode = "describe"
	}

	// ── 2. Read uploaded file ─────────────────────────────────────────────
	file, header, err := r.FormFile("image")
	if err != nil {
		renderFragment(w, captionResult{Error: "Please select an image to upload."})
		return
	}
	defer file.Close()

	if header.Size > maxFileSize {
		renderFragment(w, captionResult{Error: fmt.Sprintf(
			"Image is too large (%s). Maximum size is 5 MB.",
			formatFileSize(header.Size),
		)})
		return
	}

	// ── 3. Read into memory and validate the actual file content ──────────
	// io.ReadAll is appropriate here (unlike Project 04's PDF reader) because
	// base64 encoding needs the complete byte slice in memory anyway — there
	// is no streaming or random-access requirement for this operation.
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("read upload: %v", err)
		renderFragment(w, captionResult{Error: "Could not read the uploaded file."})
		return
	}

	// http.DetectContentType sniffs the first 512 bytes against a table of
	// known file signatures. This checks what the file actually is, not what
	// its extension claims — the same principle as the magic-byte validation
	// theme from Project 04, applied here at upload time instead of parse time.
	mimeType := http.DetectContentType(imageBytes)
	label, ok := allowedTypes[mimeType]
	if !ok {
		renderFragment(w, captionResult{Error: fmt.Sprintf(
			"Unsupported file type (%s detected). Please upload a JPEG, PNG, or WEBP image.",
			mimeType,
		)})
		return
	}

	// ── 4. Base64-encode for Ollama ────────────────────────────────────────
	// This is the core mechanism of multimodal input in Ollama's API: the
	// raw image bytes become a base64 string, attached to a Message.Images
	// slice. No multipart encoding, no separate upload endpoint on Ollama's
	// side — one JSON field carrying the whole image as text.
	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	// ── 5. Build the prompt for the requested mode ─────────────────────────
	prompt := promptForMode(mode)

	// ── 6. Call ai.Chat() with the image attached ───────────────────────────
	// Note the model: ai.VisionModel (llava:7b), not ai.DefaultModel.
	// This is the same Chat() function used in Projects 02-05 — only the
	// model name and the Images field differ.
	caption, err := ai.Chat(ai.VisionModel, []ai.Message{
		{
			Role:    "user",
			Content: prompt,
			Images:  []string{encoded},
		},
	})
	if err != nil {
		log.Printf("ai caption error: %v", err)
		renderFragment(w, captionResult{Error: "AI request failed: " + err.Error() +
			". Make sure llava:7b is pulled (run: make pull-vision-model)."})
		return
	}

	// ── 7. Build result and render fragment ─────────────────────────────────
	data := captionResult{
		ImageData: encoded,
		ImageMIME: mimeType,
		FileName:  header.Filename,
		Mode:      mode,
		ModeLabel: modeLabel(mode),
		Caption:   template.HTML(template.HTMLEscapeString(strings.TrimSpace(caption))),
		AltText:   buildAltText(strings.TrimSpace(caption)),
	}
	_ = label // label is used only for validation; kept for log clarity if needed later

	renderFragment(w, data)
}

// ── Private helpers ───────────────────────────────────────────────────────────

// renderFragment writes the "result" named template as a text/html response.
// Identical pattern to every HTMX fragment handler since Project 03.
func renderFragment(w http.ResponseWriter, data captionResult) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "result", data); err != nil {
		log.Printf("fragment render error: %v", err)
	}
}

// promptForMode returns the instruction sent alongside the image for each
// caption mode. Unlike text-only prompts in prior projects, these are
// deliberately short: vision models perform best with direct, concrete
// instructions rather than long structured system prompts. There is no
// separate system message here — LLaVA's chat template expects the
// instruction directly in the user message alongside the image.
func promptForMode(mode string) string {
	switch mode {
	case "alt-text":
		return "Write a concise, literal alt-text description of this image for a screen reader. " +
			"Describe only what is visibly present: people, objects, setting, and composition. " +
			"Do not interpret mood or meaning. Maximum one sentence. No preamble."

	case "social":
		return "Write an engaging, upbeat social media caption for this image. " +
			"You may use light humor or personality. Maximum two sentences. No hashtags. No preamble."

	case "detailed":
		return "Describe this image in detailed, structured prose: the subject, setting, colors, " +
			"composition, lighting, and any notable details. Three to five sentences. No preamble."

	default: // "describe"
		return "Describe what is happening in this image in one to two clear, natural sentences. " +
			"No preamble such as 'This image shows'. Begin directly with the description."
	}
}

// modeLabel maps a mode key to its display name in the result UI.
func modeLabel(mode string) string {
	labels := map[string]string{
		"describe": "Description",
		"alt-text": "Alt Text",
		"social":   "Social Caption",
		"detailed": "Detailed Description",
	}
	if l, ok := labels[mode]; ok {
		return l
	}
	return "Description"
}

// buildAltText strips any residual markdown or quote characters from the
// caption so it is safe to drop directly into an HTML alt="" attribute.
// html/template already escapes attribute values, but trimming wrapping
// quotes here avoids a caption rendering as ""A cat on a chair"" with
// doubled quote marks if the model wraps its own output in quotes.
func buildAltText(caption string) string {
	return strings.Trim(caption, `"'`)
}

// formatFileSize returns a human-readable file size string.
// Identical helper to the one introduced in Project 04.
func formatFileSize(size int64) string {
	const mb = 1 << 20
	if size >= mb {
		return fmt.Sprintf("%.1f MB", float64(size)/mb)
	}
	return fmt.Sprintf("%d KB", size/1024)
}
