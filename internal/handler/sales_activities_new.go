package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_new.go — form GET Sales Activity (buat/sunting) + pembangun opsi
// picker (target polimorfik & kontak). Dipisah dari halaman baca (list/detail) agar
// tiap file di bawah ambang file-health. Aksi POST di sales_activities.go.

// activityTargetPickerLimit membatasi opsi per-tipe di dropdown target (guardrail:
// picker tak boleh memuat seluruh tabel). Named-const, bukan angka telanjang.
const activityTargetPickerLimit = 200

// ActivityNew — GET /w/{workspace}/activities/new?kind=task|call|note. Form buat
// kosong untuk satu kind. kind tak sah / kosong → default "task" (form tetap
// terbuka, bukan 404 — tautan nav mengarah dengan kind valid).
func (h *Handler) ActivityNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if _, ok := validActivityKinds[kind]; !ok {
		kind = "task"
	}

	targets, err := h.activityTargetOptions(ctx)
	if err != nil {
		h.Log.Error("activities: target options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	contacts, err := h.activityContactOptions(ctx, kind)
	if err != nil {
		h.Log.Error("activities: contact options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Catat Aktivitas", "/activities",
		panel.ActivityForm(panel.ActivityFormView{
			Base:        base,
			Action:      base + "/activities",
			IsEdit:      false,
			Err:         wsErrMsg(r.URL.Query().Get("err")),
			Kind:        kind,
			Targets:     targets,
			Contacts:    contacts,
			Priorities:  activityPriorityOptions,
			Statuses:    taskStatusOptions,
			Directions:  activityDirectionOptions,
			CallResults: callResultOptions,
		}))
}

// ActivityEdit — GET /w/{workspace}/activities/{id}/edit. Form terisi. kind &
// target immutable (target ditampilkan read-only). Di luar cakupan F3 → 404.
func (h *Handler) ActivityEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedActivity(w, r, id)
	if !ok {
		return
	}

	contacts, err := h.activityContactOptions(ctx, a.Kind)
	if err != nil {
		h.Log.Error("activities: contact options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(a.ID, 10)
	h.renderWorkspaceShell(w, r, "Sunting Aktivitas", "/activities",
		panel.ActivityForm(panel.ActivityFormView{
			Base:        base,
			Action:      base + "/activities/" + idStr,
			IsEdit:      true,
			Err:         wsErrMsg(r.URL.Query().Get("err")),
			Kind:        a.Kind,
			TargetValue: a.TargetType + ":" + strconv.FormatInt(a.TargetID, 10),
			TargetLabel: h.targetLabel(ctx, a.TargetType, a.TargetID),
			Subject:     a.Subject,
			DueDate:     dateStr(a.DueDate),
			Priority:    deref(a.Priority),
			Status:      deref(a.Status),
			ContactID:   int32ContactStr(a.ContactID),
			Contacts:    contacts,
			Direction:   deref(a.Direction),
			ActivityAt:  dateTimeStr(a.ActivityAt),
			Duration:    int32PtrStr(a.DurationMin),
			CallResult:  deref(a.CallResult),
			Body:        deref(a.Body),
			Notes:       deref(a.Notes),
			Priorities:  activityPriorityOptions,
			Statuses:    taskStatusOptions,
			Directions:  activityDirectionOptions,
			CallResults: callResultOptions,
		}))
}

// activityTargetOptions memuat target dalam cakupan aktor (F3): deal + account +
// contact. Nilai opsi = "type:id" (dikonsumsi parseActivityTarget); label diberi
// awalan tipe agar tercampur jelas. Tiap tipe satu query berbatas (bukan N+1,
// bukan seluruh tabel).
func (h *Handler) activityTargetOptions(ctx context.Context) ([]panel.ActivityTargetOption, error) {
	uid := session.UserID(ctx)
	scope := session.BusinessDataScope(ctx)
	at, cid := firstPageCursor()
	opts := make([]panel.ActivityTargetOption, 0, activityTargetPickerLimit)

	df := db.DealsListFilterFor(scope)
	deals, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: df.ScopeAll, IsOwn: df.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, d := range deals {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "deal:" + strconv.FormatInt(d.ID, 10),
			Label: "Deal · " + d.DealName,
		})
	}

	af := db.AccountsListFilterFor(scope)
	accounts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: af.ScopeAll, IsSales: af.IsOwn, IsCsm: af.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "account:" + strconv.FormatInt(a.ID, 10),
			Label: "Desa · " + a.VillageName,
		})
	}

	contacts, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: af.ScopeAll, IsSales: af.IsOwn, IsCsm: af.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, c := range contacts {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "contact:" + strconv.FormatInt(c.ID, 10),
			Label: "Kontak · " + contactFullName(c),
		})
	}
	return opts, nil
}

// activityContactOptions memuat kontak dalam cakupan aktor (F3 desa induk) untuk
// dropdown contact_id pada Call. Kind selain "call" → nil (field tak dirender).
func (h *Handler) activityContactOptions(ctx context.Context, kind string) ([]panel.AccountMemberOption, error) {
	if kind != "call" {
		return nil, nil
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	at, cid := firstPageCursor()
	rows, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll, IsSales: filter.IsOwn, IsCsm: filter.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	opts := make([]panel.AccountMemberOption, 0, len(rows))
	for _, c := range rows {
		opts = append(opts, panel.AccountMemberOption{ID: c.ID, Label: contactFullName(c)})
	}
	return opts, nil
}

// int32ContactStr memformat *int64 contact_id untuk prefill dropdown: nil → "".
func int32ContactStr(n *int64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(*n, 10)
}
