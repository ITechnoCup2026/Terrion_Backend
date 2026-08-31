create table weather_daily (
  grid_lat numeric(9,6) not null,
  grid_lng numeric(9,6) not null,
  date     date not null,
  temp_min numeric(5,2) not null,
  temp_max numeric(5,2) not null,
  primary key (grid_lat, grid_lng, date)
);

create table weather_normals (
  grid_lat    numeric(9,6) not null,
  grid_lng    numeric(9,6) not null,
  day_of_year int not null check (day_of_year between 1 and 366),
  mean_c      numeric(5,2) not null,
  sd_c        numeric(5,2) not null,
  primary key (grid_lat, grid_lng, day_of_year)
);
