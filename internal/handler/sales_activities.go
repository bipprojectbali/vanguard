package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_activities.go — AKSI CREATE Sales Activity (4.4). Update/status/delete di
// sales_activities_update.go; loader F3 & pemeriksa target di
// sales_activities_helpers.go. Dipisah karena aksi tumbuh dengan aturan TULIS (F2
// write), halaman (sales_activities_page.go) dengan aturan LIHAT (F2 read + F3).
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
		StartAt:         form.StartAt,
		EndAt:           form.EndAt,
		Location:        form.Location,
		MeetingType:     form.MeetingType,
		Channel:         form.Channel,
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
