package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// customer_success.go — gerbang F2 per-section (Health/Journey+Onboarding/
// Adoption, Modul 6 slice B1) + halaman BACA. SATU baris `customer_success`
// per desa tapi TIGA objek Casbin berbeda (crm:health/crm:journey/crm:adoption)
// — section yang aktor tak berhak baca disembunyikan di view (di sini), section
// yang tak berhak tulis di-mask saat SAVE (customer_success_save.go), bukan di
// gerbang GET.
//
// F3 diwarisi desa induk (loadOwnedAccount) — CS TANPA ownership sendiri, sama
// seperti kb_articles/playbooks/sla_policies TANPA F3, tapi di sini lewat account
// bukan lewat RLS/workspace polos (mirip contacts.go).

func canReadCSHealth(ctx context.Context) bool  { return authz.CanBusiness(ctx, "crm:health", "read") }
func canWriteCSHealth(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:health", "write") }

func canReadCSJourney(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:journey", "read") }
func canWriteCSJourney(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:journey", "write")
}

func canReadCSAdoption(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "read")
}
func canWriteCSAdoption(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "write")
}

// canReadCS = boleh membuka halaman DETAIL bila berhak membaca MINIMAL satu
// section (mis. Support hanya Health) — section yang tak berhak disembunyikan
// di view, bukan seluruh halaman ditolak.
func canReadCS(ctx context.Context) bool {
	return canReadCSHealth(ctx) || canReadCSJourney(ctx) || canReadCSAdoption(ctx)
}

// canWriteCS = boleh membuka form SUNTING bila berhak menulis MINIMAL satu
// section — masking per-section terjadi saat SAVE, bukan saat gerbang GET ini.
func canWriteCS(ctx context.Context) bool {
	return canWriteCSHealth(ctx) || canWriteCSJourney(ctx) || canWriteCSAdoption(ctx)
}

// renderCSForbidden — 403 + penjelasan; mirror renderAccountsForbidden.
func (h *Handler) renderCSForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Customer Success", "/accounts", panel.CustomerSuccessForbidden())
}

// CustomerSuccessDetail — GET /w/{workspace}/accounts/{id}/customer-success.
// F3 via loadOwnedAccount; F2 minimal-satu-section via canReadCS. Baris CS
// absen (pgx.ErrNoRows) → empty-state, BUKAN 404: desanya tetap ada, cuma
// belum pernah diisi Customer Success-nya.
func (h *Handler) CustomerSuccessDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canReadCS(ctx) {
		h.renderCSForbidden(w, r)
		return
	}
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return
	}

	cs, err := h.q(ctx).GetCustomerSuccessByAccountID(ctx, accountID)
	exists := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("customer_success: get", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		exists = false
		cs = db.CustomerSuccess{AccountID: accountID}
	}

	// BL-114: apakah desa PELANGGAN aktif (≥1 langganan hidup) — menentukan apakah
	// penyuntingan Customer Success dibuka (CanWrite) atau dikunci (prospek/churned).
	hasLiveSub, err := h.q(ctx).AccountHasLiveSubscription(ctx, accountID)
	if err != nil {
		h.Log.Error("customer_success: live subscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	v := customerSuccessDetailView(ctx, base, account, cs, exists, hasLiveSub)

	// BL-102: entry point ke daftar onboarding ter-filter desa ini. Gerbang F2
	// SAMA dgn halaman /impl-tasks & /trainings (objek crm:journey) — tak menambah
	// sumbu izin; F3 di daftar tujuan menyaring baris. BL-146: tombol ini adalah
	// aksi kerja onboarding/adopsi — hanya relevan untuk PELANGGAN aktif (hasLiveSub),
	// sama seperti CanWrite; desa non-pelanggan (prospek/churned) tak melihatnya
	// walau berhak baca crm:journey.
	idStr := strconv.FormatInt(accountID, 10)
	if canViewImplTasks(ctx) && hasLiveSub {
		v.CanViewImplTasks = true
		v.ImplTasksHref = base + "/impl-tasks?account=" + idStr
	}
	if canViewTrainings(ctx) && hasLiveSub {
		v.CanViewTrainings = true
		v.TrainingsHref = base + "/trainings?account=" + idStr
	}

	// BL-108: kartu Penugasan CS dipindah ke halaman ini dari form edit desa.
	// Gerbang SAMA dengan aksi POST /assign (crm:accounts write & tak read-only)
	// — bukan sumbu izin baru; keempat role ber-tulis-account juga bisa membuka
	// halaman ini (punya ≥1 section CS read), jadi tak ada yang kehilangan akses.
	// Kandidat CSM dimuat DI SINI (assignableMembers = query DB) bukan di mapper.
	// BL-146: penugasan CS juga aksi tulis khusus pelanggan aktif — kunci sama dgn
	// CanWrite (hasLiveSub) di samping gerbang F2/read-only yang sudah ada.
	if canWriteAccountsPerm(ctx) && !IsReadOnly(ctx) && hasLiveSub {
		members, err := h.assignableMembers(ctx)
		if err != nil {
			h.Log.Error("customer_success: assignable members", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		v.CanAssign = true
		v.AssignAction = base + "/accounts/" + strconv.FormatInt(accountID, 10) + "/assign"
		v.Members = members
		v.AssignedCSM = int64PtrStr(account.AssignedCsm)
		v.BackupCSM = int64PtrStr(account.BackupCsm)
	}

	h.renderWorkspaceShell(w, r, account.VillageName+" · Customer Success", "/accounts",
		panel.CustomerSuccessDetail(v))
}
