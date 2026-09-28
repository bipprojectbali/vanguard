package claudeai

import (
	"encoding/json"
	"testing"
)

func TestBuildMessages_NoHistory(t *testing.T) {
	msgs := buildMessages("dok", nil, "pertanyaan pertama")
	if len(msgs) != 1 {
		t.Fatalf("messages count = %d, want 1 (perilaku lama, tanpa riwayat)", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("messages[0].Role = %q, want user", msgs[0].Role)
	}
	if len(msgs[0].Content) != 2 || msgs[0].Content[1].Text != "pertanyaan pertama" {
		t.Errorf("messages[0].Content harus [guardrail+dok, pertanyaan], got %+v", msgs[0].Content)
	}
}

// TestBuildMessages_WithHistory_Alternates: 3 giliran lama + 1 pertanyaan baru
// harus berselang-seling murni user/assistant, dimulai & diakhiri user (syarat
// keras Anthropic Messages API — role sama berturut-turut ditolak).
func TestBuildMessages_WithHistory_Alternates(t *testing.T) {
	history := []Turn{
		{Question: "q1", Answer: "a1"},
		{Question: "q2", Answer: "a2"},
		{Question: "q3", Answer: "a3"},
	}
	msgs := buildMessages("dok", history, "q4 (baru)")

	wantRoles := []string{"user", "assistant", "user", "assistant", "user", "assistant", "user"}
	if len(msgs) != len(wantRoles) {
		t.Fatalf("messages count = %d, want %d", len(msgs), len(wantRoles))
	}
	for i, want := range wantRoles {
		if msgs[i].Role != want {
			t.Errorf("messages[%d].Role = %q, want %q (harus berselang-seling murni)", i, msgs[i].Role, want)
		}
	}

	// Isi harus sesuai urutan riwayat: q1 masuk openingMessage (bersama
	// guardrail), a1 di messages[1], q2/a2 di [2]/[3], q3/a3 di [4]/[5],
	// pertanyaan baru di [6] (terakhir).
	if msgs[0].Content[1].Text != "q1" {
		t.Errorf("openingMessage harus memuat q1 (giliran pertama riwayat), got %q", msgs[0].Content[1].Text)
	}
	if msgs[1].Content[0].Text != "a1" {
		t.Errorf("messages[1] harus a1, got %q", msgs[1].Content[0].Text)
	}
	if msgs[2].Content[0].Text != "q2" || msgs[3].Content[0].Text != "a2" {
		t.Errorf("messages[2]/[3] harus q2/a2, got %q/%q", msgs[2].Content[0].Text, msgs[3].Content[0].Text)
	}
	if msgs[4].Content[0].Text != "q3" || msgs[5].Content[0].Text != "a3" {
		t.Errorf("messages[4]/[5] harus q3/a3, got %q/%q", msgs[4].Content[0].Text, msgs[5].Content[0].Text)
	}
	if msgs[6].Content[0].Text != "q4 (baru)" {
		t.Errorf("messages terakhir harus pertanyaan baru, got %q", msgs[6].Content[0].Text)
	}
}

// TestBuildMessages_CacheAnchorStable: content-block PERTAMA pesan PERTAMA
// (guardrail+knowledgeMD, cache_control ephemeral) harus BYTE-IDENTIK antar
// call selama knowledgeMD & giliran-pertama-riwayat sama — walau pertanyaan
// baru berbeda. Syarat prompt-cache hit (Anthropic): jangkar cache tak boleh
// pindah/berubah gara-gara riwayat.
func TestBuildMessages_CacheAnchorStable(t *testing.T) {
	history := []Turn{{Question: "q1", Answer: "a1"}}

	msgsA := buildMessages("dok pengetahuan", history, "pertanyaan kedua")
	msgsB := buildMessages("dok pengetahuan", history, "pertanyaan KETIGA yang beda")

	blockA, err := json.Marshal(msgsA[0].Content[0])
	if err != nil {
		t.Fatalf("marshal block A: %v", err)
	}
	blockB, err := json.Marshal(msgsB[0].Content[0])
	if err != nil {
		t.Fatalf("marshal block B: %v", err)
	}
	if string(blockA) != string(blockB) {
		t.Errorf("content-block pertama (jangkar cache) harus byte-identik antar call, got:\nA=%s\nB=%s", blockA, blockB)
	}
	if msgsA[0].Content[0].CacheControl == nil || msgsA[0].Content[0].CacheControl.Type != "ephemeral" {
		t.Error("content-block pertama tetap harus cache_control ephemeral walau ada riwayat")
	}
}
