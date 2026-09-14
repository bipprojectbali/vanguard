package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_activities_detail_test.go — currentPath sidebar di halaman detail satu
// Activity (BL-161). Sebelum perbaikan, ActivityDetail SELALU render dgn
// currentPath="/activities" hardcode; baris feed lintas-context /activity-log
// menautkan ke URL detail yang SAMA, jadi sidebar tiba-tiba pindah menyala
// "Sales Activities" walau user datang dari "Activities". Perbaikan via penanda
// "?from=log" (all_activities.go) dibaca activityCurrentPathFromQuery (helper).

// TestActivityDetail_SidebarActive_DefaultStaysSalesActivities: klik dari daftar
// Sales Activities sendiri (tanpa "?from=log") → regresi, perilaku lama tetap
// benar: sidebar tetap menyala "Sales Activities".
func TestActivityDetail_SidebarActive_DefaultStaysSalesActivities(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa BL161", &uid, nil, nil)
	a := env.seedSalesActivity(t, "note", "account", acc.ID, "Catatan BL161", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities/"+itoa(a.ID), nil, itoa(a.ID))
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityDetail status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activities") {
		t.Errorf("tanpa from=log, sidebar harus tetap menyala \"Sales Activities\" (/activities):\n%s", body)
	}
	if navItemActive(body, "/w/test/activity-log") {
		t.Errorf("tanpa from=log, \"Activities\" (/activity-log) tak boleh menyala:\n%s", body)
	}
}

// TestActivityDetail_SidebarActive_FromLogHighlightsActivities: klik baris dari
// feed /activity-log ("?from=log") → sidebar menyala "Activities", BUKAN
// "Sales Activities" (bug asli BL-161 — sumber: user 12 Sep).
func TestActivityDetail_SidebarActive_FromLogHighlightsActivities(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa BL161b", &uid, nil, nil)
	a := env.seedSalesActivity(t, "note", "account", acc.ID, "Catatan BL161b", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities/"+itoa(a.ID)+"?from=log", nil, itoa(a.ID))
	// admin: punya baik crm:sales_activity maupun crm:activities (skenario asli
	// user — kedua menu sama-sama enabled, hanya salah satu yang boleh menyala).
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ActivityDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityDetail status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activity-log") {
		t.Errorf("dgn from=log, sidebar harus menyala \"Activities\" (/activity-log):\n%s", body)
	}
	if navItemActive(body, "/w/test/activities") {
		t.Errorf("dgn from=log, \"Sales Activities\" (/activities) tak boleh menyala:\n%s", body)
	}
}

// TestActivityDetail_Forbidden_FromLogHighlightsActivities: peran tanpa
// crm:sales_activity (mis. support, hanya crm:activities) yang klik baris dari
// /activity-log → 403 ("Sales Activities" memang di luar cakupannya), TAPI
// sidebar halaman forbidden itu SAMA sekali tak boleh salah pindah — tetap
// menyala "Activities", tempat asal klik (renderActivitiesForbidden dipakai
// jalur yang sama).
func TestActivityDetail_Forbidden_FromLogHighlightsActivities(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa BL161c", &uid, nil, nil)
	a := env.seedSalesActivity(t, "note", "account", acc.ID, "Catatan BL161c", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities/"+itoa(a.ID)+"?from=log", nil, itoa(a.ID))
	rec := env.runAccount(uid, "member", "support", req, env.h.ActivityDetail)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ActivityDetail (support, tanpa crm:sales_activity) status %d, want 403\n%s",
			rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !navItemActive(body, "/w/test/activity-log") {
		t.Errorf("halaman forbidden dari from=log harus tetap menyala \"Activities\":\n%s", body)
	}
}

// navItemActive memeriksa apakah item nav ber-href tertentu dirender dgn
// aria-current="page" (navLinkWith, internal/ui/shellnav.go) — pendekatan
// string sederhana cukup krn href unik per menu & atribut selalu berdekatan
// dalam satu tag <a>.
func navItemActive(body, href string) bool {
	idx := strings.Index(body, `href="`+href+`"`)
	if idx == -1 {
		return false
	}
	// Cari batas tag <a ...> terdekat di sekitar href utk membatasi pencarian
	// aria-current ke TAG YANG SAMA (bukan tag <a> lain di halaman).
	tagStart := strings.LastIndex(body[:idx], "<a ")
	tagEnd := strings.Index(body[idx:], ">")
	if tagStart == -1 || tagEnd == -1 {
		return false
	}
	tag := body[tagStart : idx+tagEnd]
	return strings.Contains(tag, `aria-current="page"`)
}
