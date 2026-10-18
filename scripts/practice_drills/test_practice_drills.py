import datetime
import os
import unittest
from pathlib import Path

import practice_drills as pd

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
CATALOG = SPECS_DIR / 'catalogs/practice-drills.yaml'
MAP = SPECS_DIR / 'catalogs/knowledge-map.yaml'


def template(key, kind=None, response='option_choice', timed=True, rung='know', second_way=None, comes_back=None):
    entry = {'key': key, 'item_kind': kind or key.split(':')[0], 'response_type': response, 'timed': timed,
             'rung': rung, 'names': {'en': key, 'pt_BR': key}, 'id': pd.template_id(key)}
    if second_way is not None:
        entry['second_way'] = second_way
    if comes_back is not None:
        entry['comes_back'] = comes_back
    return entry


def threshold(template_key, version=1, effective_from='2026-10-01', fluent_net_ms=6000, source='default'):
    return {'template': template_key, 'version': version, 'effective_from': effective_from,
            'fluent_net_ms': fluent_net_ms, 'source': source, 'sessions': 0, 'students': 0,
            'id': pd.threshold_id(template_key, version)}


def catalog(templates=(), thresholds=()):
    return {'templates': list(templates), 'thresholds': list(thresholds)}


class ValidationTests(unittest.TestCase):
    def assertRejected(self, raw, fragment):
        with self.assertRaises(ValueError) as ctx:
            pd.build(raw)
        self.assertIn(fragment, str(ctx.exception))

    def test_a_valid_catalog_builds(self):
        drills = pd.build(catalog(
            templates=[template('exercise:text_response')],
            thresholds=[threshold('exercise:text_response'),
                        threshold('exercise:text_response', 2, '2026-11-01', 4500, 'benchmark')]))
        self.assertEqual(['exercise:text_response'], [t.key for t in drills.templates])
        self.assertEqual([1, 2], [t.version for t in drills.thresholds])
        self.assertEqual(pd.stable_id('drill-threshold/exercise:text_response/v2'), drills.thresholds[1].id)

    def test_an_id_that_does_not_derive_from_its_key_is_rejected(self):
        bad = template('exercise:text_response')
        bad['id'] = pd.template_id('exercise:image_choice')
        self.assertRejected(catalog(templates=[bad]), 'id')

    def test_a_threshold_for_an_unknown_template_is_rejected(self):
        self.assertRejected(catalog(thresholds=[threshold('exercise:text_response')]), 'unknown template')

    def test_a_threshold_for_an_untimed_template_is_rejected(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response', timed=False)],
                                    thresholds=[threshold('exercise:text_response')]), 'not timed')

    def test_versions_must_count_from_one_without_gaps(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response')],
                                    thresholds=[threshold('exercise:text_response', 2)]), 'version')

    def test_a_later_version_must_start_later(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response')],
                                    thresholds=[threshold('exercise:text_response'),
                                                threshold('exercise:text_response', 2, '2026-10-01')]), 'effective_from')

    def test_an_unknown_source_is_rejected(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response')],
                                    thresholds=[threshold('exercise:text_response', source='guess')]), 'source')

    def test_a_fluent_time_must_be_positive(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response')],
                                    thresholds=[threshold('exercise:text_response', fluent_net_ms=0)]), 'fluent_net_ms')

    def test_a_template_keeps_its_rung_and_second_way(self):
        drills = pd.build(catalog(templates=[
            template('fretboard_cell:name_the_note', second_way='fretboard_cell:find_the_note'),
            template('fretboard_cell:find_the_note', second_way='fretboard_cell:name_the_note'),
            template('exercise:audio_selection', rung='hear')]))
        self.assertEqual(['know', 'know', 'hear'], [t.rung for t in drills.templates])
        self.assertEqual(['fretboard_cell:find_the_note', 'fretboard_cell:name_the_note', None],
                         [t.second_way for t in drills.templates])

    def test_a_rung_off_the_ladder_is_rejected(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response', rung='listen')]),
                            'exercise:text_response')

    def test_a_second_way_no_template_has_is_rejected(self):
        self.assertRejected(catalog(templates=[
            template('fretboard_cell:name_the_note', second_way='fretboard_cell:hear_the_note')]),
            'fretboard_cell:name_the_note')

    def test_a_second_way_asking_another_kind_of_item_is_rejected(self):
        self.assertRejected(catalog(templates=[
            template('fretboard_cell:find_the_note'),
            template('diatonic_chord:numeral_to_chord', second_way='fretboard_cell:find_the_note')]),
            'diatonic_chord:numeral_to_chord')

    def test_a_template_that_is_its_own_second_way_is_rejected(self):
        self.assertRejected(catalog(templates=[
            template('fretboard_cell:name_the_note', second_way='fretboard_cell:name_the_note')]),
            'fretboard_cell:name_the_note')

    def test_a_template_comes_back_only_when_marked(self):
        drills = pd.build(catalog(templates=[
            template('heard_progression:progression_by_number', rung='hear', comes_back=True),
            template('exercise:text_response')]))
        self.assertEqual([True, False], [t.comes_back for t in drills.templates])

    def test_comes_back_must_be_true_or_false(self):
        self.assertRejected(catalog(templates=[
            template('heard_progression:progression_by_number', rung='hear', comes_back='yes')]), 'comes_back')

    def test_a_hear_tempo_for_every_level_is_accepted(self):
        raw = catalog()
        raw['hear_tempo_bpm'] = {'new': 70, 'learning': 70, 'accurate': 90, 'fluent': 110, 'retained': 110}
        self.assertEqual(90, pd.build(raw).hear_tempo_bpm['accurate'])

    def test_a_hear_tempo_missing_a_level_is_rejected(self):
        raw = catalog()
        raw['hear_tempo_bpm'] = {'new': 70, 'learning': 70, 'accurate': 90, 'fluent': 110}
        self.assertRejected(raw, 'retained')

    def test_a_hear_tempo_must_be_a_positive_whole_number(self):
        raw = catalog()
        raw['hear_tempo_bpm'] = {'new': 70, 'learning': 0, 'accurate': 90, 'fluent': 110, 'retained': 110}
        self.assertRejected(raw, 'learning')

    def test_a_template_key_appears_once(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response'), template('exercise:text_response')]),
                            'more than once')


