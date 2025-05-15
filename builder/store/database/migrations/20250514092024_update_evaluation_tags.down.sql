SET statement_timeout = 0;

--bun:split

UPDATE tags SET name = UPPER(name) where category = 'evaluation';
