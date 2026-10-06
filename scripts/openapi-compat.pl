#!/usr/bin/env perl
# Rewrites the bundled core-domain OpenAPI file in place so oapi-codegen 2.4
# can generate from it. Only the local, gitignored bundle is touched: the spec
# in motifpath-specs keeps authoring in OpenAPI 3.1 style.
#
# Usage: perl scripts/openapi-compat.pl .bundled/core-domain-service.yaml
use strict;
use warnings;

local $/;
my $path = shift or die "usage: $0 <bundled-spec.yaml>\n";
open(my $in, '<', $path) or die "read $path: $!\n";
my $spec = <$in>;
close($in);

# 3.1's "type: [<type>, null]" nullable shorthand, which redocly's bundler
# re-serializes as a two-item YAML list, becomes 3.0's "nullable: true".
$spec =~ s/^([ \t]*)type:\n[ \t]*- (string|integer|number|boolean|array|object)\n[ \t]*- .null.\n/$1type: $2\n$1nullable: true\n/mg;

# "oneOf: [$ref, {type: null}]" (a nullable reference) becomes a single-item
# allOf marked nullable, which generates a pointer to the referenced type.
$spec =~ s/^([ \t]*)oneOf:\n[ \t]*- (\$ref: '[^']+')\n[ \t]*- type: .null.\n/$1allOf:\n$1  - $2\n$1nullable: true\n/mg;

# UpdateDiagramRequest's mode and default_playback_id have three states:
# omitted leaves the value unchanged, null clears it, a value replaces it. A
# plain pointer can't tell omitted from null, so these two fields decode into
# nullable.Nullable. x-go-type is ignored on an allOf, so mode is declared as
# a string here; the domain still validates it against the mode enum.
$spec =~ s{(^    UpdateDiagramRequest:\n(?:(?!    \S).*\n)*)}{
    my $block = $1;
    $block =~ s/^(\s+)allOf:\n\s+- \$ref: '#\/components\/schemas\/DiagramMode'\n/$1type: string\n/m;
    my %go_type = (mode => 'DiagramMode', default_playback_id => 'openapi_types.UUID');
    $block =~ s/^(\s+)(mode|default_playback_id):\n/$1$2:\n$1  x-go-type: nullable.Nullable[$go_type{$2}]\n$1  x-go-type-import:\n$1    path: github.com\/oapi-codegen\/nullable\n$1  x-go-type-skip-optional-pointer: true\n/mg;
    $block;
}me;

# UpdateKnowledgeNodeRequest's descriptions and parent_id have the same
# three states, so they decode into nullable.Nullable the same way.
$spec =~ s{(^    UpdateKnowledgeNodeRequest:\n(?:(?!    \S).*\n)*)}{
    my $block = $1;
    $block =~ s/^(\s+)allOf:\n\s+- \$ref: '#\/components\/schemas\/LocalizedDescription'\n/$1type: object\n/m;
    my %go_type = (descriptions => 'LocalizedDescription', parent_id => 'openapi_types.UUID');
    $block =~ s/^(\s+)(descriptions|parent_id):\n/$1$2:\n$1  x-go-type: nullable.Nullable[$go_type{$2}]\n$1  x-go-type-import:\n$1    path: github.com\/oapi-codegen\/nullable\n$1  x-go-type-skip-optional-pointer: true\n/mg;
    $block;
}me;

open(my $out, '>', $path) or die "write $path: $!\n";
print $out $spec;
close($out);
