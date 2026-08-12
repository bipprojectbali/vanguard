package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// sales_activities.go — AKSI atas Sales Activity (4.4): create, update, ubah
// status, soft-delete. Dipisah dari sales_activities_page.go (baca): aksi tumbuh
// dengan aturan TULIS (F2 write), halaman dengan aturan LIHAT (F2 read + F3).
// Meniru sales_deals.go.
//
// Gerbang SEMUA aksi = canWriteSalesActivityPerm (crm:sales_activity write).
// Ownership F3 (owner_id) menjaga baris yang di-EDIT/HAPUS/UBAH-STATUS lewat
// loadOwnedActivity (404 di luar cakupan). RLS tetap mengurung workspace.
// Integritas TARGET (deal/account/contact dalam cakupan aktor) diverifikasi
// targetInScope SEBELUM insert — target_id BUKAN FK, jadi handler penjaganya.

// activityContextSales = nilai activity_context yang ditulis modul Sales (4.4).
// Named-const, bukan literal telanjang tersebar (aturan hardcode); memisah view
// Sales dari CS (6.5) & general (M7) di tabel activities yang sama.
const activityContextSales = "sales"

// requireActivityWrite = gerbang tulis bersama. false & menulis penolakan bila
// aktor tak berhak (izin F2 write; read-only workspace ditolak lebih awal).
func (h *Handler) requireActivityWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteSalesActivityPerm(r.Context()) {
		h.renderActivitiesForbidden(w, r)
		return false
	}
	return true
}

