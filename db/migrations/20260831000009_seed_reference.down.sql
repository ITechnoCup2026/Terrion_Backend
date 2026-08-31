-- Reference rows only. Anything a cooperative registered against them is left
-- alone: the foreign keys would take plots and blocks with them, and a rollback
-- of a seed must not delete somebody's land.
delete from region_stat;
delete from reference_price;
delete from fertiliser_rate;
delete from variety;
delete from commodity where slug in
  ('generik','padi','jagung','wortel','cabai','kentang','beri');
