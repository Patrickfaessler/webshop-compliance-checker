-- 0003: A5 keyboard probe options and probed pages per scan, notes on rule results (specs/rules/A5-keyboard.md, A5-7, A5-8).
ALTER TABLE scans ADD COLUMN keyboard_all_pages boolean NOT NULL DEFAULT false;
ALTER TABLE scans ADD COLUMN keyboard_max_presses integer NOT NULL DEFAULT 200;
ALTER TABLE scans ADD COLUMN keyboard_pages text[] NOT NULL DEFAULT '{}';
ALTER TABLE rule_results ADD COLUMN note_de text NOT NULL DEFAULT '';
