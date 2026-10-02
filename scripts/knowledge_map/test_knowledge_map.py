import os
import unittest
from pathlib import Path

import knowledge_map as km

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
MAP = SPECS_DIR / 'catalogs/knowledge-map.yaml'


def node(key, kind='skill', parent=None, instruments='all', applies=None):
    n = {'key': key, 'names': {'en': key, 'pt_BR': key}, 'instruments': instruments}
    if parent:
        n['parent'] = parent
    if applies:
        n['applies'] = applies
    return n


def catalog(concepts=(), skills=(), requires=()):
    return {'instruments': {'guitars': ['guitar', 'electric-guitar'], 'bass': ['electric-bass']},
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
        graph = km.build(catalog(concepts=[node('child', 'concept', 'root'), node('root', 'concept')]))
        self.assertEqual(['root', 'child'], [n.key for n in graph.nodes])

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
            ('a requires cycle', catalog(skills=[node('a'), node('b')], requires=[
                {'from': 'a', 'to': 'b', 'level': 'fluent'}, {'from': 'b', 'to': 'a', 'level': 'fluent'}]), 'cycle'),
            ('requires between nodes sharing no instrument', catalog(
                skills=[node('a', instruments='guitars'), node('b', instruments='bass')],
                requires=[{'from': 'a', 'to': 'b', 'level': 'fluent'}]), 'share'),
        ]
        for name, raw, fragment in cases:
            with self.subTest(name):
                self.assertRejected(raw, fragment)


@unittest.skipUnless(MAP.exists(), f'{MAP} not found; set SPECS_DIR')
class CatalogTests(unittest.TestCase):
    def test_the_reviewed_map_builds_with_its_counts(self):
        graph = km.load(MAP)
        kinds = [n.kind for n in graph.nodes]
        self.assertEqual((216, 124), (kinds.count('skill'), kinds.count('concept')))
        types = [e.type for e in graph.edges]
        self.assertEqual((335, 136), (types.count('applies'), types.count('requires')))

    def test_rendering_is_deterministic(self):
        self.assertEqual(km.render_sql(km.load(MAP)), km.render_sql(km.load(MAP)))


if __name__ == '__main__':
    unittest.main()
