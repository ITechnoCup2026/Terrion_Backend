create or replace function current_cooperative_id() returns uuid
language sql stable security definer set search_path = public as $$
  select cooperative_id from app_user where id = auth.uid()
$$;

create or replace function current_user_role() returns user_role
language sql stable security definer set search_path = public as $$
  select role from app_user where id = auth.uid()
$$;

alter table cooperative             enable row level security;
alter table app_user                enable row level security;
alter table member                  enable row level security;
alter table plot                    enable row level security;
alter table block                   enable row level security;
alter table cooperative_capacity    enable row level security;
alter table calibration             enable row level security;
alter table input_order             enable row level security;
alter table input_order_line        enable row level security;
alter table supply_contract_request enable row level security;

alter table commodity       enable row level security;
alter table variety         enable row level security;
alter table fertiliser_rate enable row level security;
alter table reference_price enable row level security;
alter table region_stat     enable row level security;
alter table weather_daily   enable row level security;
alter table weather_normals enable row level security;

create policy ref_read on commodity       for select using (true);
create policy ref_read on variety         for select using (true);
create policy ref_read on fertiliser_rate for select using (true);
create policy ref_read on reference_price for select using (true);
create policy ref_read on region_stat     for select using (true);
create policy ref_read on weather_daily   for select using (true);
create policy ref_read on weather_normals for select using (true);

create policy coop_read  on cooperative for select using (true);
create policy coop_write on cooperative for update
  using (id = current_cooperative_id() and current_user_role() = 'pengurus');

create policy self_read on app_user for select using (id = auth.uid());

create policy tenant_read on member for select
  using (cooperative_id = current_cooperative_id());
create policy tenant_write on member for all
  using (cooperative_id = current_cooperative_id() and current_user_role() in ('kader', 'pengurus'))
  with check (cooperative_id = current_cooperative_id() and current_user_role() in ('kader', 'pengurus'));

create policy tenant_read on plot for select
  using (cooperative_id = current_cooperative_id());
create policy tenant_write on plot for all
  using (cooperative_id = current_cooperative_id() and current_user_role() in ('kader', 'pengurus'))
  with check (cooperative_id = current_cooperative_id() and current_user_role() in ('kader', 'pengurus'));

create policy tenant_read on block for select
  using (exists (select 1 from plot p
                 where p.id = block.plot_id
                   and p.cooperative_id = current_cooperative_id()));
create policy tenant_write on block for all
  using (current_user_role() in ('kader', 'pengurus')
         and exists (select 1 from plot p
                     where p.id = block.plot_id
                       and p.cooperative_id = current_cooperative_id()))
  with check (current_user_role() in ('kader', 'pengurus')
         and exists (select 1 from plot p
                     where p.id = block.plot_id
                       and p.cooperative_id = current_cooperative_id()));

create policy tenant_read on cooperative_capacity for select
  using (cooperative_id = current_cooperative_id());
create policy tenant_write on cooperative_capacity for all
  using (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus')
  with check (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus');

create policy tenant_read on calibration for select
  using (cooperative_id = current_cooperative_id());

create policy tenant_read on input_order for select
  using (cooperative_id = current_cooperative_id());
create policy tenant_write on input_order for all
  using (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus')
  with check (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus');

create policy tenant_read on input_order_line for select
  using (exists (select 1 from input_order o
                 where o.id = input_order_line.input_order_id
                   and o.cooperative_id = current_cooperative_id()));
create policy tenant_write on input_order_line for all
  using (current_user_role() = 'pengurus'
         and exists (select 1 from input_order o
                     where o.id = input_order_line.input_order_id
                       and o.cooperative_id = current_cooperative_id()))
  with check (current_user_role() = 'pengurus'
         and exists (select 1 from input_order o
                     where o.id = input_order_line.input_order_id
                       and o.cooperative_id = current_cooperative_id()));

create policy coop_or_buyer_read on supply_contract_request for select
  using (cooperative_id = current_cooperative_id() or buyer_id = auth.uid());
create policy buyer_insert on supply_contract_request for insert
  with check (buyer_id = auth.uid());
create policy coop_respond on supply_contract_request for update
  using (cooperative_id = current_cooperative_id() and current_user_role() = 'pengurus');
