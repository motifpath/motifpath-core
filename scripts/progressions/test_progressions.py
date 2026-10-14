import os
import unittest
import uuid
from pathlib import Path

import progressions as pg

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
CATALOG = SPECS_DIR / 'catalogs/progressions.yaml'
MAP = SPECS_DIR / 'catalogs/knowledge-map.yaml'
NODES = {'common-progressions', 'cadences', 'blues-form'}


def entry(key, bars, confusable=(), nodes=('common-progressions',)):
    return {'key': key, 'bars': list(bars), 'nodes': list(nodes), 'confusable': list(confusable),
            'names': {'en': key, 'pt_BR': key}, 'id': pg.progression_id(key)}


def catalog(*entries, mode='major'):
    return {'mode': mode, 'progressions': list(entries)}


POP = entry('I-V-vi-IV', ['I', 'V', 'vi', 'IV'], ['I-vi-IV-V'])
DOO_WOP = entry('I-vi-IV-V', ['I', 'vi', 'IV', 'V'], ['I-V-vi-IV'])


class ValidationTests(unittest.TestCase):
    def assertRejected(self, raw, fragment):
        with self.assertRaises(ValueError) as ctx:
            pg.build(raw, NODES)
        self.assertIn(fragment, str(ctx.exception))

    def test_a_valid_catalog_builds_in_catalog_order(self):
        built = pg.build(catalog(POP, DOO_WOP), NODES)
        self.assertEqual(['I-V-vi-IV', 'I-vi-IV-V'], [p.key for p in built])
        self.assertEqual(['I', 'V', 'vi', 'IV'], built[0].bars)
        self.assertEqual(['I-vi-IV-V'], built[0].confusable)

    def test_ids_are_uuid5_of_the_key_under_the_catalog_namespace(self):
        self.assertEqual(str(uuid.uuid5(uuid.UUID('4ac75155-7804-5527-a6ba-01b73c0e3e1a'), 'progression/ii-V-I')),
                         pg.progression_id('ii-V-I'))

    def test_an_id_that_does_not_derive_from_its_key_is_rejected(self):
        bad = dict(POP, id=pg.progression_id('other'))
        self.assertRejected(catalog(bad, DOO_WOP), 'I-V-vi-IV')

    def test_only_major_keys_are_known(self):
        self.assertRejected(catalog(POP, DOO_WOP, mode='minor'), 'minor')

    def test_a_numeral_off_the_major_key_is_rejected(self):
        self.assertRejected(catalog(entry('I-bVII-IV-I', ['I', 'bVII', 'IV', 'I'])), 'bVII')

    def test_a_dominant_seventh_numeral_is_accepted(self):
        built = pg.build(catalog(entry('I7-IV7-I7-V7', ['I7', 'IV7', 'I7', 'V7'])), NODES)
        self.assertEqual(['I7', 'IV7', 'I7', 'V7'], built[0].bars)

    def test_a_progression_is_four_bars_or_a_twelve_bar_form(self):
        self.assertRejected(catalog(entry('I-V-I', ['I', 'V', 'I'])), 'I-V-I')

    def test_a_confusable_progression_must_exist(self):
        self.assertRejected(catalog(POP), 'I-vi-IV-V')

    def test_confusable_progressions_are_listed_both_ways(self):
        one_way = dict(DOO_WOP, confusable=[])
        self.assertRejected(catalog(POP, one_way), 'both ways')

    def test_a_progression_is_never_confusable_with_itself(self):
        self.assertRejected(catalog(entry('ii-V-I', ['ii', 'V', 'I', 'I'], ['ii-V-I'])), 'itself')

    def test_every_node_must_be_on_the_knowledge_map(self):
        self.assertRejected(catalog(entry('ii-V-I', ['ii', 'V', 'I', 'I'], nodes=['jazz-harmony'])), 'jazz-harmony')

    def test_a_key_appears_once(self):
        self.assertRejected(catalog(POP, DOO_WOP, POP), 'more than once')

    def test_a_name_is_needed_in_each_language(self):
        nameless = dict(POP, names={'en': 'pop'})
        self.assertRejected(catalog(nameless, DOO_WOP), 'I-V-vi-IV')


class RenderTests(unittest.TestCase):
    def test_the_sql_installs_every_progression_with_its_fixed_id(self):
        sql = pg.render_sql(pg.build(catalog(POP, DOO_WOP), NODES), {'common-progressions': 'node-1'})
        self.assertIn('INSERT INTO "progressions" ("id", "key", "mode", "bars", "node_ids", "confusable", "names") VALUES', sql)
        self.assertIn(f"('{pg.progression_id('I-V-vi-IV')}', 'I-V-vi-IV', 'major', '[\"I\",\"V\",\"vi\",\"IV\"]', '[\"node-1\"]', '[\"I-vi-IV-V\"]'", sql)

    def test_rendering_is_deterministic(self):
        built = pg.build(catalog(POP, DOO_WOP), NODES)
        ids = {'common-progressions': 'node-1'}
        self.assertEqual(pg.render_sql(built, ids), pg.render_sql(built, ids))


class SpecsCatalogTests(unittest.TestCase):
    # The catalog grows in motifpath-specs on its own schedule, so these check what must hold
    # of it, never its exact contents.
    def test_the_specs_catalog_builds_with_the_common_progressions(self):
        keys = [p.key for p in pg.load(CATALOG, MAP)]
        for key in ('I-V-vi-IV', 'I-vi-IV-V', 'ii-V-I', '12-bar-blues'):
            self.assertIn(key, keys)


if __name__ == '__main__':
    unittest.main()
