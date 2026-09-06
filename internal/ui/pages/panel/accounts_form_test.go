package panel

import (
	"strings"
	"testing"
)

// accounts_form_test.go — regresi BL-60: penyederhanaan form Tambah/Sunting Desa.
//   (1) input "Kode Sistem" (entity_code) dilepas dari UI (kode dipakai =
//       village_code Kemendagri; entity_code tetap otomatis di backend).
//   (2) field "Teritori" dilepas dari UI (data lama dipertahankan handler).
//   (3) Anggaran (APBDes) jadi input UANG: prefix "Rp" + data-numgroup +
//       inputmode numeric, dan form memuat numgroup.js. BUKAN teks bebas.

func renderAccountForm(t *testing.T, v AccountFormView) string {
	t.Helper()
	var sb strings.Builder
	if err := AccountForm(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// baseFormView = view minimal utk render form (tambah).
func baseFormView() AccountFormView {
	return AccountFormView{
		Base:            "/w/desa",
		Action:          "/w/desa/accounts",
		IsEdit:          false,
		RegionsJSON:     "[]",
		VillagesURL:     "/w/desa/accounts/villages",
		PhoneEditable:   true,
		Types:           []string{"prospect", "customer"},
		Statuses:        []string{"Desa"},
		Classifications: []string{"Maju"},
	}
}

// TestAccountForm_DropsSystemCodeAndTerritory: input kode sistem & teritori TAK
// boleh ada di form (add maupun edit).
func TestAccountForm_DropsSystemCodeAndTerritory(t *testing.T) {
	for _, isEdit := range []bool{false, true} {
		v := baseFormView()
		v.IsEdit = isEdit
		out := renderAccountForm(t, v)

		for _, banned := range []string{
			`name="entity_code"`, // input kode sistem
			"Kode Sistem",        // label kode sistem
			`name="territory"`,   // input teritori
			"Teritori",           // label teritori
		} {
			if strings.Contains(out, banned) {
				t.Errorf("isEdit=%v: form TAK boleh memuat %q (dilepas BL-60):\n%s", isEdit, banned, out)
			}
		}
	}
}

// TestAccountForm_HasVillageSelect (BL-66): form memuat dropdown Desa/Kelurahan
// (name="village_id") — level 4 wilayah yang jadi SATU-SATUNYA sumber village_code
// Kemendagri — dgn kait lazy-fetch (data-villages-url dari VillagesURL) &
// placeholder "— Pilih Desa/Kelurahan —"; wajib (required) di form tambah.
func TestAccountForm_HasVillageSelect(t *testing.T) {
	out := renderAccountForm(t, baseFormView())

	for _, want := range []string{
		`name="village_id"`,                             // dropdown Desa level 4
		`data-region-level="4"`,                         // ditangani regions.js sbg level 4
		`data-villages-url="/w/desa/accounts/villages"`, // endpoint lazy-fetch (VillagesURL)
		"— Pilih Desa/Kelurahan —",                      // placeholder
		"Desa/Kelurahan",                                // label
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form tambah harus memuat %q utk dropdown Desa (BL-66):\n%s", want, out)
		}
	}
}

// TestAccountForm_BudgetIsMoneyField: Anggaran (APBDes) = input uang ber-prefix
// "Rp" + kait pengelompokan ribuan, form memuat numgroup.js, dan field itu tak
// memakai type="number" (yang menolak nilai terformat).
func TestAccountForm_BudgetIsMoneyField(t *testing.T) {
	out := renderAccountForm(t, baseFormView())

	for _, want := range []string{
		`name="village_budget"`, // field anggaran tetap ada
		`data-numgroup`,         // kait pengelompokan ribuan
		`inputmode="numeric"`,   // keypad angka mobile
		`pattern="[0-9.]*"`,     // pola uang (titik ribuan ditoleransi)
		">Rp<",                  // afiks Rp di kiri input
		`/static/numgroup.js`,   // skrip format+normalisasi termuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form harus memuat %q utk Anggaran (APBDes):\n%s", want, out)
		}
	}
}

// TestAccountForm_BudgetPrefillNoDecimal: nilai prefill anggaran dirender apa
// adanya (digit polos, tanpa ".00") — numgroup.js membuang non-digit, jadi
// ".00" akan salah tampil. Handler memberi moneyRupiahStr; view hanya menaruh.
func TestAccountForm_BudgetPrefillNoDecimal(t *testing.T) {
	v := baseFormView()
	v.IsEdit = true
	v.Fields.VillageBudget = "750000000" // seperti moneyRupiahStr hasilkan
	out := renderAccountForm(t, v)

	if !strings.Contains(out, `value="750000000"`) {
		t.Errorf("prefill anggaran harus digit polos:\n%s", out)
	}
	if strings.Contains(out, `value="750000000.00"`) {
		t.Errorf("prefill anggaran TAK boleh membawa desimal (.00):\n%s", out)
	}
}

// TestAccountForm_HintAsTapInfoIcon (BL-65): field ber-hint (mis. Tipe Akun) TAK
// lagi melebar sm:col-span-2 — cell tetap single-column `grid gap-1 min-w-0` —
// dan keterangan pindah ke ikon ⓘ tap-friendly (<details class="hint-reveal">),
// bukan baris teks di bawah input. Keterangan tetap ADA & tap-target ikon ≥44px.
func TestAccountForm_HintAsTapInfoIcon(t *testing.T) {
	out := renderAccountForm(t, baseFormView())

	// (1) cell field ber-hint single-column + LANGSUNG membungkus reveal ⓘ →
	//     membuktikan pelebaran sm:col-span-2 lama sudah dilepas.
	if !strings.Contains(out, `class="grid gap-1 min-w-0"><details class="hint-reveal`) {
		t.Errorf("field ber-hint harus single-column + membungkus <details hint-reveal> (BL-65):\n%s", out)
	}
	// (2) keterangan tetap ada (contoh: hint Tipe Akun), dipindah ke ikon ⓘ.
	if !strings.Contains(out, "Prospect = calon pelanggan") {
		t.Errorf("keterangan (hint) harus tetap ada:\n%s", out)
	}
	// (3) tap-target ikon ⓘ ≥44px.
	if !strings.Contains(out, "min-h-11 min-w-11") {
		t.Errorf("ikon ⓘ harus tap-target ≥44px (min-h-11 min-w-11):\n%s", out)
	}
}