// ActivityCreate — POST /w/{workspace}/activities. Mencatat aktivitas Sales.
// kind & target ditetapkan di sini (immutable setelahnya). activity_context
// dipaksa 'sales'; owner = pembuat (dasar F3 ScopeOwn). Target diverifikasi
// dalam cakupan aktor sebelum insert (integritas polimorfik).
func (h *Handler) ActivityCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()

	kind := strings.TrimSpace(r.FormValue("kind"))
	if _, ok := validActivityKinds[kind]; !ok {
		wsRedirect(w, r, "/activities/new", "activity_kind")
		return
	}
	newPath := "/activities/new?kind=" + kind

	targetType, targetID, ok := parseActivityTarget(r.FormValue("target"))
	if !ok {
		wsRedirect(w, r, newPath, "activity_target")
		return
	}
	inScope, err := h.targetInScope(ctx, targetType, targetID)
	if err != nil {
		h.Log.Error("activities: target scope", "err", err)
		wsRedirect(w, r, newPath, "failed")
		return
	}
	if !inScope {
		// Target tak ada / di luar cakupan aktor → tolak sebagai target tak sah
		// (bukan 404: ini validasi form, bukan navigasi ke baris).
		wsRedirect(w, r, newPath, "activity_target")
		return
	}

	form, errCode := parseActivityForm(r.FormValue, kind)
	if errCode != "" {
		wsRedirect(w, r, newPath, errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	salesCtx := activityContextSales
	a, err := h.q(ctx).CreateActivity(ctx, db.CreateActivityParams{
		TenantID:        tenantID,
		Kind:            kind,
		Subject:         form.Subject,
		TargetType:      targetType,
		TargetID:        targetID,
		OwnerID:         &uid, // pembuat = pemilik awal (dasar F3 ScopeOwn)
		ActivityContext: &salesCtx,
		Status:          form.Status,
		Notes:           form.Notes,
		DueDate:         form.DueDate,
		Priority:        form.Priority,
		ContactID:       form.ContactID,
		Direction:       form.Direction,
		ActivityAt:      form.ActivityAt,
		DurationMin:     form.DurationMin,
		CallResult:      form.CallResult,
		Body:            form.Body,
		CreatedBy:       &uid,
	})
	if err != nil {
		h.Log.Error("activities: create", "err", err)
		wsRedirect(w, r, newPath, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.create", tenantID, map[string]string{
		"activity_id": strconv.FormatInt(a.ID, 10), "kind": kind,
	})
	wsRedirectOK(w, r, "/activities/"+strconv.FormatInt(a.ID, 10), "created")
}

// ActivityUpdate — POST /w/{workspace}/activities/{id}. Menyimpan sunting field
// (per kind baris — form dibentuk oleh a.Kind). kind, target, owner, & status
// TAK disentuh: kind/target immutable; owner tak diam-diam berpindah; status
// punya jalur khusus (ActivityStatus). reminder_at belum ber-form v1 → dipertahankan.
func (h *Handler) ActivityUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedActivity(w, r, id)
	if !ok {
		return
	}
	editPath := "/activities/" + strconv.FormatInt(id, 10) + "/edit"

	form, errCode := parseActivityForm(r.FormValue, a.Kind)
	if errCode != "" {
		wsRedirect(w, r, editPath, errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateActivity(ctx, db.UpdateActivityParams{
		Subject:     form.Subject,
		OwnerID:     a.OwnerID,    // pertahankan pemilik
		ReminderAt:  a.ReminderAt, // pertahankan (belum ber-form v1)
		Notes:       form.Notes,
		DueDate:     form.DueDate,
		Priority:    form.Priority,
		ContactID:   form.ContactID,
		Direction:   form.Direction,
		ActivityAt:  form.ActivityAt,
		DurationMin: form.DurationMin,
		CallResult:  form.CallResult,
		Body:        form.Body,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("activities: update", "err", err)
		wsRedirect(w, r, editPath, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.update", session.TenantID(ctx), map[string]string{
		"activity_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/activities/"+strconv.FormatInt(id, 10), "saved")
}

// ActivityStatus — POST /w/{workspace}/activities/{id}/status. Memindah status
// Task (aksi tersendiri, cermin DealStage). Enum divalidasi allowlist di sini +
// CHECK DB jaring terakhir. Hanya Task ber-status di UI v1.
func (h *Handler) ActivityStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedActivity(w, r, id); !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)

	status := strings.TrimSpace(r.FormValue("status"))
	if _, valid := validTaskStatuses[status]; !valid {
		wsRedirect(w, r, "/activities/"+idStr, "activity_status")
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpdateActivityStatus(ctx, db.UpdateActivityStatusParams{
		Status: &status, UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("activities: status", "err", err)
		wsRedirect(w, r, "/activities/"+idStr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.status", session.TenantID(ctx), map[string]string{
		"activity_id": idStr, "status": status,
	})
	wsRedirectOK(w, r, "/activities/"+idStr, "status")
}

// ActivityDelete — POST /w/{workspace}/activities/{id}/delete. Soft-delete
// (reversibel di DB via deleted_at; audit mencatat siapa & kapan).
func (h *Handler) ActivityDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedActivity(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteActivity(ctx, db.SoftDeleteActivityParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("activities: delete", "err", err)
		wsRedirect(w, r, "/activities/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.delete", session.TenantID(ctx), map[string]string{
		"activity_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/activities", "deleted")
}

// loadOwnedActivity memuat satu aktivitas & menegakkan F3 (owner_id): di luar
// cakupan aktor → 404 (kembaran ActivityDetail). (activity, true) bila boleh.
func (h *Handler) loadOwnedActivity(w http.ResponseWriter, r *http.Request, id int64) (db.Activity, bool) {
	ctx := r.Context()
	a, err := h.q(ctx).GetActivity(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Activity{}, false
		}
		h.Log.Error("activities: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Activity{}, false
	}
	filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.OwnerID) {
		http.NotFound(w, r)
		return db.Activity{}, false
	}
	return a, true
}

// targetInScope memverifikasi target polimorfik (deal/account/contact) ADA &
// dalam cakupan F3 aktor — target_id bukan FK, jadi integritas ditegakkan di
// sini sebelum insert. (false, nil) = tak ada / di luar cakupan (tolak form);
// (false, err) = galat query (500-worthy). Contact memakai cakupan AKUN induk
// (tak ada ContactsListFilter tersendiri — visibilitas kontak ikut akunnya).
func (h *Handler) targetInScope(ctx context.Context, targetType string, id int64) (bool, error) {
	uid := session.UserID(ctx)
	scope := session.BusinessDataScope(ctx)
	switch targetType {
	case "deal":
		d, err := h.q(ctx).GetDeal(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.DealsListFilterFor(scope).Allows(uid, d.DealOwner), nil
	case "account":
		a, err := h.q(ctx).GetAccount(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.AccountsListFilterFor(scope).Allows(uid, a.AccountOwner, a.AssignedCsm, a.BackupCsm), nil
	case "contact":
		c, err := h.q(ctx).GetContact(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		a, err := h.q(ctx).GetAccount(ctx, c.AccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.AccountsListFilterFor(scope).Allows(uid, a.AccountOwner, a.AssignedCsm, a.BackupCsm), nil
	default:
		return false, nil
	}
}
