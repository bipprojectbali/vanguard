package ui

import (
	"strings"
	"testing"
)

func TestJenaAIWidget_HiddenWhenNotShown(t *testing.T) {
	out := renderNode(t, JenaAIWidget(false, "/w/desa/jena-ai/ask"))
	if out != "" {
		t.Errorf("show=false harus tak render apa pun (bukan CSS-hidden), dapat:\n%s", out)
	}
}

func TestJenaAIWidget_PositionNoCollision(t *testing.T) {
	out := renderNode(t, JenaAIWidget(true, "/w/desa/jena-ai/ask"))

	// Tombol trigger = kanan-bawah tapi bottom-24 (bukan bottom-4) — offset
	// vertikal dari toast yang juga kanan-bawah (bottom-4 right-4, TestToast)
	// supaya tak tumpuk visual walau sama sisi.
	for _, want := range []string{"fixed", "bottom-24", "right-4", "z-40"} {
		if !strings.Contains(out, want) {
			t.Errorf("tombol Jena AI kurang %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bottom-4 right-4") {
		t.Errorf("tombol Jena AI tak boleh bentrok posisi toast (bottom-4 right-4):\n%s", out)
	}
	if strings.Contains(out, "top-4 left-4") {
		t.Errorf("tombol Jena AI tak boleh bentrok posisi hamburger (top-4 left-4):\n%s", out)
	}
}

func TestJenaAIWidget_PostURLWired(t *testing.T) {
	out := renderNode(t, JenaAIWidget(true, "/w/desa/jena-ai/ask"))
	if !strings.Contains(out, "/w/desa/jena-ai/ask") {
		t.Errorf("form harus post ke postURL yang dioper handler, tak dirakit view:\n%s", out)
	}
	if !strings.Contains(out, `id="jena-thread"`) {
		t.Errorf("panel harus punya container #jena-thread (target SSE PatchElements append):\n%s", out)
	}
}

func TestJenaAIMessagePair_EscapesContent(t *testing.T) {
	out := renderNode(t, JenaAIMessagePair("<script>alert(1)</script>", "jawaban <b>aman</b>"))
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Errorf("pertanyaan user harus di-escape (gotcha #15), dapat markup mentah:\n%s", out)
	}
	if strings.Contains(out, "<b>aman</b>") {
		t.Errorf("jawaban AI harus di-escape (gotcha #15), dapat markup mentah:\n%s", out)
	}
}

// TestJenaAITurn_DataNodeEscapesScriptBreakout (BL-179): berbeda dari semua
// precedent <script type="application/json"> lain di project (yang isinya
// selalu enum/konfigurasi internal), isi di sini genuinely user/AI-controlled
// — "</script>" mentah di teks pertanyaan/jawaban TAK BOLEH memutus tag,
// harus keluar sbg escape JSON (\u003c/script\u003e), bukan literal.
func TestJenaAITurn_DataNodeEscapesScriptBreakout(t *testing.T) {
	out := renderNode(t, JenaAITurn("</script><script>alert(1)</script>", "jawaban aman"))
	if strings.Contains(out, "</script><script>alert(1)</script>") {
		t.Errorf("node data harus meng-escape </script> mentah (potensi breakout tag), dapat:\n%s", out)
	}
	if !strings.Contains(out, `\u003c/script\u003e`) {
		t.Errorf("json.Marshal harus meng-escape </script> jadi \\u003c/script\\u003e, dapat:\n%s", out)
	}
	if !strings.Contains(out, `class="jena-turn-data"`) {
		t.Errorf("node data harus punya class jena-turn-data (ditangkap ai-widget.js):\n%s", out)
	}
	if !strings.Contains(out, `type="application/json"`) {
		t.Errorf("node data harus type=application/json (CSP-safe, tak dieksekusi):\n%s", out)
	}
	// Bubble tampilan (JenaAIMessagePair) tetap harus ada di fragment yang sama.
	if !strings.Contains(out, "chat-bubble-primary") {
		t.Errorf("JenaAITurn harus tetap memuat bubble tampilan:\n%s", out)
	}
}

func TestJenaAITurn_DataNodeSafeOnMarshalableText(t *testing.T) {
	out := renderNode(t, JenaAITurn("pertanyaan biasa", "jawaban \"berkutip\" & normal"))
	if !strings.Contains(out, `"q":"pertanyaan biasa"`) {
		t.Errorf("node data harus memuat q apa adanya utk teks tanpa karakter berbahaya:\n%s", out)
	}
}
