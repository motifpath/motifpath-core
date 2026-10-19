"""Generates the chord catalog from motifpath-specs catalogs/chord-voicings.yaml:
every chord definition, every voicing (open voicings as written, templates
materialized per root) and the chord_voicing diagram each voicing plays.

Every voicing passes the musical validator before anything is emitted, and a
voicing that fails aborts the build, so a wrong fingering never becomes a
migration. A voicing is accepted only when its sounded pitch classes are the
chord's formula less the tones it declares omitted (a slash bass may sound
too), the root always sounds, only the quality's omittable tones are left
out, a slash chord's lowest string sounds its bass, every fret is between 0
and 15, a movable shape sounds no open string, and no other voicing of the
chord plays the same frets.

Usage: python chord_catalog.py --output catalog/chord-voicings-v1 [--specs ../../../motifpath-specs]
"""
import argparse
import hashlib
import json
import sys
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'diagram_catalog'))
import catalog as basic  # noqa: E402  (a sibling script, not a package)

stable_id = basic.stable_id
pitch_class = basic.pitch_class
spell = basic.spell
sql_text = basic.sql_text
compact = basic.compact
batches = basic.batches

TUNING_FINGERPRINT = 'E2-A2-D3-G3-B3-E4'
MAX_FRET = 15
INSTALLED_AT = "'2026-10-06T00:00:00Z'"

# Each quality's canonical suffix and its other accepted ones. They must
# match what the chord symbol parser accepts, or a catalog alias would be a
# spelling no search finds.
SUFFIXES = {
    'major': ['', 'M', 'maj'], 'minor': ['m', 'min', '-'], 'power': ['5'],
    'diminished': ['dim', '°'], 'augmented': ['aug', '+'], 'sus2': ['sus2'], 'sus4': ['sus4', 'sus'],
    'major_6': ['6', 'M6', 'maj6'], 'minor_6': ['m6', 'min6', '-6'], 'dominant_7': ['7'],
    'major_7': ['maj7', 'M7', 'Δ7', 'Δ', 'ma7'], 'minor_7': ['m7', 'min7', '-7'],
    'minor_major_7': ['mMaj7', 'mM7', 'm(maj7)', 'minMaj7', 'mΔ7', '-Δ7'],
    'half_diminished_7': ['m7b5', 'm7(b5)', '-7b5', 'ø', 'ø7'], 'diminished_7': ['dim7', '°7'],
    'dominant_7_sus4': ['7sus4', '7sus'], 'add_9': ['add9', '(add9)'], 'minor_add_9': ['madd9', 'm(add9)'],
    'dominant_9': ['9'], 'major_9': ['maj9', 'M9', 'Δ9'], 'minor_9': ['m9', 'min9', '-9'],
    'dominant_11': ['11'], 'minor_11': ['m11', 'min11', '-11'], 'dominant_13': ['13'],
    'dominant_7_flat_5': ['7b5', '7(b5)'], 'dominant_7_sharp_5': ['7#5', '7(#5)', '7+5', '+7', 'aug7'],
    'dominant_7_flat_9': ['7b9', '7(b9)'], 'dominant_7_sharp_9': ['7#9', '7(#9)'],
}
SUFFIX = {quality: suffixes[0] for quality, suffixes in SUFFIXES.items()}
QUALITY_BY_SUFFIX = {suffix: quality for quality, suffixes in SUFFIXES.items() for suffix in suffixes}

# Semitones above the root of each formula interval, within one octave.
SEMITONES = {'R': 0, 'b9': 1, '2': 2, '9': 2, 'b3': 3, '#9': 3, '3': 4, '4': 5, '11': 5, 'b5': 6,
             '5': 7, '#5': 8, '6': 9, 'bb7': 9, '13': 9, 'b7': 10, '7': 11}
