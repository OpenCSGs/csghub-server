SET statement_timeout = 0;

--bun:split

update tag_categories set name = 'scene' WHERE scope = 'mcp' and name = 'task';

--bun:split

update tags set category = 'scene' WHERE scope = 'mcp' and category = 'task';
