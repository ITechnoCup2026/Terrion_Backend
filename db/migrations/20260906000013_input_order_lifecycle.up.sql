alter table input_order alter column status drop default;
alter type order_status rename to order_status_old;
create type order_status as enum ('draft','submitted','completed','cancelled');
alter table input_order
  alter column status type order_status using status::text::order_status;
alter table input_order alter column status set default 'draft';
drop type order_status_old;

alter table input_order add column created_by_id uuid references app_user(id) on delete set null;
alter table input_order add column created_by_name text;
alter table input_order add column status_changed_at timestamptz;
alter table input_order add column status_changed_by_id uuid references app_user(id) on delete set null;
alter table input_order add column status_changed_by_name text;

alter table input_order_line add column quantity_rdkk numeric(12,2);

create unique index input_order_one_open_per_season
  on input_order (cooperative_id, season_label)
  where status in ('draft','submitted');
