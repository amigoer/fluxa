-- Wrong-guess counter for one-time codes.
--
-- A code is six digits and lives for five minutes, and nothing bounded
-- how many times it could be guessed inside that window: the whole space
-- is a million values, so an attacker who can send requests quickly does
-- not need luck. Counting attempts against the code row -- rather than
-- against the caller -- is what makes the limit hold no matter how the
-- guesses are spread across connections or addresses.
ALTER TABLE local_otp_codes ADD COLUMN attempts integer NOT NULL DEFAULT 0;
