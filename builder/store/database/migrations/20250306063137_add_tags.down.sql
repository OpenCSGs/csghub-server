SET statement_timeout = 0;

--bun:split
DELETE FROM public.tags WHERE name = 'image-text-to-text';

DELETE FROM public.tags WHERE name = 'text-to-video';

DELETE FROM public.tags WHERE name = 'video-text-to-text';

DELETE FROM public.tags WHERE name = 'any-to-any';

DELETE FROM public.tags WHERE name = 'audio-text-to-text';
