package claudeai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAsk_RequestShape: verifikasi bentuk request sesuai temuan uji proxy
// 17 Sep — system TIDAK dipakai (proxy mengabaikannya), instruksi masuk
// content-block PERTAMA pesan user, cache_control ada di block itu.
func TestAsk_RequestShape(t *testing.T) {
	var gotReq map[string]any
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"jawaban dari Jena"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-cp-test-token")
	answer, err := c.Ask(context.Background(), "dokumen pengetahuan X", "apa itu X?")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "jawaban dari Jena" {
		t.Errorf("answer = %q, want %q", answer, "jawaban dari Jena")
	}

	if got := gotHeaders.Get("x-api-key"); got != "sk-cp-test-token" {
		t.Errorf("header x-api-key = %q, want token", got)
	}
	if _, hasSystem := gotReq["system"]; hasSystem {
		t.Error("request tidak boleh punya field system — proxy mengabaikannya, harus masuk pesan user")
	}
	if got := gotReq["model"]; got != model {
		t.Errorf("model = %v, want %q (kuirk proxy mengabaikan field ini, tapi tetap kirim nilai benar)", got, model)
	}

	messages, _ := gotReq["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(messages))
	}
	msg0, _ := messages[0].(map[string]any)
	if msg0["role"] != "user" {
		t.Errorf("messages[0].role = %v, want user", msg0["role"])
	}
	content, _ := msg0["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content blocks = %d, want 2 (dokumen + pertanyaan)", len(content))
	}

	block0, _ := content[0].(map[string]any)
	text0, _ := block0["text"].(string)
	if !strings.Contains(text0, "dokumen pengetahuan X") {
		t.Error("content-block pertama harus memuat dokumen pengetahuan")
	}
	if !strings.Contains(text0, "DILARANG mengarang") {
		t.Error("content-block pertama harus memuat guardrail anti-halusinasi")
	}
	cacheControl, _ := block0["cache_control"].(map[string]any)
	if cacheControl["type"] != "ephemeral" {
		t.Errorf("content-block pertama harus punya cache_control ephemeral, got %v", block0["cache_control"])
	}

	block1, _ := content[1].(map[string]any)
	if block1["text"] != "apa itu X?" {
		t.Errorf("content-block kedua harus pertanyaan user, got %v", block1["text"])
	}
	if _, hasCache := block1["cache_control"]; hasCache {
		t.Error("content-block kedua (pertanyaan) tidak boleh di-cache — berubah tiap request")
	}
}

func TestAsk_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "bad-token")
	_, err := c.Ask(context.Background(), "doc", "q")
	if err == nil {
		t.Fatal("want error pada status non-2xx, got nil")
	}
}

func TestAsk_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	_, err := c.Ask(context.Background(), "doc", "q")
	if err == nil {
		t.Fatal("want error saat response tanpa content, got nil")
	}
}

func TestAsk_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	_, err := c.Ask(context.Background(), "doc", "q")
	if err == nil {
		t.Fatal("want error saat response bukan JSON valid, got nil")
	}
}

