select
  json_agg(
    case
      when "user."."id" is null then null
      else json_strip_nulls(json_build_object('id', "user."."id"))
    end
  ) "user._json"
from
  "user" as "user."
