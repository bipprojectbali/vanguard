package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/session"
)

// sales_view.go — gerbang F2 (Casbin bisnis) modul Sales: Leads & Deals. Satu
// tempat objek Casbin "crm:leads"/"crm:deals" disebut, agar menu sidebar & gate
// halaman selalu menyebut objek/aksi yang SAMA (nol menu hantu — pintu yang
// tampil tak pernah ditolak 403 saat diketuk). Meniru accounts_view.go.
//
// TIGA sumbu tegak lurus tetap berlaku (sistem-dan-role §3): F2 di sini (boleh
// buka modul apa), F3 ownership (atas data siapa, ownership.go), F4 FLS (field
// mana, fls.go). RLS mengurung workspace di bawah semuanya (h.q ber-tenant).

// canViewLeads = gerbang READ modul Leads. Sumber tunggal untuk menu & gate
// halaman LeadsList/LeadDetail.
func canViewLeads(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:leads", "read")
}

// canWriteLeadsPerm = izin F2 mentah tulis leads (tanpa cek arsip). Dipakai
// gate aksi POST; write mencakup read di Casbin.
func canWriteLeadsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:leads", "write")
}

// canWriteLeads = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteLeads(ctx context.Context) bool {
	return canWriteLeadsPerm(ctx) && !IsReadOnly(ctx)
}

// canViewDeals = gerbang READ modul Deals.
func canViewDeals(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:deals", "read")
}

// canWriteDealsPerm = izin F2 mentah tulis deals.
func canWriteDealsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:deals", "write")
}

// canWriteDeals = tombol tulis deals (write & tak read-only).
func canWriteDeals(ctx context.Context) bool {
	return canWriteDealsPerm(ctx) && !IsReadOnly(ctx)
}

// canApproveDeals = aksi persetujuan diskon/deal (objek crm:deals aksi approve,
// kolom approve khusus Deals di business_defaults). Belum dipakai di slice ini
// (approval flow menyusul); diekspos agar gate & menu punya sumber tunggal saat
// fitur mendarat.
func canApproveDeals(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:deals", "approve")
}

// canViewSalesActivity = gerbang READ Sales Activity Log (4.4). Objek Casbin
// crm:sales_activity. Sumber tunggal untuk menu & gate ActivitiesList/ActivityDetail.
func canViewSalesActivity(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:sales_activity", "read")
}

// canViewAllActivities = gerbang halaman Activities lintas-context (M7, /activity-log).
// Platform roles (super_admin/staff) BYPASS sumbu bisnis: mereka operator sistem &
// butuh visibilitas semua aktivitas tanpa harus diberi business_role. Aktor CRM
// biasa ikut canViewSalesActivity (crm:sales_activity read — gate paling dekat
// sampai permission pass M9 membuat objek crm:activities tersendiri).
// TODO(activities): ganti ke crm:activities read saat permission pass M9.
func canViewAllActivities(ctx context.Context) bool {
	return canViewSalesActivity(ctx) || isPlatformRole(session.Role(ctx))
}

// canWriteSalesActivityPerm = izin F2 mentah tulis aktivitas (tanpa cek arsip).
// Dipakai gate aksi POST; write mencakup read di Casbin.
func canWriteSalesActivityPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:sales_activity", "write")
}

// canWriteSalesActivity = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteSalesActivity(ctx context.Context) bool {
	return canWriteSalesActivityPerm(ctx) && !IsReadOnly(ctx)
}
