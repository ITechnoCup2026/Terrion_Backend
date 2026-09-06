create table plan_share_token (
  id              uuid primary key default gen_random_uuid(),
  plan_id         uuid not null references season_plan(id) on delete cascade,
  member_id       uuid not null references member(id) on delete cascade,
  created_at      timestamptz not null default now(),
  first_viewed_at timestamptz,
  last_viewed_at  timestamptz,
  unique (plan_id, member_id)
);
create index plan_share_token_plan_idx on plan_share_token(plan_id);

alter table plan_share_token enable row level security;

create policy tenant_read on plan_share_token for select
  using (exists (select 1 from season_plan p
                 where p.id = plan_share_token.plan_id
                   and p.cooperative_id = current_cooperative_id()));
create policy tenant_write on plan_share_token for all
  using (current_user_role() = 'pengurus'
         and exists (select 1 from season_plan p
                     where p.id = plan_share_token.plan_id
                       and p.cooperative_id = current_cooperative_id()))
  with check (current_user_role() = 'pengurus'
         and exists (select 1 from season_plan p
                     where p.id = plan_share_token.plan_id
                       and p.cooperative_id = current_cooperative_id()));
