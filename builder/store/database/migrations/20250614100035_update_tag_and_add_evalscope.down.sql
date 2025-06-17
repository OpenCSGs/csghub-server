SET statement_timeout = 0;

--bun:split

delete from tags where tag_name = 'evalscope';

--bun:split

delete from tag_rules where runtime_framework = 'evalscope';
