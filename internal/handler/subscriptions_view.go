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
var subscriptionStatuses = []string{"Trial", "Active", "Suspended", "Expired", "Cancelled", "Churned"}

// canViewSubscriptions = gerbang READ daftar langganan. Sumber tunggal untuk menu &
// gate halaman SubscriptionsList/SubscriptionDetail. Admin (glob crm:*), manager,
// sales, csm punya read; support & "" tidak.
func canViewSubscriptions(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:subscriptions", "read")
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
// keluar saat tersembunyi. MRR TIDAK disamarkan (terlihat semua viewer).
func maskSubscriptionARR(formatted, businessRole string) string {
	if canSeeSubscriptionARR(businessRole) {
		return formatted
	}
	return flsHidden
}
