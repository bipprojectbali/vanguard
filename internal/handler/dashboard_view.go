package handler

import (
	"context"

	"go_starter/internal/authz"
)

// dashboard_view.go — gerbang F2 (Casbin bisnis) Beranda ruang kerja (Modul 1,
// tasks.md M1). Objek "crm:dashboard" sudah terdaftar di business_defaults.go
// (read untuk kelima role bisnis + admin lewat glob crm:*) — di sini hanya
// dirujuk, tak didefinisikan ulang, agar satu-satunya sumber tetap Casbin.
// Meniur sales_view.go/subscriptions_view.go (satu file = gerbang F2 per modul).

// canViewDashboard = gerbang READ kartu KPI Beranda. WorkspaceHome jatuh ke
// Placeholder bila false (anggota tanpa business_role, mis. baru diundang) —
// bukan 403, karena Beranda tetap harus terbuka untuk semua anggota workspace.
func canViewDashboard(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:dashboard", "read")
}
