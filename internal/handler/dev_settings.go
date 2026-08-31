package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/appmode"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/settings"
	"go_starter/internal/ui/pages/dev"
)

// dev_settings.go — pengaturan seluruh platform (/dev/settings). Nilainya hidup
// di tabel platform_settings + cache in-process, jadi perubahan berlaku SEKETIKA
// tanpa restart — beda dari env MAX_WORKSPACES_PER_USER yang hanya jadi nilai
// awal saat user dibuat dan tak pernah menyentuh user yang sudah ada.

// DevSettings — GET /dev/settings.
func (h *Handler) DevSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Berapa user memegang hak khusus — konteks dampak sebelum operator menekan
	// Simpan. Fail-soft: gagal hitung → 0 (halaman tetap tampil & bisa dipakai).
	overrides, err := h.q(ctx).CountQuotaOverrides(ctx)
	if err != nil {
		h.Log.Error("settings: hitung override", "err", err)
	}
	// Nama workspace primer dipakai sebagai frasa konfirmasi kenaikan mode.
	// Fail-soft: gagal baca → nama kosong, dan handler POST tetap menolak karena
	// konfirmasinya tak akan cocok (arah aman untuk aksi permanen).
	primaryName := ""
	if prim, e := h.primaryTenant(ctx); e == nil {
		primaryName = prim.Name
	} else {
		h.Log.Error("settings: baca workspace primer", "err", e)
	}
	h.renderShell(w, r, "Pengaturan", devBrand(), "/dev/settings", devNav(r.Context()),
		dev.Settings(dev.SettingsView{
			SingleMode:    appmode.IsSingle(),
			PrimaryName:   primaryName,
			QuotaDefault:  settings.WorkspaceQuotaDefault(),
			QuotaMin:      settings.MinWorkspaceQuota,
			QuotaMax:      settings.MaxWorkspaceQuota,
			OverrideN:     int(overrides),
			RetentionDays: settings.AuditRetentionDays(),
			RetentionMin:  settings.MinAuditRetentionDays,
			RetentionMax:  settings.MaxAuditRetentionDays,
			Msg:           settingsMsg(r.URL.Query().Get("ok")),
			Err:           settingsErr(r.URL.Query().Get("err")),
		}))
}

// DevSettingsQuota — POST /dev/settings/quota. Ubah default global.
func (h *Handler) DevSettingsQuota(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := strconv.Atoi(r.FormValue("quota"))
	// Divalidasi di BACKEND, bukan cuma lewat atribut min/max di form: nilainya
	// user-controlled, dan 0 akan mengunci semua orang dari membuat workspace.
	if err != nil || !settings.ValidWorkspaceQuota(n) {
		http.Redirect(w, r, "/dev/settings?err=quota", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)
	value := strconv.Itoa(n)
	if err := h.q(ctx).UpsertSetting(ctx, db.UpsertSettingParams{
		Key: settings.KeyWorkspaceQuotaDefault, Value: value, UpdatedBy: &uid,
	}); err != nil {
		h.Log.Error("settings: simpan kuota", "err", err)
		http.Redirect(w, r, "/dev/settings?err=failed", http.StatusSeeOther)
		return
	}
	// DB dulu, cache kemudian. Terbalik = tulis yang gagal meninggalkan cache
	// berbohong sampai proses restart.
	settings.Set(settings.KeyWorkspaceQuotaDefault, value)
	h.auditPlatform(ctx, uid, "settings.workspace_quota", 0, map[string]string{"value": value})
	http.Redirect(w, r, "/dev/settings?ok=saved", http.StatusSeeOther)
}

// DevUserQuota — POST /dev/users/{id}/quota. Beri HAK KHUSUS: user ini memegang
// angkanya sendiri dan kebal perubahan default global.
func (h *Handler) DevUserQuota(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.FormValue("quota"))
	if err != nil || !settings.ValidWorkspaceQuota(n) {
		http.Redirect(w, r, "/dev/users?err=quota", http.StatusSeeOther)
		return
	}
	q := int32(n)
	if err := h.q(ctx).UpdateUserQuota(ctx, db.UpdateUserQuotaParams{
		ID: id, WorkspaceQuota: &q,
	}); err != nil {
		h.Log.Error("settings: set kuota user", "user_id", id, "err", err)
		http.Redirect(w, r, "/dev/users?err=failed", http.StatusSeeOther)
		return
	}
	h.audit(ctx, session.UserID(ctx), "user.quota.set", id, map[string]string{"value": strconv.Itoa(n)})
	http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
}

// DevUserQuotaReset — POST /dev/users/{id}/quota/reset. Cabut hak khusus: NULL
// berarti user kembali MENGIKUTI default global, termasuk perubahannya nanti.
func (h *Handler) DevUserQuotaReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if err := h.q(ctx).UpdateUserQuota(ctx, db.UpdateUserQuotaParams{
		ID: id, WorkspaceQuota: nil, // NULL = ikut global
	}); err != nil {
		h.Log.Error("settings: reset kuota user", "user_id", id, "err", err)
		http.Redirect(w, r, "/dev/users?err=failed", http.StatusSeeOther)
		return
	}
	h.audit(ctx, session.UserID(ctx), "user.quota.reset", id, nil)
	http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
}
