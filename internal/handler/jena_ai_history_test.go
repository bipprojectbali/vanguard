package handler

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseJenaHistory_Empty(t *testing.T) {
	if got := parseJenaHistory(""); got != nil {
		t.Errorf("string kosong harus nil, got %+v", got)
	}
	if got := parseJenaHistory("   "); got != nil {
		t.Errorf("whitespace-only harus nil, got %+v", got)
	}
}

// TestParseJenaHistory_Malformed: field ini SEPENUHNYA client-controlled
// (sessionStorage) — JSON rusak/bukan array TAK BOLEH panic atau menggagalkan
// seluruh request, cukup degradasi ke riwayat kosong.
func TestParseJenaHistory_Malformed(t *testing.T) {
	cases := []string{
		"bukan json",
		`{"q":"tak berbentuk array"}`,
		`[{"q": 123, "a": "angka bukan string"}]`,
		`[`,
	}
	for _, raw := range cases {
		got := parseJenaHistory(raw)
		if got != nil {
			t.Errorf("parseJenaHistory(%q) = %+v, want nil (JSON rusak/tak sah)", raw, got)
		}
	}
}

func TestParseJenaHistory_ValidDecodesInOrder(t *testing.T) {
	raw := `[{"q":"q1","a":"a1"},{"q":"q2","a":"a2"}]`
	got := parseJenaHistory(raw)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Question != "q1" || got[0].Answer != "a1" {
		t.Errorf("turn[0] = %+v, want q1/a1", got[0])
	}
	if got[1].Question != "q2" || got[1].Answer != "a2" {
		t.Errorf("turn[1] = %+v, want q2/a2", got[1])
	}
}

// TestParseJenaHistory_CapsToLastNTurns: lebih dari jenaMaxHistoryTurns harus
// dipotong ke N TERAKHIR (bukan N pertama) — riwayat terbaru yang paling
// relevan sebagai konteks, bukan yang paling lama.
func TestParseJenaHistory_CapsToLastNTurns(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 1; i <= jenaMaxHistoryTurns+3; i++ {
		if i > 1 {
			sb.WriteString(",")
		}
		n := strconv.Itoa(i)
		sb.WriteString(`{"q":"q` + n + `","a":"a` + n + `"}`)
	}
	sb.WriteString("]")

	got := parseJenaHistory(sb.String())
	if len(got) != jenaMaxHistoryTurns {
		t.Fatalf("len = %d, want %d (cap)", len(got), jenaMaxHistoryTurns)
	}
	// Giliran pertama yang tersisa harus giliran ke-4 — 3 giliran tertua
	// (q1..q3) harus terbuang, bukan giliran terbaru.
	wantFirst := "q4"
	if got[0].Question != wantFirst {
		t.Errorf("giliran tersisa pertama = %q, want %q (harus buang giliran TERTUA)", got[0].Question, wantFirst)
	}
	wantLast := "q" + strconv.Itoa(jenaMaxHistoryTurns+3)
	if got[len(got)-1].Question != wantLast {
		t.Errorf("giliran tersisa terakhir = %q, want %q", got[len(got)-1].Question, wantLast)
	}
}

func TestParseJenaHistory_DropsEmptyTurnsAfterTrim(t *testing.T) {
	raw := `[{"q":"  ","a":"a1"},{"q":"q2","a":"   "},{"q":"q3","a":"a3"}]`
	got := parseJenaHistory(raw)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (dua giliran ber-Q/A kosong setelah trim harus dibuang)", len(got))
	}
	if got[0].Question != "q3" || got[0].Answer != "a3" {
		t.Errorf("turn tersisa = %+v, want q3/a3", got[0])
	}
}

func TestParseJenaHistory_TruncatesOverlongFields(t *testing.T) {
	longQ := strings.Repeat("q", jenaMaxQuestionChars+500)
	longA := strings.Repeat("a", jenaMaxHistoryAnswerChars+500)
	raw := `[{"q":"` + longQ + `","a":"` + longA + `"}]`

	got := parseJenaHistory(raw)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(got[0].Question) != jenaMaxQuestionChars {
		t.Errorf("panjang Q = %d, want dipotong ke %d", len(got[0].Question), jenaMaxQuestionChars)
	}
	if len(got[0].Answer) != jenaMaxHistoryAnswerChars {
		t.Errorf("panjang A = %d, want dipotong ke %d", len(got[0].Answer), jenaMaxHistoryAnswerChars)
	}
}
