#!/usr/bin/env perl
# Rewrites the bundled core-domain OpenAPI file in place for the few fields
# oapi-codegen can't type from the spec alone. Only the local, gitignored
# bundle is touched: the spec in motifpath-specs stays free of Go extensions.
#
# Usage: perl scripts/openapi-compat.pl .bundled/core-domain-service.yaml
use strict;
use warnings;

local $/;
my $path = shift or die "usage: $0 <bundled-spec.yaml>\n";
open(my $in, '<', $path) or die "read $path: $!\n";
my $spec = <$in>;
close($in);

# A nullable reference ("oneOf: [$ref, {type: null}]") generates a union type
# that x-go-type can't override, so the tri-state fields below declare a plain
# type instead; the domain still validates the value.
sub plain_type {
    my ($block, $ref, $type) = @_;
    $block =~ s/^(\s+)oneOf:\n\s+- \$ref: '#\/components\/schemas\/$ref'\n\s+- type: .null.\n/$1type: $type\n/m;
    return $block;
}

# Marks each named field of a schema block to decode into nullable.Nullable.
sub nullable_fields {
    my ($block, %go_type) = @_;
    my $names = join('|', keys %go_type);
    $block =~ s/^(\s+)($names):\n/$1$2:\n$1  x-go-type: nullable.Nullable[$go_type{$2}]\n$1  x-go-type-import:\n$1    path: github.com\/oapi-codegen\/nullable\n$1  x-go-type-skip-optional-pointer: true\n/mg;
    return $block;
}

# UpdateDiagramRequest's mode and default_playback_id have three states:
# omitted leaves the value unchanged, null clears it, a value replaces it. A
# plain pointer can't tell omitted from null, so these two fields decode into
# nullable.Nullable.
$spec =~ s{(^    UpdateDiagramRequest:\n(?:(?!    \S).*\n)*)}{
    nullable_fields(plain_type($1, 'DiagramMode', 'string'),
        mode => 'DiagramMode', default_playback_id => 'openapi_types.UUID');
}me;

# UpdateKnowledgeNodeRequest's descriptions and parent_id have the same
# three states, so they decode into nullable.Nullable the same way.
$spec =~ s{(^    UpdateKnowledgeNodeRequest:\n(?:(?!    \S).*\n)*)}{
    nullable_fields(plain_type($1, 'LocalizedDescription', 'object'),
        descriptions => 'LocalizedDescription', parent_id => 'openapi_types.UUID');
}me;

open(my $out, '>', $path) or die "write $path: $!\n";
print $out $spec;
close($out);
