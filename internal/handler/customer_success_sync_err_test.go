package handler

import (
	"net/http"
	"strings"
	"testing"
)

// customer_success_sync_err_test.go — BL-27 regresi: CustomerSuccessSync SELALU
// redirect ke halaman detail (CustomerSuccessDetail) dengan ?err=desaplus_*,
// tapi handler detail sempat memetakan kode itu lewat wsErrMsg (peta kode
// workspace umum) alih-alih customerSuccessErrMsg (peta kode modul CS, tempat
// desaplus_* sebenarnya didefinisikan) — kode tak dikenal wsErrMsg jatuh ke ""
// dan alert-nya HILANG SENYAP: user klik "Sinkron dari Desa+", halaman termuat
// ulang, tak ada penjelasan apa pun (persis kelas bug yang diperingatkan
// komentar workspace_errmsg.go sendiri). customer_success_sync_test.go hanya
// menguji Location header sampai err=desaplus_not_found ADA di URL — tak pernah
// menguji GET berikutnya benar-benar merender pesannya. Test ini menutup gap
// itu: memanggil CustomerSuccessDetail LANGSUNG dengan tiap kode ?err= yang
// dikirim CustomerSuccessSync, memastikan toast berisi teks yang bisa dibaca.
func TestCustomerSuccessDetail_DesaPlusErrDirender(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"desaplus_disabled", "belum dikonfigurasi"},
		{"desaplus_no_code", "belum punya kode desa"},
		{"desaplus_not_found", "belum terpasang di sisi Desa+"},
		{"desaplus_unauthorized", "Token integrasi Desa+ ditolak"},
		{"desaplus_failed", "Gagal menghubungi Desa+"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Err Render", &uid, nil, nil)
			env.makeCustomer(t, a.ID, "Active")

			req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success?err="+c.code, nil, itoa(a.ID))
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, c.want) {
				t.Errorf("?err=%s harus merender pesan yang memuat %q — alert hilang senyap (lihat wsErrMsg vs customerSuccessErrMsg)", c.code, c.want)
			}
		})
	}
}
