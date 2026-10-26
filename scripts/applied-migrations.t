#!/usr/bin/env perl
# Tests for applied-migrations.sh. Run with: prove scripts/applied-migrations.t
use strict;
use warnings;
use Cwd qw(abs_path);
use File::Basename qw(dirname);
use File::Path qw(make_path);
use File::Temp qw(tempdir);
use Test::More;

my $script = abs_path(dirname(__FILE__) . '/applied-migrations.sh');
my $dir = 'migrations';

# repo is a fresh git repository whose main branch holds two applied
# migrations, with a feature branch checked out from it.
sub repo {
    my $root = tempdir(CLEANUP => 1);
    chdir $root or die;
    git('init', '-q', '-b', 'main');
    git('config', 'user.email', 'test@example.com');
    git('config', 'user.name', 'Test');
    make_path($dir);
    write_file("$dir/001_schema.up.sql", "CREATE TABLE a (id int);\n");
    write_file("$dir/002_data.up.sql", "INSERT INTO a VALUES (1);\n");
    write_file("$dir/atlas.sum", "h1\n");
    commit('applied');
    git('checkout', '-q', '-b', 'feature');
    return $root;
}

sub git { system('git', @_) == 0 or die "git @_ failed" }

sub write_file {
    my ($path, $content) = @_;
    open my $fh, '>', $path or die "$path: $!";
    print $fh $content;
    close $fh;
}

sub commit {
    git('add', '-A');
    git('commit', '-q', '-m', shift);
}

# check runs the script against main and returns its exit code and output.
sub check {
    my $out = `bash '$script' main '$dir' 2>&1`;
    return ($? >> 8, $out);
}

{
    repo();
    write_file("$dir/003_more.up.sql", "INSERT INTO a VALUES (2);\n");
    write_file("$dir/atlas.sum", "h2\n");
    commit('add a migration');
    my ($code, $out) = check();
    is($code, 0, 'a new migration, and atlas.sum rehashed for it, pass') or diag $out;
}

{
    repo();
    write_file("$dir/002_data.up.sql", "INSERT INTO a VALUES (1), (2);\n");
    commit('edit an applied migration');
    my ($code, $out) = check();
    is($code, 1, 'editing a migration main has fails');
    like($out, qr/002_data\.up\.sql/, 'and names it');
}

{
    repo();
    unlink "$dir/001_schema.up.sql" or die;
    commit('delete an applied migration');
    my ($code, $out) = check();
    is($code, 1, 'deleting a migration main has fails');
    like($out, qr/001_schema\.up\.sql/, 'and names it');
}

{
    repo();
    write_file("$dir/002_data.up.sql", "INSERT INTO a VALUES (1), (2);\n");
    commit('edit');
    write_file("$dir/002_data.up.sql", "INSERT INTO a VALUES (1);\n");
    commit('put it back');
    my ($code, $out) = check();
    is($code, 0, 'an edit put back before merging passes') or diag $out;
}

{
    repo();
    git('checkout', '-q', 'main');
    write_file("$dir/003_hotfix.up.sql", "INSERT INTO a VALUES (9);\n");
    commit('hotfix migration on main');
    git('checkout', '-q', 'feature');
    my ($code, $out) = check();
    is($code, 0, "a migration added on main after the branch started isn't the branch's change") or diag $out;
}

done_testing();
