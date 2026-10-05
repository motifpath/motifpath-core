"""Compiles the practice drill catalog (motifpath-specs catalogs/practice-drills.yaml)
into the frozen migration that installs its drill templates and timed thresholds.
Deterministic and offline; needs PyYAML. The generated fretboard cells come with the
fretboard drill, so the catalog's fretboard_cells section is not read here.

Usage: python3 practice_drills.py [--specs ../../../motifpath-specs]
Then run `atlas migrate hash` on the migrations directory.
"""
import argparse
import datetime
import json
import re
import uuid
from dataclasses import dataclass
from pathlib import Path

import yaml

NAMESPACE = uuid.UUID('4ac75155-7804-5527-a6ba-01b73c0e3e1a')
LANGUAGES = ('en', 'pt_BR')
TEMPLATE_KEY = re.compile(r'^[a-z][a-z_]*:[a-z][a-z_]*$')
SOURCES = ('default', 'benchmark', 'calibrated')

MIGRATIONS_DIR = Path(__file__).resolve().parents[2] / 'services/core-domain/internal/adapters/repo/ent/migrate/migrations'
DRILLS_FILE = '20261005184400_practice_drills.up.sql'


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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--specs', type=Path, default=Path(__file__).resolve().parents[3] / 'motifpath-specs')
    parser.add_argument('--migrations', type=Path, default=MIGRATIONS_DIR)
    args = parser.parse_args()
    frozen = args.migrations / DRILLS_FILE
    installed = installed_threshold_ids(frozen.read_text()) if frozen.exists() else set()
    drills = load(args.specs / 'catalogs/practice-drills.yaml', installed, datetime.date.today())
    frozen.write_text(render_sql(drills))
    print(json.dumps(dict(templates=len(drills.templates), thresholds=len(drills.thresholds))))


if __name__ == '__main__':
    main()
