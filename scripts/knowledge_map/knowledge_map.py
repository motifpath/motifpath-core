"""Compiles the reviewed knowledge map (motifpath-specs catalogs/knowledge-map.yaml)
into the frozen migrations that install it, with the catalog instruments it scopes
nodes to. Deterministic and offline; needs PyYAML.

Usage: python3 knowledge_map.py [--specs ../../../motifpath-specs]
Then run `atlas migrate hash` on the migrations directory.
"""
import argparse
import json
import re
import uuid
from dataclasses import dataclass, field
from pathlib import Path

import yaml

NAMESPACE = uuid.UUID('4ac75155-7804-5527-a6ba-01b73c0e3e1a')
LANGUAGES = ('en', 'pt_BR')
KEY = re.compile(r'^[a-z0-9]+(-[a-z0-9]+)*$')
LEVELS = ('accurate', 'fluent', 'retained')
INSTALLED_AT = '2026-10-02T00:00:00Z'

# The catalog instruments, by key. Their IDs derive from the key, so renaming one
# never changes its ID.
INSTRUMENTS = {
    'guitar': dict(names={'en': 'Acoustic guitar', 'pt_BR': 'Violão'}, tuning=['E2', 'A2', 'D3', 'G3', 'B3', 'E4'], voice='acoustic-guitar', icon='acoustic_guitar'),
    'electric-guitar': dict(names={'en': 'Electric guitar', 'pt_BR': 'Guitarra elétrica'}, tuning=['E2', 'A2', 'D3', 'G3', 'B3', 'E4'], voice='acoustic-guitar', icon='electric_guitar'),
    'electric-bass': dict(names={'en': 'Electric bass', 'pt_BR': 'Contrabaixo elétrico'}, tuning=['E1', 'A1', 'D2', 'G2'], voice='electric-bass', icon='electric_bass'),
}

MIGRATIONS_DIR = Path(__file__).resolve().parents[2] / 'services/core-domain/internal/adapters/repo/ent/migrate/migrations'
INSTRUMENTS_FILE = '20261002123300_catalog_instruments.up.sql'
# The icon column came later than the instruments, so their icons install after it.
INSTRUMENT_ICONS_FILE = '20261004165340_catalog_instrument_icons.up.sql'
MAP_FILE = '20261002123400_knowledge_map.up.sql'


def stable_id(name):
    return str(uuid.uuid5(NAMESPACE, name))


def instrument_id(key):
    return stable_id('instrument/' + key)


@dataclass
class Node:
    key: str
    kind: str
    names: dict
    parent: str | None
    instruments: list = field(default_factory=list)  # instrument keys; empty = every instrument

    @property
    def id(self):
        return stable_id('knowledge-node/' + self.key)

    @property
    def instrument_ids(self):
        return [instrument_id(k) for k in self.instruments]


@dataclass
class Edge:
    type: str
    source: str
    target: str
    level: str | None = None

    @property
    def id(self):
        return stable_id(f'knowledge-edge/{self.type}/{self.source}/{self.target}')


@dataclass
class Graph:
    nodes: list
    edges: list


def wider(child, parent):
    """Whether a node for child instruments reaches beyond parent's; empty = every instrument."""
    if not parent:
        return False
    return not child or not set(child) <= set(parent)


def share_instrument(a, b):
    return not a or not b or bool(set(a) & set(b))


def build(raw):
    """Validates raw (the parsed YAML) against the map's rules and returns its
    nodes, parents before children, and its edges, applies before requires."""
    aliases = raw.get('instruments') or {}
    for alias, keys in aliases.items():
        for key in keys:
            if key not in INSTRUMENTS:
                raise ValueError(f'alias {alias!r} names unknown instrument {key!r}')

    nodes = {}
    for kind, entries in (('concept', raw.get('concepts') or []), ('skill', raw.get('skills') or [])):
        for entry in entries:
            key = entry.get('key', '')
            if not KEY.match(key) or len(key) > 100:
                raise ValueError(f'{key!r} is not a lowercase kebab-case key of at most 100 characters')
            if key in nodes:
                raise ValueError(f'{key!r} appears more than once')
            names = entry.get('names') or {}
            for lang in LANGUAGES:
                if not str(names.get(lang, '')).strip() or len(names[lang]) > 200:
                    raise ValueError(f'{key!r} needs a {lang} name of at most 200 characters')
            if set(names) != set(LANGUAGES):
                raise ValueError(f'{key!r} has names outside {LANGUAGES}')
            scope = entry.get('instruments')
            if scope == 'all':
                instruments = []
            elif scope in aliases:
                instruments = list(aliases[scope])
            else:
                raise ValueError(f'{key!r} has unknown instruments {scope!r}')
            if kind == 'concept' and entry.get('applies'):
                raise ValueError(f'concept {key!r} cannot apply anything')
            nodes[key] = (Node(key, kind, {lang: names[lang] for lang in LANGUAGES}, entry.get('parent'), instruments), entry.get('applies') or [])

    for key, (n, _) in nodes.items():
        if n.parent is None:
            continue
        if n.parent not in nodes:
            raise ValueError(f'{key!r} has unknown parent {n.parent!r}')
        parent = nodes[n.parent][0]
        if parent.kind != n.kind:
            raise ValueError(f'{key!r} is a {n.kind} under {parent.key!r} of another kind')
        if wider(n.instruments, parent.instruments):
            raise ValueError(f'{key!r} is for instruments wider than its parent {parent.key!r}')

    ordered, placed = [], set()

    def place(key, path=()):
        if key in placed:
            return
        if key in path:
            raise ValueError(f'parent cycle through {key!r}')
        n = nodes[key][0]
        if n.parent:
            place(n.parent, path + (key,))
        placed.add(key)
        ordered.append(n)

    for key in nodes:
        place(key)

    edges = []
    for key, (n, applies) in nodes.items():
        for target in applies:
            if target not in nodes or nodes[target][0].kind != 'concept':
                raise ValueError(f'{key!r} applies {target!r}, which is not a concept')
            edges.append(Edge('applies', key, target))

    requires = {}
    for entry in raw.get('requires') or []:
        source, target, level = entry.get('from'), entry.get('to'), entry.get('level')
        for end in (source, target):
            if end not in nodes:
                raise ValueError(f'requires names unknown node {end!r}')
        if source == target:
            raise ValueError(f'{source!r} requires itself')
        if level not in LEVELS:
            raise ValueError(f'requires {source!r} → {target!r} needs a level in {LEVELS}')
        if not share_instrument(nodes[source][0].instruments, nodes[target][0].instruments):
            raise ValueError(f'requires {source!r} → {target!r}: the two nodes share no instrument')
        if (source, target) in requires:
            raise ValueError(f'requires {source!r} → {target!r} appears more than once')
        requires[(source, target)] = level
        edges.append(Edge('requires', source, target, level))

    graph = {}
    for source, target in requires:
        graph.setdefault(source, []).append(target)
    state = {}

    def visit(key):
        state[key] = 'open'
        for target in graph.get(key, []):
            if state.get(target) == 'open':
                raise ValueError(f'requires cycle through {key!r} and {target!r}')
            if target not in state:
                visit(target)
        state[key] = 'done'

    for key in graph:
        if key not in state:
            visit(key)
    return Graph(ordered, edges)


