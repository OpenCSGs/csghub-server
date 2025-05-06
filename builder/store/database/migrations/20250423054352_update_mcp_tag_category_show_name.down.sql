SET statement_timeout = 0;

--bun:split

update tag_categories set show_name = 'Task' WHERE scope = 'mcp' and name = 'task';
update tag_categories set show_name = 'License' WHERE scope = 'mcp' and name = 'license';
update tag_categories set show_name = 'Publisher' WHERE scope = 'mcp' and name = 'publisher';
update tag_categories set show_name = 'Run Mode' WHERE scope = 'mcp' and name = 'runmode';
update tag_categories set show_name = 'Program Language' WHERE scope = 'mcp' and name = 'program_language';
