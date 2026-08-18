-- What a company may change about the mails this system sends. Deliberately
-- not the markup: the skeleton has to survive Outlook and blocked remote
-- images, and an admin editing HTML without a rendering target breaks
-- that silently. Brand colour and logo are absent for the same reason
-- DESIGN.md 6.1 gives -- Fluxa is not white-labelled.
--
-- One row, like auth_settings: these are deployment-wide, and a table
-- that can only ever hold one row is simpler to read than a key/value
-- bag whose keys are documented somewhere else.
CREATE TABLE mail_settings (
    id           boolean PRIMARY KEY DEFAULT true CHECK (id),
    org_name     text NOT NULL DEFAULT '',
    sign_off     text NOT NULL DEFAULT '',
    contact_line text NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL DEFAULT now()
);

INSERT INTO mail_settings (id) VALUES (true);
