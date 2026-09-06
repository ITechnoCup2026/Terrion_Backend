-- Nomor kontak koperasi, untuk arah sebaliknya.
--
-- 20260908000014 memberi pembeli sebuah nomor sehingga koperasi bisa membalas
-- permintaan pasokan. Ini pasangannya: tanpa nomor koperasi, pembeli yang
-- permintaannya diterima tidak punya cara menanyakan kapan truknya datang.
--
-- Nomor koperasi, bukan nomor pengurus. Pengurus berganti; nomor yang tercetak
-- di papan nama koperasi tidak. Menautkan kontak ke akun perorangan berarti
-- pembeli kehilangan jalur setiap kali kepengurusan berubah.
alter table cooperative add column phone text;
