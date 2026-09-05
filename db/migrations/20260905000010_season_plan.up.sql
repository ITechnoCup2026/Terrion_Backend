create table season_plan (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  label          text not null,
  season_start   date not null,
  season_end     date not null,
  objective      text not null check (objective in ('aman', 'pendapatan', 'pasar')),
  engine         text not null check (engine in ('ai-service', 'fallback')),
  status         text not null default 'applied' check (status in ('applied', 'cancelled')),
  created_at     timestamptz not null default now(),

  check (season_end > season_start)
);

create index season_plan_coop_idx on season_plan(cooperative_id, season_start);

-- Satu rencana yang berlaku per koperasi per musim. Rencana yang dibatalkan
-- tetap tersimpan sebagai riwayat dan tidak menahan slot ini.
create unique index season_plan_one_active_idx
  on season_plan(cooperative_id, season_start)
  where status = 'applied';

alter table block add column season_plan_id uuid references season_plan(id) on delete set null;
create index block_season_plan_idx on block(season_plan_id);
