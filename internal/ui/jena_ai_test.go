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
