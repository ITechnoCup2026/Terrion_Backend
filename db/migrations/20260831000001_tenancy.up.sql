create type user_role as enum ('kader', 'pengurus', 'buyer');

create table cooperative (
  id              uuid primary key default gen_random_uuid(),
  name            text not null,
  village         text not null,
  district        text not null,
  district_code   text,
  province        text not null,
  lat             numeric(9,6) not null,
  lng             numeric(9,6) not null,
  stagger_applied jsonb not null default '[]'::jsonb,
  created_at      timestamptz not null default now()
);

create table app_user (
  id             uuid primary key references auth.users(id) on delete cascade,
  role           user_role not null,
  cooperative_id uuid references cooperative(id) on delete cascade,
  full_name      text not null,
  organisation   text,
  created_at     timestamptz not null default now(),
  constraint buyer_has_no_coop check (
    (role = 'buyer' and cooperative_id is null) or
    (role <> 'buyer' and cooperative_id is not null)
  )
);

create table member (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  name           text not null,
  nik_hash       text,
  created_at     timestamptz not null default now()
);
create index member_coop_idx on member(cooperative_id);
