package handler

import (
	"encoding/json"
	"strings"

	"go_starter/internal/claudeai"
)

// jena_ai_history.go — parsing riwayat percakapan Jena AI (BL-179), dipisah
// dari jena_ai.go (sudah dekat batas 150 baris route/handler, CLAUDE.md §8)
// supaya satu tanggung jawab per file.

// jenaMaxHistoryTurns membatasi giliran riwayat yang DITERIMA dari klien —
// cap SAMA dgn JENA_HISTORY_MAX di static/ai-widget.js (keputusan user 28
// Sep: 5 giliran terakhir), ditegakkan ULANG di sini krn field ini SEPENUHNYA
// client-controlled (sessionStorage bisa ditempering user, gotcha #15:
// validasi signal client-controlled di backend, jangan percaya batas JS).
const jenaMaxHistoryTurns = 5

// jenaMaxHistoryAnswerChars membatasi panjang tiap jawaban riwayat — jawaban
// (beda dari pertanyaan) bisa sepanjang maxTokens model (~4000 karakter),
// dan payload API dibayar per token; mencegah field hasil tempering jadi
// vektor biaya token.
const jenaMaxHistoryAnswerChars = 4000

// jenaHistoryTurnWire = bentuk JSON riwayat dari klien (field "history",
// diisi ai-widget.js dari sessionStorage) — nama field pendek (q/a) SAMA
// dgn yang ditanam ui.JenaAITurn ke <script class="jena-turn-data">, satu
// kontrak dipakai dua arah (server→klien via turn-data, klien→server lewat
// field ini).
type jenaHistoryTurnWire struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// parseJenaHistory mendekode riwayat dari form field "history" — SEPENUHNYA
// client-controlled (sessionStorage), jadi divalidasi ketat: JSON
// rusak/bukan array/field aneh → riwayat kosong (BUKAN gagalkan seluruh
// request — riwayat cuma penambah konteks, bukan syarat pertanyaan valid),
// dipotong ke jenaMaxHistoryTurns TERAKHIR, tiap Q/A dipotong ke batas
// karakter masing-masing, giliran dgn Q/A kosong (setelah trim) dibuang.
func parseJenaHistory(raw string) []claudeai.Turn {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var wire []jenaHistoryTurnWire
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return nil
	}
	if len(wire) > jenaMaxHistoryTurns {
		wire = wire[len(wire)-jenaMaxHistoryTurns:]
	}

	turns := make([]claudeai.Turn, 0, len(wire))
	for _, w := range wire {
		q := strings.TrimSpace(w.Q)
		a := strings.TrimSpace(w.A)
		if q == "" || a == "" {
			continue
		}
		if len(q) > jenaMaxQuestionChars {
			q = q[:jenaMaxQuestionChars]
		}
		if len(a) > jenaMaxHistoryAnswerChars {
			a = a[:jenaMaxHistoryAnswerChars]
		}
		turns = append(turns, claudeai.Turn{Question: q, Answer: a})
	}
	return turns
}
