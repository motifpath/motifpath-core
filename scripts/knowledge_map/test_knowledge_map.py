import os
import unittest
from pathlib import Path

import knowledge_map as km

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
MAP = SPECS_DIR / 'catalogs/knowledge-map.yaml'


def node(key, kind='skill', parent=None, instruments='all', applies=None, level=None, unapplied=None):
    n = {'key': key, 'names': {'en': key, 'pt_BR': key}, 'instruments': instruments}
    if parent:
        n['parent'] = parent
    if applies:
        n['applies'] = applies
    if level:
        n['level'] = level
    if unapplied is not None:
        n['unapplied'] = unapplied
    return n


def catalog(concepts=(), skills=(), requires=()):
    return {'instruments': {'guitars': ['guitar', 'electric-guitar'], 'acoustic': ['guitar'], 'bass': ['electric-bass']},
            'concepts': list(concepts), 'skills': list(skills), 'requires': list(requires)}


class ValidationTests(unittest.TestCase):
    def assertRejected(self, raw, fragment):
        with self.assertRaises(ValueError) as ctx:
            km.build(raw)
        self.assertIn(fragment, str(ctx.exception))

    def test_a_valid_map_builds(self):
        graph = km.build(catalog(
            concepts=[node('scales', 'concept'), node('pentatonic-shapes', 'concept', 'scales', 'guitars')],
            skills=[node('play-pentatonic', applies=['pentatonic-shapes'], instruments='guitars')],
            requires=[{'from': 'play-pentatonic', 'to': 'scales', 'level': 'accurate'}]))
        self.assertEqual(['scales', 'pentatonic-shapes', 'play-pentatonic'], [n.key for n in graph.nodes])
        self.assertEqual([], graph.nodes[0].instrument_ids)
        self.assertEqual([km.instrument_id('guitar'), km.instrument_id('electric-guitar')], graph.nodes[1].instrument_ids)
        self.assertEqual(km.stable_id('knowledge-node/scales'), graph.nodes[0].id)
        self.assertEqual(km.stable_id('knowledge-edge/requires/play-pentatonic/scales'), graph.edges[-1].id)

    def test_parents_install_before_children(self):
        graph = km.build(catalog(concepts=[node('child', 'concept', 'root'), node('root', 'concept')],
                                 skills=[node('s', applies=['child'])]))
        self.assertEqual(['root', 'child', 's'], [n.key for n in graph.nodes])

    def test_rejections(self):
        cases = [
            ('a duplicate key', catalog(concepts=[node('x', 'concept')], skills=[node('x')]), 'more than once'),
            ('a key that is not kebab-case', catalog(skills=[node('Play X')]), 'kebab-case'),
            ('a missing translation', catalog(skills=[{'key': 'x', 'names': {'en': 'X'}, 'instruments': 'all'}]), 'pt_BR'),
            ('an unknown parent', catalog(skills=[node('x', parent='ghost')]), 'ghost'),
            ('a parent of the other kind', catalog(concepts=[node('c', 'concept')], skills=[node('s', parent='c')]), 'kind'),
            ('a child wider than its parent', catalog(skills=[node('p', instruments='bass'), node('c', parent='p')]), 'wider'),
            ('an unknown instrument alias', catalog(skills=[node('x', instruments='banjo')]), 'banjo'),
            ('applies to a skill', catalog(skills=[node('a'), node('b', applies=['a'])]), 'concept'),
            ('requires without a level', catalog(skills=[node('a'), node('b')], requires=[{'from': 'a', 'to': 'b'}]), 'level'),
            ('a requires cycle', catalog(concepts=[node('c', 'concept')], skills=[node('a', applies=['c']), node('b', applies=['c'])], requires=[
                {'from': 'a', 'to': 'b', 'level': 'fluent'}, {'from': 'b', 'to': 'a', 'level': 'fluent'}]), 'cycle'),
            ('requires between nodes sharing no instrument', catalog(
                skills=[node('a', instruments='guitars'), node('b', instruments='bass')],
                requires=[{'from': 'a', 'to': 'b', 'level': 'fluent'}]), 'share'),
        ]
        for name, raw, fragment in cases:
            with self.subTest(name):
                self.assertRejected(raw, fragment)

    def test_classification_and_readiness_reach_every_instrument(self):
        theory = node('theory', 'concept')
        cases = [
            ('a leaf skill with no concept', catalog(concepts=[theory], skills=[node('s', applies=['theory']), node('t')]),
             "'t' applies no concept for"),
            ('a leaf skill whose concepts miss one of its instruments', catalog(
                concepts=[theory, node('shapes', 'concept', instruments='guitars')],
                skills=[node('s', applies=['theory']), node('t', instruments='guitars', applies=['shapes']),
                        node('u', applies=['shapes'])]),
             "'u' applies no concept for 'electric-bass'"),
            ('a reason to apply nothing, with concepts too', catalog(
                concepts=[theory], skills=[node('s', applies=['theory'], unapplied='a habit')]), 'not both'),
            ('a blank reason to apply nothing', catalog(concepts=[theory], skills=[node('s', applies=['theory']), node('t', unapplied=' ')]),
             'not both'),
            ('a reason to apply nothing on a concept', catalog(
                concepts=[node('c', 'concept', unapplied='why')], skills=[node('s', applies=['c'])]), 'only a leaf skill'),
            ('an unreachable concept', catalog(concepts=[theory, node('modes', 'concept', level='I')],
                                               skills=[node('s', applies=['theory'])]),
             "'modes' is neither applied nor required"),
            ('a requires target with no leaf where the edge counts', catalog(
                concepts=[theory],
                skills=[node('fingerstyle', instruments='guitars'),
                        node('pima', 'skill', 'fingerstyle', instruments='acoustic', applies=['theory']),
                        node('hybrid', instruments='guitars', applies=['theory'])],
                requires=[{'from': 'hybrid', 'to': 'fingerstyle', 'level': 'accurate'}]),
             "counts for 'electric-guitar'"),
            ('requires that count for one instrument only', catalog(
                concepts=[theory],
                skills=[node('a', instruments='guitars', applies=['theory']), node('b', applies=['theory'])],
                requires=[{'from': 'b', 'to': 'a', 'level': 'accurate'}]),
             "'b' requires nothing that counts for 'electric-bass'"),
        ]
        for name, raw, fragment in cases:
            with self.subTest(name):
                self.assertRejected(raw, fragment)

    def test_placeholders_and_explained_skills_build(self):
        graph = km.build(catalog(
            concepts=[node('theory', 'concept'), node('extensions', 'concept', level='A')],
            skills=[node('s', applies=['theory']), node('plan', unapplied='a routine organises other skills')]))
        self.assertEqual(['theory', 'extensions', 's', 'plan'], [n.key for n in graph.nodes])


class InstrumentIconTests(unittest.TestCase):
    def test_every_catalog_instrument_gets_its_icon_by_its_fixed_id(self):
        sql = km.render_instrument_icons_sql()
        for key, icon in (('guitar', 'acoustic_guitar'), ('electric-guitar', 'electric_guitar'), ('electric-bass', 'electric_bass')):
            self.assertIn(f"""UPDATE "instruments" SET "icon" = '{icon}' WHERE "id" = '{km.instrument_id(key)}';""", sql)


@unittest.skipUnless(MAP.exists(), f'{MAP} not found; set SPECS_DIR')
class CatalogTests(unittest.TestCase):
    def test_the_reviewed_map_builds_with_its_counts(self):
        graph = km.load(MAP)
        kinds = [n.kind for n in graph.nodes]
        self.assertEqual((219, 125), (kinds.count('skill'), kinds.count('concept')))
        types = [e.type for e in graph.edges]
        self.assertEqual((366, 144), (types.count('applies'), types.count('requires')))

    def test_rendering_is_deterministic(self):
        self.assertEqual(km.render_sql(km.load(MAP)), km.render_sql(km.load(MAP)))


if __name__ == '__main__':
    unittest.main()
