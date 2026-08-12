package handler

import (
	"context"

	"go_starter/internal/authz"
)

// plans_view.go — gerbang F2 (Casbin bisnis) modul Plans & Pricing (Modul 5).
// Satu tempat objek Casbin "crm:plans" disebut, agar menu sidebar & gate halaman
// selalu menyebut objek/aksi yang SAMA (nol menu hantu). Meniru sales_view.go.
//
// Katalog master TANPA F3 ownership: plan milik WORKSPACE, bukan per-desa — RLS
// (h.q ber-tenant) mengurungnya, tak ada kolom pemilik. Tulis = admin saja
// (default hanya crm:* glob admin yang punya crm:plans write; peran lain read).

// canViewPlans = gerbang READ katalog. Sumber tunggal untuk menu & gate halaman
// PlansList/PlanEdit.
func canViewPlans(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:plans", "read")
}

// canWritePlansPerm = izin F2 mentah tulis plan (tanpa cek arsip). Dipakai gate
// aksi POST; write mencakup read di Casbin.
func canWritePlansPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:plans", "write")
}

// canWritePlans = tombol tulis di view: izin F2 write DAN workspace tak read-only
// (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWritePlans(ctx context.Context) bool {
	return canWritePlansPerm(ctx) && !IsReadOnly(ctx)
}

// plansMsg memetakan ?ok= → pesan sukses Plans (dipisah dari plansErrMsg agar
// alert sukses & galat tak pernah tertukar variannya).
func plansMsg(code string) string {
	switch code {
	case "created":
		return "Plan ditambahkan ke katalog."
	case "saved":
		return "Perubahan plan disimpan."
	case "retired":
		return "Plan dipensiunkan — tak lagi tampil di picker quote baru."
	case "activated":
		return "Plan diaktifkan kembali."
	default:
		return ""
	}
}

// plansErrMsg memetakan ?err= → pesan galat form Plans. Lokal ke modul (tak
// menumpang wsErrMsg bersama) agar kode galat khas plan terkumpul di satu tempat.
func plansErrMsg(code string) string {
	switch code {
	case "required":
		return "Nama dan kode plan wajib diisi."
	case "category":
		return "Kategori harus salah satu: Core, Add-on, atau Module."
	case "billing":
		return "Siklus tagih harus Monthly atau Annual."
	case "base_price":
		return "Harga dasar harus berupa angka yang sah."
	case "setup_fee":
		return "Biaya setup harus berupa angka yang sah."
	case "plan_code_dup":
		return "Kode plan sudah dipakai plan lain di workspace ini."
	case "failed":
		return "Gagal menyimpan plan. Coba lagi."
	default:
		return ""
	}
}
