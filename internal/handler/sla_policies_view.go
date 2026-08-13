package handler

import (
	"context"

	"go_starter/internal/authz"
)

// sla_policies_view.go — gerbang F2 (Casbin bisnis) modul SLA Policies (Modul 6
// Customer Success, slice A1). Satu tempat objek Casbin "crm:sla" disebut, agar
// menu sidebar & gate halaman selalu menyebut objek/aksi yang SAMA (nol menu
// hantu). Meniru plans_view.go.
//
// Katalog master TANPA F3 ownership: kebijakan SLA milik WORKSPACE, bukan
// per-desa — RLS (h.q ber-tenant) mengurungnya, tak ada kolom pemilik. Tulis =
// manager+admin (default Casbin: manager write, csm/support read, sales tak
// punya objek ini — lihat business_policy.csv).

// canViewSLAPolicies = gerbang READ katalog. Sumber tunggal untuk menu & gate
// halaman SLAPoliciesList/SLAPolicyEdit.
func canViewSLAPolicies(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:sla", "read")
}

// canWriteSLAPoliciesPerm = izin F2 mentah tulis kebijakan (tanpa cek arsip).
// Dipakai gate aksi POST; write mencakup read di Casbin.
func canWriteSLAPoliciesPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:sla", "write")
}

// canWriteSLAPolicies = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteSLAPolicies(ctx context.Context) bool {
	return canWriteSLAPoliciesPerm(ctx) && !IsReadOnly(ctx)
}

// slaPoliciesMsg memetakan ?ok= → pesan sukses SLA Policies (dipisah dari
// slaPoliciesErrMsg agar alert sukses & galat tak pernah tertukar variannya).
func slaPoliciesMsg(code string) string {
	switch code {
	case "created":
		return "Kebijakan SLA ditambahkan ke katalog."
	case "saved":
		return "Perubahan kebijakan SLA disimpan."
	case "retired":
		return "Kebijakan SLA dipensiunkan — tak lagi tampil di picker tiket baru."
	case "activated":
		return "Kebijakan SLA diaktifkan kembali."
	default:
		return ""
	}
}

// slaPoliciesErrMsg memetakan ?err= → pesan galat form SLA Policies. Lokal ke
// modul (tak menumpang wsErrMsg bersama) agar kode galat khas kebijakan
// terkumpul di satu tempat.
func slaPoliciesErrMsg(code string) string {
	switch code {
	case "required":
		return "Nama kebijakan wajib diisi."
	case "priority":
		return "Prioritas harus salah satu: Kritis, Tinggi, Sedang, atau Rendah."
	case "business_hours":
		return "Jam berlaku harus salah satu: Jam Kerja atau 24/7."
	case "number":
		return "Target respon/selesai harus berupa menit (angka bulat, tidak negatif)."
	case "failed":
		return "Gagal menyimpan kebijakan SLA. Coba lagi."
	default:
		return ""
	}
}
