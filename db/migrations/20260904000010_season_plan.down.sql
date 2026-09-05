drop index if exists block_plan_idx;
alter table block drop column if exists season_plan_id;

drop table if exists season_plan_item;
drop table if exists season_plan;

drop type if exists plan_status;
drop type if exists planning_objective;
