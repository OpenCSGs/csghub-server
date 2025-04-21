SET statement_timeout = 0;

--bun:split

update tag_categories set name = 'task' WHERE scope = 'mcp' and name = 'scene';

--bun:split

update tags set category = 'task' WHERE scope = 'mcp' and category = 'scene';

--bun:split

update tags set name = 'hybrid' WHERE scope = 'mcp' and category = 'runmode' and name = 'hybird';
