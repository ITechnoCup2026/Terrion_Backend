create type planning_objective as enum ('aman', 'pendapatan', 'pasar');
create type plan_status as enum ('applied', 'cancelled');

create table season_plan (
  id             uuid primary key default gen_random_uuid(),
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  season_label   text not null,
  season_start   date not null,
  season_end     date not null,
  objective      planning_objective not null,
  status         plan_status not null default 'applied',
  created_by     uuid not null references app_user(id) on delete cascade,
  created_at     timestamptz not null default now(),
  cancelled_at   timestamptz,
  constraint season_ordered check (season_end >= season_start)
);
create unique index season_plan_active_idx
  on season_plan (cooperative_id, season_label) where status = 'applied';

create table season_plan_item (
  id                     uuid primary key default gen_random_uuid(),
  plan_id                uuid not null references season_plan(id) on delete cascade,
  plot_id                uuid not null references plot(id) on delete cascade,
  member_id              uuid not null references member(id) on delete cascade,
  commodity_id           uuid not null references commodity(id),
  variety_id             uuid not null references variety(id),
  planting_date          date not null,
  area_ha                numeric(8,4) not null check (area_ha > 0),
  expected_tonnes_low    numeric(12,3) not null,
  expected_tonnes_mid    numeric(12,3) not null,
  expected_tonnes_high   numeric(12,3) not null,
  expected_harvest_start date not null,
  expected_harvest_end   date not null,
  plausibility           text not null,
  block_id               uuid references block(id) on delete set null
);
create index season_plan_item_plan_idx on season_plan_item(plan_id);

alter table block add column season_plan_id uuid references season_plan(id) on delete set null;
create index block_plan_idx on block(season_plan_id);

alter table season_plan      enable row level security;
alter table season_plan_item enable row level security;

create policy tenant_read on season_plan for select
  using (cooperative_id = current_cooperative_id());
create policy tenant_write on season_plan for all
  using (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus')
  with check (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus');

create policy tenant_read on season_plan_item for select
  using (exists (select 1 from season_plan p
                 where p.id = season_plan_item.plan_id
                   and p.cooperative_id = current_cooperative_id()));
create policy tenant_write on season_plan_item for all
  using (current_user_role() = 'pengurus'
         and exists (select 1 from season_plan p
                     where p.id = season_plan_item.plan_id
                       and p.cooperative_id = current_cooperative_id()))
  with check (current_user_role() = 'pengurus'
         and exists (select 1 from season_plan p
                     where p.id = season_plan_item.plan_id
                       and p.cooperative_id = current_cooperative_id()));
