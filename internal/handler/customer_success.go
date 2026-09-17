package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// customer_success.go — halaman BACA Customer Success (Modul 6 slice B1). SATU
// baris `customer_success` per desa tapi TIGA objek Casbin berbeda
// (crm:health/crm:journey/crm:adoption) — section yang aktor tak berhak baca
// disembunyikan di view (di sini), section yang tak berhak tulis di-mask saat
// SAVE (customer_success_save.go), bukan di gerbang GET. Gerbang F2 per-section
// + 403 di customer_success_gates.go.
//
// F3 diwarisi desa induk (loadOwnedAccount) — CS TANPA ownership sendiri, sama
// seperti kb_articles/playbooks/sla_policies TANPA F3, tapi di sini lewat account
// bukan lewat RLS/workspace polos (mirip contacts.go).

// csImplTrackerEntryEnabled (BL-167) = kill-switch tombol entry point
// "Implementation Tracker" di kartu Journey & Onboarding. false = DIPAUSE atas
// permintaan user (17 Sep) — bukan dihapus; route/handler/tabel/gate tetap hidup,
// hanya jalur masuk dari halaman ini yang disembunyikan sementara.
const csImplTrackerEntryEnabled = false

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
	v.Err = wsErrMsg(r.URL.Query().Get("err"))
	// Dua sumber ?ok= mendarat di halaman yang sama: AccountAssign ("assigned",
	// via accountsMsg) & customer_success_save.go ("created"/"saved", via
	// customerSuccessMsg — keduanya kebetulan PUNYA kode "created"/"saved" juga
	// tapi teksnya beda konteks; customerSuccessMsg dicek DULU krn halaman ini
	// milik CS, accountsMsg cuma fallback utk kode "assigned" yang khas dia).
	v.Msg = customerSuccessMsg(r.URL.Query().Get("ok"))
	if v.Msg == "" {
		v.Msg = accountsMsg(r.URL.Query().Get("ok"))
	}

	// BL-102: entry point ke daftar onboarding ter-filter desa ini. Gerbang F2
	// SAMA dgn halaman /impl-tasks & /trainings (objek crm:journey) — tak menambah
	// sumbu izin; F3 di daftar tujuan menyaring baris. BL-146: tombol ini adalah
	// aksi kerja onboarding/adopsi — hanya relevan untuk PELANGGAN aktif (hasLiveSub),
	// sama seperti CanWrite; desa non-pelanggan (prospek/churned) tak melihatnya
	// walau berhak baca crm:journey.
	//
	// BL-167 (paused, 17 Sep — keputusan user, screenshot desa "Parang Loe"):
	// tombol "Implementation Tracker" DISEMBUNYIKAN sementara — LEVEL A, sama pola
	// BL-78 (hide-only, bukan hapus): route/handler/tabel `cs_impl_tasks`/gate
	// `canViewImplTasks`/`crm:journey` TETAP hidup, URL /impl-tasks?account={id}
	// masih bisa diakses langsung. "Training Schedule" TAK terdampak — tetap
	// tampil (di luar cakupan permintaan). Untuk mengaktifkan lagi: set
	// csImplTrackerEntryEnabled = true.
	idStr := strconv.FormatInt(accountID, 10)
	if csImplTrackerEntryEnabled && canViewImplTasks(ctx) && hasLiveSub {
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
