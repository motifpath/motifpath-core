"""Compiles the practice drill catalog (motifpath-specs catalogs/practice-drills.yaml)
into the SQL that installs its drill templates, its timed thresholds and the
fretboard cell ranges each skill's generated cells come from. Deterministic and
offline; needs PyYAML. Cell ranges are checked against the knowledge map
(catalogs/knowledge-map.yaml) and the catalog instruments.
scripts/reference_data/build.py writes that SQL into the baseline_reference_data
migration, keeping the threshold versions it already installs; run on its own,
this prints the catalog's counts.

Usage: python3 practice_drills.py [--specs ../../../motifpath-specs]
"""
import argparse
import datetime
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
TEMPLATE_KEY = re.compile(r'^[a-z][a-z_]*:[a-z][a-z_]*$')
SOURCES = ('default', 'benchmark', 'calibrated')



def stable_id(name):
    return str(uuid.uuid5(NAMESPACE, name))


def template_id(key):
    return stable_id('drill-template/' + key)


def threshold_id(template_key, version):
    return stable_id(f'drill-threshold/{template_key}/v{version}')


@dataclass
class Template:
    key: str
    item_kind: str
    response_type: str
    timed: bool
    names: dict

    @property
    def id(self):
        return template_id(self.key)


@dataclass
class Threshold:
    template: str
    version: int
    effective_from: datetime.date
    fluent_net_ms: int
    source: str
    sessions: int
    students: int

    @property
    def id(self):
        return threshold_id(self.template, self.version)


@dataclass
class Drills:
    templates: list
    thresholds: list


def build(raw, installed=frozenset(), today=None):
    """Validates raw (the parsed YAML) against the catalog's rules and returns its
    templates in catalog order and its thresholds by template, then version.

    installed holds the ids of the versions the frozen migration already installs. Any
    other version must take effect today or later: answers are judged by the version in
    force when they were given and items are rebuilt from their evidence, so a version
    that started before it was installed would re-judge answers already folded."""
    templates = {}
    for entry in raw.get('templates') or []:
        key = entry.get('key', '')
        if not TEMPLATE_KEY.match(key):
            raise ValueError(f'{key!r} is not a <item kind>:<name> template key')
        if key in templates:
            raise ValueError(f'template {key!r} appears more than once')
        names = entry.get('names') or {}
        if set(names) != set(LANGUAGES) or not all(str(names[lang]).strip() for lang in LANGUAGES):
            raise ValueError(f'template {key!r} needs exactly a name in each of {LANGUAGES}')
        if not isinstance(entry.get('timed'), bool):
            raise ValueError(f'template {key!r} needs timed: true or false')
        t = Template(key, entry.get('item_kind', ''), entry.get('response_type', ''), entry['timed'],
                     {lang: names[lang] for lang in LANGUAGES})
        if not t.item_kind or not key.startswith(t.item_kind + ':') or not t.response_type:
            raise ValueError(f'template {key!r} needs an item_kind its key starts with, and a response_type')
        if entry.get('id') != t.id:
            raise ValueError(f'template {key!r} has id {entry.get("id")!r}, not {t.id!r}')
        templates[key] = t

    by_template = {}
    for entry in raw.get('thresholds') or []:
        key = entry.get('template')
        if key not in templates:
            raise ValueError(f'a threshold names unknown template {key!r}')
        if not templates[key].timed:
            raise ValueError(f'template {key!r} is not timed, so it has no fluent time')
        effective = entry.get('effective_from')
        if isinstance(effective, str):
            effective = datetime.date.fromisoformat(effective)
        if not isinstance(effective, datetime.date):
            raise ValueError(f'a threshold of {key!r} needs an effective_from date')
        t = Threshold(key, entry.get('version'), effective, entry.get('fluent_net_ms'), entry.get('source'),
                      entry.get('sessions', 0), entry.get('students', 0))
        if not isinstance(t.fluent_net_ms, int) or t.fluent_net_ms <= 0:
            raise ValueError(f'{key!r} v{t.version} needs a positive fluent_net_ms')
        if t.source not in SOURCES:
            raise ValueError(f'{key!r} v{t.version} has source {t.source!r}, not one of {SOURCES}')
        if not all(isinstance(n, int) and n >= 0 for n in (t.sessions, t.students)):
            raise ValueError(f'{key!r} v{t.version} needs sessions and students of 0 or more')
        if entry.get('id') != t.id:
            raise ValueError(f'{key!r} v{t.version} has id {entry.get("id")!r}, not {t.id!r}')
        if today is not None and t.id not in installed and t.effective_from < today:
            raise ValueError(f'{key!r} v{t.version} is new, so its effective_from must be {today} or later')
        by_template.setdefault(key, []).append(t)

    thresholds = []
    for key in templates:
        versions = sorted(by_template.get(key, []), key=lambda t: t.version if isinstance(t.version, int) else 0)
        for expected, t in enumerate(versions, start=1):
            if t.version != expected:
                raise ValueError(f'{key!r} versions must count from 1 without gaps or repeats')
            if expected > 1 and t.effective_from <= versions[expected - 2].effective_from:
                raise ValueError(f'{key!r} v{t.version} effective_from must be later than v{expected - 1}\'s')
        thresholds.extend(versions)
    return Drills(list(templates.values()), thresholds)


