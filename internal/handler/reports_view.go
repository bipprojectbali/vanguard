package handler

import (
	"context"

	"go_starter/internal/authz"
)

// reports_view.go — gerbang F2 (Casbin bisnis) Reports (Modul 8, tasks.md M8-1).
// BL-169: dulu SATU gerbang (crm:reports) untuk 4 halaman preset report;
// dipecah jadi 4 objek Casbin (business_defaults.go) sesuai 4 halaman nyata
// di sidebar, agar admin bisa memberi akses per-domain (mis. Sales lihat
// Sales Report saja, tanpa Support Report). Meniru dashboard_view.go (satu
// file = gerbang F2 per modul).

// canViewSalesReports = gerbang READ halaman Sales Reports.
func canViewSalesReports(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:reports_sales", "read")
}

// canViewCSReports = gerbang READ halaman Customer Success Reports.
func canViewCSReports(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:reports_cs", "read")
}

// canViewSupportReports = gerbang READ halaman Support Reports.
func canViewSupportReports(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:reports_support", "read")
}

// canViewSubscriptionReports = gerbang READ halaman Subscription Reports.
func canViewSubscriptionReports(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:reports_subscriptions", "read")
}
