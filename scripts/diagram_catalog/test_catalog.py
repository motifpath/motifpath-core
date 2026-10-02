import json
import unittest
from collections import Counter
import catalog


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
            if e['family'] == 'chromatic': self.assertEqual(len(e['positions']), 150)
            if e['family'] == 'pentatonic-box': self.assertEqual(len(e['positions']), 12)
            if e['family'] == '3nps': self.assertEqual(Counter(p['string'] for p in e['positions']), {s: 3 for s in range(1, 7)})

    def test_portuguese(self):
        for e in self.entries:
            self.assertEqual(set(e['names']), {'en', 'pt_BR'})
            self.assertNotEqual(e['names']['en'], e['names']['pt_BR'])
            self.assertNotIn('Position', e['names']['pt_BR'])
            self.assertNotIn('Minor', e['names']['pt_BR'])
        cm = next(e for e in self.entries if e['key'] == 'scale/major/C/full')
        self.assertEqual(cm['names']['pt_BR'], 'Escala maior de Dó — Braço completo')

    def test_c_major_voicing_and_diminished_seventh(self):
        chords = [e for e in self.entries if e['family'] == 'triad-inversion' and e['root_note'] == 'C' and e['formula'] == 'major-triad']
        self.assertTrue(any({(p['string'], p['fret']) for p in e['positions']} == {(3, 5), (2, 5), (1, 3)} for e in chords))
        d = next(e for e in self.entries if e['key'] == 'arpeggio/dim7/C/full')
        self.assertEqual({p['note_name'] for p in d['positions']}, {'C', 'Eb', 'Gb', 'Bbb'})

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
        self.assertIn('sequence,created_at) VALUES (', sql)
        self.assertNotIn('sequence_index', sql)
        self.assertIn('linked_at', sql)
        self.assertNotIn('DELETE FROM', sql)
        self.assertNotIn('ON CONFLICT', sql)


if __name__ == '__main__': unittest.main()