# The interval code of each simple interval a slash bass may lie at, by
# degree above the root and semitones (diagram position codes).
SIMPLE_INTERVALS = {(2, 1): 'b2', (2, 2): '2', (3, 3): 'b3', (3, 4): '3', (4, 5): '4', (4, 6): '#4',
                    (5, 6): 'b5', (5, 7): '5', (5, 8): '#5', (6, 8): 'b6', (6, 9): '6',
                    (7, 9): 'bb7', (7, 10): 'b7', (7, 11): '7'}

QUALITY_SKILL = {
    'major': 'major-triads', 'minor': 'minor-triads', 'power': 'power-chord',
    'diminished': 'diminished-augmented-triads', 'augmented': 'diminished-augmented-triads',
    'sus2': 'sus-add-chords', 'sus4': 'sus-add-chords', 'dominant_7_sus4': 'sus-add-chords',
    'add_9': 'sus-add-chords', 'minor_add_9': 'sus-add-chords',
    'major_6': 'sixth-ninth-chords', 'minor_6': 'sixth-ninth-chords',
    'dominant_7': 'dominant-seventh', 'dominant_7_flat_5': 'dominant-seventh', 'dominant_7_sharp_5': 'dominant-seventh',
    'dominant_7_flat_9': 'dominant-seventh', 'dominant_7_sharp_9': 'dominant-seventh',
    'major_7': 'seventh-chords', 'minor_7': 'seventh-chords', 'minor_major_7': 'seventh-chords',
    'half_diminished_7': 'seventh-chords', 'diminished_7': 'diminished-seventh',
    'dominant_9': 'extensions', 'major_9': 'extensions', 'minor_9': 'extensions',
    'dominant_11': 'extensions', 'minor_11': 'extensions', 'dominant_13': 'extensions',
}
SHAPE_NAMES = {
    'e_shape': ('E shape', 'forma de Mi'),
    'a_shape': ('A shape', 'forma de Lá'),
    'd_shape': ('D shape', 'forma de Ré'),
}
DIFFICULTY_ORDER = {'beginner': 0, 'intermediate': 1, 'advanced': 2}


def load(path):
    with open(path, encoding='utf-8') as f:
        return yaml.safe_load(f)


def parse_symbol(symbol):
    """The root, quality and bass of a canonical catalog symbol."""
    head, _, bass = symbol.partition('/')
    root = head[:2] if len(head) > 1 and head[1] in 'b#' else head[:1]
    return root, QUALITY_BY_SUFFIX[head[len(root):]], bass or None


def open_pitch_class(string):
    """String 1 is the highest-pitched, the last tuning entry."""
    return basic.TUNING[6 - string] % 12


def interval_above(root, note):
    """The interval code of note's distance above root, spelled by degree."""
    degree = (basic.LETTERS.index(note[0]) - basic.LETTERS.index(root[0])) % 7 + 1
    return 'R' if degree == 1 else SIMPLE_INTERVALS[(degree, (pitch_class(note) - pitch_class(root)) % 12)]


def validate_voicing(label, chord, formula, frets, declared, movable):
    """The musical validator: raises ValueError naming label unless frets
    (string -> fret) play chord exactly. Returns the omitted intervals."""
    root, bass = chord['root'], chord['bass']
    want = {(pitch_class(root) + SEMITONES[i]) % 12: i for i in formula['intervals']}
    allowed = set(want) | ({pitch_class(bass)} if bass else set())
    if not frets:
        raise ValueError(f'{label}: sounds no string')
    sounded = {(open_pitch_class(s) + f) % 12 for s, f in frets.items()}
    for s, f in frets.items():
        if not 0 <= f <= MAX_FRET:
            raise ValueError(f'{label}: fret {f} on string {s} is outside 0-{MAX_FRET}')
        if movable and f == 0:
            raise ValueError(f'{label}: a movable voicing sounds no open string')
    if sounded - allowed:
        raise ValueError(f'{label}: sounds a note that is not in {chord["canonical_symbol"]}')
    missing = [i for p, i in want.items() if p not in sounded]
    missing.sort(key=formula['intervals'].index)
    if 'R' in missing:
        raise ValueError(f'{label}: does not sound the root')
    if any(i not in formula['omittable'] for i in missing):
        raise ValueError(f'{label}: leaves out {missing}, which {chord["canonical_symbol"]} needs')
    if declared is not None and sorted(declared) != sorted(missing):
        raise ValueError(f'{label}: declares omits {declared} but leaves out {missing}')
    lowest = max(frets)
    if bass and (open_pitch_class(lowest) + frets[lowest]) % 12 != pitch_class(bass):
        raise ValueError(f'{label}: the lowest sounded string is not the bass {bass}')
    return missing


