select
  json_agg(
    json_strip_nulls(json_build_object('id', "ttt."."id"))
    order by
      "ttt."."jagger_rn"
  ) "ttt._json"
from
  lateral (
    select
      *,
      row_number() over () as jagger_rn
    from
      "ttt" as "ttt."
  ) "ttt."
