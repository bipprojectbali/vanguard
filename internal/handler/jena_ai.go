package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/claudeai"
	"go_starter/internal/session"
	"go_starter/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
)

// jena_ai.go — Jena AI (BL-162): asisten chat floating berbasis dokumen
// pengetahuan (jenaKnowledgeMD) DITAMBAH tool-calling database read-only
// (jenaTools/jenaDispatch, lihat jena_ai_tools.go) — fase 2, PoC awal (hanya
// dokumen) telah tervalidasi & dilebarkan ke semua anggota workspace 21 Sep.
// Digate Casbin ai:chat/use (routes.go + policy.csv). Non-streaming: satu
// balasan dikirim utuh sekali jadi setelah loop tool-call (bila ada) selesai.

// Asker adalah kontrak minimal yang dibutuhkan handler dari provider AI.
// Interface (bukan *claudeai.Client konkret) agar test bisa menyuntik stub
// tanpa memanggil proxy asli — pola sama dengan googleProvider (oauth_google.go).
// Ask dipertahankan di samping AskWithTools (bukan dihapus) — claudeai.Client
// tetap mengeksposnya untuk jalur yang sengaja tanpa tool.
type Asker interface {
	Ask(ctx context.Context, knowledgeMD, question string) (string, error)
	AskWithTools(ctx context.Context, knowledgeMD, question string, tools []claudeai.Tool, dispatch claudeai.ToolDispatcher) (string, error)
}

// claudeClient di-inject saat startup via SetClaudeClient bila
// CLAUDE_PROXY_URL/CLAUDE_PROXY_TOKEN terisi (config.ClaudeProxyEnabled). nil
// → Jena AI mati: tombol tak pernah tampil (render_shell.go gate ShowJenaAI),
// handler ini tetap menjaga diri sendiri (flash, bukan panic) andai ter-panggil.
var claudeClient Asker

// SetClaudeClient menyuntik provider Claude (dipanggil run.go bila kredensial
// proxy tersedia).
func SetClaudeClient(c Asker) { claudeClient = c }

// JenaAIConfigured melaporkan apakah Jena AI siap dipakai — dipakai
// render_shell.go digabung permission Casbin untuk menggerbangi tombol.
func JenaAIConfigured() bool { return claudeClient != nil }

// jenaKnowledgeMD disuntik dari main.go (go:embed docs/crm/jena-ai-knowledge.md)
// lewat SetJenaKnowledge. Tak bisa di-embed langsung di paket ini: go:embed
// tak boleh menaiki direktori, dan docs/ hidup di root repo, bukan di bawah
// internal/handler.
var jenaKnowledgeMD string

// SetJenaKnowledge menyuntik isi dokumen pengetahuan (dipanggil run.go
// bersamaan SetClaudeClient).
func SetJenaKnowledge(md string) { jenaKnowledgeMD = md }

// jenaMaxQuestionChars membatasi panjang pertanyaan — proxy dibayar per token,
// dan ini payload API (bukan HTML), jadi validasi di sini bukan di client.
const jenaMaxQuestionChars = 2000

// JenaAIAsk — POST /w/{slug}/jena-ai/ask. Digate ai:chat/use (routes.go).
func (h *Handler) JenaAIAsk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// message WAJIB dibaca SEBELUM datastar.NewSSE: NewSSE mem-flush header
	// response SEGERA saat dipanggil. Di koneksi TCP nyata (beda dari
	// httptest.NewRequest yang bodinya sintetis di memori), flush response
	// SEBELUM body request selesai dibaca membuat r.FormValue/r.Body kembali
	// KOSONG walau Content-Length benar — dikonfirmasi lewat repro
	// httptest.NewServer + http.Client asli (bug nyata BL-162, bukan salah
	// Datastar client: form/Content-Length yang dikirim browser sudah benar).
	message := strings.TrimSpace(r.FormValue("message"))

	sse := datastar.NewSSE(w, r)

	if claudeClient == nil {
		patchFlash(sse, false, "Jena AI belum dikonfigurasi")
		return
	}
	if message == "" {
		patchFlash(sse, false, "Pertanyaan tidak boleh kosong")
		return
	}
	if len(message) > jenaMaxQuestionChars {
		patchFlash(sse, false, "Pertanyaan terlalu panjang")
		return
	}
	if !jenaRateLimiter.Allow(session.UserID(ctx)) {
		patchFlash(sse, false, "Terlalu banyak pertanyaan, coba lagi beberapa menit lagi")
		return
	}

	answer, err := claudeClient.AskWithTools(ctx, jenaKnowledgeMD, message, h.jenaTools(), h.jenaDispatch)
	if err != nil {
		h.Log.Error("jena ai: ask", "err", err)
		patchFlash(sse, false, "Jena AI sedang tidak bisa menjawab, coba lagi")
		return
	}

	var sb strings.Builder
	if err := ui.JenaAIMessagePair(message, answer).Render(&sb); err != nil {
		h.Log.Error("jena ai: render fragment", "err", err)
		return
	}
	// Sisip SEBELUM #jena-pending (bukan append ke #jena-thread): #jena-pending
	// = jangkar tetap di ujung thread (bubble optimistic, ui/jena_ai.go) — pasang
	// bubble permanen TEPAT SEBELUM jangkar itu menjaga urutan kronologis, sambil
	// jangkar sendiri tetap paling akhir untuk giliran tanya berikutnya. Bukan
	// morph-replace: tiap jawaban menambah riwayat, bukan mengganti baris —
	// riwayat hidup di DOM klien saja (ephemeral per sesi browser, tanpa
	// tabel/migrasi baru, sesuai keputusan PoC).
	_ = sse.PatchElements(sb.String(), datastar.WithSelectorID("jena-pending"), datastar.WithModeBefore())

	// Audit RINGAN: siapa bertanya + panjang jawaban — BUKAN isi pertanyaan/
	// jawaban (gotcha #12/#15: no-PII/no-isi-sensitif di log).
	h.auditWorkspace(ctx, session.UserID(ctx), "jena_ai.ask", session.TenantID(ctx),
		map[string]string{"answer_chars": strconv.Itoa(len(answer))})
}
