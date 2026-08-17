-- Feishu sign-ups stored "" for an address the provider never returned,
-- so "no address on file" and "an empty address" were the same value and
-- every check had to compare against the empty string. The write path
-- stores NULL now; this brings the rows already written into line.
UPDATE members SET email = NULL WHERE email = '';
UPDATE members SET phone = NULL WHERE phone = '';
