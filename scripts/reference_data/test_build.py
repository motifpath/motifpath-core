import datetime
import os
import tempfile
import unittest
from pathlib import Path

import build

SPECS_DIR = Path(os.environ.get('SPECS_DIR', Path(__file__).resolve().parents[3] / 'motifpath-specs'))
CATALOGS = SPECS_DIR / 'catalogs'


@unittest.skipUnless((CATALOGS / 'knowledge-map.yaml').exists(), f'{CATALOGS} not found; set SPECS_DIR')
class BuildTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.sql = build.render(SPECS_DIR)

    def test_sections_come_in_dependency_order(self):
        starts = [self.sql.index(build.section_marker(name)) for name in build.SECTIONS]
        self.assertEqual(starts, sorted(starts))
        self.assertEqual(build.SECTIONS, [
            'languages-and-voices', 'catalog-instruments', 'instrument-icons', 'knowledge-map',
            'basic-guitar-catalog', 'practice-drills', 'fretboard-cells'])

    def test_languages_and_voices_are_carried_over(self):
        carried = build.section(self.sql, 'languages-and-voices')
        for code in ("'en'", "'pt_BR'", "'any'"):
            self.assertIn(code, carried)
        for voice in ("'acoustic-guitar'", "'piano'", "'electric-bass'"):
            self.assertIn(voice, carried)

    def test_each_generated_section_is_its_generator_output(self):
        self.assertIn('INSERT INTO diagrams', build.section(self.sql, 'basic-guitar-catalog'))
        self.assertIn('playbacks,default_playback_id', build.section(self.sql, 'basic-guitar-catalog'))
        self.assertIn('INSERT INTO "drill_thresholds"', build.section(self.sql, 'practice-drills'))
        self.assertNotIn('INSERT INTO "drill_thresholds"', build.section(self.sql, 'fretboard-cells'))

    def test_installed_threshold_versions_are_kept(self):
        installed = build.practice_drills.installed_threshold_ids(build.section(self.sql, 'practice-drills'))
        later = build.render(SPECS_DIR, installed, today=datetime.date(2027, 1, 1))
        self.assertEqual(build.section(later, 'practice-drills'), build.section(self.sql, 'practice-drills'))

    def test_a_backdated_threshold_nothing_installed_is_refused(self):
        with self.assertRaises(ValueError):
            build.render(SPECS_DIR, today=datetime.date(2027, 1, 1))

    def test_an_older_migration_counts_as_installed(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations, catalog_output = Path(tmp) / 'migrations', Path(tmp) / 'catalog'
            self.schema_baseline(migrations)
            old = Path(tmp) / 'old_practice_drills.up.sql'
            old.write_text(build.section(self.sql, 'practice-drills'))
            written = build.write(SPECS_DIR, migrations, catalog_output, today=datetime.date(2027, 1, 1), installed_from=old)
            self.assertEqual(build.section(written.read_text(), 'practice-drills'), build.section(self.sql, 'practice-drills'))

    def test_a_missing_section_is_an_error(self):
        with self.assertRaises(ValueError):
            build.section('-- nothing here\n', 'practice-drills')

    def schema_baseline(self, migrations):
        migrations.mkdir()
        (migrations / '20261006100000_baseline_schema.up.sql').write_text('-- schema\n')

    def test_writes_the_reference_data_right_after_the_schema_baseline(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations, catalog_output = Path(tmp) / 'migrations', Path(tmp) / 'catalog'
            self.schema_baseline(migrations)
            written = build.write(SPECS_DIR, migrations, catalog_output)
            self.assertEqual(written.name, '20261006100001_baseline_reference_data.up.sql')
            self.assertEqual(sorted(p.name for p in migrations.iterdir()), ['20261006100000_baseline_schema.up.sql', written.name])
            self.assertEqual(sorted(p.name for p in catalog_output.iterdir()), ['catalog.json', 'coverage.json'])

    def test_without_a_schema_baseline_nothing_is_written(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations = Path(tmp) / 'migrations'
            migrations.mkdir()
            with self.assertRaises(ValueError):
                build.write(SPECS_DIR, migrations, Path(tmp) / 'catalog')
            self.assertEqual(list(migrations.iterdir()), [])

    def test_rewriting_keeps_the_file_identical(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations, catalog_output = Path(tmp) / 'migrations', Path(tmp) / 'catalog'
            self.schema_baseline(migrations)
            first = build.write(SPECS_DIR, migrations, catalog_output).read_text()
            second = build.write(SPECS_DIR, migrations, catalog_output, today=datetime.date(2027, 1, 1)).read_text()
            self.assertEqual(first, second)

    def test_writes_the_chord_catalog_right_after_its_tables(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations, chord_output = Path(tmp) / 'migrations', Path(tmp) / 'chords'
            self.schema_baseline(migrations)
            (migrations / '20261006224957_chord_catalog.up.sql').write_text('-- chord tables\n')
            written = build.write_chords(SPECS_DIR, migrations, chord_output)
            self.assertEqual(written.name, '20261006224958_chord_catalog_reference_data.up.sql')
            sql = written.read_text()
            self.assertTrue(sql.startswith('-- Generated by scripts/reference_data/build.py. Never edit it by hand.'))
            self.assertIn('INSERT INTO chord_voicings', sql)
            self.assertEqual(sorted(p.name for p in chord_output.iterdir()), ['catalog.json', 'coverage.json'])
            self.assertEqual(written.read_text(), build.write_chords(SPECS_DIR, migrations, chord_output).read_text())

    def test_without_the_chord_tables_no_chord_catalog_is_written(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations = Path(tmp) / 'migrations'
            self.schema_baseline(migrations)
            with self.assertRaises(ValueError):
                build.write_chords(SPECS_DIR, migrations, Path(tmp) / 'chords')
            self.assertEqual([p.name for p in migrations.iterdir()], ['20261006100000_baseline_schema.up.sql'])

    def test_writes_the_diagram_shapes_right_after_their_tables(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations = Path(tmp) / 'migrations'
            self.schema_baseline(migrations)
            (migrations / '20261007100000_diagram_shapes.up.sql').write_text('-- shape tables\n')
            written = build.write_shapes(SPECS_DIR, migrations)
            self.assertEqual(written.name, '20261007100001_diagram_shapes_reference_data.up.sql')
            sql = written.read_text()
            self.assertTrue(sql.startswith('-- Generated by scripts/reference_data/build.py. Never edit it by hand.'))
            self.assertIn('INSERT INTO "diagram_shape_families"', sql)
            self.assertIn('INSERT INTO "diagram_shapes"', sql)
            self.assertEqual(sql, build.write_shapes(SPECS_DIR, migrations).read_text())

    def test_without_the_shape_tables_no_shapes_are_written(self):
        with tempfile.TemporaryDirectory() as tmp:
            migrations = Path(tmp) / 'migrations'
            self.schema_baseline(migrations)
            with self.assertRaises(ValueError):
                build.write_shapes(SPECS_DIR, migrations)
            self.assertEqual([p.name for p in migrations.iterdir()], ['20261006100000_baseline_schema.up.sql'])


if __name__ == '__main__':
    unittest.main()