def load(path):
    with open(path, encoding='utf-8') as f:
        return build(yaml.safe_load(f))


def sql_text(value):
    return "'" + value.replace("'", "''") + "'"


def compact(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'))


def render_instruments_sql():
    rows = [f"  -- {key}\n  ({sql_text(instrument_id(key))}, {sql_text(compact(i['names']))}, 'fretted', {len(i['tuning'])}, "
            f"{sql_text(compact(i['tuning']))}, {sql_text(i['voice'])})"
            for key, i in INSTRUMENTS.items()]
    return ('-- Frozen catalog instruments. Each id is the UUID v5 of instrument/<key>, the same in every environment.\n'
            '-- a row already holding one of these ids stops the migration.\n'
            'INSERT INTO "instruments" ("id", "names", "family", "string_count", "tuning", "default_voice_id") VALUES\n'
            + ',\n'.join(rows) + ';\n')


def render_instrument_icons_sql():
    rows = [f'UPDATE "instruments" SET "icon" = {sql_text(i["icon"])} WHERE "id" = {sql_text(instrument_id(key))};'
            for key, i in INSTRUMENTS.items()]
    return ('-- Frozen catalog instrument icons: the keys clients draw their pictures for, by the\n'
            '-- instruments\' fixed ids.\n' + '\n'.join(rows) + '\n')


def render_sql(graph):
    out = ['-- Frozen knowledge map, compiled from motifpath-specs catalogs/knowledge-map.yaml by',
           '-- scripts/knowledge_map. Each id is the UUID v5 of knowledge-node/<key> or',
           '-- knowledge-edge/<type>/<from key>/<to key>. A key or id already in use stops the migration.']
    out.append('INSERT INTO "knowledge_nodes" ("id", "kind", "key", "names", "parent_id") VALUES')
    rows = []
    for n in graph.nodes:
        parent = sql_text(stable_id('knowledge-node/' + n.parent)) if n.parent else 'NULL'
        rows.append(f"  ({sql_text(n.id)}, '{n.kind}', {sql_text(n.key)}, {sql_text(compact(n.names))}, {parent})")
    out.append(',\n'.join(rows) + ';')
    links = [(n, i) for n in graph.nodes for i in n.instruments]
    out.append('INSERT INTO "knowledge_node_instruments" ("knowledge_node_id", "instrument_id", "linked_at") VALUES')
    out.append(',\n'.join(f"  ({sql_text(n.id)}, {sql_text(instrument_id(i))}, '{INSTALLED_AT}')" for n, i in links) + ';')
    out.append('INSERT INTO "knowledge_edges" ("id", "type", "from_id", "to_id", "level") VALUES')
    out.append(',\n'.join(
        f"  ({sql_text(e.id)}, '{e.type}', {sql_text(stable_id('knowledge-node/' + e.source))}, "
        f"{sql_text(stable_id('knowledge-node/' + e.target))}, {sql_text(e.level) if e.level else 'NULL'})"
        for e in graph.edges) + ';')
    return '\n'.join(out) + '\n'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--specs', type=Path, default=Path(__file__).resolve().parents[3] / 'motifpath-specs')
    parser.add_argument('--migrations', type=Path, default=MIGRATIONS_DIR)
    args = parser.parse_args()
    graph = load(args.specs / 'catalogs/knowledge-map.yaml')
    (args.migrations / INSTRUMENTS_FILE).write_text(render_instruments_sql())
    (args.migrations / INSTRUMENT_ICONS_FILE).write_text(render_instrument_icons_sql())
    (args.migrations / MAP_FILE).write_text(render_sql(graph))
    kinds = [n.kind for n in graph.nodes]
    types = [e.type for e in graph.edges]
    print(json.dumps(dict(skills=kinds.count('skill'), concepts=kinds.count('concept'),
                          applies=types.count('applies'), requires=types.count('requires'))))


if __name__ == '__main__':
    main()
