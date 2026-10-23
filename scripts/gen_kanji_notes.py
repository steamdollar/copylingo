"""Copy each kanji's structure note from scripts/kanji_notes.json into the
vocabulary payloads of the seed JSON (ADR-073, ADR-080).

Usage (from the repo root):
  python3 -I scripts/gen_kanji_notes.py [--write]

scripts/kanji_notes.json maps a kanji to its model.KanjiNote and is the only
place a note is edited. Every vocabulary payload gets "kanji_notes" last, one
note per kanji of the word that has one, in the order the kanji first appear;
a word without any loses the key. Without --write it only prints counts and
the noted kanji no word uses. The output is deterministic, so a rerun on an
unchanged note file leaves the seed files byte-identical.
"""
import json
import os
import re
import sys

WRITE = "--write" in sys.argv[1:]
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SEED_DIR = os.path.join(SCRIPT_DIR, "..", "cmd/seeding/data/ja")
NOTES_PATH = os.path.join(SCRIPT_DIR, "kanji_notes.json")
KANJI = re.compile(r"[一-鿿]")

with open(NOTES_PATH, encoding="utf-8") as f:
    note_by_kanji = json.load(f)

words, copies, used = 0, 0, set()
for level in ("n3", "n4", "n5"):
    path = os.path.join(SEED_DIR, f"{level}.json")
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    for material in data["materials"]:
        payload = material["payload"]
        if material["category"] != "vocabulary":
            continue
        # dict.fromkeys keeps first-appearance order and drops repeats (人々).
        chars = dict.fromkeys(KANJI.findall(payload["kanji"]))
        notes = [note_by_kanji[c] for c in chars if c in note_by_kanji]
        rebuilt = {k: v for k, v in payload.items() if k != "kanji_notes"}
        if notes:
            rebuilt["kanji_notes"] = notes
            words += 1
            copies += len(notes)
            used.update(n["char"] for n in notes)
        material["payload"] = rebuilt
    if WRITE:
        with open(path, "w", encoding="utf-8") as f:
            f.write(json.dumps(data, ensure_ascii=False, indent=2) + "\n")

print(f"notes {len(note_by_kanji)}, words {words}, copies {copies}")
unused = sorted(set(note_by_kanji) - used)
print(f"unused {len(unused)}: " + "".join(unused))
