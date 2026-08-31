-- Reference data the application cannot run without: commodities and their
-- varieties drive every prediction, fertiliser rates drive the RDKK, and the
-- Atlas is an empty map without region_stat.
--
-- Copied verbatim from the Next.js repo. Every statement is idempotent, so a
-- re-run updates rather than duplicating -- which is also why the verification
-- status of each figure travels in the data itself, in the source column.

-- ---------------------------------------------------------------- commodity

-- sprite_row indexes public/sprites/crops.png (5 stages x 6 rows).
-- Row 0 is padi, which doubles as the generic fallback: green growing,
-- golden ripe, stubble is the universal crop silhouette, so any commodity
-- without bespoke art still renders correctly.

insert into commodity (slug, name, sprite_row) values
  ('generik',  'Komoditas lain', 0),
  ('padi',     'Padi',           0),
  ('jagung',   'Jagung',         1),
  ('wortel',   'Wortel',         2),
  ('cabai',    'Cabai',          3),
  ('kentang',  'Kentang',        4),
  ('beri',     'Beri',           5)
on conflict (slug) do update
  set name = excluded.name, sprite_row = excluded.sprite_row;

-- ---------------------------------------------------------------- variety

-- Agronomic parameters driving L1. gdd_requirement in degree-days, base_temp_c
-- the crop's base temperature.
--
-- VERIFICATION STATUS is marked per line. Lines marked PROVISIONAL are
-- plausible field values that are NOT yet traced to a citable document.
-- Find them all with:  grep -n PROVISIONAL supabase/seed/variety.sql
--
-- Verified base temperatures come from the FAO56rev base/upper threshold
-- review (Pereira et al., Agricultural Water Management 2025):
--   maize 10 C, rice 12 C, potato 2 C, carrot 6 C.
--
-- gdd_requirement is DERIVED, not cited. It and days_to_harvest_* describe the
-- same quantity — how long the variety takes — and when they were sourced
-- independently they disagreed: rice carried the FAO generic-rice figure of
-- ~2000 dd, which at Subang's 27.8 C lowland mean implies 127 days against
-- Ciherang's published 110-125, so every padi block predicted 'late'. Potato
-- carried 1400 dd, which at the 24.5 C highland cell matures in 62 days
-- against a 90-day minimum, and read 'implausible'.
--
-- So each variety's gdd_requirement is now derived as the degree-days its own
-- published duration accumulates at the temperature that crop is actually
-- farmed at in West Java, read from the live climate normals:
--
--   padi, jagung  ->  Subang lowland   -6.25,107.75   27.8 C annual mean
--   cabai         ->  Subang town      -6.50,107.75   27.7 C
--   kentang,      ->  Jalancagak       -6.75,107.75   24.5 C
--   wortel, beri      (highland shoulder)
--
-- Re-derive and re-check plausibility at any time with:
--   pnpm tsx --env-file=.env scripts/derive-gdd.ts
--
-- This makes the two columns consistent by construction. It does NOT make the
-- durations themselves verified — days_to_harvest_* is still the provisional
-- input, so the derived GDD inherits its status.

insert into variety (commodity_id, name, gdd_requirement, base_temp_c,
                     days_to_harvest_min, days_to_harvest_max,
                     yield_per_ha_min, yield_per_ha_max)

-- padi: base temp VERIFIED (12 C), gdd DERIVED at 27.8 C, yields PROVISIONAL
select id, 'Ciherang',    1860, 12, 110, 125, 5.0, 7.0 from commodity where slug='padi'
union all
select id, 'IR64',        1780, 12, 105, 120, 4.5, 6.5 from commodity where slug='padi'

-- jagung: base temp VERIFIED (10 C), gdd DERIVED at 27.8 C, DTM + yields PROVISIONAL
union all
select id, 'Bisi-18',     1830, 10,  95, 110, 7.0, 9.5 from commodity where slug='jagung'
union all
select id, 'Pioneer P35', 1740, 10,  90, 105, 7.5, 10.0 from commodity where slug='jagung'

