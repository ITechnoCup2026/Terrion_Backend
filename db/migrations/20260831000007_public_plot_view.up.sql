-- The boundary for every unauthenticated read: /garden, the Atlas farm view,
-- and the sibling navigation all select from this view rather than from plot.
-- It has no lat, lng, grid_lat or nik_hash column, so those endpoints cannot
-- leak a field's location even by mistake -- an exclusion that is structural
-- rather than a filter somebody has to remember to apply.
--
-- terrain_seed is safe to expose because it carries no geography at all: it
-- selects which hand-composed edge motifs frame the diagram, and the canvas
-- captions them as illustration.
create view public_plot as
  select p.public_id, p.name, p.area_ha, p.tile_size_m2,
         m.name as member_name, c.village, c.district, p.terrain_seed
  from plot p
  join member m on m.id = p.member_id
  join cooperative c on c.id = p.cooperative_id;

grant select on public_plot to anon, authenticated;
