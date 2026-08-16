-- 000006 seeded 高风险关键词 as an enabled `block` rule with an empty
-- pattern. The matcher skips empty keywords, so it never blocked
-- anything -- it just sat in the DLP rules table looking like an active
-- line of defence. Remove it where it is still untouched; an operator who
-- has since filled in keywords keeps their rule.
DELETE FROM dlp_rules
WHERE name = '高风险关键词' AND match_type = 'keyword' AND btrim(pattern) = '';
