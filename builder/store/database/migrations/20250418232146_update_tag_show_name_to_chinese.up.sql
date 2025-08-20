SET statement_timeout = 0;

--bun:split

UPDATE tags SET show_name = '本地' WHERE scope = 'mcp' and category = 'runmode' and name = 'local';
UPDATE tags SET show_name = '远程' WHERE scope = 'mcp' and category = 'runmode' and name = 'remote';
UPDATE tags SET show_name = '混合' WHERE scope = 'mcp' and category = 'runmode' and name = 'hybird';

--bun:split

UPDATE tags SET show_name = '官方发布' WHERE scope = 'mcp' and category = 'publisher' and name = 'official';
UPDATE tags SET show_name = '个人发布' WHERE scope = 'mcp' and category = 'publisher' and name = 'claimed';

--bun:split

UPDATE tags SET show_name = '文化艺术' WHERE scope = 'mcp' AND category = 'scene' AND name = 'art & culture';
UPDATE tags SET show_name = '浏览器自动化' WHERE scope = 'mcp' AND category = 'scene' AND name = 'browser automation';
UPDATE tags SET show_name = '云平台' WHERE scope = 'mcp' AND category = 'scene' AND name = 'cloud platforms';
UPDATE tags SET show_name = '通讯' WHERE scope = 'mcp' AND category = 'scene' AND name = 'communication';
UPDATE tags SET show_name = '数据平台' WHERE scope = 'mcp' AND category = 'scene' AND name = 'customer data platforms';
UPDATE tags SET show_name = '数据库' WHERE scope = 'mcp' AND category = 'scene' AND name = 'databases';
UPDATE tags SET show_name = '开发者工具' WHERE scope = 'mcp' AND category = 'scene' AND name = 'developer tools';
UPDATE tags SET show_name = '文件系统' WHERE scope = 'mcp' AND category = 'scene' AND name = 'file systems';
UPDATE tags SET show_name = '知识存储' WHERE scope = 'mcp' AND category = 'scene' AND name = 'knowledge & memory';
UPDATE tags SET show_name = '定位服务' WHERE scope = 'mcp' AND category = 'scene' AND name = 'location services';
UPDATE tags SET show_name = '市场营销' WHERE scope = 'mcp' AND category = 'scene' AND name = 'marketing';
UPDATE tags SET show_name = '监控' WHERE scope = 'mcp' AND category = 'scene' AND name = 'monitoring';
UPDATE tags SET show_name = '搜索' WHERE scope = 'mcp' AND category = 'scene' AND name = 'search';
UPDATE tags SET show_name = '版本控制' WHERE scope = 'mcp' AND category = 'scene' AND name = 'version control';
UPDATE tags SET show_name = '金融' WHERE scope = 'mcp' AND category = 'scene' AND name = 'finance';
UPDATE tags SET show_name = '数据研究' WHERE scope = 'mcp' AND category = 'scene' AND name = 'research & data';
UPDATE tags SET show_name = '社交媒体' WHERE scope = 'mcp' AND category = 'scene' AND name = 'social media';
UPDATE tags SET show_name = '系统自动化' WHERE scope = 'mcp' AND category = 'scene' AND name = 'os automation';
UPDATE tags SET show_name = '笔记' WHERE scope = 'mcp' AND category = 'scene' AND name = 'note taking';
UPDATE tags SET show_name = '云存储' WHERE scope = 'mcp' AND category = 'scene' AND name = 'cloud storage';
UPDATE tags SET show_name = '电子商务' WHERE scope = 'mcp' AND category = 'scene' AND name = 'e-commerce & retail';
UPDATE tags SET show_name = '教育学习' WHERE scope = 'mcp' AND category = 'scene' AND name = 'education & learning tools';
UPDATE tags SET show_name = '客户服务' WHERE scope = 'mcp' AND category = 'scene' AND name = 'customer support';
UPDATE tags SET show_name = '语言翻译' WHERE scope = 'mcp' AND category = 'scene' AND name = 'language translation';
UPDATE tags SET show_name = '医疗健康' WHERE scope = 'mcp' AND category = 'scene' AND name = 'healthcare';
UPDATE tags SET show_name = '图像视频处理' WHERE scope = 'mcp' AND category = 'scene' AND name = 'image & video processing';
UPDATE tags SET show_name = '安全' WHERE scope = 'mcp' AND category = 'scene' AND name = 'security';
