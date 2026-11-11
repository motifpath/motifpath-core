"""Build the godog tag expression the BDD runners use.

motifpath-specs runs ahead of this repo: a scenario is specified first and
implemented here later. A scenario a ticket will implement carries that
ticket's tag (`@MOT-187`) for good. This repo lists the tickets it implements
in bdd/implemented-tickets.txt; every ticket tag found in the specs but not
listed is skipped, so unrelated work stays green while the runners stay
strict for everything else. Adding a ticket to the list, in the same PR that
implements it, is what switches its scenarios on.

`@wip` (parked: specified, no ticket scheduled) and `@web` (client behaviour,
pinned by motifpath-web's own tests) are always skipped.

Usage: bdd_tags.py <specs features dir> <implemented-tickets file>
Prints the expression on stdout and the still-pending tickets on stderr.
"""
import re
import sys
from pathlib import Path

TICKET = re.compile(r'^@?(MOT-\d+)$')
ALWAYS_SKIPPED = ('wip', 'web')


def ticket_tags(features_dir):
    """Every ticket tag on a feature or scenario under features_dir."""
    tickets = set()
    for path in Path(features_dir).rglob('*.feature'):
        for line in path.read_text(encoding='utf-8').splitlines():
            stripped = line.strip()
            if not stripped.startswith('@'):
                continue
            for tag in stripped.split():
                match = TICKET.match(tag)
                if match and tag.startswith('@'):
                    tickets.add(match.group(1))
    return tickets


def implemented_tickets(path):
    """The tickets listed in path, one per line; '#' starts a comment."""
    tickets = set()
    for number, line in enumerate(Path(path).read_text(encoding='utf-8').splitlines(), 1):
        entry = line.split('#', 1)[0].strip()
        if not entry:
            continue
        match = TICKET.match(entry)
        if not match:
            raise ValueError(f'{path}:{number}: {entry!r} is not a ticket (expected MOT-NNN)')
        tickets.add(match.group(1))
    return tickets


def _ticket_number(ticket):
    return int(ticket.split('-', 1)[1])


def tag_expression(pending):
    """The godog expression skipping @wip, @web and every pending ticket."""
    skipped = [f'@{tag}' for tag in ALWAYS_SKIPPED]
    skipped += [f'@{ticket}' for ticket in sorted(pending, key=_ticket_number)]
    return ' && '.join(f'~{tag}' for tag in skipped)


def main(argv):
    if len(argv) != 2:
        print(__doc__.strip().splitlines()[-2], file=sys.stderr)
        return 2
    features_dir, listed = argv
    if not Path(features_dir).is_dir():
        print(f'bdd_tags: features directory {features_dir} not found', file=sys.stderr)
        return 1
    try:
        pending = ticket_tags(features_dir) - implemented_tickets(listed)
    except (OSError, ValueError) as err:
        print(f'bdd_tags: {err}', file=sys.stderr)
        return 1
    if pending:
        names = ', '.join(sorted(pending, key=_ticket_number))
        print(f'bdd_tags: skipping scenarios of tickets not implemented here yet: {names}',
              file=sys.stderr)
    print(tag_expression(pending))
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
