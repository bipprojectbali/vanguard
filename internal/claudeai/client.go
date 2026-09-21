// Package claudeai membungkus panggilan HTTP ke proxy internal Claude
// (Anthropic Messages API) untuk Jena AI (BL-162). Bukan SDK resmi — ini
// cuma satu panggilan HTTP, jadi net/http polos cukup (prinsip single-binary:
// jangan tambah dependency untuk yang sesederhana ini).
package claudeai

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

// model dikirim apa adanya walau proxy (per uji 17 Sep) mengabaikannya dan
// selalu menjawab sebagai claude-sonnet-4-6 — itu kuirk proxy, bukan alasan
// membangun di sekitarnya; kirim nilai yang benar untuk masa depan.
const model = "claude-sonnet-5"

const maxTokens = 1024

// jenaMaxToolRounds membatasi jumlah putaran tool_use↔tool_result per
// pertanyaan (keputusan 17 Sep: cegah biaya/loop lari). Di putaran TERAKHIR,
// request sengaja tak menyertakan `tools` — cara paksa model menjawab teks
// biasa alih-alih minta tool lagi, tanpa logika penolakan manual.
const jenaMaxToolRounds = 4

// guardrail disuntik SEBELUM dokumen pengetahuan di content-block PERTAMA
// pesan `user`. Proxy ini MENGABAIKAN field `system` Anthropic Messages API
// (temuan uji 17 Sep) — instruksi harus hidup di pesan user, bukan system.
// Instruksi format (baris kedua) ditambah setelah user melaporkan jawaban
// tampil sebagai satu paragraf mentah dengan sintaks **tebal**/`kode` tak
// ter-render (bubble chat = g.Text polos, gotcha #15 — sengaja TIDAK ada
// parser markdown-ke-HTML, hindari dependency baru untuk hal sesederhana ini).
// Baris baru asli (\n) tetap dikirim apa adanya; whitespace-pre-line di CSS
// (bukan di sini) yang membuatnya tampak sebagai baris terpisah di browser.
// Klausa tool-calling (BL-162 fase 2) menegaskan: PANGGIL tool bila cocok,
// dan kalau tak ada tool cocok/hasil tool bilang data tak ditemukan, jujur
// mengaku belum bisa cek — DILARANG mengarang, baik dari dokumen maupun tool.
const guardrail = "Anda adalah Jena AI, asisten yang menjawab pertanyaan HANYA " +
	"berdasarkan dokumen pengetahuan di bawah ini DAN, bila tersedia, hasil " +
	"tool yang Anda panggil. Jika pertanyaan butuh data desa/langganan/deal " +
	"yang nyata dan ADA tool yang cocok, PANGGIL tool itu — jangan menjawab " +
	"dari ingatan/tebakan. Jika tak ada tool yang cocok, atau hasil tool " +
	"menyatakan data tidak ditemukan, jawab jujur bahwa Anda belum bisa " +
	"mengeceknya — DILARANG mengarang jawaban di luar dokumen/hasil tool. " +
	"Jawab dengan TEKS POLOS (BUKAN markdown): jangan pakai tanda bintang " +
	"**tebal**/*miring*, tanda pagar #, atau backtick kode. Untuk daftar/poin, " +
	"tulis tiap poin di baris baru tersendiri (boleh diawali tanda \"-\" lalu " +
	"spasi), jangan digabung jadi satu paragraf panjang.\n\n--- DOKUMEN " +
	"PENGETAHUAN ---\n"

const guardrailFooter = "\n--- AKHIR DOKUMEN ---"

// Client memanggil proxy Claude. Dipasang lewat setter global handler
// (handler.SetClaudeClient) di run.go, mengikuti pola SetGoogleOAuth.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New membuat Client. baseURL TANPA trailing slash (mis.
// "https://claude-proxy.wibudev.com"); token dikirim sebagai header x-api-key.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Tool adalah skema satu tool Anthropic Messages API. Didaftarkan oleh
// pemanggil (internal/handler/jena_ai_tools.go) — paket ini transport murni,
// tak tahu apa isi tool-nya. InputSchema = JSON Schema object mentah.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// ToolDispatcher mengeksekusi SATU tool_use call dan mengembalikan hasilnya
// sbg string (dikirim balik ke model sbg content tool_result). Error dari
// dispatcher TIDAK menghentikan percakapan — dikirim balik sbg tool_result
// is_error, model bisa memilih menjelaskan ke user alih-alih macet. Hidup di
// internal/handler (bukan di sini) karena eksekusi butuh h.q(ctx) tenant-
// scoped + enforcement F2/F3/F4 yang tak pantas ada di paket transport ini.
type ToolDispatcher func(ctx context.Context, name string, input json.RawMessage) (string, error)