class ForwardOnlyTests(unittest.TestCase):
    """A version applies from its effective_from on, and items are rebuilt from their
    evidence, so a version installed after its effective_from would re-judge answers
    already folded under the old one."""

    def raw(self):
        return catalog(templates=[template('exercise:text_response')],
                       thresholds=[threshold('exercise:text_response'),
                                   threshold('exercise:text_response', 2, '2026-11-01', 4500, 'benchmark')])

    def test_a_new_version_may_not_take_effect_before_the_day_it_is_generated(self):
        with self.assertRaises(ValueError) as ctx:
            pd.build(self.raw(), installed=frozenset({pd.threshold_id('exercise:text_response', 1)}),
                     today=datetime.date(2026, 11, 2))
        self.assertIn('effective_from', str(ctx.exception))

    def test_a_new_version_from_today_or_later_is_accepted(self):
        drills = pd.build(self.raw(), installed=frozenset({pd.threshold_id('exercise:text_response', 1)}),
                          today=datetime.date(2026, 11, 1))
        self.assertEqual(2, len(drills.thresholds))

    def test_an_installed_version_is_not_held_to_today(self):
        installed = frozenset({pd.threshold_id('exercise:text_response', v) for v in (1, 2)})
        drills = pd.build(self.raw(), installed=installed, today=datetime.date(2027, 1, 1))
        self.assertEqual(2, len(drills.thresholds))

    def test_the_installed_versions_are_the_ids_in_the_frozen_migration(self):
        sql = pd.render_sql(pd.build(self.raw()))
        self.assertEqual({pd.threshold_id('exercise:text_response', v) for v in (1, 2)}, pd.installed_threshold_ids(sql))


