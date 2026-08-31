package handler

import (
	"strconv"
	"strings"
)

// sales_activities_parse.go — parsing & validasi form aktivitas sales
// (parseActivityForm) beserta helper leaf-nya (parseActivityTarget,
// optActivityInt64, optDuration). Dipisah dari sales_activities_form.go (enum
// sah + tipe activityForm) agar keduanya di bawah ambang tipe Route/Handler
// (150). Satu paket: enum tetap SATU sumber, parser membacanya.
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
