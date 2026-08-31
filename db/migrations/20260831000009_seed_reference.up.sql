
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


insert into variety (commodity_id, name, gdd_requirement, base_temp_c,
                     days_to_harvest_min, days_to_harvest_max,
                     yield_per_ha_min, yield_per_ha_max)

select id, 'Ciherang',    1860, 12, 110, 125, 5.0, 7.0 from commodity where slug='padi'
union all
select id, 'IR64',        1780, 12, 105, 120, 4.5, 6.5 from commodity where slug='padi'

union all
select id, 'Bisi-18',     1830, 10,  95, 110, 7.0, 9.5 from commodity where slug='jagung'
union all
select id, 'Pioneer P35', 1740, 10,  90, 105, 7.5, 10.0 from commodity where slug='jagung'

union all
select id, 'Lokal Cipanas', 1845, 6,  90, 110, 15.0, 25.0 from commodity where slug='wortel'
union all
select id, 'Nantes',        1755, 6,  85, 105, 18.0, 28.0 from commodity where slug='wortel'

union all
select id, 'Cabai rawit',  1865, 10,  90, 120,  6.0, 10.0 from commodity where slug='cabai'
union all
select id, 'Cabai merah',  1955, 10,  95, 125,  8.0, 12.0 from commodity where slug='cabai'

union all
select id, 'Granola',      2355,  2,  90, 120, 15.0, 25.0 from commodity where slug='kentang'
union all
select id, 'Atlantic',     2245,  2,  85, 115, 18.0, 28.0 from commodity where slug='kentang'

union all
select id, 'Stroberi lokal', 1850, 5, 80, 110, 8.0, 15.0 from commodity where slug='beri'
union all
select id, 'Stroberi Kalifornia', 1945, 5, 85, 115, 10.0, 18.0 from commodity where slug='beri'

union all
select id, 'Umum', 1865, 10, 90, 120, 5.0, 10.0 from commodity where slug='generik'
on conflict (commodity_id, name) do update set
  gdd_requirement     = excluded.gdd_requirement,
  base_temp_c         = excluded.base_temp_c,
  days_to_harvest_min = excluded.days_to_harvest_min,
  days_to_harvest_max = excluded.days_to_harvest_max,
  yield_per_ha_min    = excluded.yield_per_ha_min,
  yield_per_ha_max    = excluded.yield_per_ha_max;


insert into fertiliser_rate (commodity_id, input_item, kg_per_ha, source)

select id, 'urea',  289, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (N 133 kg/ha ÷ 46% N)'      from commodity where slug='jagung'
union all
select id, 'sp36',  139, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (P2O5 50 kg/ha ÷ 36%)'      from commodity where slug='jagung'
union all
select id, 'kcl',    83, 'Acuan Rekomendasi Pupuk N P K Spesifik Lokasi untuk Jagung, Kementan (K2O 50 kg/ha ÷ 60%)'       from commodity where slug='jagung'

union all
select id, 'urea',  250, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'
union all
select id, 'sp36',  100, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'
union all
select id, 'kcl',   100, 'Permentan No. 40/2007 Pemupukan Padi Sawah Spesifik Lokasi (titik tengah nasional)' from commodity where slug='padi'

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

