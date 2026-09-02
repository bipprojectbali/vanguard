package handler

import (
	"strings"
)

// sales_activities_parse.go — parsing & validasi form aktivitas sales
// (parseActivityForm). Helper leaf-nya (parseActivityTarget, optActivityInt64,
// optDuration) di sales_activities_parse_leaf.go; enum sah + tipe activityForm di
// sales_activities_form.go — semua satu paket agar di bawah ambang tipe
// Route/Handler (150). Enum tetap SATU sumber, parser membacanya.
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
	case "meeting":
		st, code := optDateTime(fv("start_at"))
		if code != "" {
			return activityForm{}, code
		}
		f.StartAt = st
		et, code := optDateTime(fv("end_at"))
		if code != "" {
			return activityForm{}, code
		}
		f.EndAt = et
		f.Location = optTrim(fv("location"))
		if s := strings.TrimSpace(fv("meeting_type")); s != "" {
			if _, ok := validMeetingTypes[s]; !ok {
				return activityForm{}, "activity_meeting_type"
			}
			f.MeetingType = &s
		}
		if s := strings.TrimSpace(fv("status")); s != "" {
			if _, ok := validMeetingStatuses[s]; !ok {
				return activityForm{}, "activity_status"
			}
			f.Status = &s
		}
		f.Notes = optTrim(fv("notes"))
	case "chat":
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
		if s := strings.TrimSpace(fv("channel")); s != "" {
			if _, ok := validChannels[s]; !ok {
				return activityForm{}, "activity_channel"
			}
			f.Channel = &s
		}
		at, code := optDateTime(fv("activity_at"))
		if code != "" {
			return activityForm{}, code
		}
		f.ActivityAt = at
		f.Notes = optTrim(fv("notes"))
	case "note":
		f.Body = optTrim(fv("body"))
	default:
		return activityForm{}, "activity_kind"
	}

	return f, ""
}
