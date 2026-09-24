package config

// config_limits.go — ambang panjang minimum rahasia (SESSION_KEY, MCP_TOKEN).
// Dipisah dari config.go (struct Config) agar file itu di bawah ambang tipe
// Config (100).

// MinSessionKeyLen = panjang minimum SESSION_KEY di production. 32 karakter
// setara ~192 bit bila di-generate acak (base64) — cukup jauh di atas ambang
// tebak-paksa, dan cukup rendah untuk tak menolak kunci yang sah.
const MinSessionKeyLen = 32

// MinMCPTokenLen = panjang minimum MCP_TOKEN bila diisi. Sama dengan SESSION_KEY
// dan alasannya sama: token ini membuka pembacaan runtime database ke pemegangnya,
// jadi harus tak-bisa-ditebak. Kosong (fitur mati) sah; diisi tapi lemah tidak.
const MinMCPTokenLen = 32
