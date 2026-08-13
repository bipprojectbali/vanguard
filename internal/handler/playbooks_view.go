package handler

import (
	"context"

	"go_starter/internal/authz"
)

// playbooks_view.go — gerbang F2 (Casbin bisnis) modul Playbooks (Modul 6
// Customer Success, slice A2). Satu tempat objek Casbin "crm:playbooks"
// disebut, agar menu sidebar & gate halaman selalu menyebut objek/aksi yang
// SAMA (nol menu hantu). Meniru sla_policies_view.go (slice A1).
//
// Katalog master TANPA F3 ownership: playbook milik WORKSPACE, bukan per-
// desa — RLS (h.q ber-tenant) mengurungnya, tak ada kolom pemilik. Tulis =
// manager+csm+admin (default Casbin: manager & csm write, sales/support tak
// punya objek ini — lihat business_policy.csv/business_defaults.go, sudah
// lengkap sejak sebelum slice ini, tak perlu diubah).

// canViewPlaybooks = gerbang READ katalog. Sumber tunggal untuk menu & gate
// halaman PlaybooksList/PlaybookEdit.
func canViewPlaybooks(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:playbooks", "read")
}

// canWritePlaybooksPerm = izin F2 mentah tulis playbook (tanpa cek arsip).
// Dipakai gate aksi POST; write mencakup read di Casbin.
func canWritePlaybooksPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:playbooks", "write")
}

// canWritePlaybooks = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil
// authz.
func canWritePlaybooks(ctx context.Context) bool {
	return canWritePlaybooksPerm(ctx) && !IsReadOnly(ctx)
}

// playbooksMsg memetakan ?ok= → pesan sukses Playbooks (dipisah dari
// playbooksErrMsg agar alert sukses & galat tak pernah tertukar variannya).
func playbooksMsg(code string) string {
	switch code {
	case "created":
		return "Playbook ditambahkan ke katalog."
	case "saved":
		return "Perubahan playbook disimpan."
	case "drafted":
		return "Playbook dijadikan draf — tak lagi tampil di picker baru."
	case "activated":
		return "Playbook diaktifkan kembali."
	default:
		return ""
	}
}

// playbooksErrMsg memetakan ?err= → pesan galat form Playbooks. Lokal ke
// modul (tak menumpang wsErrMsg bersama) agar kode galat khas playbook
// terkumpul di satu tempat.
func playbooksErrMsg(code string) string {
	switch code {
	case "required":
		return "Nama playbook wajib diisi."
	case "trigger_scenario":
		return "Skenario pemicu harus salah satu: Health Drop, Low Adoption, Renewal Approaching, atau New Onboarding."
	case "recommended_owner":
		return "Pemilik rekomendasi harus salah satu: CSM, Support, atau Sales."
	case "failed":
		return "Gagal menyimpan playbook. Coba lagi."
	default:
		return ""
	}
}
