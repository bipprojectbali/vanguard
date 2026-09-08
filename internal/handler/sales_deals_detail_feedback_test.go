package handler

import (
	"html"
	"net/http"
	"strings"
	"testing"
)

// sales_deals_detail_feedback_test.go — BL-99: halaman DETAIL deal (GET) MELALUI
// HTTP menampilkan umpan balik PRG (?err/?ok). Regresi bug "ubah tahap ke Closed
// Won/Lost tak tersimpan & tanpa umpan balik": gerbang tahap terminal yang gagal
// (win_loss/loss_reason wajib, atau subscriptionFromWonDeal gagal) me-redirect ke
// DETAIL ini dgn ?err — sebelum perbaikan, halaman tak membaca query itu sehingga
// penolakan tampak seperti "deal tak berubah". Tahap non-terminal tak melewati
// gerbang → sukses. Test ini menegakkan pesan kini SURFACED di tempat form berada.

// TestDealDetail_TerminalStageFeedbackSurfaced: GET /deals/{id}?err=CODE →
// halaman memuat pesan wsErrMsg(CODE) untuk kode gerbang tahap terminal; ?ok=staged
// → pesan sukses; tanpa query → tak ada banner umpan balik.
func TestDealDetail_TerminalStageFeedbackSurfaced(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Umpan Balik", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	// Kode err yang bisa memantulkan user ke DETAIL saat menutup deal.
	for _, code := range []string{"win_loss", "loss_reason", "sub_active_exists", "failed"} {
		t.Run("err="+code, func(t *testing.T) {
			want := wsErrMsg(code)
			if want == "" {
				t.Fatalf("prasyarat: wsErrMsg(%q) tak boleh kosong", code)
			}
			req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID)+"?err="+code, nil, itoa(deal.ID))
			rec := env.runAccount(uid, "owner", "admin", req, env.h.DealDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			// Banner me-render pesan via g.Text → HTML-escape (& → &amp;, dst).
			// Bandingkan terhadap bentuk ter-escape agar pesan ber-"&"/"'" cocok.
			body := rec.Body.String()
			if !strings.Contains(body, html.EscapeString(want)) {
				t.Errorf("err=%q: pesan %q harus tampil di detail deal, body:\n%s", code, want, body)
			}
			if !strings.Contains(body, `id="deal-err"`) {
				t.Errorf("err=%q: banner error (deal-err) harus dirender:\n%s", code, body)
			}
		})
	}

	t.Run("ok=staged", func(t *testing.T) {
		want := dealsMsg("staged")
		req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID)+"?ok=staged", nil, itoa(deal.ID))
		rec := env.runAccount(uid, "owner", "admin", req, env.h.DealDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, html.EscapeString(want)) || !strings.Contains(body, `id="deal-ok"`) {
			t.Errorf("ok=staged: banner sukses %q harus tampil, body:\n%s", want, body)
		}
	})

	t.Run("no-query", func(t *testing.T) {
		req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID), nil, itoa(deal.ID))
		rec := env.runAccount(uid, "owner", "admin", req, env.h.DealDetail)
		body := rec.Body.String()
		if strings.Contains(body, `id="deal-err"`) || strings.Contains(body, `id="deal-ok"`) {
			t.Errorf("tanpa query: banner umpan balik tak boleh dirender:\n%s", body)
		}
	})
}
