package panel

import (
	"strings"
	"testing"
)

// sales_leads_detail_test.go — invarian tata letak detail lead pasca redesign
// wireframe 4.1 (BL-119/120/121): kartu Sistem&Audit, modal hapus konfirmasi,
// baris "Alasan Unqualified" kondisional di kartu Kualifikasi, dan grup aksi
// header (Konversi / Lihat Deal). Fixture leadDetailFixture di
// sales_leads_status_control_test.go (satu paket).

// TestLeadDetail_KartuSistemAudit: BL-119 — kartu "Sistem & Audit" hadir dengan
// pembuat/pengubah + waktu (nilai sudah diformat handler).
func TestLeadDetail_KartuSistemAudit(t *testing.T) {
	v := leadDetailFixture()
	v.CreatedByName = "Budi"
	v.CreatedAt = "01 Jan 2026 09:00"
	v.UpdatedByName = "Siti"
	v.UpdatedAt = "02 Jan 2026 10:30"
	out := renderLeads(t, LeadDetail(v))

	for _, want := range []string{
		"Sistem &amp; Audit", // '&' ter-escape saat render
		"Dibuat Oleh", "Budi", "01 Jan 2026 09:00",
		"Diubah Oleh", "Siti", "Terakhir Diubah", "02 Jan 2026 10:30",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kartu Sistem & Audit harus memuat %q:\n%s", want, out)
		}
	}
}

// TestLeadDetail_AlasanUnqualifiedKondisional: BL-121 — BARIS "Alasan
// Unqualified" di KARTU Kualifikasi (<dt>) HANYA muncul saat status Unqualified;
// status lain tak menampilkan baris itu (bukan lagi "—" permanen). Diskriminator
// "</dt>": label yang sama juga hadir di MODAL status (sbg <label>), tapi baris
// kartu unik lewat penutup <dt>.
func TestLeadDetail_AlasanUnqualifiedKondisional(t *testing.T) {
	const cardRow = "Alasan Unqualified</dt>"

	// Status non-Unqualified → baris kartu tak muncul.
	v := leadDetailFixture()
	v.Status = "New"
	if out := renderLeads(t, LeadDetail(v)); strings.Contains(out, cardRow) {
		t.Errorf("status %q TAK boleh menampilkan baris Alasan Unqualified di kartu:\n%s", v.Status, out)
	}

	// Status Unqualified → baris kartu muncul dengan alasannya.
	v = leadDetailFixture()
	v.Status = "Unqualified"
	v.UnqualifiedReason = "Anggaran tak cukup"
	out := renderLeads(t, LeadDetail(v))
	if !strings.Contains(out, cardRow) || !strings.Contains(out, "Anggaran tak cukup") {
		t.Errorf("status Unqualified harus menampilkan baris kartu + alasan:\n%s", out)
	}
}

// TestLeadDetail_ModalHapus: BL-120 — aktor boleh-tulis melihat tombol "Hapus"
// (pembuka modal) + modal konfirmasi ber-form NATIVE POST ke /delete (gotcha #16).
func TestLeadDetail_ModalHapus(t *testing.T) {
	out := renderLeads(t, LeadDetail(leadDetailFixture()))
	for _, want := range []string{
		"Hapus",
		"Hapus Lead",                      // judul modal
		`action="/w/desa/leads/7/delete"`, // NATIVE POST hapus
		`method="post"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("modal hapus harus memuat %q:\n%s", want, out)
		}
	}
}

// TestLeadDetail_ModalHapusSembunyiSaatReadOnly: aktor read-only TAK melihat
// tombol/modal hapus (hapus = aksi tulis).
func TestLeadDetail_ModalHapusSembunyiSaatReadOnly(t *testing.T) {
	v := leadDetailFixture()
	v.CanWrite = false
	out := renderLeads(t, LeadDetail(v))
	if strings.Contains(out, `action="/w/desa/leads/7/delete"`) {
		t.Errorf("aktor read-only TAK boleh melihat aksi hapus:\n%s", out)
	}
}

// TestLeadDetail_AksiKonversi: lead Qualified belum dikonversi & aktor boleh
// tulis → tombol "Konversi Lead" menuju /convert.
func TestLeadDetail_AksiKonversi(t *testing.T) {
	v := leadDetailFixture()
	v.Status = "Qualified"
	v.CanConvert = true
	out := renderLeads(t, LeadDetail(v))
	for _, want := range []string{"Konversi Lead", `href="/w/desa/leads/7/convert"`} {
		if !strings.Contains(out, want) {
			t.Errorf("lead Qualified harus menampilkan aksi Konversi %q:\n%s", want, out)
		}
	}
}

// TestLeadDetail_TautanDealSaatConverted: lead terkonversi → tautan "Lihat Deal"
// menuju deal hasil konversi, BUKAN tombol Konversi/Ubah Status.
func TestLeadDetail_TautanDealSaatConverted(t *testing.T) {
	v := leadDetailFixture()
	v.Status = "Converted"
	v.Converted = true
	v.ConvertedDealID = "42"
	out := renderLeads(t, LeadDetail(v))
	if !strings.Contains(out, `href="/w/desa/deals/42"`) || !strings.Contains(out, "Lihat Deal") {
		t.Errorf("lead terkonversi harus menautkan ke Deal:\n%s", out)
	}
	if strings.Contains(out, "Konversi Lead") {
		t.Errorf("lead terkonversi TAK boleh menampilkan tombol Konversi:\n%s", out)
	}
	// Terkonversi = terminal: Sunting/Ubah Status/Hapus disembunyikan (backend
	// menolak; hanya "Lihat Deal" tersisa).
	for _, gone := range []string{
		`href="/w/desa/leads/7/edit"`,     // tombol Sunting
		"Ubah Status",                     // pemicu modal status
		`action="/w/desa/leads/7/delete"`, // form hapus (modal tak dirender)
	} {
		if strings.Contains(out, gone) {
			t.Errorf("lead terkonversi TAK boleh memuat %q:\n%s", gone, out)
		}
	}
}