def chord_definition(symbol, formulas):
    root, quality, bass = parse_symbol(symbol)
    aliases = [root + s for s in SUFFIXES[quality][1:]]
    aliases += [root.replace('b', '♭').replace('#', '♯') + s for s in SUFFIXES[quality] if ('b' in root or '#' in root)]
    if bass:
        aliases = [a + '/' + bass for a in aliases]
    return dict(chord_definition_id=stable_id('chord-definition/' + symbol), canonical_symbol=symbol,
                root=root, root_pitch_class=pitch_class(root), quality=quality,
                formula=formulas[quality]['intervals'], omittable=formulas[quality]['omittable'],
                bass=bass, bass_pitch_class=pitch_class(bass) if bass else None,
                aliases=aliases)


def materialize(template, root):
    """The template's frets for root, or None when the shape doesn't fit
    below MAX_FRET."""
    offsets = {int(s): v[0] for s, v in template['strings'].items()}
    fret = (pitch_class(root) - open_pitch_class(template['root_string'])) % 12
    while fret < 1 or fret + min(offsets.values()) < 1:
        fret += 12
    frets = {s: fret + o for s, o in offsets.items()}
    if max(frets.values()) > MAX_FRET:
        return None, fret
    return frets, fret


def shape_names(symbol, template, root_fret):
    if template is None:
        return dict(en=f'{symbol} — open', pt_BR=f'{symbol} — aberto')
    family = template.get('shape_family')
    if family in SHAPE_NAMES:
        en, pt = SHAPE_NAMES[family]
        return dict(en=f'{symbol} — {en}, fret {root_fret}', pt_BR=f'{symbol} — {pt}, casa {root_fret}')
    string = template['root_string']
    return dict(en=f'{symbol} — root on string {string}, fret {root_fret}',
                pt_BR=f'{symbol} — tônica na {string}ª corda, casa {root_fret}')


def voicing_diagram(voicing_name, chord, names, strings, skills):
    """The chord_voicing diagram that holds a voicing's positions and plays it."""
    key = 'chord-diagram/' + voicing_name
    root = chord['root']
    by_pc = {(pitch_class(root) + SEMITONES[i]) % 12: i for i in chord['formula']}
    positions = []
    for s in sorted(strings, reverse=True):
        fret = strings[s][0]
        pc = (open_pitch_class(s) + fret) % 12
        if pc in by_pc:
            interval = by_pc[pc]
            note = spell(root, interval)
        else:
            note, interval = chord['bass'], interval_above(root, chord['bass'])
        positions.append(dict(position_id=stable_id(f'{key}/string-{s}'), interval=interval, note_name=note,
                              shape='star' if interval == 'R' else 'dot', color='#EF4444' if interval == 'R' else None,
                              string=s, fret=fret, custom_label=None, note=None))
    playbacks = basic.shape_playbacks(key, positions, 'chord')
    return dict(key=key, diagram_id=stable_id(key), purpose='chord_voicing', names=names, root_note=root,
                mode=None, label_display='interval', color='#3B82F6', positions=positions, regions=[],
                playbacks=playbacks, default_playback_id=playbacks[0]['playback_id'],
                instruments=['guitar', 'electric-guitar'], skills=sorted(skills), concepts=['chords'])


