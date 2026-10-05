import os
import unittest
from pathlib import Path

import practice_drills as pd

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
CATALOG = SPECS_DIR / 'catalogs/practice-drills.yaml'


def template(key, kind='exercise', response='option_choice', timed=True):
    return {'key': key, 'item_kind': kind, 'response_type': response, 'timed': timed,
            'names': {'en': key, 'pt_BR': key}, 'id': pd.template_id(key)}


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

    def test_a_template_key_appears_once(self):
        self.assertRejected(catalog(templates=[template('exercise:text_response'), template('exercise:text_response')]),
                            'more than once')


class RenderTests(unittest.TestCase):
    def test_the_sql_installs_every_template_and_threshold_with_fixed_ids(self):
        sql = pd.render_sql(pd.build(catalog(
            templates=[template('exercise:text_response'), template('fretboard_cell:name_the_note', 'fretboard_cell', 'name_the_note')],
            thresholds=[threshold('exercise:text_response')])))
        self.assertIn('INSERT INTO "drill_templates"', sql)
        self.assertIn(pd.template_id('fretboard_cell:name_the_note'), sql)
        self.assertIn('INSERT INTO "drill_thresholds"', sql)
        self.assertIn(f"'{pd.threshold_id('exercise:text_response', 1)}', '{pd.template_id('exercise:text_response')}', 1, '2026-10-01T00:00:00Z', 6000, 'default', 0, 0", sql)

    def test_rendering_is_deterministic(self):
        raw = catalog(templates=[template('exercise:text_response')], thresholds=[threshold('exercise:text_response')])
        self.assertEqual(pd.render_sql(pd.build(raw)), pd.render_sql(pd.build(raw)))


class CatalogTests(unittest.TestCase):
    def test_the_specs_catalog_builds(self):
        drills = pd.load(CATALOG)
        self.assertEqual(7, len(drills.templates))
        self.assertEqual({'exercise:text_response', 'exercise:audio_recognition', 'exercise:image_recognition',
                          'exercise:image_choice', 'exercise:audio_selection'},
                         {t.template for t in drills.thresholds})
        self.assertTrue(all(t.source == 'default' and t.version == 1 for t in drills.thresholds))


if __name__ == '__main__':
    unittest.main()
