-- Undo 20260907000013.
--
-- The varieties are removed only where nothing stands on them. block and
-- season_plan_item both reference variety(id) without a cascade, so a plain
-- delete would either fail the whole migration or -- worse, had they cascaded
-- -- take a cooperative's planted blocks down with the reference row. A down
-- migration undoes a schema change; it does not decide that somebody's crop
-- no longer exists. Run `go run ./cmd/seed -reset-only` first if you want the
-- rows gone regardless.

delete from variety v
where v.name in (
  'Inpari 32 HDB', 'Inpari 42 Agritan GSR', 'Inpari 30 Ciherang Sub1',
  'Mekongga', 'Situ Bagendit', 'Ciliwung', 'Cisadane', 'IR42',
  'NK Perkasa', 'Nasa-29', 'Bima-20 URI', 'Pioneer P27', 'Bisi-2',
  'Srikandi Kuning',
  'Kuroda', 'Chantenay', 'Imperator', 'Lokal Tawangmangu',
  'Cabai keriting', 'Cabai rawit Bhaskara', 'Cabai merah Tanjung-2',
  'Cabai besar Lembang-1',
  'Median', 'Repita', 'Granola Kembang', 'Amudra',
  'Stroberi Sweet Charlie', 'Stroberi Earlibrite', 'Stroberi Rosalinda'
)
and not exists (select 1 from block b where b.variety_id = v.id)
and not exists (select 1 from season_plan_item i where i.variety_id = v.id);


-- Jawa Barat was published by 20260831000009 and is not this migration's to
-- remove.
delete from reference_price
where province in (
  'Jawa Tengah', 'Jawa Timur', 'Sumatera Utara', 'Sumatera Selatan',
  'Lampung', 'Sulawesi Selatan', 'Bali', 'Nusa Tenggara Barat',
  'Kalimantan Selatan'
);
