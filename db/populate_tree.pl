#!/usr/bin/perl

use strict;
use warnings;
use Data::Dumper;

use MongoDB;
use MongoDB::BSON;

use Bio::EnsEMBL::Registry;
use Bio::EnsEMBL::Compara::Utils::GeneTreeHash;

# MongoDB connection
$MongoDB::BSON::looks_like_number = 1;
my $mongo = MongoDB::MongoClient->new(host => 'localhost', port => 27017);
my $db = $mongo->get_database('genetrees');
my $col = $db->get_collection('genetree');
# Indexes
$col->ensure_index({id => 1}, {unique => 1});
#$col->ensure_index({});

# GeneTrees
my $reg = 'Bio::EnsEMBL::Registry';

$reg->load_registry_from_db(
    -host    => 'ensembldb.ensembl.org',
    -user    => 'anonymous'
    );

my ($gene_tree_stable_id) = @ARGV;
$gene_tree_stable_id ||= 'ENSGT00440000034289';
my $gtAdaptor = $reg->get_adaptor('Multi', 'Compara', 'GeneTree');
my $geneTree = $gtAdaptor->fetch_by_stable_id($gene_tree_stable_id);
my $tree_hash = Bio::EnsEMBL::Compara::Utils::GeneTreeHash->convert($geneTree, -EXON_BOUNDARIES=>1, GAPS=>1);
visit($tree_hash->{tree});


# mongoDB node format:
# {
#     id : <string>,
#     seq : <string>,
#     exon_boundaires : [<ints>],
#     gaps : [
# 	{
# 	    type : <enum:high|low>,
# 	    start : <int>,
# 	    end : <int>
# 	}]
# }
sub visit {
    my ($node) = @_;
    my $nodeDbData = {
	"id" => $node->{id}{accession},
	"seq" => $node->{sequence}{mol_seq}{seq} || '',
	"exon_boundaries" => $node->{exon_boundaries}->{positions} || [],
	"gaps" => $node->{no_gaps} || []
    };
    $col->insert($nodeDbData);
    my $children = $node->{children};
    if (defined $children) {
	for (my $i=0; $i<scalar @$children; $i++) {
	    visit($children->[$i]);
	}
    }
}