-- wortel: base temp VERIFIED (6 C), gdd DERIVED at 24.5 C, DTM + yields PROVISIONAL
union all
select id, 'Lokal Cipanas', 1845, 6,  90, 110, 15.0, 25.0 from commodity where slug='wortel'
union all
select id, 'Nantes',        1755, 6,  85, 105, 18.0, 28.0 from commodity where slug='wortel'

-- cabai: base temp PROVISIONAL (10 C assumed, warm-season), gdd DERIVED at 27.7 C,
-- all else PROVISIONAL
union all
select id, 'Cabai rawit',  1865, 10,  90, 120,  6.0, 10.0 from commodity where slug='cabai'
union all
select id, 'Cabai merah',  1955, 10,  95, 125,  8.0, 12.0 from commodity where slug='cabai'

-- kentang: base temp VERIFIED (2 C), gdd DERIVED at 24.5 C, DTM + yields PROVISIONAL
union all
select id, 'Granola',      2355,  2,  90, 120, 15.0, 25.0 from commodity where slug='kentang'
union all
select id, 'Atlantic',     2245,  2,  85, 115, 18.0, 28.0 from commodity where slug='kentang'

-- beri: gdd DERIVED at 24.5 C, everything else PROVISIONAL
union all
select id, 'Stroberi lokal', 1850, 5, 80, 110, 8.0, 15.0 from commodity where slug='beri'
union all
select id, 'Stroberi Kalifornia', 1945, 5, 85, 115, 10.0, 18.0 from commodity where slug='beri'

-- generik: deliberately mid-range. This is the fallback for any commodity a
-- kader registers that has no varietal data of its own. PROVISIONAL by design.
union all
select id, 'Umum', 1865, 10, 90, 120, 5.0, 10.0 from commodity where slug='generik'
on conflict (commodity_id, name) do update set
  gdd_requirement     = excluded.gdd_requirement,
  base_temp_c         = excluded.base_temp_c,
  days_to_harvest_min = excluded.days_to_harvest_min,
  days_to_harvest_max = excluded.days_to_harvest_max,
  yield_per_ha_min    = excluded.yield_per_ha_min,
  yield_per_ha_max    = excluded.yield_per_ha_max;

-- ---------------------------------------------------------------- fertiliser_rate

-- Flow D's RDKK output is only credible if these kg/ha figures trace to a
-- document. A judge who opens the repo, finds a bare constant and asks where
-- it came from ends the feature on the spot -- which is why `source` is not
-- optional and why unverified rows say so in the data itself.
--
-- Find every unverified row with:
--   select * from fertiliser_rate where source like 'BELUM DIVERIFIKASI%';
--
-- Nutrient-to-product conversion used below (standard Indonesian products):
--   urea   46% N     -> kg urea   = kg N     / 0.46
--   SP-36  36% P2O5  -> kg SP-36  = kg P2O5  / 0.36
--   KCl    60% K2O   -> kg KCl    = kg K2O   / 0.60

insert into fertiliser_rate (commodity_id, input_item, kg_per_ha, source)

-- JAGUNG -- VERIFIED.
-- "Acuan Rekomendasi Pupuk N, P, dan K Spesifik Lokasi untuk Jagung",
-- Kementerian Pertanian, pupukbersubsidi.pertanian.go.id.
-- Recommendation for N below 160 kg/ha: N 133, P2O5 50, K2O 50 kg/ha.
select id, 'urea',  289, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (N 133 kg/ha ÷ 46% N)'      from commodity where slug='jagung'
union all
select id, 'sp36',  139, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (P2O5 50 kg/ha ÷ 36%)'      from commodity where slug='jagung'
union all
select id, 'kcl',    83, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (K2O 50 kg/ha ÷ 60%)'       from commodity where slug='jagung'

