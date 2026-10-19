#!/usr/bin/env perl
# Tests for openapi-compat.pl. Run with: prove scripts/openapi-compat.t
use strict;
use warnings;
use File::Basename qw(dirname);
use File::Temp qw(tempfile);
use Test::More;

my $script = dirname(__FILE__) . '/openapi-compat.pl';

# The two update schemas as the bundler writes them, trimmed to the
# tri-state fields and one ordinary field each.
my $bundle = <<'YAML';
components:
  schemas:
    UpdateDiagramRequest:
      type: object
      properties:
        color:
          type: string
        mode:
          oneOf:
            - $ref: '#/components/schemas/DiagramMode'
            - type: 'null'
          description: The diagram's mode.
        default_playback_id:
          type:
            - string
            - 'null'
          format: uuid
    UpdateKnowledgeNodeRequest:
      minProperties: 1
      properties:
        descriptions:
          oneOf:
            - $ref: '#/components/schemas/LocalizedDescription'
            - type: 'null'
        parent_id:
          type:
            - string
            - 'null'
          format: uuid
YAML

# Runs the script on $spec; returns its exit status, rewritten file and stderr.
sub run_compat {
    my ($spec) = @_;
    my ($fh, $path) = tempfile(UNLINK => 1);
    print $fh $spec;
    close($fh);
    my $stderr = `perl $script $path 2>&1 >/dev/null`;
    my $status = $? >> 8;
    open(my $in, '<', $path) or die "read $path: $!\n";
    local $/;
    my $out = <$in>;
    close($in);
    return ($status, $out, $stderr);
}

subtest 'marks every tri-state field as nullable.Nullable' => sub {
    my ($status, $out) = run_compat($bundle);
    is($status, 0, 'exits 0');
    for my $field (qw(mode default_playback_id descriptions parent_id)) {
        like($out, qr/^\s+$field:\n\s+x-go-type: nullable\.Nullable\[/m, "$field decodes into nullable.Nullable");
    }
    unlike($out, qr/^\s+color:\n\s+x-go-type/m, 'leaves ordinary fields alone');
};

subtest 'replaces the nullable references with a plain type' => sub {
    my ($status, $out) = run_compat($bundle);
    is($status, 0, 'exits 0');
    unlike($out, qr/DiagramMode'\n\s+- type: .null./, 'mode no longer refers to DiagramMode');
    unlike($out, qr/LocalizedDescription'\n\s+- type: .null./, 'descriptions no longer refers to LocalizedDescription');
};

subtest 'refuses a bundle missing a tri-state field' => sub {
    (my $spec = $bundle) =~ s/^        parent_id:\n(?:          .*\n)*//m;
    my ($status, undef, $stderr) = run_compat($spec);
    isnt($status, 0, 'exits non-zero');
    like($stderr, qr/UpdateKnowledgeNodeRequest\.parent_id/, 'names the missing field');
};

subtest 'refuses a bundle missing a schema' => sub {
    (my $spec = $bundle) =~ s/^    UpdateDiagramRequest:\n(?:      .*\n)*//m;
    my ($status, undef, $stderr) = run_compat($spec);
    isnt($status, 0, 'exits non-zero');
    like($stderr, qr/UpdateDiagramRequest/, 'names the missing schema');
};

subtest 'refuses a nullable reference laid out differently' => sub {
    (my $spec = $bundle) =~ s{oneOf:\n\s+- \$ref: '#/components/schemas/DiagramMode'\n\s+- type: 'null'\n}{oneOf: [{\$ref: '#/components/schemas/DiagramMode'}, {type: 'null'}]\n};
    my ($status, undef, $stderr) = run_compat($spec);
    isnt($status, 0, 'exits non-zero');
    like($stderr, qr/UpdateDiagramRequest\.mode/, 'names the field it could not rewrite');
};

done_testing();
