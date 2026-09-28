package claudeai

// history.go — riwayat percakapan multi-turn (BL-179): Turn + buildMessages,
// dipisah dari client.go (murni transport) supaya satu tanggung jawab per
// file (CLAUDE.md §8) dan client.go tak melebihi batas 300 baris.

// Turn adalah satu giliran tanya-jawab LAMA yang sudah selesai — dioper ke
// AskWithTools sbg konteks tambahan (BL-179, keputusan user 28 Sep: 5 giliran
// terakhir, ditegakkan DI HANDLER, bukan di sini — paket ini tak peduli soal
// cap, cuma merangkai apa pun yang diterima jadi messages yang sah).
type Turn struct {
	Question string
	Answer   string
}

// buildMessages merangkai riwayat + pertanyaan baru jadi messages Anthropic
// yang SAH: role user/assistant WAJIB berselang-seling murni (syarat keras
// Messages API), dimulai dari user. Guardrail+knowledgeMD (cache_control
// ephemeral) TETAP di content-block PERTAMA pesan PERTAMA (openingMessage) —
// jangkar cache prompt tak boleh pindah walau ada riwayat: byte identik antar
// call demi cache hit (giliran pertama riwayat yang mengisi block kedua
// pesan itu, bukan pertanyaan baru yang selalu berubah). history kosong/nil
// → identik persis perilaku lama (satu openingMessage saja) — backward
// compatible utuh, tak mengubah wire behavior saat fitur riwayat tak dipakai.
func buildMessages(knowledgeMD string, history []Turn, question string) []message {
	if len(history) == 0 {
		return []message{openingMessage(knowledgeMD, question)}
	}

	msgs := make([]message, 0, 2*len(history)+1)
	msgs = append(msgs, openingMessage(knowledgeMD, history[0].Question))
	msgs = append(msgs, message{
		Role:    "assistant",
		Content: []contentBlock{{Type: "text", Text: history[0].Answer}},
	})
	for _, t := range history[1:] {
		msgs = append(msgs, message{
			Role:    "user",
			Content: []contentBlock{{Type: "text", Text: t.Question}},
		})
		msgs = append(msgs, message{
			Role:    "assistant",
			Content: []contentBlock{{Type: "text", Text: t.Answer}},
		})
	}
	msgs = append(msgs, message{
		Role:    "user",
		Content: []contentBlock{{Type: "text", Text: question}},
	})
	return msgs
}
