SET statement_timeout = 0;

--bun:split

DELETE FROM tags WHERE name = 'agentichub-skills' AND scope = 'skill';
