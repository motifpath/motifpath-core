import copy
import json
import os
import unittest
from pathlib import Path

import chord_catalog

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
CATALOG_YAML = SPECS_DIR / 'catalogs/chord-voicings.yaml'


class ChordCatalogTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.spec = chord_catalog.load(CATALOG_YAML)
        cls.catalog = chord_catalog.generate(cls.spec)
        cls.chords = {c['canonical_symbol']: c for c in cls.catalog['chords']}

    def voicings_of(self, symbol):
        chord_id = self.chords[symbol]['chord_definition_id']
        return sorted((v for v in self.catalog['voicings'] if v['chord_definition_id'] == chord_id), key=lambda v: v['recommended_rank'])

    def diagram_of(self, voicing):
        return next(d for d in self.catalog['diagrams'] if d['diagram_id'] == voicing['diagram_id'])

    def test_every_quality_on_every_root_has_a_voicing(self):
        for quality in self.spec['formulas']:
            for root in self.spec['roots']:
                symbol = root + chord_catalog.SUFFIX[quality]
                self.assertIn(symbol, self.chords)
                self.assertTrue(self.voicings_of(symbol), symbol)

    def test_counts(self):
        self.assertEqual(len(self.catalog['chords']), 341)
        self.assertEqual(len(self.catalog['voicings']), 538)
        self.assertEqual(len(self.catalog['diagrams']), 538)

    def test_ids_are_stable_names(self):
        am = self.chords['Am']
        self.assertEqual(am['chord_definition_id'], chord_catalog.stable_id('chord-definition/Am'))
        open_am = next(v for v in self.voicings_of('Am') if v['key'] == 'a-minor-open')
        self.assertEqual(open_am['chord_voicing_id'], chord_catalog.stable_id('chord-voicing/a-minor-open'))
        self.assertEqual(open_am['diagram_id'], chord_catalog.stable_id('chord-diagram/chord-voicing/a-minor-open'))
        barre = next(v for v in self.voicings_of('Bm') if v['key'] == 'minor-a-shape/B')
        self.assertEqual(barre['chord_voicing_id'], chord_catalog.stable_id('chord-voicing/minor-a-shape/B'))

    def test_determinism(self):
        again = chord_catalog.generate(chord_catalog.load(CATALOG_YAML))
        self.assertEqual(json.dumps(self.catalog, sort_keys=True), json.dumps(again, sort_keys=True))

    def test_validator_rejects_a_wrong_chord(self):
        spec = copy.deepcopy(self.spec)
        am = next(o for o in spec['open_voicings'] if o['key'] == 'a-minor-open')
        am['strings'][1] = [2, '4']  # F# on the first string
        with self.assertRaisesRegex(ValueError, 'a-minor-open.*not in Am'):
            chord_catalog.generate(spec)

    def test_validator_rejects_an_undeclared_omission(self):
        spec = copy.deepcopy(self.spec)
        c7 = next(o for o in spec['open_voicings'] if o['key'] == 'c-7-open')
        del c7['omits']
        with self.assertRaisesRegex(ValueError, 'c-7-open.*leaves out'):
            chord_catalog.generate(spec)

    def test_validator_rejects_a_wrong_slash_bass(self):
        spec = copy.deepcopy(self.spec)
        d = next(o for o in spec['open_voicings'] if o['key'] == 'd-over-f-sharp-open')
        del d['strings'][6]
        with self.assertRaisesRegex(ValueError, 'd-over-f-sharp-open.*bass'):
            chord_catalog.generate(spec)

    def test_templates_skip_roots_that_do_not_fit(self):
        keys = {v['key'] for v in self.catalog['voicings']}
        self.assertNotIn('add-9-5th-string/A', keys)
        self.assertIn('add-9-5th-string/Bb', keys)
        self.assertTrue(all(f >= 1 for v in self.catalog['voicings'] if v['template_key'] for f in self.diagram_frets(v)))

    def diagram_frets(self, voicing):
        return [p['fret'] for p in self.diagram_of(voicing)['positions']]

    def test_open_voicings_rank_first(self):
        am = self.voicings_of('Am')
        self.assertEqual(am[0]['key'], 'a-minor-open')
        self.assertEqual([v['recommended_rank'] for v in am], list(range(1, len(am) + 1)))
        self.assertEqual(am[0]['shape_family'], 'open')
        self.assertIn('open', am[0]['technique_tags'])
        self.assertFalse(am[0]['is_movable'])

    def test_positions_spell_notes_from_the_root(self):
        voicing = next(v for v in self.voicings_of('F#maj7') if v['template_key'] == 'major-7-e-shape')
        notes = {p['interval']: p['note_name'] for p in self.diagram_of(voicing)['positions']}
        self.assertEqual(notes['3'], 'A#')
        self.assertEqual(notes['7'], 'E#')

    def test_a_slash_bass_outside_the_formula_takes_its_interval(self):
        diagram = self.diagram_of(self.voicings_of('Am/G')[0])
        lowest = max(diagram['positions'], key=lambda p: p['string'])
        self.assertEqual((lowest['note_name'], lowest['interval']), ('G', 'b7'))

    def test_diagrams_are_chord_voicings_named_by_shape(self):
        open_am = self.diagram_of(self.voicings_of('Am')[0])
        self.assertEqual(open_am['purpose'], 'chord_voicing')
        self.assertEqual(open_am['names'], {'en': 'Am — open', 'pt_BR': 'Am — aberto'})
        self.assertEqual([p['names']['en'] for p in open_am['playbacks']], ['Strum down', 'Arpeggio'])
        bbmaj7 = self.diagram_of(next(v for v in self.voicings_of('Bbmaj7') if v['template_key'] == 'major-7-a-shape'))
        self.assertEqual(bbmaj7['names'], {'en': 'Bbmaj7 — A shape, fret 1', 'pt_BR': 'Bbmaj7 — forma de Lá, casa 1'})
        f = self.diagram_of(next(v for v in self.voicings_of('F') if v['template_key'] == 'major-d-shape'))
        self.assertEqual(f['names'], {'en': 'F — D shape, fret 3', 'pt_BR': 'F — forma de Ré, casa 3'})
        dim7 = self.diagram_of(next(v for v in self.voicings_of('Cdim7') if v['template_key'] == 'diminished-7-5th-string'))
        self.assertEqual(dim7['names'], {'en': 'Cdim7 — root on string 5, fret 3', 'pt_BR': 'Cdim7 — tônica na 5ª corda, casa 3'})

    def test_classification(self):
        open_am = self.diagram_of(self.voicings_of('Am')[0])
        self.assertEqual(open_am['skills'], ['chord-diagrams', 'minor-triads', 'open-chord-shapes'])
        self.assertEqual(open_am['concepts'], ['chords'])
        barre = self.diagram_of(next(v for v in self.voicings_of('Bm') if v['template_key'] == 'minor-a-shape'))
        self.assertEqual(barre['skills'], ['barre-chord-shapes', 'chord-diagrams', 'minor-triads'])
        slash = self.diagram_of(self.voicings_of('D/F#')[0])
        self.assertIn('slash-chords', slash['skills'])

    def test_sql_installs_purpose_and_tables(self):
        sql = chord_catalog.render_sql(self.catalog)
        self.assertIn("'chord_voicing'", sql)
        for table in ('diagrams', 'diagram_instruments', 'positions', 'diagram_skills', 'diagram_concepts', 'chord_definitions', 'chord_voicings'):
            self.assertIn(f'INSERT INTO {table} ', sql)


if __name__ == '__main__':
    unittest.main()
