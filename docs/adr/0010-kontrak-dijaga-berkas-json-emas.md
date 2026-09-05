# ADR-0010 — Kontrak dijaga oleh berkas JSON emas kembar

**Status:** Diterima · 5 September 2026

## Konteks

Dua repo tanpa paket bersama akan menyimpang. Pilihan lazimnya monorepo, generator kode dari skema, atau registri skema — ketiganya menuntut infrastruktur.

## Keputusan

Satu berkas JSON emas, identik byte-per-byte di dua repo: `internal/aiclient/testdata/propose_request.golden.json` dan `Terrion_AI/tests/fixtures/propose_request.golden.json`. Uji Go membangun permintaan dari fixture domain dan membandingkannya dengan berkas emas; uji Python mengurai berkas emas dan menegaskan field terisi. Hal serupa untuk respons.

## Konsekuensi

Perlu menyalin satu berkas saat kontrak berubah. Sebagai gantinya: perubahan bentuk di satu sisi memecahkan uji di sisi itu, dan mengubah berkas emasnya memecahkan uji di sisi seberang pada CI berikutnya. Tanpa monorepo, tanpa codegen, tanpa registri.

## Alternatif yang ditolak

**Monorepo.** Menghapus batas kepemilikan yang justru menjadi alasan ADR-0001.

**Codegen dari OpenAPI/JSON Schema.** Menambah build step di dua repo untuk kontrak berisi satu endpoint.

**Paket bersama.** Mustahil lintas Go dan Python.