-- PADI -- VERIFIED source, national mid-range applied.
-- Permentan No. 40 Tahun 2007 tentang Rekomendasi Pemupukan N, P, dan K pada
-- Padi Sawah Spesifik Lokasi (psp.pertanian.go.id). The regulation is
-- location-specific; the figures below are a conservative national midpoint
-- and should be narrowed to Subang before submission.
union all
select id, 'urea',  250, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'
union all
select id, 'sp36',  100, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'
union all
select id, 'kcl',   100, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'

-- The remainder are NOT verified. They are deliberately conservative so that
-- an aggregated RDKK under-states rather than over-states demand, and they
-- announce themselves in the source column.
union all
select id, 'urea',  200, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='cabai'
union all
select id, 'npk',   150, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='cabai'
union all
select id, 'urea',  150, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='wortel'
union all
select id, 'npk',   150, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='wortel'
union all
select id, 'urea',  200, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='kentang'
union all
select id, 'npk',   200, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='kentang'
union all
select id, 'urea',  100, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='beri'
union all
select id, 'npk',   100, 'BELUM DIVERIFIKASI — ganti dengan rekomendasi resmi' from commodity where slug='beri'
union all
select id, 'urea',  150, 'BELUM DIVERIFIKASI — nilai umum untuk komoditas tanpa data' from commodity where slug='generik'
union all
select id, 'npk',   150, 'BELUM DIVERIFIKASI — nilai umum untuk komoditas tanpa data' from commodity where slug='generik'

on conflict (commodity_id, input_item) do update
  set kg_per_ha = excluded.kg_per_ha, source = excluded.source;

-- ---------------------------------------------------------------- reference_price

-- Weekly farm-gate reference prices, used by impact figure 1 (effective price
-- received per kg, against a local reference).
--
-- EVERY ROW IS SYNTHETIC. The base prices are plausible Indonesian farm-gate
-- levels and the weekly variation is deterministic, not random -- so the demo
-- reproduces exactly -- but none of it is Badan Pangan Nasional or BPS data.
-- Every row says so in its `source` column.
--
-- Replace before submission:
--   select distinct source from reference_price;
--
-- Source of truth to replace it with:
--   Badan Pangan Nasional panel harga  https://panelharga.badanpangan.go.id
--
-- 156 weeks ending on the most recent Monday, for Jawa Barat -- the demo
-- province, since the generated cooperative sits in Kabupaten Subang.
--
-- Three years, not the 52 weeks originally seeded. Impact figure 1 compares a
-- block's actual price against the reference for the week it was SOLD in, and
-- the generator lays down two prior seasons so L2 has something to calibrate
-- against -- harvests reach ~112 weeks back. With 52 weeks the reference
-- window and the harvest history did not overlap at all, so every block was
-- dropped for want of a comparison and the figure read "no data" forever.

with base(slug, price, amplitude) as (values
  ('padi',     6500.0,  400.0),
  ('jagung',   4800.0,  350.0),
  ('cabai',   45000.0, 18000.0),
  ('kentang', 12000.0,  2500.0),
  ('wortel',   8000.0,  1800.0),
  ('beri',    35000.0,  6000.0)
),
weeks(week_start) as (
  select (date_trunc('week', current_date) - (n || ' weeks')::interval)::date
  from generate_series(0, 155) as n
)
insert into reference_price (commodity_id, province, week_start, price_per_kg, source)
select c.id,
       'Jawa Barat',
       w.week_start,
       round((b.price + b.amplitude *
              sin(2 * pi() * extract(doy from w.week_start) / 365.0))::numeric, 2),
       'SINTETIS — ganti dengan panel harga Badan Pangan Nasional'
from base b
join commodity c on c.slug = b.slug
cross join weeks w
on conflict (commodity_id, province, week_start) do update
  set price_per_kg = excluded.price_per_kg,
      source       = excluded.source;

-- ---------------------------------------------------------------- region_stat

