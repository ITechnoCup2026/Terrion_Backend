create table plot (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  member_id      uuid not null references member(id) on delete cascade,
  public_id      text not null unique,
  name           text not null,
  area_ha        numeric(8,4) not null check (area_ha > 0),
  lat            numeric(9,6) not null,
  lng            numeric(9,6) not null,
  grid_lat       numeric(9,6) generated always as (round(lat / 0.25) * 0.25) stored,
  grid_lng       numeric(9,6) generated always as (round(lng / 0.25) * 0.25) stored,
  tile_size_m2   int generated always as (
    case
      when area_ha * 10000 / 100  <= 400 then 100
      when area_ha * 10000 / 250  <= 400 then 250
      when area_ha * 10000 / 500  <= 400 then 500
      else 1000
    end
  ) stored,

  -- Decorative terrain. Read path is `terrain_override ?? generateTerrain(terrain_seed)`,
  -- so editing later writes a column that already exists.
  terrain_seed     int not null default ((random() * 2147483647)::int),
  terrain_override jsonb,
  decorations      jsonb not null default '[]'::jsonb,

  created_at     timestamptz not null default now()
);
create index plot_coop_idx on plot(cooperative_id);
create index plot_grid_idx on plot(grid_lat, grid_lng);

create table block (
  id                    uuid primary key default gen_random_uuid(),
  plot_id               uuid not null references plot(id) on delete cascade,
  label                 text not null,
  area_ha               numeric(8,4) not null check (area_ha > 0),
  order_index           int not null default 0,
  commodity_id          uuid not null references commodity(id),
  variety_id            uuid not null references variety(id),
  planting_date         date not null,
  actual_harvest_date   date,
  actual_yield_kg       numeric(12,2),
  actual_price_per_kg   numeric(12,2),
  payment_received_date date
);
create index block_plot_idx on block(plot_id);
