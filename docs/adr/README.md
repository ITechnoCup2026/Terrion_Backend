# Architecture Decision Records

Sepuluh keputusan yang membentuk pemisahan `Terrion_Backend` (Go) dan
`Terrion_AI` (Python). Format tiap berkas: Konteks → Keputusan → Konsekuensi →
Alternatif yang ditolak.

Kontrak yang mengikat keduanya: [`../ARCHITECTURE.md`](../ARCHITECTURE.md) §4,
versi `v1.0`.

| # | Judul |
| --- | --- |
| [0001](0001-layanan-ai-terpisah-basis-data-tidak.md) | Layanan AI terpisah, basis data tidak |
| [0002](0002-http-json-bukan-grpc-atau-antrean.md) | HTTP+JSON, bukan gRPC atau antrean |
| [0003](0003-kontrak-berversi-major-minor.md) | Kontrak berversi MAJOR.MINOR dengan 409 |
| [0004](0004-referensi-buram-per-permintaan.md) | Referensi buram per-permintaan, bukan UUID |
| [0005](0005-satu-panggilan-solve-dan-narasi.md) | Satu panggilan, bukan dua |
| [0006](0006-go-tidak-mempercayai-angka-dari-ai.md) | Go tidak pernah mempercayai angka dari AI |
| [0007](0007-determinisme-bagian-dari-kontrak.md) | Determinisme adalah bagian kontrak |
| [0008](0008-solver-go-dipertahankan-sebagai-fallback.md) | Solver Go dipertahankan sebagai fallback |
| [0009](0009-cache-di-go-bukan-di-python.md) | Cache di Go, bukan di Python |
| [0010](0010-kontrak-dijaga-berkas-json-emas.md) | Kontrak dijaga oleh berkas JSON emas kembar |