def make_voicing(label, chord, frets, fingers, spec_entry, template, root_fret):
    """One voicing and its diagram, after the validator accepts it."""
    formula = dict(intervals=chord['formula'], omittable=chord['omittable'])
    movable = template is not None
    omitted = validate_voicing(label, chord, formula, frets, None if movable else spec_entry.get('omits', []), movable)
    name = 'chord-voicing/' + label
    tags = list(spec_entry.get('technique_tags', []))
    if not movable:
        tags = ['open'] + [t for t in tags if t != 'open']
    skills = {'chord-diagrams', QUALITY_SKILL[chord['quality']]}
    if not movable:
        skills.add('open-chord-shapes')
    if 'barre' in tags:
        skills.add('barre-chord-shapes')
    if chord['bass']:
        skills.add('slash-chords')
    diagram = voicing_diagram(name, chord, shape_names(chord['canonical_symbol'], template, root_fret), {s: [f] for s, f in frets.items()}, skills)
    by_string = {p['string']: p['position_id'] for p in diagram['positions']}
    fretted = [f for f in frets.values() if f > 0]
    voicing = dict(key=label, chord_voicing_id=stable_id(name), chord_definition_id=chord['chord_definition_id'],
                   diagram_id=diagram['diagram_id'], tuning_fingerprint=TUNING_FINGERPRINT,
                   lowest_fret=min(fretted) if fretted else 0, highest_fret=max(fretted) if fretted else 0,
                   fingering=[dict(position_id=by_string[s], finger=fingers[s]) for s in sorted(fingers, reverse=True) if frets[s] > 0],
                   muted_strings=[s for s in range(6, 0, -1) if s not in frets], omitted_intervals=omitted,
                   difficulty=spec_entry['difficulty'], technique_tags=tags,
                   shape_family=template.get('shape_family') if movable else 'open', is_movable=movable,
                   template_key=template['key'] if movable else None)
    return voicing, diagram


def generate(spec):
    """The whole catalog: chords, voicings ranked within each chord, and the
    voicings' diagrams. Raises ValueError on the first rule a voicing breaks."""
    if spec['tuning'] != ['E2', 'A2', 'D3', 'G3', 'B3', 'E4']:
        raise ValueError('the chord catalog covers standard-tuning six-string guitar only')
    if set(spec['formulas']) != set(SUFFIXES):
        raise ValueError('formulas must define exactly the supported qualities')
    symbols = [root + SUFFIX[q] for q in SUFFIXES for root in spec['roots']]
    symbols += sorted({o['chord'] for o in spec['open_voicings'] if '/' in o['chord']})
    chords = {s: chord_definition(s, spec['formulas']) for s in symbols}
    pitch_keys = [(c['root_pitch_class'], c['quality'], c['bass_pitch_class']) for c in chords.values()]
    if len(pitch_keys) != len(set(pitch_keys)):
        raise ValueError('two chords share a root, quality and bass')

    pairs = []
    for o in spec['open_voicings']:
        if o['chord'] not in chords:
            raise ValueError(f'{o["key"]}: {o["chord"]} is not a catalog chord')
        frets = {int(s): v[0] for s, v in o['strings'].items()}
        fingers = {int(s): v[1] for s, v in o['strings'].items() if len(v) > 1}
        pairs.append(make_voicing(o['key'], chords[o['chord']], frets, fingers, o, None, None))
    for t in spec['templates']:
        offsets = {int(s): v for s, v in t['strings'].items()}
        if offsets.get(t['root_string'], [None])[0] != 0:
            raise ValueError(f'{t["key"]}: the root string must sit at offset 0')
        for root in spec['roots']:
            frets, root_fret = materialize(t, root)
            if frets is None:
                continue
            fingers = {s: v[1] for s, v in offsets.items()}
            pairs.append(make_voicing(f'{t["key"]}/{root}', chords[root + SUFFIX[t['quality']]], frets, fingers, t, t, root_fret))

    seen = set()
    for voicing, diagram in pairs:
        signature = (voicing['chord_definition_id'], tuple(sorted((p['string'], p['fret']) for p in diagram['positions'])))
        if signature in seen:
            raise ValueError(f'{voicing["key"]}: duplicates another voicing of its chord')
        seen.add(signature)
    for chord in chords.values():
        if '/' not in chord['canonical_symbol'] and not any(v['chord_definition_id'] == chord['chord_definition_id'] for v, _ in pairs):
            raise ValueError(f'{chord["canonical_symbol"]} has no voicing')

    by_chord = {}
    for voicing, _ in pairs:
        by_chord.setdefault(voicing['chord_definition_id'], []).append(voicing)
    for ranked in by_chord.values():
        ranked.sort(key=lambda v: (v['is_movable'], DIFFICULTY_ORDER[v['difficulty']], v['lowest_fret'], v['key']))
        for rank, voicing in enumerate(ranked, start=1):
            voicing['recommended_rank'] = rank
    diagrams = [d for _, d in pairs]
    for d in diagrams:
        validate_diagram(d)
    return dict(chords=list(chords.values()), voicings=[v for v, _ in pairs], diagrams=diagrams)


