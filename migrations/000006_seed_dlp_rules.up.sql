-- Seeds the built-in DLP rules shown in the design mockup: pattern holds
-- a discriminator ('id_card' / 'bank_card') for the checksummed
-- built-in detectors in internal/security/rules, not a literal regex --
-- see internal/security/service.go findMatches.

INSERT INTO dlp_rules (name, match_type, pattern, action, priority, enabled) VALUES
    ('身份证号识别', 'regex_checksum', 'id_card', 'mask', 10, true),
    ('银行卡号识别', 'regex_checksum', 'bank_card', 'mask', 20, true),
    ('高风险关键词', 'keyword', '', 'block', 5, true);
