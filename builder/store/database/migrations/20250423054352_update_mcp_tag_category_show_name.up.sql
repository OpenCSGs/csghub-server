SET statement_timeout = 0;

--bun:split

update tag_categories set show_name = '任务' WHERE scope = 'mcp' and name = 'task';
update tag_categories set show_name = '许可证' WHERE scope = 'mcp' and name = 'license';
update tag_categories set show_name = '发布者' WHERE scope = 'mcp' and name = 'publisher';
update tag_categories set show_name = '运行模式' WHERE scope = 'mcp' and name = 'runmode';
update tag_categories set show_name = '编程语言' WHERE scope = 'mcp' and name = 'program_language';