class RenderTests(unittest.TestCase):
    def test_the_sql_installs_every_template_and_threshold_with_fixed_ids(self):
        sql = pd.render_sql(pd.build(catalog(
            templates=[template('exercise:text_response'), template('fretboard_cell:name_the_note', 'fretboard_cell', 'name_the_note')],
            thresholds=[threshold('exercise:text_response')])))
        self.assertIn('INSERT INTO "drill_templates"', sql)
        self.assertIn(pd.template_id('fretboard_cell:name_the_note'), sql)
        self.assertIn('INSERT INTO "drill_thresholds"', sql)
        self.assertIn(f"'{pd.threshold_id('exercise:text_response', 1)}', '{pd.template_id('exercise:text_response')}', 1, '2026-10-01T00:00:00Z', 6000, 'default', 0, 0", sql)

    def test_the_sql_installs_each_template_with_its_rung_and_second_way(self):
        sql = pd.render_sql(pd.build(catalog(templates=[
            template('fretboard_cell:name_the_note', second_way='fretboard_cell:find_the_note'),
            template('fretboard_cell:find_the_note', second_way='fretboard_cell:name_the_note'),
            template('exercise:audio_selection', rung='hear')])))
        self.assertIn('"rung", "second_way"', sql)
        self.assertIn("'know', 'fretboard_cell:find_the_note'", sql)
        self.assertIn("'hear', NULL", sql)

    def test_the_sql_installs_whether_each_template_comes_back(self):
        sql = pd.render_sql(pd.build(catalog(templates=[
            template('heard_progression:progression_by_number', rung='hear', comes_back=True),
            template('exercise:audio_selection', rung='hear')])))
        self.assertIn('"second_way", "comes_back", "timed"', sql)
        self.assertIn("'hear', NULL, true, true", sql)
        self.assertIn("'hear', NULL, false, true", sql)

    def test_rendering_is_deterministic(self):
        raw = catalog(templates=[template('exercise:text_response')], thresholds=[threshold('exercise:text_response')])
        self.assertEqual(pd.render_sql(pd.build(raw)), pd.render_sql(pd.build(raw)))


class CatalogTests(unittest.TestCase):
    """The live specs catalog, which CI reads from specs main: these check what must hold
    of it, not its exact contents, so a catalog that grows keeps this suite green."""

    def test_the_specs_catalog_builds(self):
        drills = pd.load(CATALOG)
        keys = {t.key for t in drills.templates}
        self.assertLessEqual({'fretboard_cell:name_the_note', 'fretboard_cell:find_the_note', 'exercise:text_response',
                              'exercise:audio_recognition', 'exercise:image_recognition', 'exercise:image_choice',
                              'exercise:audio_selection'}, keys)
        timed = {t.key for t in drills.templates if t.timed}
        self.assertEqual(timed, {t.template for t in drills.thresholds if t.version == 1},
                         'every timed template starts with a version 1')

    def test_the_specs_catalog_gives_a_hear_tempo_for_every_level(self):
        self.assertEqual(set(pd.LEVELS), set(pd.load(CATALOG).hear_tempo_bpm))

    def test_the_specs_catalog_cells_cover_the_guitar_and_bass_fretboards(self):
        ranges = pd.load_cells(CATALOG, MAP)
        layouts = {r.layout for r in ranges}
        self.assertLessEqual({'guitar', 'electric-bass'}, layouts)
        self.assertTrue(all(r.cell_count > 0 for r in ranges))

    def test_the_specs_catalog_shapes_include_the_tier_a_families(self):
        built = pd.load_shapes(CATALOG)
        counts = {f.key: sum(1 for s in built.shapes if s.family == f.key) for f in built.families}
        tier_a = {'caged-grip': 52, 'major-pentatonic-box': 47, 'minor-pentatonic-box': 47, 'triad': 48,
                  'major-scale-window': 52, 'natural-minor-scale-window': 52}
        self.assertEqual(tier_a, {key: counts.get(key) for key in tier_a})
        self.assertTrue(all(n > 0 for n in counts.values()), 'every family takes a catalog diagram')
        grip = next(s for s in built.shapes if s.diagram_id == pd.stable_id('caged/C/A/3'))
        self.assertEqual(('caged-grip', 'A'), (grip.family, grip.shape))


def cell_entry(skill='find-notes-root-strings', **layouts):
    return {'skill': skill, 'layouts': layouts or {'guitar': {'strings': [6, 5], 'frets': [0, 11]}}}


def node_map(*skills):
    """A knowledge map with the given (key, instrument keys) skills; empty keys = every instrument."""
    return {s: pd.km.Node(s, 'skill', {'en': s, 'pt_BR': s}, None, list(instruments)) for s, instruments in skills}


FRETTED = ('guitar', 'electric-guitar', 'electric-bass')


