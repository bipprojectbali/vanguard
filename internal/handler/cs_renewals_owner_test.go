package handler

import (
	"net/http"
	"strings"
	"testing"
)

// cs_renewals_owner_test.go — BL-32: dropdown "Owner CS" di form Edit Aksi
// Renewal disaring ke himpunan CS berbasis KAPABILITAS (crm:renewal_mgmt write =
// csm/manager/admin), + default owner = assigned_csm saat kosong, tanpa data-loss
// pada owner warisan non-CS. Koneksi test = superuser (bypass RLS) → uji LOGIKA
// handler; enforcer bisnis dimuat per-tenant oleh setupAccounts.

// optionSelected melaporkan apakah <option value="<id>"> dirender `selected`.
// gomponents merender atribut sesuai urutan kemunculan → `value="5" selected`.
func optionSelected(body string, id int64) bool {
	return strings.Contains(body, `value="`+itoa(id)+`" selected`)
}

// renderRenewalEdit menjalankan GET edit sebagai peran tertentu, memastikan 200,
// dan mengembalikan body HTML form.
func (e *testEnv) renderRenewalEdit(t *testing.T, uid, subID int64, bizRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/renewal-management/"+itoa(subID)+"/edit", nil, itoa(subID))
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.CSRenewalEdit)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET edit harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// seedCSMembers menaburkan satu anggota per peran + satu tanpa business_role,
// mengembalikan peta nama-peran → user ID.
func (e *testEnv) seedCSMembers(t *testing.T) map[string]int64 {
	t.Helper()
	ids := map[string]int64{}
	for _, role := range []string{"csm", "manager", "admin", "sales", "support"} {
		u := e.seedMember(t, role+"@local", "member", 0)
		e.assignBiz(t, u.ID, role)
		ids[role] = u.ID
	}
	// Anggota tanpa business_role (belum diberi peran CRM).
	ids["none"] = e.seedMember(t, "none@local", "member", 0).ID
	return ids
}

// TestCSRenewals_OwnerDropdownFiltersToCS: dropdown hanya memuat peran ber-write
// renewal_mgmt (csm/manager/admin); sales/support/tanpa-peran dikecualikan.
func TestCSRenewals_OwnerDropdownFiltersToCS(t *testing.T) {
	env, uid := setupAccounts(t)
	ids := env.seedCSMembers(t)
	// Sub tanpa owner & tanpa assigned_csm → ensureID 0, dropdown = filter murni.
	_, _, sub := env.seedCSRenewal(t, "Desa Filter Owner", nil)

	body := env.renderRenewalEdit(t, uid, sub.ID, "admin")

	for _, role := range []string{"csm", "manager", "admin"} {
		if !strings.Contains(body, `value="`+itoa(ids[role])+`"`) {
			t.Errorf("peran CS %q harus muncul di dropdown owner", role)
		}
	}
	for _, role := range []string{"sales", "support", "none"} {
		if strings.Contains(body, `value="`+itoa(ids[role])+`"`) {
			t.Errorf("peran non-CS %q TIDAK boleh muncul di dropdown owner", role)
		}
	}
}

// TestCSRenewals_OwnerLegacyNonCSStaysSelected: owner tersimpan menunjuk anggota
// di LUAR himpunan CS (warisan) TETAP dirender sebagai opsi terpilih — cegah
// data-loss saat form disimpan ulang.
func TestCSRenewals_OwnerLegacyNonCSStaysSelected(t *testing.T) {
	env, uid := setupAccounts(t)
	ids := env.seedCSMembers(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Owner Warisan", nil)

	// Tetapkan owner warisan = sales (non-CS) langsung di DB.
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET renewal_owner = $1 WHERE id = $2`, ids["sales"], sub.ID); err != nil {
		t.Fatalf("set renewal_owner warisan: %v", err)
	}

	body := env.renderRenewalEdit(t, uid, sub.ID, "admin")

	if !strings.Contains(body, `value="`+itoa(ids["sales"])+`"`) {
		t.Errorf("owner warisan non-CS harus tetap muncul di dropdown (anti data-loss)")
	}
	if !optionSelected(body, ids["sales"]) {
		t.Errorf("owner warisan non-CS harus terpilih (selected)")
	}
}

// TestCSRenewals_OwnerDefaultToAssignedCSMWhenEmpty: owner kosong pada desa
// ber-assigned_csm → dropdown preselect ke CSM itu (bukan user aktif).
func TestCSRenewals_OwnerDefaultToAssignedCSMWhenEmpty(t *testing.T) {
	env, uid := setupAccounts(t)
	ids := env.seedCSMembers(t)
	csmID := ids["csm"]
	_, _, sub := env.seedCSRenewal(t, "Desa Default CSM", &csmID) // assigned_csm = csm

	body := env.renderRenewalEdit(t, uid, sub.ID, "admin")

	if !optionSelected(body, csmID) {
		t.Errorf("owner kosong harus preselect ke assigned_csm (%d)", csmID)
	}
}

// TestCSRenewals_OwnerDefaultEmptyWhenNoAssignedCSM: owner kosong & desa tanpa
// assigned_csm → tak ada yang dipaksakan (opsi kosong terpilih).
func TestCSRenewals_OwnerDefaultEmptyWhenNoAssignedCSM(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedCSMembers(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Tanpa CSM", nil) // assigned_csm NULL

	body := env.renderRenewalEdit(t, uid, sub.ID, "admin")

	// Opsi kosong "— Pilih Owner CS —" (value="") harus selected.
	if !strings.Contains(body, `value="" selected`) {
		t.Errorf("desa tanpa assigned_csm harus membiarkan owner kosong (opsi kosong selected)")
	}
}

// TestCSRenewals_OwnerNotResetWhenAlreadySet: owner sudah di-set TIDAK ditimpa
// default assigned_csm — idempoten, hormati keputusan sebelumnya.
func TestCSRenewals_OwnerNotResetWhenAlreadySet(t *testing.T) {
	env, uid := setupAccounts(t)
	ids := env.seedCSMembers(t)
	mgrID := ids["manager"]
	// Desa dengan assigned_csm = csm, TAPI owner sudah di-set ke manager.
	csmID := ids["csm"]
	_, _, sub := env.seedCSRenewal(t, "Desa Owner Terset", &csmID)
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET renewal_owner = $1 WHERE id = $2`, mgrID, sub.ID); err != nil {
		t.Fatalf("set renewal_owner: %v", err)
	}

	body := env.renderRenewalEdit(t, uid, sub.ID, "admin")

	if !optionSelected(body, mgrID) {
		t.Errorf("owner tersimpan (%d) harus tetap terpilih", mgrID)
	}
	if optionSelected(body, csmID) {
		t.Errorf("assigned_csm (%d) TIDAK boleh menimpa owner tersimpan", csmID)
	}
}
