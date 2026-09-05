# ADR-0006 — Go tidak pernah mempercayai angka dari layanan AI

**Status:** Diterima · 5 September 2026

## Konteks

Respons layanan AI memuat metrik: puncak tonase, nilai kotor, cakupan permintaan. Menampilkannya apa adanya akan menghemat pekerjaan.

## Keputusan

Yang diterima dari Python hanyalah **pilihan** — `candidate_ids`, kandidat mana yang masuk rencana mana. Seluruh tonase, jendela panen, dan nilai rupiah dibaca ulang dari tabel kandidat yang Go sendiri bangun. `candidate_id` yang tidak pernah diterbitkan Go ditolak dengan `plan_assignment_rejected`.

## Konsekuensi

Sedikit pekerjaan ganda. Sebagai gantinya, layanan AI yang dibobol paling buruk hanya bisa mengembalikan *pilihan* yang tidak optimal — yang tetap harus disetujui manusia lewat `POST /api/plans` sebelum menjadi apa pun.

## Alternatif yang ditolak

**Memakai metrik dari respons.** Menjadikan layanan tanpa kredensial sebagai sumber angka yang dibaca pengurus koperasi sebagai hasil perhitungan sistem.
