#!/usr/bin/env perl
# Rewrites the bundled core-domain OpenAPI file in place for the few fields
# oapi-codegen can't type from the spec alone. Only the local, gitignored
# bundle is touched: the spec in motifpath-specs stays free of Go extensions.
#
# Every rewrite must match: if the bundler's output changes shape, or a field
# is renamed in the spec, the script stops instead of silently generating a
# plain pointer that can't tell an omitted field from null.
#
# Usage: perl scripts/openapi-compat.pl .bundled/core-domain-service.yaml
use strict;
use warnings;

local $/;
my $path = shift or die "usage: $0 <bundled-spec.yaml>\n";
open(my $in, '<', $path) or die "read $path: $!\n";
my $spec = <$in>;
close($in);

# Returns the schema block named $schema, or stops if the bundle lacks it.
sub schema_block {
    my ($schema) = @_;
    $spec =~ /(^    \Q$schema\E:\n(?:(?!    \S).*\n)*)/m
        or die "openapi-compat: schema $schema not found in $path\n";
    return $1;
}

# A nullable reference ("oneOf: [$ref, {type: null}]") generates a union type
# that x-go-type can't override, so a tri-state field declares a plain type
# instead; the domain still validates the value.
sub plain_type {
    my ($schema, $block, $field, $ref, $type) = @_;
    $block =~ s/^(\s+$field:\n)(\s+)oneOf:\n\s+- \$ref: '#\/components\/schemas\/$ref'\n\s+- type: .null.\n/$1$2type: $type\n/m
        or die "openapi-compat: $schema.$field is not a nullable $ref reference in $path\n";
    return $block;
}

# Marks each named field of a schema block to decode into nullable.Nullable.
sub nullable_fields {
    my ($schema, $block, %go_type) = @_;
    for my $field (sort keys %go_type) {
        $block =~ s/^(\s+)$field:\n/$1$field:\n$1  x-go-type: nullable.Nullable[$go_type{$field}]\n$1  x-go-type-import:\n$1    path: github.com\/oapi-codegen\/nullable\n$1  x-go-type-skip-optional-pointer: true\n/m
            or die "openapi-compat: $schema.$field not found in $path\n";
    }
    return $block;
}

# Rewrites one schema's tri-state fields: omitted leaves the value unchanged,
# null clears it, a value replaces it. A plain pointer can't tell omitted from
# null, so these fields decode into nullable.Nullable.
sub tri_state {
    my ($schema, $reference, %go_type) = @_;
    my $block = schema_block($schema);
    my $rewritten = nullable_fields($schema, plain_type($schema, $block, @$reference), %go_type);
    $spec =~ s/\Q$block\E/$rewritten/;
}

tri_state('UpdateDiagramRequest', [mode => 'DiagramMode', 'string'],
    mode => 'DiagramMode', default_playback_id => 'openapi_types.UUID');

tri_state('UpdateKnowledgeNodeRequest', [descriptions => 'LocalizedDescription', 'object'],
    descriptions => 'LocalizedDescription', parent_id => 'openapi_types.UUID');

open(my $out, '>', $path) or die "write $path: $!\n";
print $out $spec;
close($out);
