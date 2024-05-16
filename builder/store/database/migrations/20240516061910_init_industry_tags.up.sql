SET statement_timeout = 0;

--bun:split

INSERT INTO public.tag_categories ("name", "scope") VALUES( 'industry', 'model') ON CONFLICT ("name", scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Automotive', 'industry', '', 'model', true, '汽车', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Manufacturing', 'industry', '', 'model', true, '制造业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Energy', 'industry', '', 'model', true, '能源', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Telecommunications and Electronic Information', 'industry', '', 'model', true, '通信与电子信息', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Transportation and Logistics', 'industry', '', 'model', true, '交通运输', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Construction and Real Estate', 'industry', '', 'model', true, '建筑与房地产', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Financial Services', 'industry', '', 'model', true, '金融服务', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Agriculture', 'industry', '', 'model', true, '农业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Chemical Industry', 'industry', '', 'model', true, '化工', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Environmental Protection', 'industry', '', 'model', true, '环保', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Healthcare and Medical Services', 'industry', '', 'model', true, '医疗与健康', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Education and Training', 'industry', '', 'model', true, '教育与培训', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Food and Beverage', 'industry', '', 'model', true, '食品与饮料', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Retail and Consumer Goods', 'industry', '', 'model', true, '零售与消费品', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Tourism and Hospitality', 'industry', '', 'model', true, '旅游与酒店', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Information Technology (IT)', 'industry', '', 'model', true, 'IT信息技术', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Culture and Entertainment', 'industry', '', 'model', true, '文化娱乐', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split


INSERT INTO public.tag_categories ("name", "scope") VALUES( 'industry', 'dataset') ON CONFLICT ("name", scope) DO NOTHING;
 
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Automotive', 'industry', '', 'dataset', true, '汽车', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Manufacturing', 'industry', '', 'dataset', true, '制造业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Energy', 'industry', '', 'dataset', true, '能源', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Telecommunications and Electronic Information', 'industry', '', 'dataset', true, '通信与电子信息', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Transportation and Logistics', 'industry', '', 'dataset', true, '交通运输', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Construction and Real Estate', 'industry', '', 'dataset', true, '建筑与房地产', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Financial Services', 'industry', '', 'dataset', true, '金融服务', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Agriculture', 'industry', '', 'dataset', true, '农业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Chemical Industry', 'industry', '', 'dataset', true, '化工', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Environmental Protection', 'industry', '', 'dataset', true, '环保', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Healthcare and Medical Services', 'industry', '', 'dataset', true, '医疗与健康', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Education and Training', 'industry', '', 'dataset', true, '教育与培训', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Food and Beverage', 'industry', '', 'dataset', true, '食品与饮料', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Retail and Consumer Goods', 'industry', '', 'dataset', true, '零售与消费品', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Tourism and Hospitality', 'industry', '', 'dataset', true, '旅游与酒店', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Information Technology (IT)', 'industry', '', 'dataset', true, 'IT信息技术', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Culture and Entertainment', 'industry', '', 'dataset', true, '文化娱乐', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split


INSERT INTO public.tag_categories ("name", "scope") VALUES( 'industry', 'code') ON CONFLICT ("name", scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Automotive', 'industry', '', 'code', true, '汽车', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Manufacturing', 'industry', '', 'code', true, '制造业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Energy', 'industry', '', 'code', true, '能源', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Telecommunications and Electronic Information', 'industry', '', 'code', true, '通信与电子信息', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Transportation and Logistics', 'industry', '', 'code', true, '交通运输', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Construction and Real Estate', 'industry', '', 'code', true, '建筑与房地产', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Financial Services', 'industry', '', 'code', true, '金融服务', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Agriculture', 'industry', '', 'code', true, '农业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Chemical Industry', 'industry', '', 'code', true, '化工', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Environmental Protection', 'industry', '', 'code', true, '环保', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Healthcare and Medical Services', 'industry', '', 'code', true, '医疗与健康', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Education and Training', 'industry', '', 'code', true, '教育与培训', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Food and Beverage', 'industry', '', 'code', true, '食品与饮料', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Retail and Consumer Goods', 'industry', '', 'code', true, '零售与消费品', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Tourism and Hospitality', 'industry', '', 'code', true, '旅游与酒店', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Information Technology (IT)', 'industry', '', 'code', true, 'IT信息技术', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Culture and Entertainment', 'industry', '', 'code', true, '文化娱乐', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split

INSERT INTO public.tag_categories ("name", "scope") VALUES( 'industry', 'space') ON CONFLICT ("name", scope) DO NOTHING;

INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Automotive', 'industry', '', 'space', true, '汽车', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Manufacturing', 'industry', '', 'space', true, '制造业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Energy', 'industry', '', 'space', true, '能源', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Telecommunications and Electronic Information', 'industry', '', 'space', true, '通信与电子信息', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Transportation and Logistics', 'industry', '', 'space', true, '交通运输', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Construction and Real Estate', 'industry', '', 'space', true, '建筑与房地产', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Financial Services', 'industry', '', 'space', true, '金融服务', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Agriculture', 'industry', '', 'space', true, '农业', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Chemical Industry', 'industry', '', 'space', true, '化工', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Environmental Protection', 'industry', '', 'space', true, '环保', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Healthcare and Medical Services', 'industry', '', 'space', true, '医疗与健康', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Education and Training', 'industry', '', 'space', true, '教育与培训', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Food and Beverage', 'industry', '', 'space', true, '食品与饮料', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Retail and Consumer Goods', 'industry', '', 'space', true, '零售与消费品', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Tourism and Hospitality', 'industry', '', 'space', true, '旅游与酒店', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Information Technology (IT)', 'industry', '', 'space', true, 'IT信息技术', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
INSERT INTO public.tags ("name", category, "group", "scope", built_in, show_name, created_at, updated_at) VALUES('Culture and Entertainment', 'industry', '', 'space', true, '文化娱乐', '2024-05-16 10:42:12.939', '2024-05-16 10:42:12.939') ON CONFLICT ("name", category, scope) DO NOTHING;
