var videoCodecs = {
	"vp8": {
		"pt":      100,
		"name":    "VP8/90000",
		"rtcp-fb": ["ccm fir", "nack", "nack pli", "goog-remb", "transport-cc"],
	},

	"vp9": {
		"pt":      101,
		"name":    "VP9/90000",
		"rtcp-fb": ["ccm fir", "nack", "nack pli", "goog-remb", "transport-cc"]
	},

	"h264": {
		"pt":      107,
		"name":    "H264/90000",
		"rtcp-fb": ["ccm fir", "nack", "nack pli", "goog-remb", "transport-cc"],
		"fmtp":    "level-asymmetry-allowed=1;packetization-mode=1",
		// "framesize": "480-360"
	},

	"red": {
		"pt":   116,
		"name": "red/90000"
	},

	"ulpfec": {
		"pt":   117,
		"name": "ulpfec/90000"
	},

	"rtx": {
		"pt": 96,
		"name": "rtx/90000",
		"fmtp": "apt=100"
	}
};

var audioCodecs = {
	"opus": {
		"pt":      111,
		"name":    "opus/48000/2",
		"rtcp-fb": ["transport-cc"],
		"fmtp":    "minptime=10;useinbandfec=1"
	},

	"g722": {
		"pt":   9,
		"name": "G722/8000"
	},

	"pcmu": {
		"pt":   0,
		"name": "PCMU/8000"
	},

	"pcma": {
		"pt":   8,
		"name": "PCMA/8000"
	},

	"isac16": {
		"pt": 103,
		"name": "ISAC/16000"
	},

	"isac32": {
		"pt": 104,
		"name": "ISAC/32000"
	},

	"cn32": {
		"pt": 106,
		"name": "CN/32000"
	},

	"cn16": {
		"pt": 105,
		"name": "CN/16000"
	},

	"cn8": {
		"pt": 13,
		"name": "CN/8000"
	},

};


function replaceCodecs(sdp, audioCodec, videoCodec) {
	var res = "";
	var mode = "";
	sdp.split("\n").forEach(function(str){
		if (str.startsWith("m=audio ")) {
			res += str.replace(/^(m=audio \d+ [A-Z\/]+).+/, '$1 '+audioCodecs[audioCodec].pt)+"\n";
			mode = "audio";
			return;
		}

		if (str.startsWith("m=video ")) {
			res += str.replace(/^(m=video \d+ [A-Z\/]+).+/, '$1 '+videoCodecs[videoCodec].pt)+"\n";
			mode = "video";
			return;
		}

		if (str.startsWith("a=rtpmap:") || str.startsWith("a=rtcp-fb:") || str.startsWith("a=fmtp:")) {
			if (mode=="audio") {
				mode = "";
				res += addCodec(audioCodecs[audioCodec])
			} else if (mode=="video") {
				mode = "";
				res += addCodec(videoCodecs[videoCodec])
			}

			return
		}

		if (str) res += str+"\n";
	})

	return res
}

function addCodec(codec) {
	var res = "a=rtpmap:" + codec.pt + " " + codec.name + "\n";
	if (codec["rtcp-fb"]) {
		codec["rtcp-fb"].forEach(function(rtcp) {
			res += "a=rtcp-fb:"+codec.pt+" "+rtcp+"\n";
		})
	}

	if (codec["fmtp"]) res += "a=fmtp:"+codec.pt+" "+codec["fmtp"]+"\n";
	if (codec["framesize"]) res += "a=framesize:"+codec.pt+" "+codec["framesize"]+"\n";

	return res
}