// dummyTool skema minimal untuk test AskWithTools — bentuk InputSchema tak
// penting di sini, cuma perlu JSON valid utk di-marshal ke request.
var dummyTool = Tool{
	Name:        "get_account_summary",
	Description: "ambil ringkasan satu account",
	InputSchema: json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"integer"}}}`),
}

// TestAskWithTools_ToolUseRoundTrip: server membalas tool_use pada putaran
// pertama, lalu teks final pada putaran kedua — verifikasi dispatch dipanggil
// dgn name+input yang benar, tool_result diselipkan balik dgn tool_use_id yang
// sama, dan jawaban akhir diambil dari putaran kedua.
func TestAskWithTools_ToolUseRoundTrip(t *testing.T) {
	round := 0
	var gotToolUseID string
	var gotSecondReq map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		if round == 1 {
			tools, _ := req["tools"].([]any)
			if len(tools) != 1 {
				t.Fatalf("putaran 1: tools count = %d, want 1", len(tools))
			}
			_, _ = w.Write([]byte(`{"stop_reason":"tool_use","content":[
				{"type":"tool_use","id":"toolu_1","name":"get_account_summary","input":{"account_id":42}}
			]}`))
			return
		}

		gotSecondReq = req
		messages, _ := req["messages"].([]any)
		last, _ := messages[len(messages)-1].(map[string]any)
		content, _ := last["content"].([]any)
		block0, _ := content[0].(map[string]any)
		id, _ := block0["tool_use_id"].(string)
		gotToolUseID = id
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"desa Sukamaju berlangganan Basic"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	var gotName string
	var gotInput json.RawMessage
	dispatch := func(ctx context.Context, name string, input json.RawMessage) (string, error) {
		gotName = name
		gotInput = input
		return `{"plan":"Basic"}`, nil
	}

	answer, err := c.AskWithTools(context.Background(), "doc", "status desa Sukamaju?", []Tool{dummyTool}, dispatch)
	if err != nil {
		t.Fatalf("AskWithTools: %v", err)
	}
	if answer != "desa Sukamaju berlangganan Basic" {
		t.Errorf("answer = %q", answer)
	}
	if round != 2 {
		t.Fatalf("jumlah putaran HTTP = %d, want 2", round)
	}
	if gotName != "get_account_summary" {
		t.Errorf("dispatch dipanggil dgn name = %q, want get_account_summary", gotName)
	}
	if !strings.Contains(string(gotInput), "42") {
		t.Errorf("dispatch input = %s, harus memuat account_id 42", gotInput)
	}
	if gotToolUseID != "toolu_1" {
		t.Errorf("tool_result.tool_use_id = %q, want toolu_1 (harus sama dgn id tool_use)", gotToolUseID)
	}

	messages, _ := gotSecondReq["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("putaran 2: messages count = %d, want 3 (user awal, assistant tool_use, user tool_result)", len(messages))
	}
	if messages[1].(map[string]any)["role"] != "assistant" {
		t.Errorf("messages[1].role harus assistant (echo tool_use)")
	}
}

// TestAskWithTools_LastRoundDropsTools: model minta tool_use terus-menerus —
// di putaran TERAKHIR, request tidak boleh menyertakan `tools` (pembatas
// paksa jumlah putaran, lihat komentar jenaMaxToolRounds).
func TestAskWithTools_LastRoundDropsTools(t *testing.T) {
	var toolsPerRound []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		tools, _ := req["tools"].([]any)
		toolsPerRound = append(toolsPerRound, len(tools))

		// Selalu balas tool_use — server "keras kepala" mensimulasikan model yg
		// terus minta tool, untuk menguji putaran terakhir dipaksa tanpa tools.
		_, _ = w.Write([]byte(`{"stop_reason":"tool_use","content":[
			{"type":"tool_use","id":"toolu_x","name":"get_account_summary","input":{}}
		]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	dispatch := func(ctx context.Context, name string, input json.RawMessage) (string, error) {
		return `{}`, nil
	}

	_, err := c.AskWithTools(context.Background(), "doc", "q", []Tool{dummyTool}, dispatch)
	if err == nil {
		t.Fatal("want error karena model tak pernah berhenti minta tool, got nil")
	}
	if len(toolsPerRound) != jenaMaxToolRounds {
		t.Fatalf("jumlah putaran = %d, want %d", len(toolsPerRound), jenaMaxToolRounds)
	}
	for i, n := range toolsPerRound {
		lastRound := i == jenaMaxToolRounds-1
		if lastRound && n != 0 {
			t.Errorf("putaran terakhir (%d) harus tools=0 (dihilangkan), got %d", i+1, n)
		}
		if !lastRound && n != 1 {
			t.Errorf("putaran %d harus tools=1, got %d", i+1, n)
		}
	}
}

// TestAskWithTools_DispatchError: error dari dispatch dikirim balik sbg
// tool_result is_error, bukan menghentikan percakapan dgn panic/error Go.
func TestAskWithTools_DispatchError(t *testing.T) {
	round := 0
	var gotIsError bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if round == 1 {
			_, _ = w.Write([]byte(`{"stop_reason":"tool_use","content":[
				{"type":"tool_use","id":"toolu_1","name":"get_account_summary","input":{}}
			]}`))
			return
		}
		messages, _ := req["messages"].([]any)
		last, _ := messages[len(messages)-1].(map[string]any)
		content, _ := last["content"].([]any)
		block0, _ := content[0].(map[string]any)
		gotIsError, _ = block0["is_error"].(bool)
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"maaf, gagal cek"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	dispatch := func(ctx context.Context, name string, input json.RawMessage) (string, error) {
		return "", errors.New("db: unreachable")
	}

	answer, err := c.AskWithTools(context.Background(), "doc", "q", []Tool{dummyTool}, dispatch)
	if err != nil {
		t.Fatalf("AskWithTools: %v", err)
	}
	if answer != "maaf, gagal cek" {
		t.Errorf("answer = %q", answer)
	}
	if !gotIsError {
		t.Error("tool_result harus is_error=true saat dispatch mengembalikan error")
	}
}
