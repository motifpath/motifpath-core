#!/usr/bin/env perl
# Tests for next-release.sh. Run with: prove scripts/next-release.t
use strict;
use warnings;
use Cwd qw(abs_path);
use File::Basename qw(dirname);
use File::Temp qw(tempdir);
use Test::More;

my $script = abs_path(dirname(__FILE__) . '/next-release.sh');

sub git { system('git', @_) == 0 or die "git @_ failed" }

# repo is a fresh git repository with one commit carrying the given tags.
sub repo {
    my $root = tempdir(CLEANUP => 1);
    chdir $root or die;
    git('init', '-q', '-b', 'main');
    git('config', 'user.email', 'test@example.com');
    git('config', 'user.name', 'Test');
    git('commit', '-q', '--allow-empty', '-m', 'release');
    git('tag', $_) for @_;
}

# release runs the script with the merged PR's labels and returns its exit
# code and output.
sub release {
    my $args = join ' ', map { "'$_'" } @_;
    my $out = `bash '$script' $args 2>&1`;
    chomp $out;
    return ($? >> 8, $out);
}

repo('v0.6.0', 'v0.7.0');
is_deeply([release('release:patch')], [0, 'v0.7.1'], 'patch bumps the newest tag');
is_deeply([release('release:minor')], [0, 'v0.8.0'], 'minor resets the patch');
is_deeply([release('release:major')], [0, 'v1.0.0'], 'major resets minor and patch');
is_deeply([release('automated', 'release:minor', 'sync')], [0, 'v0.8.0'], 'other labels are ignored');

repo('v0.9.0', 'v0.10.0');
is_deeply([release('release:minor')], [0, 'v0.11.0'], 'the newest tag is chosen by version, not by name');

repo('v0.7.0', 'v0.9.0-rc.1', 'v2', 'nightly');
is_deeply([release('release:patch')], [0, 'v0.7.1'], 'only vX.Y.Z tags count');

repo();
is_deeply([release('release:patch')], [0, 'v0.0.1'], 'the first release counts from v0.0.0');

repo('v0.7.0');
my ($code, $out) = release();
is($code, 1, 'no label fails');
like($out, qr/release:major, release:minor or release:patch/, 'and names the labels');

($code, $out) = release('bug');
is($code, 1, 'no release label among others fails');

($code, $out) = release('release:minor', 'release:patch');
is($code, 1, 'two release labels fail');
like($out, qr/exactly one/, 'and says only one is allowed');

($code, $out) = release('release:huge');
is($code, 1, 'an unknown release label fails');

done_testing();
