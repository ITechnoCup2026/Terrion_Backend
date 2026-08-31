delete from region_stat;
delete from reference_price;
delete from fertiliser_rate;
delete from variety;
delete from commodity where slug in
  ('generik','padi','jagung','wortel','cabai','kentang','beri');
