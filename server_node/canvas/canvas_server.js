var Canvas = require('canvas');
var Data = require('./data');

var canvas;

var height = 0;
var width = 0;
var tracks = [];

var img = function (from, to) {
    var scale = Scale()
	.domain([0, width])
	.range([from, to]);

    canvas = new Canvas (height, width);
    var ctx = canvas.getContext('2d');
    var y_offset = 0;
    for (var i=0; i<tracks.length; i++) {
	var track = tracks[i];
	console.log("CREATING TRACK: " + track.name);
	var blocks = Data(10, 1);

	y_offset += track.height;
    }
};

img.render = function () {
    return canvas.toDataURL("image/png");
};

img.width = function (w) {
    if (!arguments.length) {
	return width;
    }
    width = w;
    return img;
};

img.height = function (h) {
    if (!arguments.length) {
	return height;
    }
    height = h;
    return img;
};

img.tracks = function (ts) {
    if (!arguments.length) {
	return tracks;
    }
    console.log("SETTING TRACKS:");
    console.log(tracks);
    tracks = ts;
    return img;
};

if (module && module.exports) {
    module.exports = img;
}

// Scale
var scale = function () {
    var range = [];
    var domain = [];

    var s = function (pos) {
	
    };

    s.domain = function (arref) {
	if (!arguments.length) {
	    return domain;
	}
	domain = arref;
	return s;
    };

    s.range = function (arref) {
	if (!arguments.length) {
	    return range;
	}
	range = arref;
	return s;
    };

    return s;
};
