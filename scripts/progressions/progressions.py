"""Compiles the progression catalog (motifpath-specs catalogs/progressions.yaml) into
the SQL that installs its progressions. Deterministic and offline; needs PyYAML.
Each progression's nodes are checked against the knowledge map
(catalogs/knowledge-map.yaml) and stored as their fixed ids.
scripts/reference_data/build.py writes that SQL into the
progressions_reference_data migration; run on its own, this prints the
catalog's counts.

Usage: python3 progressions.py [--specs ../../../motifpath-specs]
"""
import argparse
import json
import re
import sys
import uuid
from dataclasses import dataclass
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'knowledge_map'))
import knowledge_map as km  # noqa: E402  (a sibling script, not a package)

NAMESPACE = uuid.UUID('4ac75155-7804-5527-a6ba-01b73c0e3e1a')
LANGUAGES = ('en', 'pt_BR')
MODES = ('major',)
# A major key's numerals, with an optional 7 for a dominant seventh.
NUMERAL = re.compile(r'^(I|ii|iii|IV|V|vi|vii°)7?$')
# Four bars, or the 12 bars of a blues form.
BAR_COUNTS = (4, 12)


def progression_id(key):
    return str(uuid.uuid5(NAMESPACE, 'progression/' + key))


@dataclass
class Progression:
    key: str
    mode: str
    bars: list
    nodes: list
    confusable: list
    names: dict

    @property
    def id(self):
        return progression_id(self.key)


def build(raw, node_keys):
    """Validates raw (the parsed YAML) against the catalog's rules, its nodes
    against node_keys, and returns its progressions in catalog order."""
    mode = raw.get('mode')
    if mode not in MODES:
        raise ValueError(f'mode {mode!r} is not one of {MODES}')
    built = {}
    for entry in raw.get('progressions') or []:
        key = entry.get('key', '')
        if key in built:
            raise ValueError(f'progression {key!r} appears more than once')
        names = entry.get('names') or {}
        if set(names) != set(LANGUAGES) or not all(str(names[lang]).strip() for lang in LANGUAGES):
            raise ValueError(f'progression {key!r} needs exactly a name in each of {LANGUAGES}')
        p = Progression(key, mode, list(entry.get('bars') or []), list(entry.get('nodes') or []),
                        list(entry.get('confusable') or []), {lang: names[lang] for lang in LANGUAGES})
        for numeral in p.bars:
            if not NUMERAL.match(str(numeral)):
                raise ValueError(f'progression {key!r} has {numeral!r}, not a numeral of a major key')
        if len(p.bars) not in BAR_COUNTS:
            raise ValueError(f'progression {key!r} has {len(p.bars)} bars, not one of {BAR_COUNTS}')
        if not p.nodes:
            raise ValueError(f'progression {key!r} needs at least one node')
        for node in p.nodes:
            if node not in node_keys:
                raise ValueError(f'progression {key!r} names node {node!r}, which the knowledge map lacks')
        if key in p.confusable:
            raise ValueError(f'progression {key!r} is listed as confusable with itself')
        if entry.get('id') != p.id:
            raise ValueError(f'progression {key!r} has id {entry.get("id")!r}, not {p.id!r}')
        built[key] = p
    for p in built.values():
        for other in p.confusable:
            if other not in built:
                raise ValueError(f'progression {p.key!r} is confusable with unknown progression {other!r}')
            if p.key not in built[other].confusable:
                raise ValueError(f'{p.key!r} and {other!r} must list each other as confusable, both ways')
    return list(built.values())


def load(path, map_path):
    with open(path, encoding='utf-8') as f:
        raw = yaml.safe_load(f) or {}
    return build(raw, {n.key for n in km.load(map_path).nodes})


def node_ids(map_path):
    """The fixed id of each knowledge node, by key."""
    return {n.key: n.id for n in km.load(map_path).nodes}


def sql_text(value):
    return "'" + value.replace("'", "''") + "'"


def compact(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'))


def render_sql(progressions, ids):
    """The SQL installing progressions, each node stored as its id from ids."""
    out = ['-- Frozen progression catalog, compiled from motifpath-specs catalogs/progressions.yaml by',
           '-- scripts/progressions. Each id is the UUID v5 of progression/<key>, and node_ids are the',
           '-- knowledge nodes\' fixed ids. A row already holding one of these ids stops the migration.',
           'INSERT INTO "progressions" ("id", "key", "mode", "bars", "node_ids", "confusable", "names") VALUES']
    out.append(',\n'.join(
        f"  ({sql_text(p.id)}, {sql_text(p.key)}, {sql_text(p.mode)}, {sql_text(compact(p.bars))}, "
        f"{sql_text(compact([ids[n] for n in p.nodes]))}, {sql_text(compact(p.confusable))}, {sql_text(compact(p.names))})"
        for p in progressions) + ';')
    return '\n'.join(out) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--specs', default=str(Path(__file__).resolve().parents[3] / 'motifpath-specs'))
    args = parser.parse_args()
    catalogs = Path(args.specs) / 'catalogs'
    built = load(catalogs / 'progressions.yaml', catalogs / 'knowledge-map.yaml')
    print(f'{len(built)} progressions, {sum(len(p.confusable) for p in built) // 2} confusable pairs')


if __name__ == '__main__':
    main()