def validate_diagram(d):
    """Every position sounds the note and interval it is labelled with, the
    names and playbacks are complete in en and pt_BR, and every playback step
    names the diagram's own positions."""
    if set(d['names']) != {'en', 'pt_BR'} or any(not n or len(n) > 200 for n in d['names'].values()):
        raise ValueError(f'{d["key"]}: incomplete names')
    for p in d['positions']:
        sounded = (basic.TUNING[6 - p['string']] + p['fret']) % 12
        if p['interval'] not in basic.INTERVALS:
            raise ValueError(f'{d["key"]}: unsupported interval {p["interval"]}')
        if sounded != pitch_class(p['note_name']) or sounded != (pitch_class(d['root_note']) + basic.semitones(p['interval'])) % 12:
            raise ValueError(f'{d["key"]}: string {p["string"]} is mislabelled')
    basic.validate_playbacks(d, {p['position_id'] for p in d['positions']})


def render_sql(catalog):
    """The SQL that installs the catalog after the chord catalog's tables."""
    profile = sql_text(basic.SYSTEM_CATALOG_USER_ID)
    sql = ['-- Frozen chord catalog. The fixed system profile owns every chord voicing diagram.',
           '-- Each assertion deliberately divides by zero when its precondition is false.',
           f'SELECT 1 / (SELECT CASE WHEN EXISTS (SELECT 1 FROM users WHERE id={profile}) THEN 1 ELSE 0 END) AS system_catalog_profile_installed;']
    diagrams, links, positions, skills, concepts = [], [], [], [], []
    for d in catalog['diagrams']:
        diagrams.append('(' + ','.join([sql_text(d['diagram_id']), sql_text(stable_id('instrument/' + basic.LAYOUT_INSTRUMENT)),
                                        sql_text(compact(d['names'])), "'basic'", "'chord_voicing'", profile, sql_text(d['root_note']),
                                        "'interval'", "'#3B82F6'", 'NULL', sql_text(compact(d['playbacks'])),
                                        sql_text(d['default_playback_id']), INSTALLED_AT]) + ')')
        links += [f"({sql_text(d['diagram_id'])},{sql_text(stable_id('instrument/' + key))},{INSTALLED_AT})" for key in d['instruments']]
        for ordinal, p in enumerate(d['positions']):
            positions.append('(' + ','.join([sql_text(p['position_id']), sql_text(d['diagram_id']), str(ordinal), sql_text(p['interval']),
                                             sql_text(p['note_name']), sql_text(p['shape']), sql_text(p['color']) if p['color'] else 'NULL',
                                             str(p['string']), str(p['fret'])]) + ')')
        skills += [f"({sql_text(d['diagram_id'])},{sql_text(stable_id('knowledge-node/' + s))},{INSTALLED_AT})" for s in d['skills']]
        concepts += [f"({sql_text(d['diagram_id'])},{sql_text(stable_id('knowledge-node/' + c))},{INSTALLED_AT})" for c in d['concepts']]
    chords = ['(' + ','.join([sql_text(c['chord_definition_id']), sql_text(c['canonical_symbol']), sql_text(c['root']), str(c['root_pitch_class']),
                              sql_text(c['quality']), sql_text(compact(c['formula'])), sql_text(compact(c['omittable'])),
                              sql_text(c['bass']) if c['bass'] else 'NULL', str(c['bass_pitch_class']) if c['bass'] else 'NULL',
                              sql_text(compact(c['aliases']))]) + ')' for c in catalog['chords']]
    voicings = ['(' + ','.join([sql_text(v['chord_voicing_id']), sql_text(v['chord_definition_id']), sql_text(v['diagram_id']),
                                sql_text(stable_id('instrument/' + basic.LAYOUT_INSTRUMENT)), sql_text(v['tuning_fingerprint']),
                                str(v['lowest_fret']), str(v['highest_fret']), sql_text(compact(v['fingering'])),
                                sql_text(compact(v['muted_strings'])), sql_text(compact(v['omitted_intervals'])), sql_text(v['difficulty']),
                                sql_text(compact(v['technique_tags'])), sql_text(v['shape_family']) if v['shape_family'] else 'NULL',
                                'true' if v['is_movable'] else 'false', str(v['recommended_rank']), "'active'",
                                sql_text(v['template_key']) if v['template_key'] else 'NULL']) + ')' for v in catalog['voicings']]
    for group in batches(diagrams, 250):
        sql.append('INSERT INTO diagrams (id,instrument_id,names,kind,purpose,created_by,root_note,label_display,color,mode,playbacks,default_playback_id,created_at) VALUES ' + ','.join(group) + ';')
    for group in batches(links, 1000):
        sql.append('INSERT INTO diagram_instruments (diagram_id,instrument_id,linked_at) VALUES ' + ','.join(group) + ';')
    for group in batches(positions, 1000):
        sql.append('INSERT INTO positions (id,diagram_id,ordinal,interval,note_name,shape,color,string_number,fret) VALUES ' + ','.join(group) + ';')
    for group in batches(skills, 1000):
        sql.append('INSERT INTO diagram_skills (diagram_id,skill_id,linked_at) VALUES ' + ','.join(group) + ';')
    for group in batches(concepts, 1000):
        sql.append('INSERT INTO diagram_concepts (diagram_id,concept_id,linked_at) VALUES ' + ','.join(group) + ';')
    for group in batches(chords, 500):
        sql.append('INSERT INTO chord_definitions (id,canonical_symbol,root,root_pitch_class,quality,formula,omittable,bass,bass_pitch_class,aliases) VALUES ' + ','.join(group) + ';')
    for group in batches(voicings, 500):
        sql.append('INSERT INTO chord_voicings (id,chord_definition_id,diagram_id,instrument_id,tuning_fingerprint,lowest_fret,highest_fret,fingering,muted_strings,omitted_intervals,difficulty,technique_tags,shape_family,is_movable,recommended_rank,status,template_key) VALUES ' + ','.join(group) + ';')
    return '\n'.join(sql) + '\n'


def write_outputs(catalog, output):
    """Writes the catalog payload and its coverage report. The SQL lives only
    in the reference data migration."""
    output.mkdir(parents=True, exist_ok=True)
    payload = compact(catalog) + '\n'
    (output / 'catalog.json').write_text(payload)
    report = dict(chords=len(catalog['chords']), voicings=len(catalog['voicings']),
                  open=sum(1 for v in catalog['voicings'] if not v['is_movable']),
                  templates=len({v['template_key'] for v in catalog['voicings'] if v['template_key']}),
                  sha256=hashlib.sha256(payload.encode()).hexdigest())
    (output / 'coverage.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    return report


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--specs', type=Path, default=Path(__file__).resolve().parents[3] / 'motifpath-specs')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    report = write_outputs(generate(load(args.specs / 'catalogs/chord-voicings.yaml')), args.output)
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
