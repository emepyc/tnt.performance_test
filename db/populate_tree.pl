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
my $annot_col = $db->get_collection('annot');
my $tree_col = $db->get_collection('tree');
# Indexes on annot
$annot_col->ensure_index({id => 1}, {unique => 1});
$annot_col->ensure_index({length => 1});
$annot_col->ensure_index({gaps => 1});
$annot_col->ensure_index({exon_boundaries => 1});
# Indexes on tree
$tree_col->ensure_index({id => 1}, {unique => 1});

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
my $treeHash = Bio::EnsEMBL::Compara::Utils::GeneTreeHash->convert($geneTree, -EXON_BOUNDARIES=>1, GAPS=>1, ALIGNED=>1);

# Store the tree in tree collection in newick format
my $newick = $geneTree->newick_format("ryo", "%{-i}%{o-}:%{d}");

my $treeData = {
		id => $treeHash->{id},
		newick => $newick
	       };

$tree_col->insert($treeData);
visit($treeHash->{tree}, $treeHash->{id});


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
    my ($node, $genetree_stable_id) = @_;
    my $nodeDbData = {
		      "genetree" => $genetree_stable_id,
		      "id" => $node->{id}{accession},
		      "seq" => $node->{sequence}{mol_seq}{seq} || '',
		      "exon_boundaries" => $node->{exon_boundaries}->{positions} || [],
		      "gaps" => $node->{no_gaps} || [],
		      "length" => length($node->{sequence}{mol_seq}{seq} || '')
    };
    $annot_col->insert($nodeDbData);
    my $children = $node->{children};
    if (defined $children) {
	for (my $i=0; $i<scalar @$children; $i++) {
	    visit($children->[$i], $genetree_stable_id);
	}
    }
}