class CellTests(unittest.TestCase):
    def assertRejected(self, entries, nodes, *fragments):
        with self.assertRaises(ValueError) as ctx:
            pd.build_cells(entries, nodes)
        for fragment in fragments:
            self.assertIn(fragment, str(ctx.exception))

    def test_a_range_lists_its_strings_and_frets_with_a_fixed_id(self):
        [r] = pd.build_cells([cell_entry()], node_map(('find-notes-root-strings', FRETTED)))
        self.assertEqual(('find-notes-root-strings', 'guitar', [6, 5], 0, 11), (r.skill, r.layout, r.strings, r.from_fret, r.to_fret))
        self.assertEqual(24, r.cell_count)
        self.assertEqual(pd.stable_id('fretboard-cells/find-notes-root-strings/guitar'), r.id)

    def test_a_string_the_layout_doesnt_have_is_rejected(self):
        self.assertRejected([cell_entry('find-notes-top-strings', **{'electric-bass': {'strings': [2, 5], 'frets': [0, 11]}})],
                            node_map(('find-notes-top-strings', FRETTED)), "'electric-bass'", 'string 5')

    def test_a_skill_that_doesnt_suit_the_layout_is_rejected(self):
        self.assertRejected([cell_entry('play-e-shape-barre', **{'electric-bass': {'strings': [4], 'frets': [0, 11]}})],
                            node_map(('play-e-shape-barre', ('guitar', 'electric-guitar'))), "'play-e-shape-barre'", "'electric-bass'")

    def test_an_unknown_skill_is_rejected(self):
        self.assertRejected([cell_entry('no-such-skill')], node_map(), "'no-such-skill'")

    def test_an_unknown_layout_is_rejected(self):
        self.assertRejected([cell_entry(ukulele={'strings': [1], 'frets': [0, 11]})],
                            node_map(('find-notes-root-strings', ())), "'ukulele'")

    def test_frets_out_of_order_are_rejected(self):
        self.assertRejected([cell_entry(guitar={'strings': [6], 'frets': [5, 2]})],
                            node_map(('find-notes-root-strings', FRETTED)), 'frets')

    def test_a_cell_in_two_skills_is_rejected(self):
        self.assertRejected([cell_entry('find-notes-root-strings', guitar={'strings': [6, 5], 'frets': [0, 11]}),
                             cell_entry('find-notes-top-strings', guitar={'strings': [5, 4], 'frets': [0, 11]})],
                            node_map(('find-notes-root-strings', FRETTED), ('find-notes-top-strings', FRETTED)),
                            'string 5')

    def test_two_layouts_of_the_same_geometry_for_one_skill_are_rejected(self):
        self.assertRejected([cell_entry(guitar={'strings': [6], 'frets': [0, 11]},
                                        **{'electric-guitar': {'strings': [5], 'frets': [0, 11]}})],
                            node_map(('find-notes-root-strings', FRETTED)), "'guitar'", "'electric-guitar'")

    def test_the_sql_installs_every_range_with_fixed_ids(self):
        sql = pd.render_cells_sql(pd.build_cells([cell_entry()], node_map(('find-notes-root-strings', FRETTED))))
        self.assertIn('INSERT INTO "fretboard_cell_ranges"', sql)
        self.assertIn(f"('{pd.stable_id('fretboard-cells/find-notes-root-strings/guitar')}', "
                      f"'{pd.km.stable_id('knowledge-node/find-notes-root-strings')}', '{pd.km.instrument_id('guitar')}', '[6,5]', 0, 11)", sql)


def family(key='caged-grip', diagrams='caged/{root}/{shape}/{shift}', members=('C', 'A', 'G', 'E', 'D')):
    return {'family': key, 'diagrams': diagrams, 'names': {'en': key, 'pt_BR': key},
            'members': [{'shape': m, 'names': {'en': f'{m} shape', 'pt_BR': f'Forma de {m}'}} for m in members]}


def diagrams(*keys):
    """Catalog entries, as the diagram catalog generates them, for keys."""
    return [{'key': k, 'diagram_id': pd.stable_id(k)} for k in keys]


GRIPS = diagrams('caged/C/A/3', 'caged/C/E/8', 'caged/D/A/5', 'chromatic/C')


