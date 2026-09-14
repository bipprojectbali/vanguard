package handler

import (
	"net/http"
	"testing"
)

// sales_activities_new_test.go — currentPath sidebar di form GET "Tambah
// Aktivitas" (BL-161 lanjutan). Sebelum perbaikan, ActivityNew SELALU render
// dgn currentPath="/activities" hardcode; tombol "Tambah Aktivitas" di
// AllActivitiesList (/activity-log) menautkan ke form yang SAMA, jadi sidebar
// tiba-tiba pindah menyala "Sales Activities" walau user datang dari menu
// "Activities" — sumber: user 14 Sep ("saat tambah aktiviti dari menu
// aktiviti, menu yg active ada sales activiti bukan menu aktiviti"). Perbaikan
// via penanda "?from=log" (sama pola BL-161, all_activities.go →
// activityNewButton) dibaca activityCurrentPathFromQuery (helper bersama).

// TestActivityNew_SidebarActive_DefaultStaysSalesActivities: buka form dari
// tombol "Tambah Aktivitas" di halaman Sales Activities sendiri (tanpa
// "?from=log") → regresi, perilaku lama tetap benar: sidebar tetap menyala
// "Sales Activities".
func TestActivityNew_SidebarActive_DefaultStaysSalesActivities(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/w/test/activities/new", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityNew status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activities") {
		t.Errorf("tanpa from=log, sidebar harus tetap menyala \"Sales Activities\" (/activities):\n%s", body)
	}
	if navItemActive(body, "/w/test/activity-log") {
		t.Errorf("tanpa from=log, \"Activities\" (/activity-log) tak boleh menyala:\n%s", body)
	}
}

// TestActivityNew_SidebarActive_FromLogHighlightsActivities: buka form dari
// tombol "Tambah Aktivitas" di AllActivitiesList ("?from=log") → sidebar
// menyala "Activities", BUKAN "Sales Activities" (bug asli lanjutan BL-161).
func TestActivityNew_SidebarActive_FromLogHighlightsActivities(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/w/test/activities/new?from=log", nil, "")
	// admin: punya baik crm:sales_activity maupun crm:activities (skenario
	// asli user — kedua menu sama-sama enabled, hanya salah satu boleh menyala).
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ActivityNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityNew status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activity-log") {
		t.Errorf("dgn from=log, sidebar harus menyala \"Activities\" (/activity-log):\n%s", body)
	}
	if navItemActive(body, "/w/test/activities") {
		t.Errorf("dgn from=log, \"Sales Activities\" (/activities) tak boleh menyala:\n%s", body)
	}
}

// TestActivityNew_Forbidden_FromLogHighlightsActivities: peran tanpa
// crm:sales_activity (mis. support, hanya crm:activities) yang membuka form
// dari tombol di /activity-log → 403 ("Sales Activities" memang di luar
// cakupannya), TAPI sidebar halaman forbidden itu tetap menyala "Activities",
// tempat asal klik (renderActivitiesForbidden dipakai jalur yang sama, lihat
// sales_activities_detail_test.go).
func TestActivityNew_Forbidden_FromLogHighlightsActivities(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/w/test/activities/new?from=log", nil, "")
	rec := env.runAccount(uid, "member", "support", req, env.h.ActivityNew)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ActivityNew (support, tanpa crm:sales_activity) status %d, want 403\n%s",
			rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activity-log") {
		t.Errorf("halaman forbidden dari from=log harus tetap menyala \"Activities\":\n%s", body)
	}
}
