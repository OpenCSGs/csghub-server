SET statement_timeout = 0;

--bun:split

DELETE FROM tag_categories WHERE scope = 'mcp';

--bun:split

DELETE FROM tags WHERE scope = 'mcp';
