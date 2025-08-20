SET statement_timeout = 0;

--bun:split

UPDATE tags SET show_name = 'Local' WHERE scope = 'mcp' and category = 'runmode' and name = 'local';
UPDATE tags SET show_name = 'Remote' WHERE scope = 'mcp' and category = 'runmode' and name = 'remote';
UPDATE tags SET show_name = 'Hybird' WHERE scope = 'mcp' and category = 'runmode' and name = 'hybird';

--bun:split

UPDATE tags SET show_name = 'Official' WHERE scope = 'mcp' and category = 'publisher' and name = 'official';
UPDATE tags SET show_name = 'Claimed' WHERE scope = 'mcp' and category = 'publisher' and name = 'claimed';

--bun:split

UPDATE tags SET show_name = 'Art & Culture' WHERE scope = 'mcp' AND category = 'scene' AND name = 'art & culture';
UPDATE tags SET show_name = 'Browser Automation' WHERE scope = 'mcp' AND category = 'scene' AND name = 'browser automation';
UPDATE tags SET show_name = 'Cloud Platforms' WHERE scope = 'mcp' AND category = 'scene' AND name = 'cloud platforms';
UPDATE tags SET show_name = 'Communication' WHERE scope = 'mcp' AND category = 'scene' AND name = 'communication';
UPDATE tags SET show_name = 'Customer Data Platforms' WHERE scope = 'mcp' AND category = 'scene' AND name = 'customer data platforms';
UPDATE tags SET show_name = 'Databases' WHERE scope = 'mcp' AND category = 'scene' AND name = 'databases';
UPDATE tags SET show_name = 'Developer Tools' WHERE scope = 'mcp' AND category = 'scene' AND name = 'developer tools';
UPDATE tags SET show_name = 'File Systems' WHERE scope = 'mcp' AND category = 'scene' AND name = 'file systems';
UPDATE tags SET show_name = 'Knowledge & Memory' WHERE scope = 'mcp' AND category = 'scene' AND name = 'knowledge & memory';
UPDATE tags SET show_name = 'Location Services' WHERE scope = 'mcp' AND category = 'scene' AND name = 'location services';
UPDATE tags SET show_name = 'Marketing' WHERE scope = 'mcp' AND category = 'scene' AND name = 'marketing';
UPDATE tags SET show_name = 'Monitoring' WHERE scope = 'mcp' AND category = 'scene' AND name = 'monitoring';
UPDATE tags SET show_name = 'Search' WHERE scope = 'mcp' AND category = 'scene' AND name = 'search';
UPDATE tags SET show_name = 'Version Control' WHERE scope = 'mcp' AND category = 'scene' AND name = 'version control';
UPDATE tags SET show_name = 'Finance' WHERE scope = 'mcp' AND category = 'scene' AND name = 'finance';
UPDATE tags SET show_name = 'Research & Data' WHERE scope = 'mcp' AND category = 'scene' AND name = 'research & data';
UPDATE tags SET show_name = 'Social Media' WHERE scope = 'mcp' AND category = 'scene' AND name = 'social media';
UPDATE tags SET show_name = 'OS Automation' WHERE scope = 'mcp' AND category = 'scene' AND name = 'os automation';
UPDATE tags SET show_name = 'Note Taking' WHERE scope = 'mcp' AND category = 'scene' AND name = 'note taking';
UPDATE tags SET show_name = 'Cloud Storage' WHERE scope = 'mcp' AND category = 'scene' AND name = 'cloud storage';
UPDATE tags SET show_name = 'E-commerce & Retail' WHERE scope = 'mcp' AND category = 'scene' AND name = 'e-commerce & retail';
UPDATE tags SET show_name = 'Education & Learning Tools' WHERE scope = 'mcp' AND category = 'scene' AND name = 'education & learning tools';
UPDATE tags SET show_name = 'Customer Support' WHERE scope = 'mcp' AND category = 'scene' AND name = 'customer support';
UPDATE tags SET show_name = 'Language Translation' WHERE scope = 'mcp' AND category = 'scene' AND name = 'language translation';
UPDATE tags SET show_name = 'Healthcare' WHERE scope = 'mcp' AND category = 'scene' AND name = 'healthcare';
UPDATE tags SET show_name = 'Image & Video Processing' WHERE scope = 'mcp' AND category = 'scene' AND name = 'image & video processing';
UPDATE tags SET show_name = 'Security' WHERE scope = 'mcp' AND category = 'scene' AND name = 'security';

