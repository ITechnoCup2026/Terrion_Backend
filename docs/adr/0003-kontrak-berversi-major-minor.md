# ADR-0003 — Kontrak berversi MAJOR.MINOR dengan 409 saat tak cocok

**Status:** Diterima · 5 September 2026

## Konteks

Dua layanan yang di-deploy terpisah akan, cepat atau lambat, berjalan pada versi kontrak yang berbeda. Kegagalan diam pada saat itu menghasilkan data yang salah, bukan galat.

## Keputusan

`contract_version` berbentuk `MAJOR.MINOR` dan wajib ada di setiap permintaan. Python menerima `MAJOR` yang sama dengan `MINOR` ≤ miliknya; selain itu menjawab `409 contract_version_unsupported`. Penambahan field opsional menaikkan `MINOR`; perubahan yang merusak menaikkan `MAJOR` dan memindahkan jalur ke `/v2/`. Kedua sisi mengabaikan field yang tidak dikenal.

## Konsekuensi

Perlu disiplin menaikkan versi saat bentuk berubah. Sebagai gantinya, deploy yang tidak seiring gagal **berisik** — dan karena `409` berakhir di fallback seperti galat lain, pengguna tetap mendapat rencana.

## Alternatif yang ditolak

**Tanpa versi.** Ketidakcocokan akan muncul sebagai field yang hilang atau angka yang salah, ditemukan berminggu-minggu kemudian.

**Versi lewat jalur URL saja.** Cukup untuk perubahan merusak, tidak menangkap penambahan field yang belum dipahami sisi lain.
