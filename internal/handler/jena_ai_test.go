package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/claudeai"
	"go_starter/internal/mw"
	"go_starter/internal/session"
)

// stubAsker mengimplementasikan Asker tanpa memanggil proxy Claude asli — pola
// sama stubProvider (oauth_google_test.go). AskWithTools (jalur yang dipakai
// JenaAIAsk sejak BL-162 fase 2) mencatat calls/lastMD/lastQ sama seperti Ask
// dulu — test lama yang menegaskan field-field itu tetap berlaku tanpa ubah.
type stubAsker struct {
	answer string
	err    error
	calls  int
	lastMD string
	lastQ  string

	// dispatchTool, bila diisi, dipanggil sekali dari AskWithTools — dipakai
	// test yang perlu mensimulasikan model MEMANGGIL tool (mis. verifikasi
	// audit jena_ai.tool_call). Kosong (default) → AskWithTools tak pernah
	// menyentuh dispatcher, meniru jawaban langsung dari dokumen saja.
	dispatchTool string
	dispatchArgs json.RawMessage
}

func (s *stubAsker) Ask(ctx context.Context, knowledgeMD, question string) (string, error) {
	s.calls++
	s.lastMD = knowledgeMD
	s.lastQ = question
	if s.err != nil {
		return "", s.err
	}
	return s.answer, nil
}

func (s *stubAsker) AskWithTools(ctx context.Context, knowledgeMD, question string, tools []claudeai.Tool, dispatch claudeai.ToolDispatcher) (string, error) {
	s.calls++
	s.lastMD = knowledgeMD
	s.lastQ = question
	if s.dispatchTool != "" {
		if _, err := dispatch(ctx, s.dispatchTool, s.dispatchArgs); err != nil {
			return "", err
		}
	}
	if s.err != nil {
		return "", s.err
	}
	return s.answer, nil
}

// setupJenaAI menyiapkan env + enforcer Casbin nyata (policy.csv embed, jadi
// baris `p, admin, ai:chat, use` dari BL-162 ikut teruji) + reset claudeClient
// setelah test (var paket, bocor ke test lain kalau tak dibersihkan).
func setupJenaAI(t *testing.T) (*testEnv, int64) {
	t.Helper()
	env, ownerID := setupTest(t) // seed test@local, role owner di tenant default

	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	authz.Init(e)

	t.Cleanup(func() {
		claudeClient = nil
		jenaKnowledgeMD = ""
	})
	return env, ownerID
}

