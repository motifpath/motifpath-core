import io
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path

import bdd_tags


def write(root, name, text):
    path = Path(root) / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    return path


class TicketTagsTests(unittest.TestCase):
    def test_collects_feature_and_scenario_ticket_tags_from_nested_files(self):
        with tempfile.TemporaryDirectory() as root:
            write(root, 'practice/a.feature', '@MOT-52\nFeature: A\n\n  Scenario: x\n    Given y\n')
            write(root, 'content/b.feature', 'Feature: B\n\n  @web @MOT-187\n  Scenario: x\n    Given y\n')
            self.assertEqual(bdd_tags.ticket_tags(root), {'MOT-52', 'MOT-187'})

    def test_ignores_comments_step_text_and_other_tags(self):
        with tempfile.TemporaryDirectory() as root:
            write(root, 'a.feature', '# stays @MOT-1 until core ships\n@wip @web\nFeature: A\n'
                                     '  Scenario: x\n    Given "bob@MOT-2.com" signs in\n')
            self.assertEqual(bdd_tags.ticket_tags(root), set())

    def test_only_reads_feature_files(self):
        with tempfile.TemporaryDirectory() as root:
            write(root, 'README.md', '@MOT-9\n')
            self.assertEqual(bdd_tags.ticket_tags(root), set())


class ImplementedTicketsTests(unittest.TestCase):
    def test_reads_one_ticket_per_line_skipping_comments_and_blanks(self):
        with tempfile.TemporaryDirectory() as root:
            path = write(root, 'implemented-tickets.txt',
                         '# tickets core implements\n\nMOT-52\n@MOT-53  # calibration\n')
            self.assertEqual(bdd_tags.implemented_tickets(path), {'MOT-52', 'MOT-53'})

    def test_refuses_a_line_that_is_not_a_ticket(self):
        with tempfile.TemporaryDirectory() as root:
            path = write(root, 'implemented-tickets.txt', 'MOT-52\nmot 53\n')
            with self.assertRaises(ValueError) as ctx:
                bdd_tags.implemented_tickets(path)
            self.assertIn('mot 53', str(ctx.exception))


class TagExpressionTests(unittest.TestCase):
    def test_without_pending_tickets_skips_only_wip_and_web(self):
        self.assertEqual(bdd_tags.tag_expression(set()), '~@wip && ~@web')

    def test_skips_each_pending_ticket_in_numeric_order(self):
        self.assertEqual(bdd_tags.tag_expression({'MOT-187', 'MOT-52'}),
                         '~@wip && ~@web && ~@MOT-52 && ~@MOT-187')


class MainTests(unittest.TestCase):
    def test_prints_the_expression_and_reports_pending_tickets(self):
        with tempfile.TemporaryDirectory() as root:
            features = Path(root) / 'features'
            write(features, 'a.feature', '@MOT-52\nFeature: A\n')
            write(features, 'b.feature', '@MOT-53\nFeature: B\n')
            listed = write(root, 'implemented-tickets.txt', 'MOT-53\n')
            out, err = io.StringIO(), io.StringIO()
            with redirect_stdout(out), redirect_stderr(err):
                status = bdd_tags.main([str(features), str(listed)])
            self.assertEqual(status, 0)
            self.assertEqual(out.getvalue(), '~@wip && ~@web && ~@MOT-52\n')
            self.assertIn('MOT-52', err.getvalue())
            self.assertNotIn('MOT-53', err.getvalue())

    def test_fails_when_the_features_directory_is_missing(self):
        with tempfile.TemporaryDirectory() as root:
            listed = write(root, 'implemented-tickets.txt', '')
            err = io.StringIO()
            with redirect_stderr(err), redirect_stdout(io.StringIO()):
                status = bdd_tags.main([str(Path(root) / 'nowhere'), str(listed)])
            self.assertEqual(status, 1)
            self.assertIn('nowhere', err.getvalue())


if __name__ == '__main__':
    unittest.main()
