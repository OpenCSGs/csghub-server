SET statement_timeout = 0;

--bun:split

insert into tag_categories ("name", "scope", show_name) 
VALUES 
    ('program_language', 'mcp', 'Program Language'),
    ('runmode', 'mcp', 'Runmode'),
    ('publisher', 'mcp', 'Publisher'),
    ('scene', 'mcp', 'Scene'),
    ('license', 'mcp', 'License')
ON CONFLICT ("name", "scope") DO NOTHING;

--bun:split

INSERT INTO tags ("name", category, "scope", built_in, show_name, "group") 
VALUES
    ('TypeScript', 'program_language', 'mcp', true, 'TypeScript', ''),
    ('Python', 'program_language', 'mcp', true, 'Python', ''),
    ('Go', 'program_language', 'mcp', true, 'Go', ''),
    ('Java', 'program_language', 'mcp', true, 'Java', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;

INSERT INTO tags ("name", category, "scope", built_in, show_name, "group") 
VALUES
    ('Local', 'runmode', 'mcp', true, 'Local', ''),
    ('Remote', 'runmode', 'mcp', true, 'Remote', ''),
    ('Hybird', 'runmode', 'mcp', true, 'Hybird', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;

INSERT INTO tags ("name", category, "scope", built_in, show_name, "group") 
VALUES
    ('Official', 'publisher', 'mcp', true, 'Official', ''),
    ('Claimed', 'publisher', 'mcp', true, 'Claimed', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;

INSERT INTO tags ("name", category, "scope", built_in, show_name, "group") 
VALUES
    ('Art & Culture', 'scene', 'mcp', true, 'Art & Culture', ''),
    ('Browser Automation', 'scene', 'mcp', true, 'Browser Automation', ''),
    ('Cloud Platforms', 'scene', 'mcp', true, 'Cloud Platforms', ''),
    ('Communication', 'scene', 'mcp', true, 'Communication', ''),
    ('Customer Data Platforms', 'scene', 'mcp', true, 'Customer Data Platforms', ''),
    ('Databases', 'scene', 'mcp', true, 'Databases', ''),
    ('Developer Tools', 'scene', 'mcp', true, 'Developer Tools', ''),
    ('File Systems', 'scene', 'mcp', true, 'File Systems', ''),
    ('Knowledge & Memory', 'scene', 'mcp', true, 'Knowledge & Memory', ''),
    ('Location Services', 'scene', 'mcp', true, 'Location Services', ''),
    ('Marketing', 'scene', 'mcp', true, 'Marketing', ''),
    ('Monitoring', 'scene', 'mcp', true, 'Monitoring', ''),
    ('Search', 'scene', 'mcp', true, 'Search', ''),
    ('Version Control', 'scene', 'mcp', true, 'Version Control', ''),
    ('Finance', 'scene', 'mcp', true, 'Finance', ''),
    ('Research & Data', 'scene', 'mcp', true, 'Research & Data', ''),
    ('Social Media', 'scene', 'mcp', true, 'Social Media', ''),
    ('OS Automation', 'scene', 'mcp', true, 'OS Automation', ''),
    ('Note Taking', 'scene', 'mcp', true, 'Note Taking', ''),
    ('Cloud Storage', 'scene', 'mcp', true, 'Cloud Storage', ''),
    ('E-commerce & Retail', 'scene', 'mcp', true, 'E-commerce & Retail', ''),
    ('Education & Learning Tools', 'scene', 'mcp', true, 'Education & Learning Tools', ''),
    ('Customer Support', 'scene', 'mcp', true, 'Customer Support', ''),
    ('Language Translation', 'scene', 'mcp', true, 'Language Translation', ''),
    ('Healthcare', 'scene', 'mcp', true, 'Healthcare', ''),
    ('Image & Video Processing', 'scene', 'mcp', true, 'Image & Video Processing', ''),
    ('Security', 'scene', 'mcp', true, 'Security', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;

INSERT INTO tags ("name", category, "scope", built_in, show_name, "group") 
VALUES
    ('MIT', 'license', 'mcp', true, 'MIT', ''),
    ('Apache-2.0', 'license', 'mcp', true, 'Apache-2.0', ''),
    ('GPL', 'license', 'mcp', true, 'GPL', ''),
    ('GPL-2.0', 'license', 'mcp', true, 'GPL-2.0', ''),
    ('GPL-3.0', 'license', 'mcp', true, 'GPL-3.0', ''),
    ('AGPL', 'license', 'mcp', true, 'AGPL', ''),
    ('LGPL', 'license', 'mcp', true, 'LGPL', ''),
    ('BSD-3-Clause', 'license', 'mcp', true, 'BSD-3-Clause', ''),
    ('afl-3.0', 'license', 'mcp', true, 'afl-3.0', ''),
    ('ecl-2.0', 'license', 'mcp', true, 'ecl-2.0', ''),
    ('cc-by-4.0', 'license', 'mcp', true, 'cc-by-4.0', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;

