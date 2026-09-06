-- Wider reference data: more varieties per commodity, and a price panel that
-- covers every province the seed now plants a cooperative in.
--
-- No new COMMODITY rows. A commodity carries a sprite_row that indexes into
-- the frontend's crops.png, and the sheet holds seven rows; an eighth
-- commodity would silently draw as one of the existing crops. Varieties are
-- free -- they carry agronomy, not a picture.
--
-- The agronomic figures below are PLAUSIBLE PLACEHOLDERS on the same scale as
-- the twelve varieties seeded in 20260831000009, not verified release data
-- from a Kementan/Balitbangtan descriptor sheet. They are internally
-- consistent (a longer-duration variety carries more GDD, base temperatures
-- match the commodity) so the projection behaves sensibly, and that is all
-- they are for. The names are real released varieties; the numbers attached
-- to them are not yet theirs. Replace them before any figure here is shown to
-- a farmer as advice.

insert into variety (commodity_id, name, gdd_requirement, base_temp_c,
                     days_to_harvest_min, days_to_harvest_max,
                     yield_per_ha_min, yield_per_ha_max)

-- Padi. Spread deliberately across duration: Situ Bagendit is the short one
-- and IR42 the long one, so a cooperative mixing them staggers its own peak.
select id, 'Inpari 32 HDB',         1815, 12, 108, 120, 5.5,  7.5  from commodity where slug='padi'
union all
select id, 'Inpari 42 Agritan GSR', 1890, 12, 112, 125, 6.0,  8.0  from commodity where slug='padi'
union all
select id, 'Inpari 30 Ciherang Sub1', 1855, 12, 108, 122, 5.5, 7.2 from commodity where slug='padi'
union all
select id, 'Mekongga',              1830, 12, 110, 125, 5.5,  7.0  from commodity where slug='padi'
union all
select id, 'Situ Bagendit',         1760, 12, 100, 115, 4.5,  6.0  from commodity where slug='padi'
union all
select id, 'Ciliwung',              1840, 12, 110, 125, 5.0,  6.8  from commodity where slug='padi'
union all
select id, 'Cisadane',              1900, 12, 120, 135, 5.0,  6.5  from commodity where slug='padi'
union all
select id, 'IR42',                  1950, 12, 125, 140, 4.5,  6.0  from commodity where slug='padi'

-- Jagung
union all
select id, 'NK Perkasa',            1800, 10,  95, 108, 8.0, 11.0  from commodity where slug='jagung'
union all
select id, 'Nasa-29',               1810, 10,  95, 110, 8.5, 12.0  from commodity where slug='jagung'
union all
select id, 'Bima-20 URI',           1780, 10,  93, 107, 7.0, 10.0  from commodity where slug='jagung'
union all
select id, 'Pioneer P27',           1770, 10,  92, 105, 7.5, 10.0  from commodity where slug='jagung'
union all
select id, 'Bisi-2',                1790, 10,  95, 110, 7.0,  9.0  from commodity where slug='jagung'
union all
select id, 'Srikandi Kuning',       1730, 10,  90, 105, 5.5,  8.0  from commodity where slug='jagung'

-- Wortel
union all
select id, 'Kuroda',                1790,  6,  85, 105, 20.0, 30.0 from commodity where slug='wortel'
union all
select id, 'Chantenay',             1720,  6,  80, 100, 16.0, 26.0 from commodity where slug='wortel'
union all
select id, 'Imperator',             1810,  6,  90, 110, 17.0, 27.0 from commodity where slug='wortel'
union all
select id, 'Lokal Tawangmangu',     1800,  6,  90, 110, 14.0, 24.0 from commodity where slug='wortel'

-- Cabai
union all
select id, 'Cabai keriting',        1930, 10,  95, 125,  7.0, 11.0 from commodity where slug='cabai'
union all
select id, 'Cabai rawit Bhaskara',  1880, 10,  92, 120,  6.5, 10.5 from commodity where slug='cabai'
union all
select id, 'Cabai merah Tanjung-2', 1910, 10,  95, 122,  8.5, 12.5 from commodity where slug='cabai'
union all
select id, 'Cabai besar Lembang-1', 1975, 10, 100, 130,  9.0, 13.0 from commodity where slug='cabai'

-- Kentang
union all
select id, 'Median',                2300,  2,  88, 118, 16.0, 26.0 from commodity where slug='kentang'
union all
select id, 'Repita',                2280,  2,  87, 115, 17.0, 27.0 from commodity where slug='kentang'
union all
select id, 'Granola Kembang',       2330,  2,  90, 120, 16.0, 26.0 from commodity where slug='kentang'
union all
select id, 'Amudra',                2260,  2,  85, 112, 18.0, 29.0 from commodity where slug='kentang'

-- Beri
union all
select id, 'Stroberi Sweet Charlie', 1820, 5,  78, 105,  9.0, 16.0 from commodity where slug='beri'
union all
select id, 'Stroberi Earlibrite',    1880, 5,  82, 110, 10.0, 17.0 from commodity where slug='beri'
union all
select id, 'Stroberi Rosalinda',     1900, 5,  84, 112,  9.5, 16.5 from commodity where slug='beri'

on conflict (commodity_id, name) do update set
  gdd_requirement     = excluded.gdd_requirement,
  base_temp_c         = excluded.base_temp_c,
  days_to_harvest_min = excluded.days_to_harvest_min,
  days_to_harvest_max = excluded.days_to_harvest_max,
  yield_per_ha_min    = excluded.yield_per_ha_min,
  yield_per_ha_max    = excluded.yield_per_ha_max;


-- The price panel, for every province the seed plants a cooperative in.
--
-- ReferencePriceRepository.FindForCommodities filters on province with no
-- fallback, so a cooperative outside Jawa Barat had an empty Dampak panel and
-- no seasonal benchmark -- which was already true of Brebes before this.
--
-- The regional factor is a flat multiplier standing in for what a real panel
-- would show: islands further from the Java glut sit higher, the surplus rice
-- provinces lower. Still synthetic, and still labelled as such.
with province(nama, factor) as (values
  ('Jawa Barat',          1.00),
  ('Jawa Tengah',         0.96),
  ('Jawa Timur',          0.97),
  ('Sumatera Utara',      1.08),
  ('Sumatera Selatan',    1.05),
  ('Lampung',             1.02),
  ('Sulawesi Selatan',    0.94),
  ('Bali',                1.12),
  ('Nusa Tenggara Barat', 1.06),
  ('Kalimantan Selatan',  1.10)
),
base(slug, price, amplitude) as (values
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
       p.nama,
       w.week_start,
       round(((b.price * p.factor) + (b.amplitude * p.factor) *
              sin(2 * pi() * extract(doy from w.week_start) / 365.0))::numeric, 2),
       'SINTETIS — ganti dengan panel harga Badan Pangan Nasional'
from base b
join commodity c on c.slug = b.slug
cross join province p
cross join weeks w
on conflict (commodity_id, province, week_start) do update
  set price_per_kg = excluded.price_per_kg,
      source       = excluded.source;