// contentBlock dipakai DUA ARAH: dibentuk manual utk request (text/tool_result,
// lihat helper di bawah) dan diisi json.Unmarshal utk response (text/tool_use).
// Satu struct dgn field omitempty lebih sederhana drpd dua tipe terpisah,
// karena AskWithTools harus mengecho content block RESPONSE apa adanya
// sbg pesan `assistant` di putaran berikutnya (lihat AskWithTools).
type contentBlock struct {
	Type string `json:"type"`

	// text
	Text         string        `json:"text,omitempty"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`

	// tool_use (diisi dari response, dikirim balik apa adanya sbg echo)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result (dibentuk manual utk request)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"`
}

type message struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type toolWire struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type askRequest struct {
	Model     string     `json:"model"`
	MaxTokens int        `json:"max_tokens"`
	Messages  []message  `json:"messages"`
	Tools     []toolWire `json:"tools,omitempty"`
}

type askResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

func openingMessage(knowledgeMD, question string) message {
	return message{
		Role: "user",
		Content: []contentBlock{
			{
				Type:         "text",
				Text:         guardrail + knowledgeMD + guardrailFooter,
				CacheControl: &cacheControl{Type: "ephemeral"},
			},
			{Type: "text", Text: question},
		},
	}
}

// Ask mengirim satu pertanyaan ke Claude, dibatasi HANYA pada knowledgeMD
// (tanpa tool-calling DB). Non-streaming: balasan dikembalikan utuh setelah
// lengkap. Dipertahankan (dipakai jalur yang sengaja tanpa tool) di samping
// AskWithTools — bukan alias, supaya call-site tanpa tool tak perlu mengoper
// slice/dispatcher kosong.
func (c *Client) Ask(ctx context.Context, knowledgeMD, question string) (string, error) {
	resp, err := c.send(ctx, askRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  []message{openingMessage(knowledgeMD, question)},
	})
	if err != nil {
		return "", err
	}
	return extractText(resp.Content)
}

// AskWithTools sama seperti Ask, ditambah loop tool_use↔tool_result (maks
// jenaMaxToolRounds putaran). dispatch dipanggil SINKRON per tool_use block
// yang diminta model — untuk allowlist saat ini (BL-162 fase 2) modelnya
// selalu minta satu tool per putaran, tapi kode ini menangani banyak block
// sekaligus karena begitu bentuk response Anthropic yang sah.
func (c *Client) AskWithTools(ctx context.Context, knowledgeMD, question string, tools []Tool, dispatch ToolDispatcher) (string, error) {
	wireTools := make([]toolWire, len(tools))
	for i, t := range tools {
		wireTools[i] = toolWire{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}

	messages := []message{openingMessage(knowledgeMD, question)}

	for round := 0; round < jenaMaxToolRounds; round++ {
		req := askRequest{Model: model, MaxTokens: maxTokens, Messages: messages}
		if round < jenaMaxToolRounds-1 {
			req.Tools = wireTools
		}

		resp, err := c.send(ctx, req)
		if err != nil {
			return "", err
		}
		if resp.StopReason != "tool_use" {
			return extractText(resp.Content)
		}

		messages = append(messages, message{Role: "assistant", Content: resp.Content})

		var results []contentBlock
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			result, err := dispatch(ctx, block.Name, block.Input)
			isError := err != nil
			if isError {
				result = fmt.Sprintf("error: %v", err)
			}
			results = append(results, contentBlock{
				Type:      "tool_result",
				ToolUseID: block.ID,
				Content:   result,
				IsError:   isError,
			})
		}
		if len(results) == 0 {
			// stop_reason tool_use tapi tak ada block tool_use terbaca — jaga
			// diri dari loop tanpa progres, jawab apa adanya dari teks yang ada.
			return extractText(resp.Content)
		}
		messages = append(messages, message{Role: "user", Content: results})
	}

	return "", fmt.Errorf("claudeai: batas putaran tool-call (%d) tercapai tanpa jawaban akhir", jenaMaxToolRounds)
}

// send menjalankan satu panggilan HTTP ke proxy, dipakai bersama Ask &
// AskWithTools.
func (c *Client) send(ctx context.Context, body askRequest) (askResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return askResponse{}, fmt.Errorf("claudeai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return askResponse{}, fmt.Errorf("claudeai: buat request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return askResponse{}, fmt.Errorf("claudeai: kirim request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return askResponse{}, fmt.Errorf("claudeai: baca response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return askResponse{}, fmt.Errorf("claudeai: proxy membalas status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var parsed askResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return askResponse{}, fmt.Errorf("claudeai: parse response: %w", err)
	}
	return parsed, nil
}

func extractText(blocks []contentBlock) (string, error) {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type != "text" || b.Text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(b.Text)
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("claudeai: response tanpa konten teks")
	}
	return sb.String(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
