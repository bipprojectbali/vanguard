package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// customer_success_gates.go — gerbang F2 per-section (Health/Journey+Onboarding)
// + 403, dipisah dari customer_success.go (file health, ambang Route/Handler
// 150). SATU baris `customer_success` per desa tapi DUA objek Casbin berbeda;
// lihat customer_success.go untuk rasional lengkap.
//
// BL-169: section "Adoption Score"/"Feature Adoption Rate" pindah numpang ke
// crm:journey (canReadCSAdoption/canWriteCSAdoption DIHAPUS) — objek
// crm:adoption kini gerbang khusus Training Schedule (cs_trainings_view.go),
// bukan lagi section di halaman CS 360.

func canReadCSHealth(ctx context.Context) bool  { return authz.CanBusiness(ctx, "crm:health", "read") }
func canWriteCSHealth(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:health", "write") }

func canReadCSJourney(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:journey", "read") }
func canWriteCSJourney(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:journey", "write")
}

// canReadCS = boleh membuka halaman DETAIL bila berhak membaca MINIMAL satu
// section (mis. Support hanya Health) — section yang tak berhak disembunyikan
// di view, bukan seluruh halaman ditolak.
func canReadCS(ctx context.Context) bool {
	return canReadCSHealth(ctx) || canReadCSJourney(ctx)
}

// canWriteCS = boleh membuka form SUNTING bila berhak menulis MINIMAL satu
// section — masking per-section terjadi saat SAVE, bukan saat gerbang GET ini.
func canWriteCS(ctx context.Context) bool {
	return canWriteCSHealth(ctx) || canWriteCSJourney(ctx)
}

// renderCSForbidden — 403 + penjelasan; mirror renderAccountsForbidden.
func (h *Handler) renderCSForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Customer Success", "/accounts", panel.CustomerSuccessForbidden())
}