@dataclass
class CellRange:
    skill: str
    layout: str
    strings: list
    from_fret: int
    to_fret: int

    @property
    def id(self):
        return stable_id(f'fretboard-cells/{self.skill}/{self.layout}')

    @property
    def cell_count(self):
        return len(self.strings) * (self.to_fret - self.from_fret + 1)


def build_cells(entries, nodes):
    """Validates the catalog's fretboard_cells entries against nodes (knowledge-map
    nodes by key) and the catalog instruments, and returns one range per skill and
    layout, in catalog order. A layout is a catalog instrument; a string must be one
    it has, the skill must suit it, no cell may serve two skills, and a skill lists one
    layout per fretboard geometry."""
    ranges = []
    owner = {}
    for entry in entries or []:
        skill = entry.get('skill')
        node = nodes.get(skill)
        if node is None or node.kind != 'skill':
            raise ValueError(f'fretboard cells name {skill!r}, which is not a skill of the knowledge map')
        geometries = {}
        for layout, spec in (entry.get('layouts') or {}).items():
            instrument = km.INSTRUMENTS.get(layout)
            if instrument is None:
                raise ValueError(f'fretboard cells of {skill!r} name layout {layout!r}, which is not a catalog instrument')
            # Instruments of the same tuning share one fretboard: listing two of them
            # would give each note two item keys, so progress on one never counts for
            # the other.
            twin = geometries.setdefault(tuple(instrument['tuning']), layout)
            if twin != layout:
                raise ValueError(f'fretboard cells of {skill!r} list layouts {twin!r} and {layout!r}, which share one fretboard')
            if node.instruments and layout not in node.instruments:
                raise ValueError(f'skill {skill!r} is not for layout {layout!r}')
            strings = list(spec.get('strings') or [])
            frets = list(spec.get('frets') or [])
            if len(frets) != 2 or not 0 <= frets[0] <= frets[1]:
                raise ValueError(f'fretboard cells of {skill!r} on layout {layout!r} need frets [from, to] in order')
            for string in strings:
                if not isinstance(string, int) or not 1 <= string <= len(instrument['tuning']):
                    raise ValueError(f'layout {layout!r} has no string {string} (skill {skill!r})')
                for fret in range(frets[0], frets[1] + 1):
                    other = owner.setdefault((layout, string, fret), skill)
                    if other != skill:
                        raise ValueError(f'layout {layout!r} string {string} fret {fret} serves both {other!r} and {skill!r}')
            if not strings:
                raise ValueError(f'fretboard cells of {skill!r} on layout {layout!r} list no strings')
            ranges.append(CellRange(skill, layout, strings, frets[0], frets[1]))
    return ranges


