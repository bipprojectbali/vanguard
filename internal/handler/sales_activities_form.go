package handler

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// sales_activities_form.go — parsing & validasi form Sales Activity (4.4), dipakai
// bersama create & update. Dipisah dari handler aksi agar aturan validasi (enum,
// panjang, angka, tanggal) punya SATU tempat: create & edit tak boleh menerima
// nilai yang berbeda sahnya untuk kolom yang sama. Meniru sales_deals_form.go.
//
// Enum di sini = CERMIN CHECK constraint migrasi 00011 + 00031 (kind/target_type/
// status/priority/direction/call_result/meeting_type/channel) — penolakan terjadi
// di sini SEBELUM DB; CHECK jaring terakhir. UI kini menyurfacekan 5 kind
// (task/meeting/call/chat/note); email & kolom CS-only belum ber-form.

const maxActivitySubjectLen = 200

var (
	// validActivityKinds = kind yang ber-UI. Cermin subset kind_chk (email belum
	// ber-form → tak ditawarkan).
	validActivityKinds = map[string]struct{}{
		"task": {}, "meeting": {}, "call": {}, "chat": {}, "note": {},
	}
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
	// validMeetingStatuses = daur hidup Pertemuan (subset status_chk 00011:
	// Planned/Held/Cancelled/No-Show) — tak butuh migrasi baru.
	validMeetingStatuses = map[string]struct{}{
		"Planned": {}, "Held": {}, "Cancelled": {}, "No-Show": {},
	}
	// validMeetingTypes = tipe Pertemuan. Cermin meeting_type_chk (00031).
	validMeetingTypes = map[string]struct{}{"Tatap Muka": {}, "Daring": {}}
	// validActivityDirections = arah Call/Chat. Cermin direction_chk.
	validActivityDirections = map[string]struct{}{"Inbound": {}, "Outbound": {}}
	// validCallResults = hasil Call. Cermin call_result_chk.
	validCallResults = map[string]struct{}{
		"Connected": {}, "No Answer": {}, "Busy": {}, "Voicemail": {},
		"Follow-up": {}, "No Respond": {},
	}
	// validChannels = kanal Chat. Cermin channel_chk (00031).
	validChannels = map[string]struct{}{
		"WhatsApp": {}, "Telegram": {}, "SMS": {}, "Lainnya": {},
	}
)

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00011/00031; urutan untuk
// tampilan. Urutan kind mengelompokkan yang serumpun (task, lalu interaksi
// berwaktu meeting/call/chat, lalu note).
var (
	activityKindOptions      = []string{"task", "meeting", "call", "chat", "note"}
	activityPriorityOptions  = []string{"Low", "Normal", "High"}
	taskStatusOptions        = []string{"Not Started", "In Progress", "Completed", "Deferred"}
	meetingStatusOptions     = []string{"Planned", "Held", "Cancelled", "No-Show"}
	meetingTypeOptions       = []string{"Tatap Muka", "Daring"}
	activityDirectionOptions = []string{"Inbound", "Outbound"}
	callResultOptions        = []string{"Connected", "No Answer", "Busy", "Voicemail", "Follow-up", "No Respond"}
	channelOptions           = []string{"WhatsApp", "Telegram", "SMS", "Lainnya"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(activityKindOptions) != len(validActivityKinds) ||
		len(activityPriorityOptions) != len(validActivityPriorities) ||
		len(taskStatusOptions) != len(validTaskStatuses) ||
		len(meetingStatusOptions) != len(validMeetingStatuses) ||
		len(meetingTypeOptions) != len(validMeetingTypes) ||
		len(activityDirectionOptions) != len(validActivityDirections) ||
		len(callResultOptions) != len(validCallResults) ||
		len(channelOptions) != len(validChannels) {
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
	// Meeting
	StartAt     pgtype.Timestamptz
	EndAt       pgtype.Timestamptz
	Location    *string
	MeetingType *string
	// Chat
	Channel *string
	// Note
	Body *string
}