class ShapeTests(unittest.TestCase):
    def assertRejected(self, entries, catalog_diagrams, *fragments):
        with self.assertRaises(ValueError) as ctx:
            pd.build_shapes(entries, catalog_diagrams)
        for fragment in fragments:
            self.assertIn(fragment, str(ctx.exception))

    def test_each_matching_catalog_diagram_is_a_shape_of_its_member(self):
        built = pd.build_shapes([family()], GRIPS)
        self.assertEqual([(pd.stable_id('caged/C/A/3'), 'caged-grip', 'A'), (pd.stable_id('caged/C/E/8'), 'caged-grip', 'E'),
                          (pd.stable_id('caged/D/A/5'), 'caged-grip', 'A')],
                         [(s.diagram_id, s.family, s.shape) for s in built.shapes])

    def test_a_family_keeps_its_members_in_catalog_order_with_a_fixed_id(self):
        [f] = pd.build_shapes([family()], GRIPS).families
        self.assertEqual(['C', 'A', 'G', 'E', 'D'], [m['shape'] for m in f.members])
        self.assertEqual({'en': 'A shape', 'pt_BR': 'Forma de A'}, f.members[1]['names'])
        self.assertEqual(pd.stable_id('diagram-shape-family/caged-grip'), f.id)

    def test_a_diagram_matching_no_family_is_not_a_shape(self):
        built = pd.build_shapes([family()], GRIPS)
        self.assertNotIn(pd.stable_id('chromatic/C'), {s.diagram_id for s in built.shapes})

    def test_a_placeholder_matches_one_key_segment_only(self):
        built = pd.build_shapes([family()], diagrams('caged/C/A/3', 'caged/C/A/3/extra'))
        self.assertEqual([pd.stable_id('caged/C/A/3')], [s.diagram_id for s in built.shapes])

    def test_a_family_matching_no_catalog_diagram_is_rejected(self):
        self.assertRejected([family(), family('lydian-window', 'caged-window/lydian/{root}/{shape}/{shift}')], GRIPS,
                            "'lydian-window'")

    def test_a_diagram_whose_shape_is_not_a_member_is_rejected(self):
        self.assertRejected([family(members=('C', 'A', 'G', 'E'))], diagrams('caged/C/A/3', 'caged/C/D/10'),
                            "'caged-grip'", "'D'")

    def test_a_diagram_in_two_families_is_rejected(self):
        self.assertRejected([family(), family('grips-again')], GRIPS, "'caged/C/A/3'", "'caged-grip'", "'grips-again'")

    def test_a_pattern_without_a_shape_placeholder_is_rejected(self):
        self.assertRejected([family(diagrams='caged/{root}/A/{shift}')], GRIPS, "'caged-grip'", '{shape}')

    def test_a_family_appears_once(self):
        self.assertRejected([family(), family()], GRIPS, "'caged-grip'")

    def test_a_member_appears_once(self):
        self.assertRejected([family(members=('C', 'A', 'A', 'G', 'E', 'D'))], GRIPS, "'caged-grip'", "'A'")

    def test_a_member_needs_a_name_in_each_language(self):
        entry = family()
        entry['members'][0]['names'] = {'en': 'C shape'}
        self.assertRejected([entry], GRIPS, "'caged-grip'", "'C'")

    def test_the_sql_installs_every_family_and_shape_with_fixed_ids(self):
        sql = pd.render_shapes_sql(pd.build_shapes([family()], GRIPS))
        self.assertIn('INSERT INTO "diagram_shape_families"', sql)
        self.assertIn(f"('{pd.stable_id('diagram-shape-family/caged-grip')}', 'caged-grip', ", sql)
        self.assertIn('{"names":{"en":"C shape","pt_BR":"Forma de C"},"shape":"C"}', sql)
        self.assertIn('INSERT INTO "diagram_shapes"', sql)
        self.assertIn(f"('{pd.stable_id('diagram-shape/' + pd.stable_id('caged/C/A/3'))}', '{pd.stable_id('caged/C/A/3')}', "
                      f"'{pd.stable_id('diagram-shape-family/caged-grip')}', 'A')", sql)



# Chord catalog voicings as the chord catalog generator gives them, by voicing key.
VOICINGS = {
    'a-minor-open': {'diagram_id': pd.stable_id('chord-voicing/a-minor-open'), 'chord': 'Am'},
    'c-open': {'diagram_id': pd.stable_id('chord-voicing/c-open'), 'chord': 'C'},
    'g-open': {'diagram_id': pd.stable_id('chord-voicing/g-open'), 'chord': 'G'},
}


