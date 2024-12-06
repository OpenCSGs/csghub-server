SET statement_timeout = 0;

--bun:split

Delete from public.tags where name in ('Knowledge','Reasoning','Examination','Understanding','Code','Other');

--bun:split

Delete from public.tag_rules;
