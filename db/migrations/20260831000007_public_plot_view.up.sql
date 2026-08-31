create view public_plot as
  select p.public_id, p.name, p.area_ha, p.tile_size_m2,
         m.name as member_name, c.village, c.district, p.terrain_seed
  from plot p
  join member m on m.id = p.member_id
  join cooperative c on c.id = p.cooperative_id;

grant select on public_plot to anon, authenticated;