-- Atlas base layer. Without this the map is empty and reads as broken.
--
-- WHAT IS REAL HERE: the 38 province codes and names are the actual BPS
-- codes, so the TopoJSON join is correct and the map draws properly.
--
-- WHAT IS NOT REAL: every production_tonnes and harvested_area_ha figure is
-- SYNTHETIC. It is shaped to match two published facts -- national padi
-- production and Java's share of it -- but no individual province figure is
-- BPS data, and every row says so in its `source` column.
--
--   Real anchors used for the shape only:
--     national rice production 2024 = 30.62 million tonnes (BPS)
--     Java = 54.19% of national padi production, led by Jawa Timur,
--     Jawa Tengah, Jawa Barat, then Sulawesi Selatan and Sumatera Selatan
--
-- Replace before submission:
--   select distinct source from region_stat;
--   -- every row should cite a BPS table, not 'SINTETIS'
--
-- Source of truth to replace it with:
--   https://www.bps.go.id/id/statistics-table/2/MTQ5OCMy/luas-panen-produksi-dan-produktivitas-padi-menurut-provinsi.html

with province(code, nama, weight) as (values
  ('11','Aceh',                      0.0300),
  ('12','Sumatera Utara',            0.0400),
  ('13','Sumatera Barat',            0.0250),
  ('14','Riau',                      0.0040),
  ('15','Jambi',                     0.0100),
  ('16','Sumatera Selatan',          0.0500),
  ('17','Bengkulu',                  0.0090),
  ('18','Lampung',                   0.0450),
  ('19','Kepulauan Bangka Belitung', 0.0010),
  ('21','Kepulauan Riau',            0.0005),
  ('31','DKI Jakarta',               0.0002),
  ('32','Jawa Barat',                0.1500),
  ('33','Jawa Tengah',               0.1700),
  ('34','DI Yogyakarta',             0.0090),
  ('35','Jawa Timur',                0.1800),
  ('36','Banten',                    0.0330),
  ('51','Bali',                      0.0120),
  ('52','Nusa Tenggara Barat',       0.0230),
  ('53','Nusa Tenggara Timur',       0.0140),
  ('61','Kalimantan Barat',          0.0170),
  ('62','Kalimantan Tengah',         0.0090),
  ('63','Kalimantan Selatan',        0.0220),
  ('64','Kalimantan Timur',          0.0030),
  ('65','Kalimantan Utara',          0.0015),
  ('71','Sulawesi Utara',            0.0090),
  ('72','Sulawesi Tengah',           0.0130),
  ('73','Sulawesi Selatan',          0.0900),
  ('74','Sulawesi Tenggara',         0.0110),
  ('75','Gorontalo',                 0.0060),
  ('76','Sulawesi Barat',            0.0060),
  ('81','Maluku',                    0.0020),
  ('82','Maluku Utara',              0.0015),
  ('91','Papua Barat',               0.0010),
  ('92','Papua Barat Daya',          0.0008),
  ('93','Papua',                     0.0020),
  ('94','Papua Selatan',             0.0030),
  ('95','Papua Tengah',              0.0005),
  ('96','Papua Pegunungan',          0.0005)
),
-- national totals per commodity, in tonnes. padi is the published 2024
-- figure; the rest are order-of-magnitude placeholders.
national(slug, tonnes, yield_t_ha) as (values
  ('padi',    30620000.0, 5.2),
  ('jagung',  16000000.0, 5.5),
  ('cabai',    2900000.0, 9.0),
  ('kentang',  1300000.0, 18.0),
  ('wortel',    700000.0, 20.0),
  ('beri',       25000.0, 12.0)
)
insert into region_stat (region_code, region_name, level, commodity_id, year,
                         production_tonnes, harvested_area_ha, source)
select p.code,
       p.nama,
       'province'::region_level,
       c.id,
       2024,
       round((n.tonnes * p.weight)::numeric, 2),
       round((n.tonnes * p.weight / n.yield_t_ha)::numeric, 2),
       'SINTETIS — kode provinsi BPS asli, angka produksi belum diganti data BPS'
from province p
cross join national n
join commodity c on c.slug = n.slug
on conflict (region_code, commodity_id, year) do update
  set production_tonnes = excluded.production_tonnes,
      harvested_area_ha = excluded.harvested_area_ha,
      source            = excluded.source;

