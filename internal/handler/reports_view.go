package handler

import (
	"context"

	"go_starter/internal/authz"
)

// reports_view.go — gerbang F2 (Casbin bisnis) Reports (Modul 8, tasks.md M8-1).
// Objek "crm:reports" sudah terdaftar di business_defaults.go (read utk manager/
// sales/csm/support + admin lewat glob crm:*) — di sini hanya dirujuk. Meniru
// dashboard_view.go (satu file = gerbang F2 per modul).

// canViewReports = gerbang READ preset report (Sales/Subscription). SATU gerbang
// untuk kedua preset — Reports bukan objek data granular per tabel (skema.md §8).
func canViewReports(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:reports", "read")
}