def chord_drills(voicings=('a-minor-open', 'c-open', 'g-open'), changes=(('a-minor-open', 'c-open'),), **settings):
    raw = {
        'bass_layout': 'electric-bass',
        'voicings': [{'voicing': v, 'chord': VOICINGS[v]['chord'] if v in VOICINGS else 'X',
                      'id': pd.stable_id('chord-drill/' + v)} for v in voicings],
        'changes': [{'from': a, 'to': b, 'id': pd.stable_id(f'chord-change/{a}/{b}')} for a, b in changes],
        'max_voicings_per_session': 4, 'max_changes_per_session': 2, 'voicing_seconds': 20, 'change_seconds': 90,
        'change_first_tempo_bpm': 40, 'change_tempo_step_bpm': 5, 'change_min_tempo_bpm': 20,
        'change_rating_from': {'clean': 0.9, 'almost': 0.6},
    }
    raw.update(settings)
    return raw


def change_target(version=1, effective_from='2026-10-18', changes_per_minute=60, source='default'):
    return {'version': version, 'effective_from': effective_from, 'changes_per_minute': changes_per_minute,
            'source': source, 'id': pd.stable_id(f'change-target/v{version}')}


class ChordDrillTests(unittest.TestCase):
    def assertRejected(self, raw, *fragments):
        with self.assertRaises(ValueError) as ctx:
            pd.build_chord_drills(raw, VOICINGS)
        for fragment in fragments:
            self.assertIn(fragment, str(ctx.exception))

    def test_each_chord_drill_names_its_voicing_diagram_and_chord_with_a_fixed_id(self):
        built = pd.build_chord_drills(chord_drills(), VOICINGS)
        first = built.voicings[0]
        self.assertEqual((pd.stable_id('chord-drill/a-minor-open'), VOICINGS['a-minor-open']['diagram_id'], 'Am'),
                         (first.id, first.diagram_id, first.chord))
        self.assertEqual(['a-minor-open', 'c-open', 'g-open'], [v.voicing for v in built.voicings])

    def test_each_change_pairs_two_chord_drills_in_the_order_listed(self):
        change = pd.build_chord_drills(chord_drills(), VOICINGS).changes[0]
        self.assertEqual((pd.stable_id('chord-change/a-minor-open/c-open'), 'a-minor-open', 'c-open'),
                         (change.id, change.from_voicing, change.to_voicing))

    def test_a_voicing_the_chord_catalog_doesnt_have_is_rejected(self):
        self.assertRejected(chord_drills(voicings=('a-minor-open', 'c-open', 'h-open')), "'h-open'")

    def test_a_chord_that_isnt_its_voicings_chord_is_rejected(self):
        raw = chord_drills()
        raw['voicings'][0]['chord'] = 'A'
        self.assertRejected(raw, "'a-minor-open'", "'Am'")

    def test_a_voicing_listed_twice_is_rejected(self):
        self.assertRejected(chord_drills(voicings=('a-minor-open', 'c-open', 'a-minor-open')), "'a-minor-open'")

    def test_a_change_of_a_chord_that_isnt_a_chord_drill_is_rejected(self):
        self.assertRejected(chord_drills(voicings=('a-minor-open', 'c-open'), changes=(('c-open', 'g-open'),)), "'g-open'")

    def test_a_change_of_a_chord_to_itself_is_rejected(self):
        self.assertRejected(chord_drills(changes=(('c-open', 'c-open'),)), "'c-open'")

    def test_a_change_listed_twice_in_either_order_is_rejected(self):
        self.assertRejected(chord_drills(changes=(('a-minor-open', 'c-open'), ('c-open', 'a-minor-open'))),
                            "'c-open'", "'a-minor-open'")

    def test_a_wrong_fixed_id_is_rejected(self):
        raw = chord_drills()
        raw['changes'][0]['id'] = pd.stable_id('chord-change/c-open/a-minor-open')
        self.assertRejected(raw, 'id')

    def test_the_bass_layout_must_be_a_catalog_instrument(self):
        self.assertRejected(chord_drills(bass_layout='tuba'), "'tuba'")

    def test_every_session_setting_must_be_a_positive_whole_number(self):
        self.assertRejected(chord_drills(change_min_tempo_bpm=0), 'change_min_tempo_bpm')

    def test_the_first_tempo_may_not_be_below_the_floor(self):
        self.assertRejected(chord_drills(change_first_tempo_bpm=15), 'change_first_tempo_bpm')

    def test_the_rating_shares_must_fall_from_clean_to_almost(self):
        self.assertRejected(chord_drills(change_rating_from={'clean': 0.6, 'almost': 0.9}), 'change_rating_from')

    def test_the_sql_installs_every_chord_drill_and_change_with_fixed_ids(self):
        sql = pd.render_chord_drills_sql(pd.build_chord_drills(chord_drills(), VOICINGS), [])
        self.assertIn('INSERT INTO "chord_drills" ("id", "diagram_id", "chord", "ordinal") VALUES', sql)
        self.assertIn(f"('{pd.stable_id('chord-drill/a-minor-open')}', '{VOICINGS['a-minor-open']['diagram_id']}', 'Am', 1)", sql)
        self.assertIn('INSERT INTO "chord_change_pairs" ("id", "from_chord_drill_id", "to_chord_drill_id", "ordinal") VALUES', sql)
        self.assertIn(f"('{pd.stable_id('chord-change/a-minor-open/c-open')}', '{pd.stable_id('chord-drill/a-minor-open')}', "
                      f"'{pd.stable_id('chord-drill/c-open')}', 1)", sql)


