package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_activities_form.go — parsing & validasi form Sales Activity (4.4), dipakai
// bersama create & update. Dipisah dari handler aksi agar aturan validasi (enum,
// panjang, angka, tanggal) punya SATU tempat: create & edit tak boleh menerima
// nilai yang berbeda sahnya untuk kolom yang sama. Meniru sales_deals_form.go.
//
// Enum di sini = CERMIN CHECK constraint migrasi 00011 (kind/target_type/status/
// priority/direction/call_result) — penolakan terjadi di sini SEBELUM DB; CHECK
// jaring terakhir. UI hanya menyurfacekan 3 kind Inti (task/call/note); kolom
// per-kind lain (meeting/email/CS) tak ber-form v1.

const maxActivitySubjectLen = 200

var (
	// validActivityKinds = kind yang ber-UI v1 (Inti-3). Cermin subset kind_chk.
	validActivityKinds = map[string]struct{}{"task": {}, "call": {}, "note": {}}
	// validActivityTargetTypes = tipe target yang ber-picker v1 (Deal/Account/
	// Contact). Cermin subset target_type_chk (ticket/subscription belum ber-UI).
	validActivityTargetTypes = map[string]struct{}{"deal": {}, "account": {}, "contact": {}}
	// validActivityPriorities = prioritas Task. Cermin priority_chk.
	validActivityPriorities = map[string]struct{}{"Low": {}, "Normal": {}, "High": {}}
	// validTaskStatuses = daur hidup Task (subset status_chk yang relevan untuk
	// task; nilai meeting seperti Planned/Held tak ditawarkan di UI task).
	validTaskStatuses = map[string]struct{}{
		"Not Started": {}, "In Progress": {}, "Completed": {}, "Deferred": {},
	}
	// validActivityDirections = arah Call. Cermin direction_chk.
	validActivityDirections = map[string]struct{}{"Inbound": {}, "Outbound": {}}
	// validCallResults = hasil Call. Cermin call_result_chk.
	validCallResults = map[string]struct{}{
		"Connected": {}, "No Answer": {}, "Busy": {}, "Voicemail": {},
		"Follow-up": {}, "No Respond": {},
	}
)

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00011; urutan untuk tampilan.
var (
	activityKindOptions      = []string{"task", "call", "note"}
	activityPriorityOptions  = []string{"Low", "Normal", "High"}
	taskStatusOptions        = []string{"Not Started", "In Progress", "Completed", "Deferred"}
	activityDirectionOptions = []string{"Inbound", "Outbound"}
	callResultOptions        = []string{"Connected", "No Answer", "Busy", "Voicemail", "Follow-up", "No Respond"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(activityKindOptions) != len(validActivityKinds) ||
		len(activityPriorityOptions) != len(validActivityPriorities) ||
		len(taskStatusOptions) != len(validTaskStatuses) ||
		len(activityDirectionOptions) != len(validActivityDirections) ||
		len(callResultOptions) != len(validCallResults) {
		panic("activities: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// activityForm = nilai form aktivitas yang SUDAH divalidasi & siap dipetakan ke
// Create/UpdateActivityParams. Kolom opsional pointer (nil = NULL). kind & target
// TAK di sini: kind menentukan bentuk form (immutable saat edit) & target
// ditetapkan saat create (parseActivityTarget), keduanya bukan field editable.
type activityForm struct {
	Subject     string
	Status      *string
	Notes       *string
	DueDate     pgtype.Date
	Priority    *string
	ContactID   *int64
	Direction   *string
	ActivityAt  pgtype.Timestamptz
	DurationMin *int32
	CallResult  *string
	Body        *string
}

// parseActivityForm membaca & memvalidasi form untuk sebuah kind. (form, "") bila
// sah, atau (zero, kode) yang dipetakan wsErrMsg. Cabang per-kind hanya mengisi
// kolom yang relevan — sisanya tetap NULL. subject wajib untuk semua kind.
func parseActivityForm(fv func(string) string, kind string) (activityForm, string) {
	var f activityForm

	f.Subject = strings.TrimSpace(fv("subject"))
	if f.Subject == "" || len(f.Subject) > maxActivitySubjectLen {
		return activityForm{}, "activity_subject"
	}

	switch kind {
	case "task":
		dd, code := optDate(fv("due_date"))
		if code != "" {
			return activityForm{}, code
		}
		f.DueDate = dd
		if s := strings.TrimSpace(fv("priority")); s != "" {
			if _, ok := validActivityPriorities[s]; !ok {
				return activityForm{}, "activity_priority"
			}
			f.Priority = &s
		}
		if s := strings.TrimSpace(fv("status")); s != "" {
			if _, ok := validTaskStatuses[s]; !ok {
				return activityForm{}, "activity_status"
			}
			f.Status = &s
		}
		f.Notes = optTrim(fv("notes"))
	case "call":
		cid, code := optActivityInt64(fv("contact_id"))
		if code != "" {
			return activityForm{}, code
		}
		f.ContactID = cid
		if s := strings.TrimSpace(fv("direction")); s != "" {
			if _, ok := validActivityDirections[s]; !ok {
				return activityForm{}, "activity_direction"
			}
			f.Direction = &s
		}
		at, code := optDateTime(fv("activity_at"))
		if code != "" {
			return activityForm{}, code
		}
		f.ActivityAt = at
		dur, code := optDuration(fv("duration_min"))
		if code != "" {
			return activityForm{}, code
		}
		f.DurationMin = dur
		if s := strings.TrimSpace(fv("call_result")); s != "" {
			if _, ok := validCallResults[s]; !ok {
				return activityForm{}, "activity_call_result"
			}
			f.CallResult = &s
		}
		f.Notes = optTrim(fv("notes"))
	case "note":
		f.Body = optTrim(fv("body"))
	default:
		return activityForm{}, "activity_kind"
	}

	return f, ""
}

// parseActivityTarget mengurai nilai picker target "type:id" (mis. "deal:123").
// Bentuk & enum type divalidasi di sini; KEBERADAAN & cakupan baris diverifikasi
// handler (targetInScope) — nilai user-controlled, backend penjaga sesungguhnya.
func parseActivityTarget(s string) (targetType string, id int64, ok bool) {
	t, idStr, found := strings.Cut(strings.TrimSpace(s), ":")
	if !found {
		return "", 0, false
	}
	if _, valid := validActivityTargetTypes[t]; !valid {
		return "", 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return t, n, true
}

// optActivityInt64 mengurai id referensi opsional (contact_id pada Call): kosong →
// (nil, ""); terisi wajib bilangan bulat positif → (&v, ""); else (nil,
// "activity_contact"). Keberadaan baris tak dicek di sini (ON DELETE SET NULL).
func optActivityInt64(s string) (*int64, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return nil, "activity_contact"
	}
	return &n, ""
}

// optDuration mengurai durasi menit opsional (Call): kosong → (nil, ""); terisi
// wajib bilangan bulat ≥ 0 → (&v, ""); else (nil, "activity_duration").
func optDuration(s string) (*int32, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 0 {
		return nil, "activity_duration"
	}
	v := int32(n)
	return &v, ""
}
