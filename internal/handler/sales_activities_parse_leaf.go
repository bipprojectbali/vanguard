package handler

import (
	"strconv"
	"strings"
)

// sales_activities_parse_leaf.go — helper leaf parser aktivitas (parseActivityTarget,
// optActivityInt64, optDuration) dipisah dari sales_activities_parse.go
// (parseActivityForm) agar keduanya di bawah ambang tipe Route/Handler (150).

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
