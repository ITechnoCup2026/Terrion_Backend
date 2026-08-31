create table cooperative_capacity (
  cooperative_id  uuid not null references cooperative(id) on delete cascade,
  commodity_id    uuid not null references commodity(id) on delete cascade,
  tonnes_per_week numeric(10,2) not null check (tonnes_per_week > 0),
  primary key (cooperative_id, commodity_id)
);

create table calibration (
  cooperative_id uuid not null references cooperative(id) on delete cascade,
  variety_id     uuid not null references variety(id) on delete cascade,
  offset_days    numeric(6,2) not null,
  n_observations int not null,
  residual_sd    numeric(6,2) not null,
  updated_at     timestamptz not null default now(),
  primary key (cooperative_id, variety_id)
);
