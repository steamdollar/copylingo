"""Write each vocabulary word's per-kanji readings into the seed JSON (ADR-074).

Usage (from the repo root):
  python3 -I scripts/gen_kanji_readings.py \
      <kanjidic2.xml.gz> <JMdict_e.gz> <JmdictFurigana.json> [--write]

Sources (EDRDG licence, CC BY-SA 4.0; see NOTICE):
  KANJIDIC2       http://www.edrdg.org/kanjidic/kanjidic2.xml.gz
  JMdict          http://ftp.edrdg.org/pub/Nihongo/JMdict_e.gz
  JmdictFurigana  https://github.com/Doublevil/JmdictFurigana/releases

Every vocabulary payload with kanji gets "kanji_readings" right before
"kanji_notes" (or last), replacing any previous value:
  [{"text", "reading", "type": "on"|"kun"|"jukuji", "on": [...]}] in word order
  - text/reading: JmdictFurigana's alignment of the word's reading to its kanji
  - type: "on" when the reading is one of the kanji's KANJIDIC2 on'yomi
    (rendaku and sokuon allowed), "jukuji" when one reading covers a group
  - on: the kanji's on'yomi used in common JMdict words (news1/ichi1/spec1/
    spec2/gai1), most used first, at most three plus the word's own
Without --write it only prints counts and the words it cannot align; add
those to MANUAL. The output is deterministic, so a rerun on unchanged inputs
leaves the files byte-identical.
"""
import gzip
import json
import os
import re
import sys
import xml.etree.ElementTree as ET
from collections import Counter, defaultdict

KANJIDIC_PATH, JMDICT_PATH, FURIGANA_PATH = sys.argv[1:4]
WRITE = "--write" in sys.argv[4:]
SEED_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "cmd/seeding/data/ja")
COMMON_TAGS = {"news1", "ichi1", "spec1", "spec2", "gai1"}
KANJI = re.compile(r"[一-鿿]")
MAX_COMMON_ON = 3

# Words JmdictFurigana cannot align (phrases, counters, ateji, the shortened
# on'yomi of 日本), written by hand as (text, reading) segments.
MANUAL = {
    ("可愛らしい", "かわいらしい"): [("可愛", "かわい")],
    ("真っ赤", "まっか"): [("真っ赤", "まっか")],
    ("良い", "いい"): [("良", "い")],
    ("顔を洗う", "かおをあらう"): [("顔", "かお"), ("洗", "あら")],
    ("二匹", "にひき"): [("二", "に"), ("匹", "ひき")],
    ("三匹", "さんびき"): [("三", "さん"), ("匹", "びき")],
    ("日本", "にほん"): [("日", "に"), ("本", "ほん")],
    ("日本", "にっぽん"): [("日", "にっ"), ("本", "ぽん")],
    ("日本語", "にほんご"): [("日", "に"), ("本", "ほん"), ("語", "ご")],
}

# (word, kanji) whose surface match misleads: 差す and 放る are Joyo-table
# kun'yomi that happen to equal an on form, and 日本 and 文字 shorten the
# on'yomi (ニチ→に, モン→も).
TYPE_OVERRIDE = {
    ("差す", "差"): "kun",
    ("放る", "放"): "kun",
    ("日本", "日"): "on",
    ("日本語", "日"): "on",
    ("文字", "文"): "on",
}

VOICED = dict(zip("かきくけこさしすせそたちつてとはひふへほはひふへほ",
                  "がぎぐげござじずぜぞだぢづでどばびぶべぼぱぴぷぺぽ"))


def hiragana(text):
    return "".join(chr(ord(c) - 0x60) if "ァ" <= c <= "ヶ" else c for c in text)


def on_forms(on):
    """Surface forms an on'yomi takes inside a word: rendaku and sokuon."""
    on = hiragana(on)
    forms = {on, on[:-1] + "っ"}
    for plain, voiced in VOICED.items():
        if on[0] == plain:
            forms |= {voiced + on[1:], voiced + on[1:-1] + "っ"}
    return forms


def load_kanjidic_on():
    on_by_kanji = {}
    with gzip.open(KANJIDIC_PATH) as f:
        for _, el in ET.iterparse(f):
            if el.tag != "character":
                continue
            on_by_kanji[el.findtext("literal")] = [
                r.text for r in el.iter("reading") if r.get("r_type") == "ja_on"
            ]
            el.clear()
    return on_by_kanji


