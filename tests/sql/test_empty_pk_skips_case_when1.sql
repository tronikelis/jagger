select
  json_agg(
    json_strip_nulls(json_build_object('id', "ttt."."id"))
  ) "ttt._json"
from
  "ttt" as "ttt."
