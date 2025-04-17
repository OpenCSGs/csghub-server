SET statement_timeout = 0;

--bun:split

update tags set name=lower(name) where scope = 'mcp';
