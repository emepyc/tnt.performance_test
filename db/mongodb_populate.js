if (process.argv.length < 3) {
    console.log("USAGE: " + process.argv[0] + " " + process.argv[1] + "tracks elements span sep");
    process.exit();
}

var n_tracks = process.argv[2] || 1;
var n_elements = process.argv[3] || 10;
var span = process.argv[4] || 1;
var sep  = process.argv[5] || 10;

var blocks = [];
console.log("Generating BLOCKS (" + n_tracks + " tracks, " + n_elements + " elements, " + span + " span, " + sep, "sep)");
// for all the tracks
for (var i=0; i<n_tracks; i++) {
    var name = "track_" + i;
    // for all the elements
    for (var j=0; j<n_elements; j++) {
	blocks.push ( {"name"  : name,
		       "start" : (j+1) * sep,
		       "end"   : parseInt(((j+1) * sep)) + parseInt(span)
		      });
    }
}
console.log("DONE");

var url= "mongodb://localhost:27017/mytestdb";
var mongo = require('mongodb').MongoClient;
mongo.connect(url, function (err, db) {
    var collection = db.collection("testData");

    // remove collection
    collection.remove(function (err) { 
	collection.insert(blocks, function (err, result) {
	    if (err) {
		console.log("NOT OK!");
	    } else {
		console.log("OK");
	    }
	    db.close();
	})
    });
});


