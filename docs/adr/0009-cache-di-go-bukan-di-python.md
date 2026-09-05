# ADR-0009 — Cache di Go, bukan di Python

**Status:** Diterima · 5 September 2026

## Konteks

Respons `Propose` mahal dan sangat bisa di-cache: soalnya hanya berubah kalau lahan, blok, varietas, harga acuan, atau permintaan pembeli berubah.

## Keputusan

Cache hidup di Redis sisi Go, berkunci `terrion:ai:plan:v1:<sha256(JSON kanonik permintaan)>` dengan `request_id` dikosongkan sebelum di-hash, TTL 6 jam. Python tetap **stateless penuh**. Kunci ikut dihapus saat sebuah rencana diterapkan atau dibatalkan.

## Konsekuensi

Python tidak bisa mengoptimalkan cache internal — tidak dibutuhkan. Cache juga melindungi jalur fallback: permintaan berulang tidak menyentuh jaringan sama sekali. `elapsed_ms` dikecualikan dari kunci.

## Alternatif yang ditolak

**Cache di Python.** Menjadikan layanan itu stateful, melanggar ADR-0007 dari arah lain, dan menambah komponen yang harus di-host untuk sesuatu yang Redis di Go sudah lakukan.
