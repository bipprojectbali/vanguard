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

// subscriptionStatuses = himpunan status legal langganan (cermin subs_status_chk,
// migrasi 00012). Dipakai HANYA untuk merakit dropdown filter di daftar — bukan
// validasi transisi (itu urusan slice mutasi berikutnya). Urutan = daur hidup
// alami (Trial → Active → … → Churned) agar filter terbaca logis.
var subscriptionStatuses = []string{"Trial", "Active", "PendingApproval", "Suspended", "Expired", "Cancelled", "Churned"}

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
	case "churned":
		return "Langganan ditandai churn."
	default:
		return ""
	}
}

// canSeeSubscriptionARR — ARR (annual recurring revenue) hanya utk pengambil
// keputusan komersial: admin + manager. Sales/CSM lihat MRR tapi ARR di-mask
// (spec M5-4 tasks.md). Sengaja BEDA dari kebijakan ARR deals (maskARR) yang
// mengizinkan sales/csm — langganan lebih sensitif nilai tahunannya.
func canSeeSubscriptionARR(businessRole string) bool {
	return businessRole == authz.BusinessRoleAdmin || businessRole == authz.BusinessRoleManager
}

// maskSubscriptionARR mengembalikan ARR siap-tampil (string SUDAH diformat) bila
// berhak, atau penanda tersembunyi (flsHidden) bila tidak. Nilai asli tak pernah
// keluar saat tersembunyi. MRR pakai kebijakan terpisah (maskARR, kebijakan umum
// kecuali Support) — diperbaiki audit FLS M9-1, sebelumnya sengaja tanpa masking.
func maskSubscriptionARR(formatted, businessRole string) string {
	if canSeeSubscriptionARR(businessRole) {
		return formatted
	}
	return flsHidden
}
