drop index if exists block_season_plan_idx;
alter table block drop column if exists season_plan_id;
drop index if exists season_plan_one_active_idx;
drop index if exists season_plan_coop_idx;
drop table if exists season_plan;
