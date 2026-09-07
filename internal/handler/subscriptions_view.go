package handler

import (
	"context"

	"go_starter/internal/authz"
)

// subscriptions_view.go — gerbang F2 (Casbin bisnis) + kebijakan masking ARR (F4)
// modul Active Subscriptions (Modul 5). Satu tempat objek Casbin "crm:subscriptions"
// disebut, agar menu sidebar & gate halaman selalu menyebut objek/aksi yang SAMA
// (nol menu hantu). Meniru plans_view.go / sales_view.go.
//
// F3 ownership (subscription_owner) ditegakkan di layer query (SubscriptionsListFilter,
// ownership.go) untuk daftar dan per-baris untuk detail — bukan di sini.

// subscriptionStatuses = himpunan status langganan yang DITAWARKAN di dropdown
// filter daftar. Dipakai HANYA untuk merakit dropdown filter — bukan validasi
// transisi (itu urusan slice mutasi). Urutan = daur hidup alami (Trial → Active
// → … → Churned) agar filter terbaca logis.
//
// SENGAJA tanpa "Suspended" (BL-22): status itu adalah nilai CADANGAN yang belum
// di-wire — tak ada aksi app yang menghasilkannya, jadi tab filternya selalu
// kosong dan menyesatkan. Enum DB (subs_status_chk) TETAP menerima "Suspended".
// Untuk tunggakan pakai payment_status=Overdue; untuk penghentian pakai
// churn_type=Involuntary. Bila kelak ada aksi Tangguhkan/Unsuspend, kembalikan
// "Suspended" ke daftar ini.
var subscriptionStatuses = []string{"Trial", "Active", "PendingApproval", "Expired", "Cancelled", "Churned"}

// canViewSubscriptions = gerbang READ daftar langganan. Sumber tunggal untuk menu &
// gate halaman SubscriptionsList/SubscriptionDetail. Admin (glob crm:*), manager,
// sales, csm punya read; support & "" tidak.
func canViewSubscriptions(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:subscriptions", "read")
}

// canRenewSubscriptions = gerbang WRITE renewal (perpanjang langganan). Admin
// (glob) & manager punya; sales/csm hanya read (lihat matriks business_defaults).
func canRenewSubscriptions(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewals", "write")
}

// canChurnSubscriptions = gerbang WRITE churn (tandai langganan berhenti). Admin,
// manager, csm punya; sales hanya read.
func canChurnSubscriptions(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:churn", "write")
}

// canActivateSubscriptions = gerbang aktivasi langganan Trial → Active (BL-73).
// Aktivasi sekelas renewal ("bikin langganan hidup") → pakai kapabilitas
// crm:renewals write (admin/manager), konsisten dgn canRenewSubscriptions. Sengaja
// gate TERPISAH bernama agar bila kelak aktivasi perlu izin berbeda, cukup ubah di
// sini tanpa menyentuh renewal.
func canActivateSubscriptions(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewals", "write")
}

// canViewRenewals = gerbang READ ringkasan renewal (rate, jatuh tempo) untuk
// Beranda (BL-59b). Berbeda dari canRenewSubscriptions (WRITE) — melihat metrik
// renewal tak sama dengan berhak memperpanjang. Default: admin (glob), manager,
// sales, csm punya crm:renewals read (business_defaults); support tidak.
func canViewRenewals(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewals", "read")
}

// canViewChurn = gerbang READ metrik churn (rate, MRR hilang) untuk Beranda
// (BL-59b). Berbeda dari canChurnSubscriptions (WRITE tandai churn). Default:
// admin, manager, sales, csm punya crm:churn read; support tidak.
func canViewChurn(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:churn", "read")
}

// canApproveRenewal = gerbang APPROVE renewal Upsell yang menunggu. HANYA admin &
// manager (aksi approve, bukan write); csm punya write renewal_mgmt tapi TAK approve.
func canApproveRenewal(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewal_mgmt", "approve")
}

// subscriptionsMsg memetakan kode sukses PRG (`?ok=CODE`) → kalimat konfirmasi di
// halaman detail langganan. Pasangan positif wsErrMsg; kode tak dikenal → "".
func subscriptionsMsg(code string) string {
	switch code {
	case "renewed":
		return "Langganan diperpanjang — periode baru aktif."
	case "renew_pending":
		return "Renewal upsell dibuat — menunggu persetujuan Manager."
	case "renew_approved":
		return "Renewal disetujui — langganan periode baru aktif."
	case "renew_rejected":
		return "Renewal ditolak — langganan lama tetap berjalan."
	case "activated":
		return "Langganan diaktifkan — status kini Active."
	case "churned":
		return "Langganan ditandai churn."
	default:
		return ""
	}
}

// canSeeSubscriptionARR — visibilitas ARR (annual recurring revenue) kini
// KAPABILITAS ter-matriks (crm:subscriptions/arr), bukan cek nama role hardcode
// (BL-58). Sebelumnya `role == admin || == manager`, yang mengunci ARR ke nama
// peran sistem sehingga peran KUSTOM (mis. "Direktur"/"Finance") tak pernah bisa
// melihat ARR walau diberi data_scope=all — kontra desain role-aware F2/F3.
// Default kapabilitas ini diberikan ke Administrator (glob crm:*) + Manager
// (business_defaults.go); operator bisa memberikannya ke peran lain lewat editor
// peran. Sengaja BEDA dari MRR (maskARR, umum kecuali Support) — ARR langganan
// lebih sensitif nilai tahunannya (spec M5-4). Dihitung di batas handler (punya
// ctx ber-sesi), hasilnya (bool) dialirkan ke view-mapper murni-data.
func canSeeSubscriptionARR(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:subscriptions", "arr")
}

// maskSubscriptionARR mengembalikan ARR siap-tampil (string SUDAH diformat) bila
// canSee, atau penanda tersembunyi (flsHidden) bila tidak. Nilai asli tak pernah
// keluar saat tersembunyi. Menerima bool (bukan ctx) agar view-mapper tetap
// murni-data & unit-testable tanpa enforcer — pemanggil hitung sekali via
// canSeeSubscriptionARR(ctx). MRR pakai kebijakan terpisah (maskARR).
func maskSubscriptionARR(formatted string, canSee bool) string {
	if canSee {
		return formatted
	}
	return flsHidden
}