def load_cells(path, map_path):
    with open(path, encoding='utf-8') as f:
        entries = (yaml.safe_load(f) or {}).get('fretboard_cells')
    nodes = {n.key: n for n in km.load(map_path).nodes}
    return build_cells(entries, nodes)


def load(path, installed=frozenset(), today=None):
    with open(path, encoding='utf-8') as f:
        return build(yaml.safe_load(f), installed, today)


def installed_threshold_ids(sql):
    """The ids of the threshold versions a rendered migration installs."""
    rows = sql.split('INSERT INTO "drill_thresholds"', 1)
    if len(rows) < 2:
        return set()
    return set(re.findall(r"^\s*\('([0-9a-f-]{36})'", rows[1], re.MULTILINE))


def sql_text(value):
    return "'" + value.replace("'", "''") + "'"


def compact(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'))


def render_sql(drills):
    out = ['-- Frozen practice drill catalog, compiled from motifpath-specs catalogs/practice-drills.yaml by',
           '-- scripts/practice_drills. Each id is the UUID v5 of drill-template/<key> or',
           '-- drill-threshold/<template key>/v<version>. A row already holding one of these ids stops the migration.',
           'INSERT INTO "drill_templates" ("id", "key", "item_kind", "response_type", "timed", "names") VALUES']
    out.append(',\n'.join(
        f"  ({sql_text(t.id)}, {sql_text(t.key)}, {sql_text(t.item_kind)}, {sql_text(t.response_type)}, "
        f"{'true' if t.timed else 'false'}, {sql_text(compact(t.names))})"
        for t in drills.templates) + ';')
    if drills.thresholds:
        out.append('INSERT INTO "drill_thresholds" ("id", "template_id", "version", "effective_from", "fluent_net_ms", '
                   '"source", "sessions", "students") VALUES')
        out.append(',\n'.join(
            f"  ({sql_text(t.id)}, {sql_text(template_id(t.template))}, {t.version}, "
            f"'{t.effective_from.isoformat()}T00:00:00Z', {t.fluent_net_ms}, {sql_text(t.source)}, {t.sessions}, {t.students})"
            for t in drills.thresholds) + ';')
    return '\n'.join(out) + '\n'


def render_cells_sql(ranges):
    out = ['-- Frozen fretboard cell ranges, compiled from motifpath-specs catalogs/practice-drills.yaml by',
           '-- scripts/practice_drills. Each id is the UUID v5 of fretboard-cells/<skill key>/<layout key>.',
           '-- Every string and fret in a range is a generated practice item of its skill.']
    if ranges:
        out.append('INSERT INTO "fretboard_cell_ranges" ("id", "skill_id", "layout_instrument_id", "strings", "from_fret", "to_fret") VALUES')
        out.append(',\n'.join(
            f"  ({sql_text(r.id)}, {sql_text(km.stable_id('knowledge-node/' + r.skill))}, {sql_text(km.instrument_id(r.layout))}, "
            f"{sql_text(compact(r.strings))}, {r.from_fret}, {r.to_fret})"
            for r in ranges) + ';')
    return '\n'.join(out) + '\n'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--specs', type=Path, default=Path(__file__).resolve().parents[3] / 'motifpath-specs')
    args = parser.parse_args()
    drills = load(args.specs / 'catalogs/practice-drills.yaml', set(), datetime.date.today())
    ranges = load_cells(args.specs / 'catalogs/practice-drills.yaml', args.specs / 'catalogs/knowledge-map.yaml')
    print(json.dumps(dict(templates=len(drills.templates), thresholds=len(drills.thresholds),
                          cell_ranges=len(ranges), cells=sum(r.cell_count for r in ranges))))


if __name__ == '__main__':
    main()
