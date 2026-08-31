package handler

import (
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
