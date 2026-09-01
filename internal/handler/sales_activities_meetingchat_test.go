package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_activities_meetingchat_test.go — dua kind baru di form tunggal
// (BL-19 lanjutan): "meeting" (Pertemuan) & "chat". Yang dijaga:
//
//   - Create meeting: meeting_type dari form tersimpan (kolom meeting_type).
//   - Create chat: channel dari form tersimpan (kolom channel).
//   - meeting_type / channel tak sah → ?err spesifik (cermin CHECK 00031),
//     tak tersimpan diam-diam.
//   - Form buat merender grup "Detail Pertemuan" & "Detail Chat" + toggle
//     Datastar keduanya, dan dropdown Jenis menawarkan 5 kind.
//
// Koneksi test = superuser (bypass RLS) → uji logika handler; readback lewat
// env.h.Pool.

// activityTextColBySubject membaca satu kolom teks nullable via subjek (superuser,
// bypass RLS) — untuk membuktikan nilai per-kind yang benar-benar tersimpan.
func (e *testEnv) activityTextColBySubject(t *testing.T, col, subject string) string {
	t.Helper()
	var val *string
	// col berasal dari literal test (bukan input user) → aman diinterpolasi.
	q := "SELECT " + col + " FROM activities WHERE tenant_id=$1 AND subject=$2"
	if err := e.h.Pool.QueryRow(t.Context(), q, e.tenantID, subject).Scan(&val); err != nil {
		t.Fatalf("readback %s subjek %q: %v", col, subject, err)
	}
	if val == nil {
		return ""
	}
	return *val
}

// TestActivityCreate_MeetingTypePersists: Jenis=meeting + meeting_type=Daring →
// tersimpan sebagai Daring (kolom meeting_type kini ber-form).
func TestActivityCreate_MeetingTypePersists(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Rapat", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "meeting")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Pertemuan dari Form")
	form.Set("meeting_type", "Daring")
	form.Set("start_at", "2026-09-10T09:00")
	form.Set("location", "Google Meet")
	form.Set("status", "Planned")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create meeting harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.activityKindBySubject(t, "Pertemuan dari Form"); got != "meeting" {
		t.Errorf("kind tersimpan = %q, ingin \"meeting\"", got)
	}
	if got := env.activityTextColBySubject(t, "meeting_type", "Pertemuan dari Form"); got != "Daring" {
		t.Errorf("meeting_type tersimpan = %q, ingin \"Daring\"", got)
	}
}

// TestActivityCreate_ChatChannelPersists: Jenis=chat + channel=WhatsApp →
// tersimpan sebagai WhatsApp (kolom channel kini ber-form).
func TestActivityCreate_ChatChannelPersists(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Chat", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "chat")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Chat dari Form")
	form.Set("channel", "WhatsApp")
	form.Set("direction", "Outbound")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create chat harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.activityKindBySubject(t, "Chat dari Form"); got != "chat" {
		t.Errorf("kind tersimpan = %q, ingin \"chat\"", got)
	}
	if got := env.activityTextColBySubject(t, "channel", "Chat dari Form"); got != "WhatsApp" {
		t.Errorf("channel tersimpan = %q, ingin \"WhatsApp\"", got)
	}
}

// TestActivityCreate_InvalidMeetingTypeRejected: meeting_type di luar enum →
// ?err=activity_meeting_type, tak tersimpan (validasi validMeetingTypes sebelum insert).
func TestActivityCreate_InvalidMeetingTypeRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Rapat Sah", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "meeting")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Meeting Tak Sah")
	form.Set("meeting_type", "Hologram") // di luar enum

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=activity_meeting_type") {
		t.Errorf("meeting_type tak sah harus err=activity_meeting_type, got %q", loc)
	}
}

// TestActivityCreate_InvalidChannelRejected: channel di luar enum →
// ?err=activity_channel, tak tersimpan (validasi validChannels sebelum insert).
func TestActivityCreate_InvalidChannelRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Chat Sah", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "chat")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Chat Tak Sah")
	form.Set("channel", "Merpati") // di luar enum

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=activity_channel") {
		t.Errorf("channel tak sah harus err=activity_channel, got %q", loc)
	}
}

// TestActivityNew_RendersMeetingAndChatGroups: form buat merender grup field
// Pertemuan & Chat + toggle Datastar keduanya, dan dropdown Jenis kini 5 kind.
func TestActivityNew_RendersMeetingAndChatGroups(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/activities/new", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivityNew status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Detail Pertemuan", "Detail Chat",
		// datastar meng-escape kutip tunggal jadi &#39; di atribut.
		`data-show="$kind == &#39;meeting&#39;"`, `data-show="$kind == &#39;chat&#39;"`,
		`name="meeting_type"`, `name="channel"`,
		`value="meeting"`, `value="chat"`, // opsi dropdown Jenis
	} {
		if !strings.Contains(body, want) {
			t.Errorf("form buat harus memuat %q", want)
		}
	}
}
