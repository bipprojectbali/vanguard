package panel

import (
	"strings"
	"testing"
)

// sales_leads_status_control_test.go — BL-83: kontrol "Ubah Status" di detail
// lead (dipisah dari form profil). Menegakkan: kartu hadir hanya saat aktor
// boleh-tulis & lead belum dikonversi; select lead_status ter-bind signal
// $leadstatus; "Alasan Unqualified" (BL-80) kondisional via data-show; legenda
// status di balik ikon ⓘ; galat PRG (?err) disurfacing.

func leadDetailFixture() LeadDetailView {
	return LeadDetailView{
		Base:     "/w/desa",
		ID:       7,
		LeadName: "Desa Contoh",
		Status:   "New",
		Statuses: []string{"New", "Contacted", "Qualified", "Unqualified"},
		CanWrite: true,
	}
}

// TestLeadStatusControl_HadirSaatBolehTulis: aktor boleh-tulis atas lead belum
// terkonversi → kartu kontrol status ter-render lengkap (select ter-bind +
// tombol simpan + alasan kondisional + legenda).
func TestLeadStatusControl_HadirSaatBolehTulis(t *testing.T) {
	out := renderLeads(t, LeadDetail(leadDetailFixture()))

	for _, want := range []string{
		"Ubah Status",                     // judul kartu
		`action="/w/desa/leads/7/status"`, // NATIVE POST ke aksi status
		`name="lead_status"`,              // select status
		`data-bind="leadstatus"`,          // select menyetir $leadstatus
		`data-signals=`,                   // signal diinisialisasi dari status tersimpan
		"Simpan Status",                   // tombol submit
		// Alasan Unqualified (BL-80) kondisional: tampil hanya saat Unqualified.
		`data-show="$leadstatus == &#39;Unqualified&#39;"`,
		`name="unqualified_reason"`,
		// Legenda status di balik reveal ⓘ (BL-69 dibawa ke kontrol status).
		"Contacted:", "Sudah dihubungi",
		"Unqualified:", "Tak cocok",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kontrol status harus memuat %q:\n%s", want, out)
		}
	}

	// Struktur: pembungkus data-show SEBELUM textarea (textarea di dalam region).
	iShow := strings.Index(out, `data-show="$leadstatus == &#39;Unqualified&#39;"`)
	iTextarea := strings.Index(out, `name="unqualified_reason"`)
	if iShow < 0 || iTextarea < 0 || iShow > iTextarea {
		t.Errorf("BL-80: textarea alasan harus di dalam pembungkus data-show "+
			"(iShow=%d, iTextarea=%d):\n%s", iShow, iTextarea, out)
	}
}

// TestLeadStatusControl_AlasanRequiredKondisional: "Alasan Unqualified" wajib
// (required) HANYA saat Unqualified — di-set via data-attr Datastar (ekspresi
// sama dgn data-show) agar field tersembunyi tak memblokir submit status lain.
// Kalimat penjelasan lama ("Ubah status kualifikasi lead…") sudah dihapus.
func TestLeadStatusControl_AlasanRequiredKondisional(t *testing.T) {
	out := renderLeads(t, LeadDetail(leadDetailFixture()))
	if !strings.Contains(out, `data-attr="{required: $leadstatus == &#39;Unqualified&#39;}"`) {
		t.Errorf("alasan Unqualified harus required KONDISIONAL (data-attr):\n%s", out)
	}
	if strings.Contains(out, "Ubah status kualifikasi lead") {
		t.Errorf("kalimat penjelasan status harus dihapus:\n%s", out)
	}
}

// TestLeadStatusControl_SembunyiSaatConverted: lead terkonversi = status terminal
// 'Converted' → kontrol status TAK dirender (converted tak boleh diputar balik;
// selaras guard query AND NOT converted).
func TestLeadStatusControl_SembunyiSaatConverted(t *testing.T) {
	v := leadDetailFixture()
	v.Status = "Converted"
	v.Converted = true
	out := renderLeads(t, LeadDetail(v))

	if strings.Contains(out, "Ubah Status") || strings.Contains(out, `action="/w/desa/leads/7/status"`) {
		t.Errorf("lead terkonversi TAK boleh menampilkan kontrol Ubah Status:\n%s", out)
	}
}

// TestLeadStatusControl_SembunyiSaatReadOnly: aktor tanpa izin tulis (CanWrite
// false) → kontrol status TAK dirender (kartu adalah aksi tulis).
func TestLeadStatusControl_SembunyiSaatReadOnly(t *testing.T) {
	v := leadDetailFixture()
	v.CanWrite = false
	out := renderLeads(t, LeadDetail(v))

	if strings.Contains(out, "Ubah Status") || strings.Contains(out, `action="/w/desa/leads/7/status"`) {
		t.Errorf("aktor read-only TAK boleh menampilkan kontrol Ubah Status:\n%s", out)
	}
}

// TestLeadDetail_SurfaceErr: galat PRG (?err=CODE → v.Err) ter-render sebagai
// alert di detail lead (kontrol status BL-83 memberi umpan balik kegagalan).
func TestLeadDetail_SurfaceErr(t *testing.T) {
	v := leadDetailFixture()
	v.Err = "Status lead tidak valid."
	out := renderLeads(t, LeadDetail(v))

	if !strings.Contains(out, "Status lead tidak valid.") {
		t.Errorf("detail lead harus menampilkan pesan galat PRG:\n%s", out)
	}
}
