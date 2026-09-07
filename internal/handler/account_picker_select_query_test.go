package handler

import (
	"testing"

	"go_starter/internal/db"
)

// account_picker_select_query_test.go — BL-76: ListAccountsForSelect (konsumen
// picker Desa di form "Tambah Kontak" global) WAJIB mengalirkan village_code
// Kemendagri agar accountPickerLabel bisa menyusun "kode — nama". Sekaligus
// membuktikan fallback (village_code NULL → nama saja) di sisi query→helper.
//
// Koneksi test = superuser (bypass RLS) → uji aliran kolom + label, bukan RLS.
func TestListAccountsForSelect_MembawaVillageCode(t *testing.T) {
	env, uid := setupTest(t)

	code := "3201052008"
	withCode, err := env.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     env.tenantID,
		VillageName:  "Sukamaju",
		VillageCode:  &code, // desa Kemendagri (pasca BL-66)
		AccountType:  "prospect",
		AccountOwner: &uid,
		CreatedBy:    &uid,
	})
	if err != nil {
		t.Fatalf("seed akun ber-village_code: %v", err)
	}
	noCode, err := env.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     env.tenantID,
		VillageName:  "Cibeureum",
		VillageCode:  nil, // akun lama pra-BL-66
		AccountType:  "prospect",
		AccountOwner: &uid,
		CreatedBy:    &uid,
	})
	if err != nil {
		t.Fatalf("seed akun tanpa village_code: %v", err)
	}

	rows, err := env.q.ListAccountsForSelect(t.Context(), db.ListAccountsForSelectParams{
		ScopeAll: true, // aktor melihat semua desa writable
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("ListAccountsForSelect: %v", err)
	}

	labels := map[int64]string{}
	for _, r := range rows {
		labels[r.ID] = accountPickerLabel(r.VillageCode, r.VillageName)
	}

	if got, want := labels[withCode.ID], "3201052008 — Sukamaju"; got != want {
		t.Errorf("akun ber-village_code: label = %q, mau %q", got, want)
	}
	if got, want := labels[noCode.ID], "Cibeureum"; got != want {
		t.Errorf("akun tanpa village_code: label = %q, mau %q (tanpa em-dash menggantung)", got, want)
	}
}
