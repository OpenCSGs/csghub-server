SET statement_timeout = 0;

--bun:split

UPDATE tags SET name = LOWER(name) where category = 'evaluation';
