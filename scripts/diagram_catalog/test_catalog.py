import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from collections import Counter
import catalog

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'knowledge_map'))
import knowledge_map  # noqa: E402

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
KNOWLEDGE_MAP = SPECS_DIR / 'catalogs/knowledge-map.yaml'


class CatalogTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.entries = catalog.generate()

    def test_spelling(self):
        self.assertEqual(catalog.spell('C', 'bb7'), 'Bbb')
        self.assertEqual(catalog.spell('F', '4'), 'Bb')
        self.assertEqual(catalog.spell('C', '#4'), 'F#')
        self.assertEqual(catalog.spell('C', 'b5'), 'Gb')
        self.assertEqual(catalog.pitch_class('B#'), 0)

    def test_determinism(self):
        self.assertEqual(json.dumps(self.entries, sort_keys=True), json.dumps(catalog.generate(), sort_keys=True))
        ids = [e['diagram_id'] for e in self.entries]
        self.assertEqual(len(ids), len(set(ids)))

    def test_contract_and_musical_consistency(self):
        catalog.validate(self.entries)
        for e in self.entries:
            for p in e['positions']:
                self.assertEqual((catalog.TUNING[6-p['string']] + p['fret']) % 12, catalog.pitch_class(p['note_name']), e['key'])
                self.assertEqual((catalog.pitch_class(e['root_note']) + catalog.semitones(p['interval'])) % 12, catalog.pitch_class(p['note_name']), e['key'])

    def test_all_tiers_and_tonics(self):
        self.assertEqual({e['tier'] for e in self.entries}, {'A', 'B', 'C'})
        for family in {e['family'] for e in self.entries}:
            self.assertEqual({catalog.pitch_class(e['root_note']) for e in self.entries if e['family'] == family}, set(range(12)), family)

    def test_shape_cardinalities(self):
        for e in self.entries:
            self.assertEqual(e['instruments'], ['guitar', 'electric-guitar'])
            self.assertTrue(all(p['fret'] <= 12 for p in e['positions']), e['key'])
            if e['family'] == 'chromatic': self.assertEqual(len(e['positions']), 78)
            if e['family'] == 'pentatonic-box': self.assertEqual(len(e['positions']), 12)
            if e['family'] == '3nps': self.assertEqual(Counter(p['string'] for p in e['positions']), {s: 3 for s in range(1, 7)})

    def test_portuguese(self):
        for e in self.entries:
            self.assertEqual(set(e['names']), {'en', 'pt_BR'})
            self.assertNotEqual(e['names']['en'], e['names']['pt_BR'])
            self.assertNotIn('Position', e['names']['pt_BR'])
            self.assertNotIn('Minor', e['names']['pt_BR'])
        cm = next(e for e in self.entries if e['key'] == 'scale/major/C/frets-0-12')
        self.assertEqual(cm['names']['pt_BR'], 'Escala maior de Dó — Casas 0–12')

    def test_c_diminished_seventh_map(self):
        d = next(e for e in self.entries if e['key'] == 'arpeggio/dim7/C/frets-0-12')
        self.assertEqual({p['note_name'] for p in d['positions']}, {'C', 'Eb', 'Gb', 'Bbb'})

    def test_playbacks_by_family(self):
        ascending = {'caged-window', 'pentatonic-box', '3nps'}
        for e in self.entries:
            roles = [p['playback_id'] for p in e['playbacks']]
            if e['family'] == 'caged':
                expected = ['strum-down', 'arpeggio']
            elif e['family'] in ascending:
                expected = ['ascending']
            else:
                expected = []
            self.assertEqual(roles, [catalog.stable_id(f"{e['key']}/playback/{role}") for role in expected], e['key'])
            self.assertEqual(e['default_playback_id'], roles[0] if roles else None, e['key'])
            self.assertNotIn('sequence', e)
            self.assertNotIn('tempo_bpm', e)

    def test_playback_names_tempo_and_meter(self):
        names = {
            'strum-down': {'en': 'Strum down', 'pt_BR': 'Batida para baixo'},
            'arpeggio': {'en': 'Arpeggio', 'pt_BR': 'Arpejo'},
            'ascending': {'en': 'Ascending', 'pt_BR': 'Ascendente'},
        }
        grip = next(e for e in self.entries if e['family'] == 'caged')
        window = next(e for e in self.entries if e['family'] == 'caged-window')
        for e, roles in ((grip, ['strum-down', 'arpeggio']), (window, ['ascending'])):
            for playback, role in zip(e['playbacks'], roles):
                self.assertEqual(playback['names'], names[role])
                self.assertEqual(playback['tempo_bpm'], 60)
                self.assertEqual(playback['time_signature'], {'beats': 4, 'beat_value': 4})

    def test_a_grip_strums_down_then_arpeggiates_from_its_lowest_pitch(self):
        grip = next(e for e in self.entries if e['key'] == 'caged/C/C/0')
        by_id = {p['position_id']: p for p in grip['positions']}
        lowest_first = sorted(grip['positions'], key=lambda p: catalog.TUNING[6-p['string']] + p['fret'])
        strum, arpeggio = grip['playbacks']
        self.assertEqual(strum['steps'], [{'position_ids': [p['position_id'] for p in lowest_first], 'value': {'num': 1, 'den': 1}, 'strum': 'down'}])
        self.assertEqual(arpeggio['steps'], [{'position_ids': [p['position_id']], 'value': {'num': 1, 'den': 4}, 'strum': 'none'} for p in lowest_first])
        self.assertEqual(set(by_id), {i for step in strum['steps'] for i in step['position_ids']})

    def test_a_window_ascends_through_every_position(self):
        for family in ('caged-window', 'pentatonic-box', '3nps'):
            e = next(e for e in self.entries if e['family'] == family)
            pitches = {p['position_id']: catalog.TUNING[6-p['string']] + p['fret'] for p in e['positions']}
            steps = e['playbacks'][0]['steps']
            self.assertEqual(len(steps), len(e['positions']), family)
            played = [pitches[step['position_ids'][0]] for step in steps]
            self.assertEqual(played, sorted(played), family)

    def test_validation_rejects_a_broken_playback(self):
        grip = next(e for e in self.entries if e['family'] == 'caged')
        broken = json.loads(json.dumps(grip)); broken['playbacks'][0]['steps'][0]['position_ids'].append('not-a-position')
        with self.assertRaises(ValueError): catalog.validate([broken])
        broken = json.loads(json.dumps(grip)); broken['default_playback_id'] = 'not-a-playback'
        with self.assertRaises(ValueError): catalog.validate([broken])
        broken = json.loads(json.dumps(grip)); broken['playbacks'][1]['names'].pop('pt_BR')
        with self.assertRaises(ValueError): catalog.validate([broken])
        silent = next(e for e in self.entries if e['family'] == 'chromatic')
        broken = json.loads(json.dumps(silent)); broken['default_playback_id'] = grip['playbacks'][0]['playback_id']
        with self.assertRaises(ValueError): catalog.validate([broken])

    def test_validation_rejects_broken_translation_and_pitch(self):
        small = json.loads(json.dumps(self.entries[:1])); small[0]['names'].pop('pt_BR')
        with self.assertRaises(ValueError): catalog.validate(small)
        small = json.loads(json.dumps(self.entries[:1])); small[0]['positions'][0]['fret'] = 99
        with self.assertRaises(ValueError): catalog.validate(small)

    def test_sql_is_escaped_and_has_preconditions(self):
        self.assertEqual(catalog.sql_text("d'água"), "'d''água'")
        sql = catalog.render_sql(self.entries[:1])
        self.assertIn('system_catalog_profile_compatible', sql)
        self.assertIn(catalog.SYSTEM_CATALOG_USER_ID, sql)
        self.assertIn(catalog.SYSTEM_CATALOG_CLERK_USER_ID, sql)
        self.assertIn("'MotifPath Catalog'", sql)
        self.assertIn(catalog.stable_id('instrument/electric-guitar'), sql)
        self.assertIn('INSERT INTO diagram_instruments', sql)
        self.assertNotIn('INSERT INTO instruments', sql)
        self.assertNotIn('INSERT INTO skills', sql)
        self.assertNotIn('INSERT INTO concepts', sql)
        self.assertIn('playbacks,default_playback_id,created_at) VALUES (', sql)
        self.assertNotIn('sequence', sql)
        self.assertNotIn('tempo_bpm', sql.split('VALUES')[0])
        self.assertIn('linked_at', sql)
        self.assertNotIn('DELETE FROM', sql)
        self.assertNotIn('ON CONFLICT', sql)

    def test_every_diagram_is_classified_by_map_key(self):
        sql = catalog.render_sql(self.entries)
        for e in self.entries:
            skill, concept = catalog.DIAGRAM_CLASSIFICATION[e['family']]
            self.assertIn(f"'{e['diagram_id']}','{catalog.stable_id('knowledge-node/' + skill)}'", sql)
            self.assertIn(f"'{e['diagram_id']}','{catalog.stable_id('knowledge-node/' + concept)}'", sql)

    @unittest.skipUnless(KNOWLEDGE_MAP.exists(), f'{KNOWLEDGE_MAP} not found; set SPECS_DIR')
    def test_classification_nodes_exist_and_suit_both_guitars(self):
        nodes = {n.key: n for n in knowledge_map.load(KNOWLEDGE_MAP).nodes}
        for family, (skill, concept) in catalog.DIAGRAM_CLASSIFICATION.items():
            for key, kind in ((skill, 'skill'), (concept, 'concept')):
                node = nodes.get(key)
                self.assertIsNotNone(node, f'{family}: {key}')
                self.assertEqual(kind, node.kind, key)
                self.assertTrue(not node.instruments or {'guitar', 'electric-guitar'} & set(node.instruments), key)

    def test_catalog_excludes_string_set_chord_templates(self):
        excluded = {'triad-inversion', 'seventh-inversion', 'drop-2', 'drop-3', 'drop-2-4', 'shell', 'interval', 'dyad'}
        self.assertFalse({e['family'] for e in self.entries} & excluded)

    def test_sql_batches_catalog_rows_for_production(self):
        sql = catalog.render_sql(self.entries)
        self.assertLess(sql.count('\nINSERT INTO '), 300)
        self.assertLess(sql.count('\nINSERT INTO positions '), 200)

    def test_migration_is_written_only_to_the_migrations_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            output, migrations = Path(tmp)/'catalog', Path(tmp)/'migrations'
            migrations.mkdir()
            catalog.write_outputs(self.entries[:1], output, migrations)
            self.assertEqual(sorted(p.name for p in output.iterdir()), ['catalog.json', 'coverage.json'])
            self.assertEqual([p.name for p in migrations.iterdir()], [catalog.MIGRATION_FILE])


if __name__ == '__main__': unittest.main()