class ChangeTargetTests(unittest.TestCase):
    def assertRejected(self, entries, fragment, installed=frozenset(), today=None):
        with self.assertRaises(ValueError) as ctx:
            pd.build_change_targets(entries, installed, today)
        self.assertIn(fragment, str(ctx.exception))

    def test_a_version_builds_with_a_fixed_id(self):
        target = pd.build_change_targets([change_target()])[0]
        self.assertEqual((pd.stable_id('change-target/v1'), 1, datetime.date(2026, 10, 18), 60, 'default'),
                         (target.id, target.version, target.effective_from, target.changes_per_minute, target.source))

    def test_versions_count_from_1_without_gaps(self):
        self.assertRejected([change_target(), change_target(version=3, effective_from='2026-11-01')], 'versions')

    def test_a_later_version_takes_effect_later(self):
        self.assertRejected([change_target(), change_target(version=2, effective_from='2026-10-18')], 'effective_from')

    def test_the_rate_must_be_a_positive_whole_number(self):
        self.assertRejected([change_target(changes_per_minute=0)], 'changes_per_minute')

    def test_a_source_must_be_known(self):
        self.assertRejected([change_target(source='guess')], 'source')

    def test_a_new_version_may_not_take_effect_in_the_past(self):
        self.assertRejected([change_target(effective_from='2026-10-01')], 'effective_from',
                            today=datetime.date(2026, 10, 18))

    def test_an_installed_version_keeps_its_date(self):
        targets = pd.build_change_targets([change_target(effective_from='2026-10-01')],
                                          {pd.stable_id('change-target/v1')}, datetime.date(2026, 10, 18))
        self.assertEqual(1, len(targets))

    def test_the_sql_installs_every_version(self):
        sql = pd.render_chord_drills_sql(pd.build_chord_drills(chord_drills(), VOICINGS),
                                         pd.build_change_targets([change_target()]))
        self.assertIn('INSERT INTO "change_targets" ("id", "version", "effective_from", "changes_per_minute", "source") VALUES', sql)
        self.assertIn(f"('{pd.stable_id('change-target/v1')}', 1, '2026-10-18T00:00:00Z', 60, 'default')", sql)


class ChordDrillCatalogTests(unittest.TestCase):
    def test_the_specs_catalog_chord_drills_build_against_the_chord_catalog(self):
        built = pd.load_chord_drills(CATALOG, SPECS_DIR / 'catalogs/chord-voicings.yaml')
        self.assertGreaterEqual(len(built.voicings), 2)
        self.assertGreaterEqual(len(built.changes), 1)

    def test_the_specs_catalog_installs_a_first_change_target(self):
        targets = pd.load_change_targets(CATALOG)
        self.assertEqual(1, targets[0].version)


if __name__ == '__main__':
    unittest.main()
