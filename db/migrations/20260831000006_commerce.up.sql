create type request_status as enum ('pending','accepted','declined','withdrawn');
create type order_status   as enum ('draft','submitted','completed');

create table supply_contract_request (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  buyer_id       uuid not null references app_user(id) on delete cascade,
  -- Who is asking, copied from the buyer's profile at insert time.
  --
  -- Denormalised on purpose, for two reasons that both still hold. It is a
  -- point-in-time record: a buyer renaming their organisation must not rewrite
  -- what the cooperative agreed to supply. And app_user carries one policy,
  -- self_read, so any client reaching the database directly cannot join to it.
  buyer_name         text not null,
  buyer_organisation text,
  commodity_id   uuid not null references commodity(id),
  volume_kg      numeric(14,2) not null check (volume_kg > 0),
  window_start   date not null,
  window_end     date not null,
  status         request_status not null default 'pending',
  notes          text,
  created_at     timestamptz not null default now(),
  responded_at   timestamptz,
  constraint window_ordered check (window_end >= window_start)
);

create table input_order (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  season_label   text not null,
  status         order_status not null default 'draft',
  created_at     timestamptz not null default now()
);

create table input_order_line (
  id                    uuid primary key default gen_random_uuid(),
  input_order_id        uuid not null references input_order(id) on delete cascade,
  item                  text not null,
  quantity              numeric(12,2) not null,
  unit                  text not null,
  retail_price_per_unit numeric(12,2),
  bulk_price_per_unit   numeric(12,2)
);