// doJenaAI menjalankan mw.RequireEnforce("ai:chat","use") + h.JenaAIAsk dalam
// session dengan role tertentu — mereplikasi rantai routes.go (gate Casbin di
// LUAR handler, bukan di dalamnya), sebab JenaAIAsk sendiri tak cek permission.
func (e *testEnv) doJenaAI(actorID int64, role, message string) *httptest.ResponseRecorder {
	form := url.Values{"message": {message}}
	req := httptest.NewRequest(http.MethodPost, "/w/test/jena-ai/ask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	guarded := mw.RequireEnforce("ai:chat", "use")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.h.JenaAIAsk(w, r.WithContext(withQueries(r.Context(), e.q)))
	}))

	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role != "" {
			session.SetIdentity(r.Context(), actorID, "test@local", role, false, e.tenantID, "Test", "test", "")
		}
		guarded.ServeHTTP(w, r)
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// TestJenaAIAsk_MemberAllowed — 21 Sep, akses dilebarkan ke SEMUA anggota
// workspace (sebelumnya admin+ dulu, lihat komentar policy.csv): member biasa
// (role dasar workspace) harus lolos gate ai:chat/use, bukan 403.
func TestJenaAIAsk_MemberAllowed(t *testing.T) {
	env, ownerID := setupJenaAI(t)
	stub := &stubAsker{answer: "Lead adalah calon desa yang belum pasti berlangganan."}
	claudeClient = stub
	jenaKnowledgeMD = "dokumen pengetahuan"

	rec := env.doJenaAI(ownerID, "member", "Apa itu Lead?")
	if rec.Code != http.StatusOK {
		t.Errorf("member harus lolos gate ai:chat/use, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.calls != 1 {
		t.Errorf("Asker.Ask harus terpanggil sekali, got %d", stub.calls)
	}
}

func TestJenaAIAsk_AdminAllowed(t *testing.T) {
	env, ownerID := setupJenaAI(t)
	stub := &stubAsker{answer: "Lead adalah calon desa yang belum pasti berlangganan."}
	claudeClient = stub
	jenaKnowledgeMD = "dokumen pengetahuan"

	rec := env.doJenaAI(ownerID, "admin", "Apa itu Lead?")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin harus lolos gate, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.calls != 1 {
		t.Errorf("Asker.Ask harus terpanggil sekali, got %d", stub.calls)
	}
	if stub.lastMD != "dokumen pengetahuan" {
		t.Errorf("knowledgeMD yang dioper harus dari jenaKnowledgeMD, got %q", stub.lastMD)
	}
	if stub.lastQ != "Apa itu Lead?" {
		t.Errorf("question yang dioper harus pesan user, got %q", stub.lastQ)
	}
	if !strings.Contains(rec.Body.String(), "Lead adalah calon desa") {
		t.Errorf("fragment SSE harus memuat jawaban:\n%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "jena-pending") {
		t.Errorf("fragment SSE harus disisip sebelum #jena-pending:\n%s", rec.Body.String())
	}

	logs, err := env.q.ListAuditLogs(t.Context(), 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("aksi harus tercatat di audit_logs (err=%v)", err)
	}
	found := false
	for _, l := range logs {
		if l.Action == "jena_ai.ask" {
			found = true
			meta := string(l.Metadata)
			if strings.Contains(meta, "Lead adalah") || strings.Contains(meta, "Apa itu Lead") {
				t.Errorf("audit TAK BOLEH memuat isi pertanyaan/jawaban (gotcha #12), got meta=%s", meta)
			}
		}
	}
	if !found {
		t.Error("audit log jena_ai.ask tidak ditemukan")
	}
}

func TestJenaAIAsk_EmptyMessage(t *testing.T) {
	env, ownerID := setupJenaAI(t)
	stub := &stubAsker{answer: "harusnya tak terpanggil"}
	claudeClient = stub

	rec := env.doJenaAI(ownerID, "admin", "   ")
	if rec.Code != http.StatusOK { // SSE flash, bukan HTTP error
		t.Fatalf("pesan kosong tetap 200 (flash SSE), got %d", rec.Code)
	}
	if stub.calls != 0 {
		t.Errorf("Asker.Ask TAK BOLEH terpanggil untuk pesan kosong, got %d panggilan", stub.calls)
	}
	if !strings.Contains(rec.Body.String(), "tidak boleh kosong") {
		t.Errorf("flash error harus menjelaskan pesan kosong:\n%s", rec.Body.String())
	}
}

func TestJenaAIAsk_NotConfigured(t *testing.T) {
	env, ownerID := setupJenaAI(t)
	claudeClient = nil // belum dikonfigurasi (CLAUDE_PROXY_URL/TOKEN kosong)

	rec := env.doJenaAI(ownerID, "admin", "Apa itu Lead?")
	if rec.Code != http.StatusOK {
		t.Fatalf("belum dikonfigurasi tetap 200 (flash SSE), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "belum dikonfigurasi") {
		t.Errorf("flash harus bilang Jena AI belum dikonfigurasi:\n%s", rec.Body.String())
	}
}

func TestJenaAIAsk_ProviderError(t *testing.T) {
	env, ownerID := setupJenaAI(t)
	claudeClient = &stubAsker{err: errors.New("proxy: 500")}

	rec := env.doJenaAI(ownerID, "admin", "Apa itu Lead?")
	if rec.Code != http.StatusOK {
		t.Fatalf("error provider tetap 200 (flash SSE), got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "proxy: 500") {
		t.Errorf("error mentah TAK BOLEH bocor ke user:\n%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tidak bisa menjawab") {
		t.Errorf("flash harus pesan generik:\n%s", rec.Body.String())
	}
}
