package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// activities.go — ±90 aktivitas polimorfik (target account/contact/deal/
// ticket/subscription YANG SUDAH DIBUAT — id nyata per tabel, bukan
// placeholder), tersebar ke nilai kind/status/priority/direction/
// call_result (migrasi 00011). context sales/cs/general dipilih mengikuti
// jenis target agar Sales/CS report tak silang data.

// activityKinds = kind yang DEMO selaras dengan yang bisa dibuat UI
// (task/meeting/call/chat/note). email belum ber-form → tak diseed agar list tak
// menampilkan jenis yang tak bisa dibuat/disunting user.
var activityKinds = []string{"task", "meeting", "call", "chat", "note"}
var activityStatuses = []string{"Not Started", "In Progress", "Completed", "Deferred", "Planned", "Held", "Cancelled", "No-Show"}

const activityTotal = 90

// seedActivities menerima ID nyata tiap tabel target polimorfik (accounts,
// contacts, deals, tickets, subscriptions) — tak boleh pakai account ID sbg
// placeholder utk target_type lain, karena tak ada FK yang menangkap salah
// isi di kolom ini (target_id polimorfik, DB tak validasi).
func seedActivities(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo, contacts, deals, subs, tickets []int64) (int, error) {
	targets := []struct {
		targetType string
		context    string
	}{
		{"account", "cs"}, {"account", "sales"}, {"contact", "sales"},
		{"deal", "sales"}, {"ticket", "cs"}, {"subscription", "cs"},
	}

	count := 0
	today := time.Now()
	for i := 0; i < activityTotal; i++ {
		t := targets[i%len(targets)]

		var targetID int64
		switch t.targetType {
		case "account":
			targetID = accounts[i%len(accounts)].ID
		case "contact":
			if len(contacts) == 0 {
				continue
			}
			targetID = contacts[i%len(contacts)]
		case "deal":
			if len(deals) == 0 {
				continue
			}
			targetID = deals[i%len(deals)]
		case "ticket":
			if len(tickets) == 0 {
				continue
			}
			targetID = tickets[i%len(tickets)]
		case "subscription":
			if len(subs) == 0 {
				continue
			}
			targetID = subs[i%len(subs)]
		}

		kind := activityKinds[i%len(activityKinds)]
		status := pick(rng, activityStatuses)
		priority := pick(rng, []string{"Low", "Normal", "High"})
		subject := subjectFor(kind, t.targetType)

		var dueDate pgtype.Date
		var activityAt pgtype.Timestamptz
		var direction *string
		var callResult *string
		var durationMin *int32
		var startAt pgtype.Timestamptz
		var endAt pgtype.Timestamptz
		var location *string
		var meetingType *string
		var channel *string

		// Kind yang bisa dibuat UI (task/meeting/call/chat/note); enum per-kind
		// mengikuti CHECK 00011 + 00031. email menyusul dengan field-nya sendiri.
		switch kind {
		case "call":
			direction = ptr(pick(rng, []string{"Inbound", "Outbound"}))
			callResult = ptr(pick(rng, []string{"Connected", "No Answer", "Busy", "Voicemail", "Follow-up", "No Respond"}))
			dur := intn32(rng, 3, 45)
			durationMin = &dur
			activityAt = pgtype.Timestamptz{Time: today.AddDate(0, 0, -rng.Intn(30)), Valid: true}
		case "meeting":
			meetingType = ptr(pick(rng, []string{"Tatap Muka", "Daring"}))
			start := today.AddDate(0, 0, rng.Intn(21)-10)
			startAt = pgtype.Timestamptz{Time: start, Valid: true}
			endAt = pgtype.Timestamptz{Time: start.Add(time.Hour), Valid: true}
			if *meetingType == "Tatap Muka" {
				location = ptr(pick(rng, []string{"Balai Desa", "Kantor Kecamatan", "Kantor Vanguard"}))
			} else {
				location = ptr(pick(rng, []string{"Google Meet", "Zoom"}))
			}
		case "chat":
			direction = ptr(pick(rng, []string{"Inbound", "Outbound"}))
			channel = ptr(pick(rng, []string{"WhatsApp", "Telegram", "SMS", "Lainnya"}))
			activityAt = pgtype.Timestamptz{Time: today.AddDate(0, 0, -rng.Intn(20)), Valid: true}
		case "task":
			dueDate = pgDate(today.AddDate(0, 0, rng.Intn(21)-7))
		case "note":
			activityAt = pgtype.Timestamptz{Time: today.AddDate(0, 0, -rng.Intn(10)), Valid: true}
		}

		var activityOwner *int64
		if rng.Intn(100) < 70 {
			activityOwner = owner
		}

		if _, err := q.CreateActivity(ctx, db.CreateActivityParams{
			TenantID:        tenantID,
			Kind:            kind,
			Subject:         subject,
			TargetType:      t.targetType,
			TargetID:        targetID,
			OwnerID:         activityOwner,
			ActivityContext: &t.context,
			Status:          &status,
			Notes:           ptr("Dicatat via seeddemo untuk keperluan demo dasbor."),
			DueDate:         dueDate,
			Priority:        &priority,
			Direction:       direction,
			ActivityAt:      activityAt,
			DurationMin:     durationMin,
			CallResult:      callResult,
			StartAt:         startAt,
			EndAt:           endAt,
			Location:        location,
			MeetingType:     meetingType,
			Channel:         channel,
			CreatedBy:       owner,
		}); err != nil {
			return count, fmt.Errorf("activity #%d (%s/%s): %w", i+1, kind, t.targetType, err)
		}
		count++
	}
	return count, nil
}

// subjectFor menyusun subjek aktivitas yang wajar dibaca sesuai kind+target,
// bukan string generik "Activity #N".
func subjectFor(kind, targetType string) string {
	// Kind yang diseed (task/meeting/call/chat/note); default = note.
	switch kind {
	case "call":
		return "Telepon tindak lanjut " + targetLabel(targetType)
	case "meeting":
		return "Pertemuan dengan " + targetLabel(targetType)
	case "chat":
		return "Percakapan dengan " + targetLabel(targetType)
	case "task":
		return "Tindak lanjut " + targetLabel(targetType)
	default:
		return "Catatan internal " + targetLabel(targetType)
	}
}

func targetLabel(targetType string) string {
	switch targetType {
	case "account":
		return "desa"
	case "contact":
		return "PIC desa"
	case "deal":
		return "peluang penjualan"
	case "ticket":
		return "tiket dukungan"
	case "subscription":
		return "langganan"
	default:
		return targetType
	}
}
