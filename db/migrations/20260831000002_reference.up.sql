create table commodity (
  id         uuid primary key default gen_random_uuid(),
  slug       text not null unique,
  name       text not null,
  sprite_row int  not null default 0
);

create table variety (
  id                  uuid primary key default gen_random_uuid(),
  commodity_id        uuid not null references commodity(id) on delete cascade,
  name                text not null,
  gdd_requirement     numeric(8,2) not null,
  base_temp_c         numeric(4,1)  not null,
  days_to_harvest_min int not null,
  days_to_harvest_max int not null,
  yield_per_ha_min    numeric(8,3) not null,
  yield_per_ha_max    numeric(8,3) not null,
  unique (commodity_id, name)
);

create table fertiliser_rate (
  commodity_id uuid not null references commodity(id) on delete cascade,
  input_item   text not null,
  kg_per_ha    numeric(8,2) not null,
  source       text not null,
  primary key (commodity_id, input_item)
);

create table reference_price (
  commodity_id uuid not null references commodity(id) on delete cascade,
  province     text not null,
  week_start   date not null,
  price_per_kg numeric(12,2) not null,
  source       text not null,
  primary key (commodity_id, province, week_start)
);

create type region_level as enum ('province', 'district');

create table region_stat (
  region_code       text not null,
  region_name       text not null,
  level             region_level not null,
  commodity_id      uuid not null references commodity(id) on delete cascade,
  year              int not null,
  production_tonnes numeric(14,2) not null,
  harvested_area_ha numeric(14,2) not null,
  source            text not null,
  primary key (region_code, commodity_id, year)
);
