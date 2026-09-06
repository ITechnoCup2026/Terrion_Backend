drop index if exists input_order_one_open_per_season;

alter table input_order_line drop column quantity_rdkk;

alter table input_order drop column status_changed_by_name;
alter table input_order drop column status_changed_by_id;
alter table input_order drop column status_changed_at;
alter table input_order drop column created_by_name;
alter table input_order drop column created_by_id;

alter table input_order alter column status drop default;
alter type order_status rename to order_status_new;
create type order_status as enum ('draft','submitted','completed');
alter table input_order
  alter column status type order_status
  using (case when status::text = 'cancelled' then 'draft' else status::text end)::order_status;
alter table input_order alter column status set default 'draft';
drop type order_status_new;
