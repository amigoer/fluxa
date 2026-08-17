-- The picture an identity source already holds for a person, so the
-- console can show colleagues as they appear in the IM everyone is
-- already looking at instead of a letter tile. Nullable: local accounts
-- have no source to take one from, and the letter tile stays their
-- fallback.
ALTER TABLE members ADD COLUMN avatar_url text;
