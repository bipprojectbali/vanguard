package handler

import (
	"net/http"
	"strings"
	"testing"
)

// customer_success_button_style_test.go — regresi visual: tiga tombol header
// halaman detail Customer Success ("Penugasan CS", "Sinkron dari Desa+",
// "Sunting") wajib SATU GAYA (btn-outline) — permintaan user 21 Sep
// (screenshot: dua tombol beda gaya dari "Sinkron dari Desa+"). Test ini
// bikin ketiga gerbang (CanAssign/CanSync/CanWrite) sekaligus true supaya
// ketiga tombol tampil bareng, lalu kunci jumlah kemunculan class btn-outline.

func TestCustomerSuccessDetail_TombolHeaderSeragam(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccountWithVillageCode(t, "Desa Tombol Seragam", nil, &uid, nil, "VG-STYLE-1")
	env.makeCustomer(t, a.ID, "Active")
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		villageSummaryOK(w, "", 0, nil)
	})

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	rec := env.runAccount(uid, "member", "csm", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	for _, want := range []string{"Penugasan CS", "Sinkron dari Desa+", "Isi Customer Success"} {
		if !strings.Contains(body, want) {
			t.Errorf("tombol %q harus tampil (prasyarat: semua gerbang aktif), body:\n%s", want, body)
		}
	}
	// Ketiga tombol header (Penugasan CS, Sinkron dari Desa+, Isi Customer
	// Success/Sunting) wajib pakai class btn-outline yang sama.
	if got := strings.Count(body, "btn-outline"); got < 3 {
		t.Errorf("ketiga tombol header CS harus btn-outline (got %d kemunculan), body:\n%s", got, body)
	}
}