def load_common_pairs():
    """(kanji form, reading) pairs JMdict marks common on both sides."""
    pairs = set()
    with gzip.open(JMDICT_PATH) as f:
        for _, el in ET.iterparse(f):
            if el.tag != "entry":
                continue
            kebs = [(k.findtext("keb"), bool({p.text for p in k.findall("ke_pri")} & COMMON_TAGS))
                    for k in el.findall("k_ele")]
            for r in el.findall("r_ele"):
                if r.find("re_nokanji") is not None:
                    continue
                reb = r.findtext("reb")
                reading_common = bool({p.text for p in r.findall("re_pri")} & COMMON_TAGS)
                restrictions = {x.text for x in r.findall("re_restr")}
                for keb, kanji_common in kebs:
                    if kanji_common and reading_common and (not restrictions or keb in restrictions):
                        pairs.add((keb, reb))
            el.clear()
    return pairs


kanjidic_on = load_kanjidic_on()


def matched_on(kanji, reading):
    for on in kanjidic_on.get(kanji, []):
        if reading in on_forms(on):
            return on
    return None


# Alignment lookup, and per kanji how many common words use each on'yomi.
common_pairs = load_common_pairs()
alignment = {}
on_usage = defaultdict(Counter)
with open(FURIGANA_PATH, encoding="utf-8-sig") as f:
    for entry in json.load(f):
        key = (entry["text"], entry["reading"])
        alignment.setdefault(key, entry["furigana"])
        if key not in common_pairs:
            continue
        for segment in entry["furigana"]:
            ruby, rt = segment.get("ruby", ""), segment.get("rt")
            if rt and len(ruby) == 1 and KANJI.match(ruby):
                on = matched_on(ruby, rt)
                if on:
                    on_usage[ruby][on] += 1


def common_on(kanji):
    return [on for on, _ in on_usage[kanji].most_common(MAX_COMMON_ON)]


def segments_for(word, kana):
    if (word, kana) in MANUAL:
        return MANUAL[(word, kana)]
    candidates = [(word, kana)]
    if word.endswith("する") and kana.endswith("する"):
        candidates.append((word[:-2], kana[:-2]))
    for prefix in ("お", "ご"):
        if kana.startswith(prefix):
            candidates.append((word[1:] if word.startswith(prefix) else word, kana[1:]))
    for candidate in candidates:
        if candidate in alignment:
            return [(s.get("ruby", ""), s.get("rt")) for s in alignment[candidate]]
    return None


def kanji_entry(word, kanji, reading):
    on = matched_on(kanji, reading)
    entry = {
        "text": kanji,
        "reading": reading,
        "type": TYPE_OVERRIDE.get((word, kanji), "on" if on else "kun"),
    }
    ons = common_on(kanji)
    if on and on not in ons:
        ons.append(on)
    if ons:
        entry["on"] = ons
    return entry


def readings_for(word, kana):
    segments = segments_for(word, kana)
    if segments is None:
        return None
    readings, seen = [], set()
    for text, reading in segments:
        if not reading or not KANJI.search(text) or (text, reading) in seen:
            continue
        seen.add((text, reading))
        if len(text) == 1:
            readings.append(kanji_entry(word, text, reading))
        elif len(text) == 2 and text[1] == "々":
            # 人々 ひとびと: the repeat mark doubles the kanji's reading.
            readings.append(kanji_entry(word, text[0], reading[: len(reading) // 2]))
        else:
            readings.append({"text": text, "reading": reading, "type": "jukuji"})
    return readings


counts, unaligned = Counter(), []
for level in ("n3", "n4", "n5"):
    path = os.path.join(SEED_DIR, f"{level}.json")
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    for material in data["materials"]:
        payload = material["payload"]
        if material["category"] != "vocabulary" or not KANJI.search(payload["kanji"]):
            continue
        readings = readings_for(payload["kanji"], payload["kana"])
        if not readings:
            unaligned.append(f'{payload["kanji"]}({payload["kana"]})')
            continue
        counts["words"] += 1
        counts.update(r["type"] for r in readings)
        rebuilt = {k: v for k, v in payload.items() if k not in ("kanji_readings", "kanji_notes")}
        rebuilt["kanji_readings"] = readings
        if "kanji_notes" in payload:
            rebuilt["kanji_notes"] = payload["kanji_notes"]
        material["payload"] = rebuilt
    if WRITE:
        with open(path, "w", encoding="utf-8") as f:
            f.write(json.dumps(data, ensure_ascii=False, indent=2) + "\n")

print(dict(counts))
print(f"unaligned {len(unaligned)}: " + " / ".join(unaligned))
